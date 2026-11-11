package cli

import (
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// renderCategoryRemoval reports how many transactions were reassigned by a category rm or merge.
func renderCategoryRemoval(deps *Deps, rt runtimeFlags, res app.RemoveCategoryResult) error {
	if rt.format == render.JSON {
		return render.WriteJSON(deps.Stdout, struct {
			Reassigned int `json:"reassigned"`
		}{res.Reassigned})
	}
	if _, err := fmt.Fprintf(deps.Stdout, "Reassigned %d transaction(s).\n", res.Reassigned); err != nil {
		return fmt.Errorf("write reassigned count: %w", err)
	}
	return nil
}
