package importing

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// Error codes of the import use cases. Like the domain and app codes, they are stable and the
// CLI maps them to exit codes.
const (
	// CodeUnmappedAccounts means the plan still has connector account refs bound to no account.
	CodeUnmappedAccounts domain.Code = "unmapped_accounts"
	// CodeInvalidStream means the connector output violates the fui/1 protocol or is truncated.
	CodeInvalidStream domain.Code = "invalid_stream"
	// CodeConnectorFailed means the connector could not run or exited with an error.
	CodeConnectorFailed domain.Code = "connector_failed"
)

// Codes returns every code of this package, for the CLI's exhaustive exit-code mapping test.
func Codes() []domain.Code {
	return []domain.Code{CodeUnmappedAccounts, CodeInvalidStream, CodeConnectorFailed}
}

// Sentinels for errors.Is; they match every error carrying the same code.
var (
	ErrUnmappedAccounts = &domain.Error{Code: CodeUnmappedAccounts, Message: "unmapped accounts"}
	ErrInvalidStream    = &domain.Error{Code: CodeInvalidStream, Message: "invalid stream"}
	ErrConnectorFailed  = &domain.Error{Code: CodeConnectorFailed, Message: "connector failed"}
)

func newError(sentinel *domain.Error, details map[string]any, format string, args ...any) *domain.Error {
	return &domain.Error{Code: sentinel.Code, Message: fmt.Sprintf(format, args...), Details: details}
}

// causedError carries a domain code and keeps the underlying cause in the errors.Is/As chain.
type causedError struct {
	coded *domain.Error
	cause error
}

func (e *causedError) Error() string   { return e.coded.Message }
func (e *causedError) Unwrap() []error { return []error{e.coded, e.cause} }

func caused(sentinel *domain.Error, cause error, format string, args ...any) error {
	return &causedError{coded: newError(sentinel, nil, format, args...), cause: cause}
}

// Service runs the import and export use cases over a Store.
type Service struct {
	store      app.Store
	clock      app.Clock
	backup     app.Backuper
	connectors Connectors
}

// New builds a Service. connectors may be nil when only file sources are planned.
func New(store app.Store, clock app.Clock, backup app.Backuper, connectors Connectors) *Service {
	return &Service{store: store, clock: clock, backup: backup, connectors: connectors}
}

// open returns the explicit source of a request, or opens the named connector.
func (s *Service) open(connector string, fetch fuiproto.FetchRequest, src Source) (Source, error) {
	if src != nil {
		return src, nil
	}
	if s.connectors == nil || connector == "" {
		return nil, newError(domain.ErrMissingField, map[string]any{"field": "connector"}, "no source and no connector given")
	}
	if fetch.Protocol == "" {
		fetch.Protocol = fuiproto.ProtocolFUI1
	}
	out, err := s.connectors.Open(connector, fetch)
	if err != nil {
		return nil, fmt.Errorf("open connector %q: %w", connector, err)
	}
	return out, nil
}

// stream is a validated fui/1 stream.
type stream struct {
	meta   fuiproto.MetaRecord
	result fuiproto.ValidationResult
	lines  map[*fuiproto.TransactionRecord]int
	sec    map[*fuiproto.SecurityOpRecord]int
	stderr string
}

// readStream reads and validates the source. A structural violation, a truncated output or a
// process failure aborts the import: nothing has been written at this point.
func readStream(ctx context.Context, src Source) (stream, error) {
	st := stream{
		lines: map[*fuiproto.TransactionRecord]int{},
		sec:   map[*fuiproto.SecurityOpRecord]int{},
	}
	var srcErr error
	line := 0
	tee := func(yield func(fuiproto.Record, error) bool) {
		for rec, err := range src.Records(ctx) {
			line++
			if err != nil {
				srcErr = err
			} else if rec.Transaction != nil {
				st.lines[rec.Transaction] = line
			} else if rec.SecurityOp != nil {
				st.sec[rec.SecurityOp] = line
			}
			if !yield(rec, err) {
				return
			}
		}
	}
	res, err := fuiproto.ValidateStream(tee)
	if s, ok := src.(StderrSource); ok {
		st.stderr = s.Stderr()
	}
	if srcErr != nil {
		return st, caused(ErrConnectorFailed, srcErr, "read source: %v", srcErr)
	}
	if err != nil {
		return st, caused(ErrInvalidStream, err, "%v", err)
	}
	st.result = res
	if len(res.Records) > 0 && res.Records[0].Meta != nil {
		st.meta = *res.Records[0].Meta
	}
	return st, nil
}
