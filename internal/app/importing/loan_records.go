package importing

import (
	"fmt"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// InstallmentFromRecord converts one fui/1 installment record (minor-unit amounts) into a
// domain.Installment in cur. Negative amounts are rejected.
func InstallmentFromRecord(in fuiproto.InstallmentRecord, cur domain.Currency) (domain.Installment, error) {
	due, err := domain.ParseDate(in.DueDate)
	if err != nil {
		return domain.Installment{}, fmt.Errorf("due_date: %w", err)
	}
	out := domain.Installment{Due: due}
	for _, f := range []struct {
		name string
		v    int64
		dst  *domain.Money
	}{
		{"principal", in.Principal, &out.Principal},
		{"interest", in.Interest, &out.Interest},
		{"fees", in.Fees, &out.Fees},
		{"remaining", in.Remaining, &out.Remaining},
	} {
		if f.v < 0 {
			return domain.Installment{}, fmt.Errorf("%s must not be negative", f.name)
		}
		if *f.dst, err = domain.NewMoney(f.v, cur); err != nil {
			return domain.Installment{}, fmt.Errorf("%s: %w", f.name, err)
		}
	}
	return out, nil
}
