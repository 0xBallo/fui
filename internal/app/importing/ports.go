package importing

import (
	"context"
	"iter"

	"github.com/0xballo/fui/pkg/fuiproto"
)

// Source yields the fui/1 records of one import. A decode or process failure is reported as the
// error of the pair, after which the sequence ends.
type Source interface {
	Records(ctx context.Context) iter.Seq2[fuiproto.Record, error]
}

// StderrSource is implemented by sources that capture the stderr of a connector process, shown
// to the user with --verbose.
type StderrSource interface {
	Stderr() string
}

// ConnectorKind says where a connector comes from.
type ConnectorKind string

// Connector kinds.
const (
	ConnectorBuiltin  ConnectorKind = "builtin"
	ConnectorExternal ConnectorKind = "external"
)

// ConnectorInfo identifies a discovered connector.
type ConnectorInfo struct {
	Name string
	Kind ConnectorKind
	Path string // executable of an external connector, empty for built-ins
}

// Connectors finds, describes and opens connectors.
type Connectors interface {
	// List returns the built-in and the discovered external connectors ordered by name.
	List(ctx context.Context) ([]ConnectorInfo, error)
	// Describe returns the metadata of the named connector.
	Describe(ctx context.Context, name string) (fuiproto.DescribeResponse, error)
	// Open returns the source that fetches req from the named connector.
	Open(name string, req fuiproto.FetchRequest) (Source, error)
}
