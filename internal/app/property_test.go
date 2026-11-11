package app_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// The balance at a date does not depend on the order movements were inserted in.
func TestBalanceIgnoresInsertionOrder(t *testing.T) {
	type move struct {
		Minor int64
		Day   int
	}
	rapid.Check(t, func(rt *rapid.T) {
		moves := rapid.SliceOfN(rapid.Custom(func(rt *rapid.T) move {
			return move{rapid.Int64Range(-1_000_000, 1_000_000).Draw(rt, "minor"), rapid.IntRange(1, 28).Draw(rt, "day")}
		}), 0, 20).Draw(rt, "moves")
		cutoff := rapid.IntRange(1, 28).Draw(rt, "cutoff")
		rot := rapid.IntRange(0, 19).Draw(rt, "rotation")

		balance := func(order []move) int64 {
			e := newEnv(rt)
			e.account(rt, "A", "cash", "JPY", "0") // JPY has no decimals: amounts are minor units
			for _, m := range order {
				e.tx(rt, app.AddTxRequest{Account: "A", Amount: fmt.Sprint(m.Minor), Date: fmt.Sprintf("2026-10-%02d", m.Day)})
			}
			ab, err := e.svc.ShowAccount(context.Background(), "A", fmt.Sprintf("2026-10-%02d", cutoff))
			if err != nil {
				rt.Fatalf("ShowAccount: %v", err)
			}
			return ab.Balance.Minor()
		}
		reordered := slices.Clone(moves)
		slices.Reverse(reordered)
		if n := len(reordered); n > 0 {
			k := rot % n
			reordered = append(reordered[k:], reordered[:k]...)
		}
		var want int64
		for _, m := range moves {
			if m.Day <= cutoff {
				want += m.Minor
			}
		}
		if got, other := balance(moves), balance(reordered); got != want || other != want {
			rt.Fatalf("balance = %d (in order) / %d (reordered), want %d", got, other, want)
		}
	})
}

// A transfer without fee moves money between accounts without changing the net worth.
func TestTransferKeepsNetWorth(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		e := newEnv(rt)
		e.account(rt, "A", "checking", "EUR", fmt.Sprintf("%d.00", rapid.IntRange(0, 100000).Draw(rt, "openA")))
		e.account(rt, "B", "savings", "EUR", fmt.Sprintf("%d.00", rapid.IntRange(0, 100000).Draw(rt, "openB")))
		cents := rapid.IntRange(1, 10_000_000).Draw(rt, "cents")
		ctx := context.Background()

		before, err := e.svc.NetWorth(ctx, app.NetWorthRequest{})
		if err != nil {
			rt.Fatalf("NetWorth: %v", err)
		}
		p, err := e.svc.PlanTransfer(ctx, app.TransferRequest{From: "A", To: "B", Amount: fmt.Sprintf("%d.%02d", cents/100, cents%100)})
		if err != nil {
			rt.Fatalf("PlanTransfer: %v", err)
		}
		if _, err := e.svc.ApplyTransfer(ctx, p); err != nil {
			rt.Fatalf("ApplyTransfer: %v", err)
		}
		after, err := e.svc.NetWorth(ctx, app.NetWorthRequest{})
		if err != nil {
			rt.Fatalf("NetWorth: %v", err)
		}
		if before.Total != after.Total {
			rt.Fatalf("net worth %v → %v after a transfer without fee", before.Total, after.Total)
		}
	})
}

// A plan never writes, whether the request is valid or not.
func TestPlanNeverWrites(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		e := newEnv(rt)
		e.account(rt, "Fineco", "checking", "EUR", "100")
		e.account(rt, "Dollars", "checking", "USD", "0")
		e.category(rt, "Food", "expense")
		tx := e.tx(rt, app.AddTxRequest{Account: "Fineco", Amount: "-5", Category: "Food"})
		ctx := context.Background()
		before := e.store.Snapshot()

		text := rapid.OneOf(rapid.String(), rapid.SampledFrom([]string{"Fineco", "fin", "Dollars", "Food", "-12.50", "10", "2026-10-01", "0.9", "USD", "x"}))
		s := func(name string) string { return text.Draw(rt, name) }
		sp := func(name string) *string { v := s(name); return &v }
		plans := map[string]func() error{
			"init": func() error { _, err := e.svc.PlanInit(ctx); return err },
			"add acct": func() error {
				_, err := e.svc.PlanAddAccount(ctx, app.AddAccountRequest{Name: s("n"), Kind: s("k"), Currency: s("c"), OpeningBalance: s("o")})
				return err
			},
			"edit acct": func() error {
				_, err := e.svc.PlanEditAccount(ctx, app.EditAccountRequest{Account: s("a"), Name: sp("n")})
				return err
			},
			"archive": func() error { _, err := e.svc.PlanArchiveAccount(ctx, s("a")); return err },
			"add tx": func() error {
				_, err := e.svc.PlanAddTx(ctx, app.AddTxRequest{Account: s("a"), Amount: s("m"), Date: s("d"), Category: s("c")})
				return err
			},
			"edit tx": func() error {
				_, err := e.svc.PlanEditTx(ctx, app.EditTxRequest{ID: tx.ID, Amount: sp("m"), Category: sp("c")})
				return err
			},
			"remove tx": func() error { _, err := e.svc.PlanRemoveTx(ctx, app.RemoveTxRequest{ID: tx.ID}); return err },
			"categorize tx": func() error {
				_, err := e.svc.PlanCategorizeTx(ctx, app.CategorizeTxRequest{IDs: []domain.TransactionID{tx.ID}, Category: s("c")})
				return err
			},
			"transfer": func() error {
				_, err := e.svc.PlanTransfer(ctx, app.TransferRequest{From: s("f"), To: s("t"), Amount: s("m"), Fee: s("fee")})
				return err
			},
			"add cat": func() error { _, err := e.svc.PlanAddCategory(ctx, app.AddCategoryRequest{Name: s("n")}); return err },
			"rename cat": func() error {
				_, err := e.svc.PlanRenameCategory(ctx, app.RenameCategoryRequest{Category: s("c"), NewName: s("n")})
				return err
			},
			"merge cat": func() error {
				_, err := e.svc.PlanMergeCategory(ctx, app.MergeCategoryRequest{Source: s("c"), Target: s("t")})
				return err
			},
			"remove cat": func() error {
				_, err := e.svc.PlanRemoveCategory(ctx, app.RemoveCategoryRequest{Category: s("c")})
				return err
			},
			"set rate": func() error {
				_, err := e.svc.PlanSetFXRate(ctx, app.SetFXRateRequest{Currency: s("c"), Date: s("d"), Rate: s("r")})
				return err
			},
		}
		for name, plan := range plans {
			_ = plan() // errors are fine: only the absence of writes matters
			if after := e.store.Snapshot(); after != before {
				rt.Fatalf("PlanX %q wrote to the store", name)
			}
		}
		if len(e.backup.reasons) != 0 {
			rt.Fatalf("planning took a backup: %v", e.backup.reasons)
		}
	})
}
