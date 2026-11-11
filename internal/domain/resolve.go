package domain

import (
	"strings"
)

// Resolve finds the single candidate that query refers to and returns its
// index. It tries, in order: exact case-insensitive match, unique prefix,
// unique substring. A step with several matches fails with ErrAmbiguousName
// (details "candidates" lists them) instead of falling through to a looser step.
func Resolve(query string, candidates []string) (int, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return 0, missingField("name")
	}
	lower := make([]string, len(candidates))
	for i, c := range candidates {
		lower[i] = strings.ToLower(c)
	}
	steps := []func(c string) bool{
		func(c string) bool { return c == q },
		func(c string) bool { return strings.HasPrefix(c, q) },
		func(c string) bool { return strings.Contains(c, q) },
	}
	for _, match := range steps {
		var hits []int
		for i, c := range lower {
			if match(c) {
				hits = append(hits, i)
			}
		}
		switch len(hits) {
		case 0:
			continue
		case 1:
			return hits[0], nil
		}
		names := make([]string, len(hits))
		for i, h := range hits {
			names[i] = candidates[h]
		}
		return 0, newError(ErrAmbiguousName, map[string]any{"query": query, "candidates": names},
			"%q is ambiguous: %s", query, strings.Join(names, ", "))
	}
	return 0, newError(ErrNotFound, map[string]any{"query": query}, "no match for %q", query)
}
