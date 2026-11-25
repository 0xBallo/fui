package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/backup"
	"github.com/0xballo/fui/internal/domain"
)

var _ app.ArchiveLoader = (*Store)(nil)

// LoadArchive replaces the archive with the rows of d, keeping ids. The rows go into a fresh
// archive built next to the current one (migrations only, no default categories); the current
// archive is then backed up with reason "restore" and atomically replaced (ADR 0011). Any
// failure before the rename leaves the current archive untouched.
func (s *Store) LoadArchive(ctx context.Context, d app.ArchiveDump) error {
	if int64(d.SchemaVersion) > currentSchemaVersion {
		return fmt.Errorf("load archive: dump schema version %d is newer than %d: %w",
			d.SchemaVersion, currentSchemaVersion, domain.ErrInvalidValue)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.CreateTemp(filepath.Dir(s.path), ".fui-restore-*.db")
	if err != nil {
		return fmt.Errorf("load archive: create temp file: %w", err)
	}
	tempPath := f.Name()
	// Open must create the file itself, or it would back up an "old archive" before migrating.
	if err := errors.Join(f.Close(), os.Remove(tempPath)); err != nil {
		return fmt.Errorf("load archive: prepare temp file: %w", err)
	}
	defer func() { _ = os.Remove(tempPath) }() // no-op once renamed

	if err := s.populateFromDump(ctx, tempPath, d); err != nil {
		return err
	}
	return s.swapLocked(ctx, tempPath, backup.ReasonRestore, "load archive")
}

func (s *Store) populateFromDump(ctx context.Context, tempPath string, d app.ArchiveDump) (err error) {
	fresh, err := Open(ctx, Options{
		Path: tempPath, BackupDir: s.backupDir, BackupKeep: s.backupKeep, Clock: s.clock,
	})
	if err != nil {
		return fmt.Errorf("load archive: open fresh archive: %w", err)
	}
	defer func() {
		if cerr := fresh.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("load archive: close fresh archive: %w", cerr)
		}
	}()
	// Sidecar files of the temp archive must not outlive it.
	defer func() {
		_ = os.Remove(tempPath + "-wal")
		_ = os.Remove(tempPath + "-shm")
	}()

	schema, err := freshSchema(ctx, fresh.writeDB)
	if err != nil {
		return err
	}
	if err := validateDump(d, schema); err != nil {
		return err
	}

	// Foreign keys are off while loading (rows arrive in table-name order) and verified after.
	conn, err := fresh.writeDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("load archive: connection: %w", mapSQLError(err))
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("load archive: disable foreign keys: %w", mapSQLError(err))
	}
	if err := loadRows(ctx, conn, d); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("load archive: enable foreign keys: %w", mapSQLError(err))
	}
	return nil
}

// loadRows replaces the content of every dumped table in one transaction, checks the foreign
// keys and rebuilds the FTS index.
func loadRows(ctx context.Context, conn *sql.Conn, d app.ArchiveDump) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("load archive: begin: %w", mapSQLError(err))
	}
	defer func() { _ = tx.Rollback() }()

	for _, t := range d.Tables {
		//nolint:gosec // table names are validated against the schema and quoted
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+quoteIdent(t.Name)); err != nil {
			return fmt.Errorf("load archive: clear %s: %w", t.Name, mapSQLError(err))
		}
	}
	for _, t := range d.Tables {
		if err := insertTable(ctx, tx, t); err != nil {
			return err
		}
	}
	if err := checkForeignKeys(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO transactions_fts (transactions_fts) VALUES ('rebuild')"); err != nil {
		return fmt.Errorf("load archive: rebuild search index: %w", mapSQLError(err))
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("load archive: commit: %w", mapSQLError(err))
	}
	return nil
}

func insertTable(ctx context.Context, tx *sql.Tx, t app.TableDump) error {
	if len(t.Rows) == 0 {
		return nil
	}
	cols := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		cols[i] = quoteIdent(c)
	}
	//nolint:gosec // identifiers are validated against the schema and quoted
	query := "INSERT INTO " + quoteIdent(t.Name) + " (" + strings.Join(cols, ", ") + ") VALUES (" +
		strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ") + ")"
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("load archive: prepare %s: %w", t.Name, mapSQLError(err))
	}
	defer func() { _ = stmt.Close() }()
	for i, row := range t.Rows {
		if _, err := stmt.ExecContext(ctx, row...); err != nil {
			return fmt.Errorf("load archive: insert %s row %d: %w", t.Name, i+1, mapSQLError(err))
		}
	}
	return nil
}

func checkForeignKeys(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("load archive: foreign key check: %w", mapSQLError(err))
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		var table, parent string
		var rowid, fkid any
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			return fmt.Errorf("load archive: foreign key check: %w", mapSQLError(err))
		}
		return fmt.Errorf("load archive: %s row %v references a missing %s row: %w",
			table, rowid, parent, domain.ErrInvalidValue)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("load archive: foreign key check: %w", mapSQLError(err))
	}
	return nil
}

// freshSchema maps each loadable table of the fresh archive to its columns.
func freshSchema(ctx context.Context, db *sql.DB) (map[string][]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table'")
	if err != nil {
		return nil, fmt.Errorf("load archive: list tables: %w", mapSQLError(err))
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("load archive: scan table name: %w", mapSQLError(err))
		}
		if !isDumpExcluded(name) {
			names = append(names, name)
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("load archive: list tables: %w", mapSQLError(err))
	}

	schema := make(map[string][]string, len(names))
	for _, name := range names {
		cols, err := tableColumns(ctx, db, name)
		if err != nil {
			return nil, err
		}
		schema[name] = cols
	}
	return schema, nil
}

func tableColumns(ctx context.Context, db *sql.DB, name string) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT name FROM pragma_table_info(?) ORDER BY cid", name)
	if err != nil {
		return nil, fmt.Errorf("load archive: columns of %s: %w", name, mapSQLError(err))
	}
	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("load archive: scan columns of %s: %w", name, mapSQLError(err))
		}
		cols = append(cols, c)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("load archive: columns of %s: %w", name, mapSQLError(err))
	}
	return cols, nil
}

// validateDump rejects unknown or repeated tables, unknown or repeated columns, ragged rows and
// cell types the STRICT schema cannot hold.
func validateDump(d app.ArchiveDump, schema map[string][]string) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("load archive: "+format+": %w", append(args, domain.ErrInvalidValue)...)
	}
	seen := map[string]bool{}
	for _, t := range d.Tables {
		known, ok := schema[t.Name]
		if !ok {
			return invalid("unknown table %q", t.Name)
		}
		if seen[t.Name] {
			return invalid("table %q listed twice", t.Name)
		}
		seen[t.Name] = true
		if len(t.Columns) == 0 {
			return invalid("table %q has no columns", t.Name)
		}
		colSeen := map[string]bool{}
		for _, c := range t.Columns {
			if !slices.Contains(known, c) {
				return invalid("unknown column %s.%s", t.Name, c)
			}
			if colSeen[c] {
				return invalid("column %s.%s listed twice", t.Name, c)
			}
			colSeen[c] = true
		}
		for i, row := range t.Rows {
			if len(row) != len(t.Columns) {
				return invalid("%s row %d has %d cells, want %d", t.Name, i+1, len(row), len(t.Columns))
			}
			for j, cell := range row {
				switch cell.(type) {
				case nil, int64, string:
				default:
					return invalid("%s.%s row %d: unsupported cell type %T", t.Name, t.Columns[j], i+1, cell)
				}
			}
		}
	}
	return nil
}
