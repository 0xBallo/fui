package memstore

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

var _ app.Resetter = (*Store)(nil)

// Reset implements app.Resetter: it replaces the content with an empty archive holding the
// default categories, or with KeepSettings the current categories. There is no backup file.
func (s *Store) Reset(ctx context.Context, opts app.ResetOptions) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("memstore: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fresh := newState()
	if opts.KeepSettings {
		fresh.Categories = s.st.Categories
		fresh.LastID["categories"] = s.st.LastID["categories"]
	} else {
		for _, c := range domain.DefaultCategories() {
			c.ID = domain.CategoryID(fresh.nextID("categories"))
			fresh.Categories[c.ID] = c
		}
	}
	s.st = fresh
	return nil
}
