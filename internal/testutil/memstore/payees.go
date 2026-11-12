package memstore

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

type payees struct{ *repos }

// check mirrors the unique name and unique alias constraints of the SQLite store.
func (p payees) check(payee domain.Payee) error {
	for _, o := range p.st.Payees {
		if o.ID != payee.ID && strings.EqualFold(o.Name, payee.Name) {
			return exists("payee", payee.Name)
		}
		if o.ID == payee.ID {
			continue
		}
		for _, a := range o.Aliases {
			if slices.ContainsFunc(payee.Aliases, func(x string) bool { return strings.EqualFold(x, a) }) {
				return exists("payee alias", a)
			}
		}
	}
	for i, a := range payee.Aliases {
		if slices.ContainsFunc(payee.Aliases[:i], func(x string) bool { return strings.EqualFold(x, a) }) {
			return exists("payee alias", a)
		}
	}
	return nil
}

func (p payees) Insert(_ context.Context, payee domain.Payee) (domain.Payee, error) {
	if err := p.writable(); err != nil {
		return domain.Payee{}, err
	}
	payee.ID = 0
	if err := p.check(payee); err != nil {
		return domain.Payee{}, err
	}
	payee.ID = domain.PayeeID(p.st.nextID("payees"))
	payee.Aliases = slices.Clone(payee.Aliases)
	p.st.Payees[payee.ID] = payee
	return payee, nil
}

func (p payees) Update(_ context.Context, payee domain.Payee) error {
	if err := p.writable(); err != nil {
		return err
	}
	if _, ok := p.st.Payees[payee.ID]; !ok {
		return notFound("payee", payee.ID)
	}
	if err := p.check(payee); err != nil {
		return err
	}
	payee.Aliases = slices.Clone(payee.Aliases)
	p.st.Payees[payee.ID] = payee
	return nil
}

func (p payees) Delete(_ context.Context, id domain.PayeeID) error {
	if err := p.writable(); err != nil {
		return err
	}
	if _, ok := p.st.Payees[id]; !ok {
		return notFound("payee", id)
	}
	for _, tx := range p.st.Txs {
		if tx.PayeeID == id {
			return fmt.Errorf("payee %d is used by transactions: %w", id, app.ErrInUse)
		}
	}
	delete(p.st.Payees, id)
	return nil
}

func (p payees) Get(_ context.Context, id domain.PayeeID) (domain.Payee, error) {
	payee, ok := p.st.Payees[id]
	if !ok {
		return domain.Payee{}, notFound("payee", id)
	}
	payee.Aliases = slices.Clone(payee.Aliases)
	return payee, nil
}

func (p payees) List(context.Context) ([]domain.Payee, error) {
	out := sortedValues(p.st.Payees)
	for i := range out {
		out[i].Aliases = slices.Clone(out[i].Aliases)
	}
	return out, nil
}

var _ app.PayeeRepo = payees{}
