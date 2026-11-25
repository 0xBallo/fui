package app

import "context"

// DumpSchemaVersion is the version of the full-dump document (v1 was the per-entity export).
const DumpSchemaVersion = 2

// TableDump is the content of one archive table: column names and rows, in primary-key order.
// A cell is nil (NULL), int64 or string: the schema is STRICT and holds no floats or blobs.
type TableDump struct {
	Name    string
	Columns []string
	Rows    [][]any
}

// ArchiveDump is the content of every archive table except the derived ones (FTS index,
// migration bookkeeping). SchemaVersion is the migration version the rows were read at.
type ArchiveDump struct {
	SchemaVersion int
	Tables        []TableDump
}

// ArchiveDumper reads the whole archive as an ArchiveDump in one consistent snapshot. The
// Backuper given to New may also implement it (the SQLite store does), like Resetter.
type ArchiveDumper interface {
	DumpArchive(ctx context.Context) (ArchiveDump, error)
}

// ArchiveLoader replaces the archive with the rows of an ArchiveDump, keeping ids: it builds a
// fresh archive (migrations), loads the rows with foreign keys checked, rebuilds the derived
// indexes, takes a "restore" backup of the current archive and atomically swaps the files
// (ADR 0011). Any failure leaves the current archive untouched. The Backuper given to New may
// also implement it.
type ArchiveLoader interface {
	LoadArchive(ctx context.Context, d ArchiveDump) error
}
