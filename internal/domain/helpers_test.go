package domain

// tb is the subset of tb shared by *testing.T and *rapid.T.
type tb interface {
	Helper()
	Fatalf(format string, args ...any)
}

func cur(t tb, code string) Currency {
	t.Helper()
	c, err := ParseCurrency(code)
	if err != nil {
		t.Fatalf("ParseCurrency(%q): %v", code, err)
	}
	return c
}

func money(t tb, minor int64, code string) Money {
	t.Helper()
	m, err := NewMoney(minor, cur(t, code))
	if err != nil {
		t.Fatalf("NewMoney: %v", err)
	}
	return m
}

// eurPrice returns a EUR price at 2 decimals.
func eurPrice(t tb, minor int64) Price {
	t.Helper()
	return priceOf(t, minor, 2, "EUR")
}

func priceOf(t tb, minor int64, scale int, code string) Price {
	t.Helper()
	p, err := NewPrice(minor, scale, cur(t, code))
	if err != nil {
		t.Fatalf("NewPrice: %v", err)
	}
	return p
}
