package memstore

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

type secPriceKey struct {
	sec  domain.SecurityID
	date domain.Date
}

type securities struct{ *repos }

// checkUnique mirrors the store's unique name, ticker and ISIN (names case-insensitively).
func (s securities) checkUnique(sec domain.Security) error {
	for _, o := range s.st.Securities {
		if o.ID == sec.ID {
			continue
		}
		if strings.EqualFold(o.Name, sec.Name) {
			return exists("security", sec.Name)
		}
		if sec.Ticker != "" && o.Ticker == sec.Ticker {
			return exists("security ticker", sec.Ticker)
		}
		if sec.ISIN != "" && o.ISIN == sec.ISIN {
			return exists("security isin", sec.ISIN)
		}
	}
	return nil
}

func (s securities) Insert(_ context.Context, sec domain.Security) (domain.Security, error) {
	if err := s.writable(); err != nil {
		return domain.Security{}, err
	}
	sec, err := domain.NewSecurity(sec)
	if err != nil {
		return domain.Security{}, err
	}
	if err := s.checkUnique(sec); err != nil {
		return domain.Security{}, err
	}
	sec.ID = domain.SecurityID(s.st.nextID("securities"))
	s.st.Securities[sec.ID] = sec
	return sec, nil
}

func (s securities) Update(_ context.Context, sec domain.Security) error {
	if err := s.writable(); err != nil {
		return err
	}
	if _, ok := s.st.Securities[sec.ID]; !ok {
		return notFound("security", sec.ID)
	}
	sec, err := domain.NewSecurity(sec)
	if err != nil {
		return err
	}
	if err := s.checkUnique(sec); err != nil {
		return err
	}
	s.st.Securities[sec.ID] = sec
	return nil
}

// Delete is restricted while prices or operations reference the security, like the FK.
func (s securities) Delete(_ context.Context, id domain.SecurityID) error {
	if err := s.writable(); err != nil {
		return err
	}
	if _, ok := s.st.Securities[id]; !ok {
		return notFound("security", id)
	}
	for _, op := range s.st.SecOps {
		if op.Security == id {
			return inUse("security", id)
		}
	}
	for k := range s.st.SecPrices {
		if k.sec == id {
			return inUse("security", id)
		}
	}
	delete(s.st.Securities, id)
	return nil
}

func (s securities) Get(_ context.Context, id domain.SecurityID) (domain.Security, error) {
	sec, ok := s.st.Securities[id]
	if !ok {
		return domain.Security{}, notFound("security", id)
	}
	return sec, nil
}

func (s securities) List(context.Context) ([]domain.Security, error) {
	return sortedValues(s.st.Securities), nil
}

func (s securities) UpsertPrice(_ context.Context, p domain.SecurityPrice) error {
	if err := s.writable(); err != nil {
		return err
	}
	if _, ok := s.st.Securities[p.Security]; !ok {
		return notFound("security", p.Security)
	}
	s.st.SecPrices[secPriceKey{p.Security, p.Date}] = p
	return nil
}

func (s securities) DeletePrice(_ context.Context, id domain.SecurityID, date domain.Date) error {
	if err := s.writable(); err != nil {
		return err
	}
	k := secPriceKey{id, date}
	if _, ok := s.st.SecPrices[k]; !ok {
		return notFound("security price", date)
	}
	delete(s.st.SecPrices, k)
	return nil
}

func (s securities) ListPrices(_ context.Context, id domain.SecurityID) ([]domain.SecurityPrice, error) {
	var out []domain.SecurityPrice
	for k, p := range s.st.SecPrices {
		if k.sec == id {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b domain.SecurityPrice) int { return a.Date.Cmp(b.Date) })
	return out, nil
}

func (s securities) PriceOnOrBefore(ctx context.Context, id domain.SecurityID, date domain.Date) (domain.SecurityPrice, error) {
	prices, _ := s.ListPrices(ctx, id)
	p, ok := domain.LastPriceOnOrBefore(prices, id, date)
	if !ok {
		return domain.SecurityPrice{}, notFound("security price", date)
	}
	return p, nil
}

func (s securities) checkOp(op domain.SecurityOp) error {
	if _, ok := s.st.Securities[op.Security]; !ok {
		return notFound("security", op.Security)
	}
	if _, ok := s.st.Accounts[op.Account]; !ok {
		return notFound("account", op.Account)
	}
	if op.CashTx != 0 {
		if _, ok := s.st.Txs[op.CashTx]; !ok {
			return notFound("transaction", op.CashTx)
		}
	}
	return nil
}

func (s securities) InsertOp(_ context.Context, op domain.SecurityOp) (domain.SecurityOp, error) {
	if err := s.writable(); err != nil {
		return domain.SecurityOp{}, err
	}
	op, err := domain.NewSecurityOp(op)
	if err != nil {
		return domain.SecurityOp{}, err
	}
	if err := s.checkOp(op); err != nil {
		return domain.SecurityOp{}, err
	}
	op.ID = domain.SecurityOpID(s.st.nextID("security_ops"))
	s.st.SecOps[op.ID] = op
	return op, nil
}

func (s securities) UpdateOp(_ context.Context, op domain.SecurityOp) error {
	if err := s.writable(); err != nil {
		return err
	}
	if _, ok := s.st.SecOps[op.ID]; !ok {
		return notFound("security operation", op.ID)
	}
	op, err := domain.NewSecurityOp(op)
	if err != nil {
		return err
	}
	if err := s.checkOp(op); err != nil {
		return err
	}
	s.st.SecOps[op.ID] = op
	return nil
}

func (s securities) DeleteOp(_ context.Context, id domain.SecurityOpID) error {
	if err := s.writable(); err != nil {
		return err
	}
	if _, ok := s.st.SecOps[id]; !ok {
		return notFound("security operation", id)
	}
	delete(s.st.SecOps, id)
	return nil
}

func (s securities) GetOp(_ context.Context, id domain.SecurityOpID) (domain.SecurityOp, error) {
	op, ok := s.st.SecOps[id]
	if !ok {
		return domain.SecurityOp{}, notFound("security operation", id)
	}
	return op, nil
}

func (s securities) ListOpsBySecurity(_ context.Context, id domain.SecurityID, from, to domain.Date) ([]domain.SecurityOp, error) {
	var out []domain.SecurityOp
	for _, op := range s.st.SecOps {
		if op.Security == id && (from.IsZero() || !op.Date.Before(from)) && (to.IsZero() || !op.Date.After(to)) {
			out = append(out, op)
		}
	}
	slices.SortFunc(out, func(a, b domain.SecurityOp) int {
		return cmp.Or(a.Date.Cmp(b.Date), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

func inUse(what string, id any) error {
	return fmt.Errorf("%s %v is referenced: %w", what, id, app.ErrInUse)
}
