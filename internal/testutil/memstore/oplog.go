package memstore

import (
	"context"
	"fmt"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

type opLog struct{ *repos }

func (o opLog) Append(_ context.Context, e app.OpEntry) (app.OpEntry, error) {
	if err := o.writable(); err != nil {
		return app.OpEntry{}, err
	}
	e.ID = o.st.nextID("op_log")
	e.UndoneAt = time.Time{}
	o.st.OpLog = append(o.st.OpLog, e)
	return e, nil
}

func (o opLog) Latest(_ context.Context) (app.OpEntry, error) {
	for i := len(o.st.OpLog) - 1; i >= 0; i-- {
		if o.st.OpLog[i].UndoneAt.IsZero() {
			return o.st.OpLog[i], nil
		}
	}
	return app.OpEntry{}, notFound("undoable operation", "")
}

func (o opLog) MarkUndone(_ context.Context, id int64, at time.Time) error {
	if err := o.writable(); err != nil {
		return err
	}
	for i, e := range o.st.OpLog {
		if e.ID == id && e.UndoneAt.IsZero() {
			o.st.OpLog[i].UndoneAt = at
			return nil
		}
	}
	return notFound("operation", id)
}

func (o opLog) RestoreTransaction(_ context.Context, t domain.Transaction) error {
	if err := o.writable(); err != nil {
		return err
	}
	if _, ok := o.st.Accounts[t.AccountID]; !ok {
		return notFound("account", t.AccountID)
	}
	if _, ok := o.st.Txs[t.ID]; ok {
		return exists("transaction", fmt.Sprint(t.ID))
	}
	o.st.Txs[t.ID] = t
	o.st.LastID["transactions"] = max(o.st.LastID["transactions"], int64(t.ID))
	return nil
}

func (o opLog) RestoreTransferGroup(_ context.Context, id domain.TransferGroupID, fee domain.Money) error {
	if err := o.writable(); err != nil {
		return err
	}
	if _, ok := o.st.Groups[id]; ok {
		return exists("transfer", fmt.Sprint(id))
	}
	o.st.Groups[id] = fee
	o.st.LastID["transfer_groups"] = max(o.st.LastID["transfer_groups"], int64(id))
	return nil
}
