package sqlite

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// idRemap maps the ids of the old archive to the ids the same settings got in the fresh one.
// Accounts are never kept, so they have no map and every account reference is dropped.
type idRemap struct {
	categories map[domain.CategoryID]domain.CategoryID
	payees     map[domain.PayeeID]domain.PayeeID
	tags       map[domain.TagID]domain.TagID
}

func newIDRemap() idRemap {
	return idRemap{
		categories: map[domain.CategoryID]domain.CategoryID{},
		payees:     map[domain.PayeeID]domain.PayeeID{},
		tags:       map[domain.TagID]domain.TagID{},
	}
}

// remapRule rewrites the ids a rule refers to. ok is false when nothing is left of its actions,
// so the rule would be invalid and must be skipped; warnings lists everything dropped.
func (m idRemap) remapRule(rule domain.Rule) (out domain.Rule, warnings []string, ok bool) {
	out = rule
	warn := func(format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf("rule %q: ", rule.Name)+fmt.Sprintf(format, args...))
	}

	out.Conditions.AccountIDs = nil
	for _, id := range rule.Conditions.AccountIDs {
		warn("dropped condition on account %d (accounts are not kept)", id)
	}

	out.Actions.CategoryID = 0
	if id := rule.Actions.CategoryID; id != 0 {
		if n, found := m.categories[id]; found {
			out.Actions.CategoryID = n
		} else {
			warn("dropped action on unknown category %d", id)
		}
	}
	out.Actions.PayeeID = 0
	if id := rule.Actions.PayeeID; id != 0 {
		if n, found := m.payees[id]; found {
			out.Actions.PayeeID = n
		} else {
			warn("dropped action on unknown payee %d", id)
		}
	}
	out.Actions.TagIDs = nil
	for _, id := range rule.Actions.TagIDs {
		if n, found := m.tags[id]; found {
			out.Actions.TagIDs = append(out.Actions.TagIDs, n)
		} else {
			warn("dropped action on unknown tag %d", id)
		}
	}

	ok = out.Actions.CategoryID != 0 || out.Actions.PayeeID != 0 || len(out.Actions.TagIDs) != 0
	if !ok {
		warn("skipped: no action left")
	}
	return out, warnings, ok
}

// pendingRef is a connector association kept by account name across a keep-settings reset.
type pendingRef struct {
	connector, ref, accountName string
}

// readPendingRefs lists the associations to carry over: the bound ones by account name plus the
// ones a previous reset left pending and no import consumed yet.
func (s *Store) readPendingRefs(ctx context.Context) ([]pendingRef, error) {
	rows, err := s.readDB.QueryContext(ctx, `
		SELECT r.connector, r.ref, a.name, 0 AS prio FROM connector_account_refs r JOIN accounts a ON a.id = r.account_id
		UNION ALL
		SELECT connector, ref, account_name, 1 FROM connector_ref_pending
		ORDER BY 1, 2, 4`)
	if err != nil {
		return nil, fmt.Errorf("reset: list connector refs: %w", mapSQLError(err))
	}
	defer func() { _ = rows.Close() }()
	var out []pendingRef
	seen := map[[2]string]bool{}
	for rows.Next() {
		var p pendingRef
		var prio int
		if err := rows.Scan(&p.connector, &p.ref, &p.accountName, &prio); err != nil {
			return nil, fmt.Errorf("reset: scan connector ref: %w", mapSQLError(err))
		}
		// A bound ref wins over a stale pending one with the same key (UNION keeps both rows).
		key := [2]string{p.connector, p.ref}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reset: list connector refs: %w", mapSQLError(err))
	}
	return out, nil
}

func (s *Store) insertPendingRefs(ctx context.Context, refs []pendingRef) error {
	for _, p := range refs {
		if _, err := s.writeDB.ExecContext(ctx,
			`INSERT INTO connector_ref_pending (connector, ref, account_name) VALUES (?, ?, ?)`,
			p.connector, p.ref, p.accountName); err != nil {
			return fmt.Errorf("reset: keep connector ref %q: %w", p.ref, mapSQLError(err))
		}
	}
	return nil
}
