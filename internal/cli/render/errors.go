package render

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/0xballo/fui/internal/domain"
)

// errorEnvelope is the --json shape of a failed command: {"error":{"code","message","details"}}.
type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// WriteError prints err to w: as the JSON envelope when asJSON, else as a plain "fui: message"
// line. A *domain.Error contributes its stable code and details; any other error gets the
// generic code "error".
func WriteError(w io.Writer, asJSON bool, err error) error {
	if !asJSON {
		if _, werr := fmt.Fprintf(w, "fui: %s\n", err); werr != nil {
			return fmt.Errorf("write error: %w", werr)
		}
		return nil
	}
	var derr *domain.Error
	body := errorBody{Code: "error", Message: err.Error()}
	if errors.As(err, &derr) {
		body.Code, body.Message, body.Details = string(derr.Code), derr.Message, derr.Details
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if werr := enc.Encode(errorEnvelope{Error: body}); werr != nil {
		return fmt.Errorf("write json error: %w", werr)
	}
	return nil
}
