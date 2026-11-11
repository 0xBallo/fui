package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorIsMatchesByCode(t *testing.T) {
	sentinels := map[Code]*Error{
		CodeAmbiguousName: ErrAmbiguousName, CodeNotFound: ErrNotFound,
		CodeCurrencyMismatch: ErrCurrencyMismatch, CodeReconciled: ErrReconciled,
		CodeOverflow: ErrOverflow, CodeInvalidAmount: ErrInvalidAmount,
		CodeInvalidDate: ErrInvalidDate, CodeMissingField: ErrMissingField,
		CodeInvalidValue: ErrInvalidValue,
	}
	if len(sentinels) != len(Codes()) {
		t.Fatalf("Codes() has %d entries, sentinels %d", len(Codes()), len(sentinels))
	}
	for _, code := range Codes() {
		s, ok := sentinels[code]
		if !ok {
			t.Fatalf("no sentinel for code %q", code)
		}
		err := fmt.Errorf("wrapped: %w", newError(s, nil, "custom %s", code))
		if !errors.Is(err, s) {
			t.Errorf("errors.Is(%q) = false", code)
		}
		for other, o := range sentinels {
			if other != code && errors.Is(err, o) {
				t.Errorf("%q wrongly matches sentinel %q", code, other)
			}
		}
		var de *Error
		if !errors.As(err, &de) || de.Code != code || de.Error() != "custom "+string(code) {
			t.Errorf("errors.As = %v", de)
		}
	}
}

func TestErrorIsIgnoresForeignErrors(t *testing.T) {
	if errors.Is(errors.New("x"), ErrNotFound) {
		t.Fatal("foreign error matched sentinel")
	}
	if ErrNotFound.Is(errors.New("x")) {
		t.Fatal("Is matched foreign error")
	}
}
