package importing

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// ListConnectors returns the built-in and the discovered external connectors.
func (s *Service) ListConnectors(ctx context.Context) ([]ConnectorInfo, error) {
	if s.connectors == nil {
		return nil, nil
	}
	out, err := s.connectors.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list connectors: %w", err)
	}
	return out, nil
}

// ConnectorDetail is a connector with the metadata it reports.
type ConnectorDetail struct {
	Info     ConnectorInfo
	Describe fuiproto.DescribeResponse
}

// ShowConnector returns the metadata of the named connector.
func (s *Service) ShowConnector(ctx context.Context, name string) (ConnectorDetail, error) {
	if s.connectors == nil {
		return ConnectorDetail{}, newError(domain.ErrNotFound, map[string]any{"query": name}, "no connector named %q", name)
	}
	all, err := s.connectors.List(ctx)
	if err != nil {
		return ConnectorDetail{}, fmt.Errorf("list connectors: %w", err)
	}
	for _, c := range all {
		if c.Name != name {
			continue
		}
		d, err := s.connectors.Describe(ctx, name)
		if err != nil {
			return ConnectorDetail{}, fmt.Errorf("describe connector %q: %w", name, err)
		}
		return ConnectorDetail{Info: c, Describe: d}, nil
	}
	return ConnectorDetail{}, newError(domain.ErrNotFound, map[string]any{"query": name}, "no connector named %q", name)
}

// TestReport is the outcome of fetching and validating a connector stream without importing it.
type TestReport struct {
	Connector    string
	Source       string
	SourceSHA256 string
	Accounts     []fuiproto.AccountRecord
	Transactions int
	Balances     int
	Errors       []RowError
	Stderr       string
}

// TestConnector runs fetch on the named connector and validates the stream. It never writes
// and never reads the archive; a broken stream is returned as the error, with the report
// carrying what the connector wrote on stderr.
func (s *Service) TestConnector(ctx context.Context, name string, fetch fuiproto.FetchRequest) (TestReport, error) {
	src, err := s.open(name, fetch, nil)
	if err != nil {
		return TestReport{}, err
	}
	st, err := readStream(ctx, src)
	rep := TestReport{Stderr: st.stderr}
	if err != nil {
		return rep, err
	}
	rep.Connector, rep.Source, rep.SourceSHA256 = st.meta.Connector, st.meta.Source.File, st.meta.Source.SHA256
	for _, rec := range st.result.Records {
		switch {
		case rec.Account != nil:
			rep.Accounts = append(rep.Accounts, *rec.Account)
		case rec.Transaction != nil:
			rep.Transactions++
		case rec.Balance != nil:
			rep.Balances++
		case rec.Error != nil:
			rep.Errors = append(rep.Errors, RowError{Line: rec.Error.Line, Message: "source: " + rec.Error.Message})
		}
	}
	for _, e := range st.result.RowErrors {
		rep.Errors = append(rep.Errors, RowError{Line: e.Line, Message: fmt.Sprintf("%s: %s", e.Type, e.Message)})
	}
	return rep, nil
}
