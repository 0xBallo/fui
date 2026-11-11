package render

import (
	"fmt"
	"io"

	"github.com/0xballo/fui/internal/app"
)

// ChangeDTO is the --json shape of one planned change.
type ChangeDTO struct {
	Op      string         `json:"op"`
	Entity  string         `json:"entity"`
	Summary string         `json:"summary"`
	Fields  map[string]any `json:"fields,omitempty"`
}

// PlanDTO is the --json shape of a Plan, printed for --dry-run or before a confirmation prompt.
type PlanDTO struct {
	Changes []ChangeDTO `json:"changes"`
	Bulk    bool        `json:"bulk"`
}

func planDTO(p app.Plan) PlanDTO {
	out := PlanDTO{Bulk: p.Bulk, Changes: make([]ChangeDTO, len(p.Changes))}
	for i, c := range p.Changes {
		out.Changes[i] = ChangeDTO{Op: string(c.Op), Entity: c.Entity, Summary: c.Summary, Fields: c.Fields}
	}
	return out
}

func planRows(p app.Plan) Rows {
	t := Rows{Header: []string{"OP", "ENTITY", "SUMMARY"}}
	for _, c := range p.Changes {
		t.Body = append(t.Body, []string{string(c.Op), c.Entity, c.Summary})
	}
	return t
}

// WritePlan renders a Plan: every format lists each change; the table and CSV add a trailing
// note when the plan is bulk (it needs confirmation or --yes to apply).
func WritePlan(w io.Writer, f Format, p app.Plan) error {
	if f == JSON {
		return WriteJSON(w, planDTO(p))
	}
	var err error
	if f == CSV {
		err = WriteCSV(w, planRows(p))
	} else {
		err = WriteTable(w, planRows(p), false)
	}
	if err != nil {
		return err
	}
	if p.Bulk {
		if _, werr := fmt.Fprintln(w, "This is a bulk change: it needs --yes or a confirmation."); werr != nil {
			return fmt.Errorf("write plan note: %w", werr)
		}
	}
	return nil
}
