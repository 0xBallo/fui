package importing_test

import (
	"testing"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/pkg/fuiproto"
)

func TestInstallmentFromRecord(t *testing.T) {
	cur := eur()
	tests := []struct {
		name    string
		in      fuiproto.InstallmentRecord
		wantErr bool
	}{
		{"ok", fuiproto.InstallmentRecord{DueDate: "2026-02-01", Principal: 100, Interest: 20, Fees: 1, Remaining: 900}, false},
		{"bad date", fuiproto.InstallmentRecord{DueDate: "nope"}, true},
		{"negative", fuiproto.InstallmentRecord{DueDate: "2026-02-01", Interest: -1}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := importing.InstallmentFromRecord(tc.in, cur)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if got.Due.String() != "2026-02-01" || got.Principal.Minor() != 100 || got.Interest.Minor() != 20 ||
				got.Fees.Minor() != 1 || got.Remaining.Minor() != 900 {
				t.Errorf("unexpected installment %+v", got)
			}
		})
	}
}
