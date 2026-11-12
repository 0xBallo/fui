package app_test

import (
	"fmt"
	"testing"

	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func splitEnv(t tb) (env, domain.Transaction) {
	t.Helper()
	e := newEnv(t)
	e.account(t, "A", "checking", "EUR", "0")
	e.category(t, "Groceries", "expense")
	e.category(t, "Household", "expense")
	tx := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-30", Category: "Groceries", Date: "2026-10-01"})
	return e, tx
}

func TestSplitTx(t *testing.T) {
	e, tx := splitEnv(t)
	ctx := t.Context()

	tests := []struct {
		name    string
		rows    []app.SplitRowRequest
		wantErr *domain.Error
	}{
		{name: "one row", rows: []app.SplitRowRequest{{Category: "Groceries", Amount: "30"}}, wantErr: domain.ErrInvalidValue},
		{
			name: "wrong sum",
			rows: []app.SplitRowRequest{
				{Category: "Groceries", Amount: "-20"},
				{Category: "Household", Amount: "-5"},
			},
			wantErr: domain.ErrInvalidValue,
		},
		{
			name: "unknown category",
			rows: []app.SplitRowRequest{
				{Category: "Groceries", Amount: "-20"},
				{Category: "zzz", Amount: "-10"},
			},
			wantErr: domain.ErrNotFound,
		},
		{
			name: "ok",
			rows: []app.SplitRowRequest{
				{Category: "Groceries", Amount: "-20", Note: "food"},
				{Category: "Household", Amount: "-10", Note: "cleaning"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := e.svc.PlanSplitTx(ctx, app.SplitTxRequest{ID: tx.ID, Rows: tt.rows})
			if tt.wantErr != nil {
				wantCode(t, err, tt.wantErr)
				return
			}
			wantNoErr(t, err)
			wantNoErr(t, e.svc.ApplySplitTx(ctx, p))
			rows, err := e.svc.ListSplits(ctx, tx.ID)
			wantNoErr(t, err)
			if len(rows) != 2 || rows[0].Note != "food" || rows[1].Note != "cleaning" {
				t.Errorf("ListSplits = %+v", rows)
			}
		})
	}
}

func TestUnsplitTx(t *testing.T) {
	e, tx := splitEnv(t)
	ctx := t.Context()
	p, err := e.svc.PlanSplitTx(ctx, app.SplitTxRequest{ID: tx.ID, Rows: []app.SplitRowRequest{
		{Category: "Groceries", Amount: "-20"},
		{Category: "Household", Amount: "-10"},
	}})
	wantNoErr(t, err)
	wantNoErr(t, e.svc.ApplySplitTx(ctx, p))

	up, err := e.svc.PlanUnsplitTx(ctx, app.UnsplitTxRequest{ID: tx.ID})
	wantNoErr(t, err)
	wantNoErr(t, e.svc.ApplyUnsplitTx(ctx, up))

	rows, err := e.svc.ListSplits(ctx, tx.ID)
	wantNoErr(t, err)
	if len(rows) != 0 {
		t.Errorf("ListSplits after unsplit = %+v, want none", rows)
	}

	_, err = e.svc.PlanSplitTx(ctx, app.SplitTxRequest{ID: 999, Rows: []app.SplitRowRequest{
		{Category: "Groceries", Amount: "-20"},
		{Category: "Household", Amount: "-10"},
	}})
	wantCode(t, err, domain.ErrNotFound)
}

// Splitting a transaction into any number of rows that respects the invariants always leaves the
// recorded rows summing to the original transaction amount, whatever the split is.
func TestSplitEditKeepsSum(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		e, tx := splitEnv(rt)
		ctx := rt.Context()
		total := tx.Amount.Minor() // -3000 minor units (EUR has 2 decimals)

		n := rapid.IntRange(2, 4).Draw(rt, "rows")
		cats := []string{"Groceries", "Household"}
		amounts := make([]int64, n)
		remaining := total
		for i := 0; i < n-1; i++ {
			// every row takes a non-zero slice of what remains, leaving at least 1 minor unit
			// per still-to-be-assigned row
			maxAbs := (-remaining) - int64(n-1-i)
			if maxAbs < 1 {
				maxAbs = 1
			}
			v := rapid.Int64Range(1, maxAbs).Draw(rt, "row")
			amounts[i] = -v
			remaining += v
		}
		amounts[n-1] = remaining

		rows := make([]app.SplitRowRequest, n)
		for i, a := range amounts {
			rows[i] = app.SplitRowRequest{Category: cats[i%len(cats)], Amount: minorToEUR(a)}
		}
		p, err := e.svc.PlanSplitTx(ctx, app.SplitTxRequest{ID: tx.ID, Rows: rows})
		if err != nil {
			rt.Fatalf("PlanSplitTx: %v", err)
		}
		if err := e.svc.ApplySplitTx(ctx, p); err != nil {
			rt.Fatalf("ApplySplitTx: %v", err)
		}
		got, err := e.svc.ListSplits(ctx, tx.ID)
		if err != nil {
			rt.Fatalf("ListSplits: %v", err)
		}
		var sum int64
		for _, r := range got {
			sum += r.Amount.Minor()
		}
		if sum != total {
			rt.Fatalf("split rows sum to %d, want %d (rows=%+v)", sum, total, got)
		}
	})
}

// minorToEUR renders minor EUR units (cents) as the decimal string PlanSplitTx expects.
func minorToEUR(minor int64) string {
	sign := ""
	if minor < 0 {
		sign = "-"
		minor = -minor
	}
	return fmt.Sprintf("%s%d.%02d", sign, minor/100, minor%100)
}
