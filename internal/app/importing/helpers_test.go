package importing_test

import (
	"context"
	"fmt"
	"iter"
	"regexp"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/testutil/memstore"
	"github.com/0xballo/fui/pkg/fuiproto"
)

type tb interface {
	Helper()
	Fatalf(format string, args ...any)
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type spyBackup struct{ reasons []string }

func (b *spyBackup) Backup(_ context.Context, reason string) (string, error) {
	b.reasons = append(b.reasons, reason)
	return "/backups/" + reason + ".db", nil
}

func (b *spyBackup) Restore(context.Context, string) error { return nil }

// sliceSource replays records; failAfter, when set, ends the stream with it as an error.
type sliceSource struct {
	recs      []fuiproto.Record
	failAfter error
	stderr    string
}

func (s sliceSource) Records(context.Context) iter.Seq2[fuiproto.Record, error] {
	return func(yield func(fuiproto.Record, error) bool) {
		for _, r := range s.recs {
			if !yield(r, nil) {
				return
			}
		}
		if s.failAfter != nil {
			yield(fuiproto.Record{}, s.failAfter)
		}
	}
}

func (s sliceSource) Stderr() string { return s.stderr }

// stream builds a complete stream: meta first, end last with the right counters.
func stream(connector, sha string, recs ...fuiproto.Record) sliceSource {
	out := []fuiproto.Record{{
		Type: fuiproto.RecordTypeMeta,
		Meta: &fuiproto.MetaRecord{
			Protocol: fuiproto.ProtocolFUI1, Connector: connector, Version: "1", Source: fuiproto.SourceMeta{File: connector + ".csv", SHA256: sha},
		},
	}}
	out = append(out, recs...)
	end := fuiproto.EndRecord{}
	for _, r := range recs {
		if r.Transaction != nil {
			end.Transactions++
		}
		if r.Error != nil {
			end.Errors++
		}
	}
	out = append(out, fuiproto.Record{Type: fuiproto.RecordTypeEnd, End: &end})
	return sliceSource{recs: out}
}

func account(ref, name, currency string, mods ...func(*fuiproto.AccountRecord)) fuiproto.Record {
	a := &fuiproto.AccountRecord{Ref: ref, Name: name, Currency: currency, Kind: fuiproto.AccountKindChecking}
	for _, m := range mods {
		m(a)
	}
	return fuiproto.Record{Type: fuiproto.RecordTypeAccount, Account: a}
}

func withIBAN(iban string) func(*fuiproto.AccountRecord) {
	return func(a *fuiproto.AccountRecord) { a.IBAN = iban }
}

func txRec(ref, id, date string, amount int64, desc string, mods ...func(*fuiproto.TransactionRecord)) fuiproto.Record {
	t := &fuiproto.TransactionRecord{
		Account: ref, ID: id, Date: date, Amount: amount, Currency: "EUR", Description: desc,
		Status: fuiproto.TransactionStatusBooked, Kind: fuiproto.TransactionKindPayment,
	}
	for _, m := range mods {
		m(t)
	}
	return fuiproto.Record{Type: fuiproto.RecordTypeTransaction, Transaction: t}
}

func kind(k fuiproto.TransactionKind) func(*fuiproto.TransactionRecord) {
	return func(t *fuiproto.TransactionRecord) { t.Kind = k }
}

func label(l string) func(*fuiproto.TransactionRecord) {
	return func(t *fuiproto.TransactionRecord) { t.Transfer = l }
}

func category(c string) func(*fuiproto.TransactionRecord) {
	return func(t *fuiproto.TransactionRecord) { t.Category = c }
}

func payeeName(p string) func(*fuiproto.TransactionRecord) {
	return func(t *fuiproto.TransactionRecord) { t.Payee = p }
}

func withTags(names ...string) func(*fuiproto.TransactionRecord) {
	return func(t *fuiproto.TransactionRecord) { t.Tags = names }
}

func withSplits(rows ...fuiproto.TransactionSplit) func(*fuiproto.TransactionRecord) {
	return func(t *fuiproto.TransactionRecord) { t.Splits = rows }
}

func counterparty(iban string) func(*fuiproto.TransactionRecord) {
	return func(t *fuiproto.TransactionRecord) { t.CounterpartyIBAN = iban }
}

func pending() func(*fuiproto.TransactionRecord) {
	return func(t *fuiproto.TransactionRecord) { t.Status = fuiproto.TransactionStatusPending }
}

func balanceAfter(n int64) func(*fuiproto.TransactionRecord) {
	return func(t *fuiproto.TransactionRecord) { t.BalanceAfter = &n }
}

type env struct {
	svc    *importing.Service
	store  *memstore.Store
	backup *spyBackup
}

func newEnv(connectors importing.Connectors) env {
	e := env{store: memstore.New(), backup: &spyBackup{}}
	e.svc = importing.New(e.store, fixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}, e.backup, connectors)
	return e
}

// must returns v, or panics: a failed setup step is a broken test, and a panic is reported
// as a failure by both testing and rapid.
func must[T any](v T, err error) T {
	if err != nil {
		panic(fmt.Sprintf("unexpected error: %v", err))
	}
	return v
}

func eur() domain.Currency { return must(domain.ParseCurrency("EUR")) }

func date(s string) domain.Date { return must(domain.ParseDate(s)) }

func money(minor int64) domain.Money { return must(domain.NewMoney(minor, eur())) }

// seedAccount inserts an EUR checking account and binds ref of connector to it.
func (e env) seedAccount(t tb, name, connector, ref string) domain.Account {
	t.Helper()
	var out domain.Account
	err := e.store.WithTx(context.Background(), func(r app.Repos) error {
		a, err := r.Accounts().Insert(context.Background(), domain.Account{
			Name: name, Kind: domain.AccountChecking, Currency: eur(), OpeningBalance: money(0), OpenedOn: date("2026-01-01"),
		})
		if err != nil {
			return err
		}
		out = a
		if ref == "" {
			return nil
		}
		return r.Imports().SetAccountRef(context.Background(), connector, ref, a.ID)
	})
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}
	return out
}

func (e env) seedTx(t tb, base domain.Transaction) domain.Transaction {
	t.Helper()
	var out domain.Transaction
	err := e.store.WithTx(context.Background(), func(r app.Repos) (err error) {
		out, err = r.Transactions().Insert(context.Background(), base)
		return err
	})
	if err != nil {
		t.Fatalf("seed transaction: %v", err)
	}
	return out
}

func (e env) txs(t tb) []domain.Transaction {
	t.Helper()
	var out []domain.Transaction
	err := e.store.Read(context.Background(), func(r app.Repos) (err error) {
		f, err := domain.NewTxFilter(domain.TxFilterSpec{Desc: true})
		if err != nil {
			return err
		}
		out, err = r.Transactions().List(context.Background(), f, nil, 0)
		return err
	})
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	return out
}

// plan plans and fails the test on error.
func (e env) plan(t tb, src importing.Source, mods ...func(*importing.PlanRequest)) importing.ImportPlan {
	t.Helper()
	req := importing.PlanRequest{Source: src}
	for _, m := range mods {
		m(&req)
	}
	return must(e.svc.PlanImport(context.Background(), req))
}

func createAccounts(r *importing.PlanRequest) { r.CreateAccounts = true }

// importAll plans with CreateAccounts and applies.
func (e env) importAll(t tb, src importing.Source) importing.ImportResult {
	t.Helper()
	return must(e.svc.ApplyImport(context.Background(), e.plan(t, src, createAccounts)))
}

var lastID = regexp.MustCompile(`LastID:map\[[^\]]*\]`)

// snapshot is the memstore dump without the ID counters, which an undo cannot rewind.
func snapshot(s *memstore.Store) string { return lastID.ReplaceAllString(s.Snapshot(), "") }
