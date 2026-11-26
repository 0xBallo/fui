package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/0xballo/fui/internal/app"
)

var _ app.ArchiveDumper = (*Store)(nil)

// migrationTable is the goose bookkeeping table; its max version is exposed as SchemaVersion.
const migrationTable = "goose_db_version"

// dumpExcluded lists the tables DumpArchive skips because they are derived or bookkeeping:
// the FTS index and its shadow tables (rebuilt from transactions) and the migration table.
// Any other table is dumped, so a new table is covered automatically; a new derived table
// must be added here (TestDumpCoversEveryTable fails otherwise).
var dumpExcluded = map[string]bool{
	"transactions_fts":         true,
	"transactions_fts_data":    true,
	"transactions_fts_idx":     true,
	"transactions_fts_docsize": true,
	"transactions_fts_config":  true,
	migrationTable:             true,
}

// isDumpExcluded reports whether a table is skipped by DumpArchive.
func isDumpExcluded(name string) bool {
	return dumpExcluded[name] || strings.HasPrefix(name, "sqlite_")
}

// DumpArchive reads every user table in one read transaction, so the result is a consistent
// snapshot. Tables are ordered by name and rows by primary key; the archive is not modified.
func (s *Store) DumpArchive(ctx context.Context) (app.ArchiveDump, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tx, err := s.readDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return app.ArchiveDump{}, fmt.Errorf("dump archive: begin: %w", mapSQLError(err))
	}
	defer func() { _ = tx.Rollback() }()

	var dump app.ArchiveDump
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(version_id), 0) FROM "+migrationTable).Scan(&dump.SchemaVersion); err != nil {
		return app.ArchiveDump{}, fmt.Errorf("dump archive: schema version: %w", mapSQLError(err))
	}

	names, err := dumpTableNames(ctx, tx)
	if err != nil {
		return app.ArchiveDump{}, err
	}
	for _, name := range names {
		table, err := dumpTable(ctx, tx, name)
		if err != nil {
			return app.ArchiveDump{}, err
		}
		dump.Tables = append(dump.Tables, table)
	}
	return dump, nil
}

func dumpTableNames(ctx context.Context, tx *sql.Tx) ([]string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("dump archive: list tables: %w", mapSQLError(err))
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("dump archive: scan table name: %w", mapSQLError(err))
		}
		if !isDumpExcluded(name) {
			names = append(names, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dump archive: list tables: %w", mapSQLError(err))
	}
	return names, nil
}

func dumpTable(ctx context.Context, tx *sql.Tx, name string) (app.TableDump, error) {
	columns, orderBy, err := dumpColumns(ctx, tx, name)
	if err != nil {
		return app.TableDump{}, err
	}
	quoted := make([]string, len(columns))
	for i, c := range columns {
		quoted[i] = quoteIdent(c)
	}
	//nolint:gosec // identifiers come from sqlite_master and are quoted
	query := "SELECT " + strings.Join(quoted, ", ") + " FROM " + quoteIdent(name) + " ORDER BY " + orderBy

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return app.TableDump{}, fmt.Errorf("dump archive: read %s: %w", name, mapSQLError(err))
	}
	defer func() { _ = rows.Close() }()

	table := app.TableDump{Name: name, Columns: columns}
	for rows.Next() {
		cells := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return app.TableDump{}, fmt.Errorf("dump archive: scan %s: %w", name, mapSQLError(err))
		}
		for i, cell := range cells {
			switch cell.(type) {
			case nil, int64, string:
			default:
				return app.TableDump{}, fmt.Errorf("dump archive: %s.%s: unsupported cell type %T", name, columns[i], cell)
			}
		}
		table.Rows = append(table.Rows, cells)
	}
	if err := rows.Err(); err != nil {
		return app.TableDump{}, fmt.Errorf("dump archive: read %s: %w", name, mapSQLError(err))
	}
	return table, nil
}

// dumpColumns returns the columns in declared order and the ORDER BY clause: the primary key,
// or every column when the table has none.
func dumpColumns(ctx context.Context, tx *sql.Tx, name string) ([]string, string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT name, pk FROM pragma_table_info(?) ORDER BY cid", name)
	if err != nil {
		return nil, "", fmt.Errorf("dump archive: columns of %s: %w", name, mapSQLError(err))
	}
	defer func() { _ = rows.Close() }()

	var columns []string
	pkByPos := map[int]string{}
	for rows.Next() {
		var col string
		var pk int
		if err := rows.Scan(&col, &pk); err != nil {
			return nil, "", fmt.Errorf("dump archive: scan columns of %s: %w", name, mapSQLError(err))
		}
		columns = append(columns, col)
		if pk > 0 {
			pkByPos[pk] = col
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("dump archive: columns of %s: %w", name, mapSQLError(err))
	}

	orderCols := make([]string, 0, len(columns))
	for pos := 1; pos <= len(pkByPos); pos++ {
		orderCols = append(orderCols, quoteIdent(pkByPos[pos]))
	}
	if len(orderCols) == 0 {
		for _, c := range columns {
			orderCols = append(orderCols, quoteIdent(c))
		}
	}
	return columns, strings.Join(orderCols, ", "), nil
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
