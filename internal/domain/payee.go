package domain

import "strings"

// Payee is a transaction counterparty. Aliases let rule matching and import
// recognise the same payee under different descriptions (e.g. the many
// variants a bank uses for the same merchant).
type Payee struct {
	ID      PayeeID
	Name    string
	Aliases []string
}

// NewPayee validates and returns a payee with the name and aliases trimmed.
// Empty aliases are dropped.
func NewPayee(name string, aliases ...string) (Payee, error) {
	n, err := requireName("name", name)
	if err != nil {
		return Payee{}, err
	}
	p := Payee{Name: n}
	for _, a := range aliases {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		p.Aliases = append(p.Aliases, a)
	}
	return p, nil
}

// NormalizePayee trims, case-folds and collapses internal whitespace, the
// same way NormalizeDescription does for transaction descriptions. It is the
// basis of case/whitespace-insensitive payee and alias matching.
func NormalizePayee(s string) string { return NormalizeDescription(s) }

// MatchesAlias reports whether s (typically a transaction description)
// matches the payee's name or one of its aliases, ignoring case and
// whitespace differences.
func (p Payee) MatchesAlias(s string) bool {
	target := NormalizePayee(s)
	if NormalizePayee(p.Name) == target {
		return true
	}
	for _, a := range p.Aliases {
		if NormalizePayee(a) == target {
			return true
		}
	}
	return false
}
