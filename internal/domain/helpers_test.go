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
