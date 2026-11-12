package importing

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// RuleEffect is what the enabled categorization rules set on an import row, with the name of the
// rule that won each field. Only unset fields are filled: what the source provided is kept.
type RuleEffect struct {
	CategoryRule string
	PayeeRule    string
	TagsRule     string
	TagIDs       []domain.TagID
}

// applyRules is step 4: after dedup, run the enabled rules over every row that is not a duplicate.
func (b *builder) applyRules(ctx context.Context) error {
	rules, err := b.r.Rules().ListEnabledByPriority(ctx)
	if err != nil {
		return fmt.Errorf("list rules: %w", err)
	}
	if len(rules) == 0 {
		return nil
	}
	byID := make(map[domain.CategoryID]domain.Category, len(b.cats))
	for _, c := range b.cats {
		byID[c.ID] = c
	}
	for i := range b.plan.Rows {
		row := &b.plan.Rows[i]
		if row.Class == domain.DedupDuplicate {
			continue
		}
		res, err := domain.Apply(rules, row.Tx, domain.NormalizeDescription(row.Tx.Description))
		if err != nil {
			return fmt.Errorf("rules on line %d: %w", row.Line, err)
		}
		if c, ok := byID[res.CategoryID]; ok && row.Category == "" {
			row.Category, row.Rules.CategoryRule = c.Name, res.CategoryRule
		}
		if res.PayeeID != 0 && row.Tx.PayeeID == 0 {
			row.Tx.PayeeID, row.Rules.PayeeRule = res.PayeeID, res.PayeeRule
		}
		if res.TagsRule != "" {
			row.Rules.TagIDs, row.Rules.TagsRule = res.TagIDs, res.TagsRule
		}
	}
	return nil
}
