package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xballo/fui/internal/backup"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/textnorm"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func ensureDBFile(path string) (bool, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false, fmt.Errorf("open sqlite store: create db dir %q: %w", dir, err)
	}

	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("open sqlite store: stat db %q: %w", path, err)
	}

	// #nosec G304 -- database path comes from configuration and is intentionally opened.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return false, fmt.Errorf("open sqlite store: create db file %q: %w", path, err)
	}
	if closeErr := f.Close(); closeErr != nil {
		return false, fmt.Errorf("open sqlite store: close new db file %q: %w", path, closeErr)
	}
	return false, nil
}

func (s *Store) migrate(ctx context.Context, existing bool) error {
	userVersion, err := pragmaUserVersion(ctx, s.writeDB)
	if err != nil {
		return fmt.Errorf("open sqlite store: read user_version: %w", mapSQLError(err))
	}
	if userVersion > currentSchemaVersion {
		return &domainErrorf{
			sentinel: errDBNewerThanBinary,
			format:   "database schema version %d is newer than binary version %d",
			args:     []any{userVersion, currentSchemaVersion},
		}
	}

	pending := userVersion < currentSchemaVersion
	if pending && existing {
		info, statErr := os.Stat(s.path)
		if statErr != nil {
			return fmt.Errorf("open sqlite store: stat existing db: %w", statErr)
		}
		if info.Size() > 0 {
			if _, err := s.backupLocked(ctx, backup.ReasonMigrate); err != nil {
				return err
			}
		}
	}

	migrationsFS, err := fs.Sub(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("open sqlite store: load migrations fs: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, s.writeDB, migrationsFS)
	if err != nil {
		return fmt.Errorf("open sqlite store: create migration provider: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return &domainErrorf{
			sentinel: errMigrationFailed,
			format:   "migrate schema: %v",
			args:     []any{err},
		}
	}
	if userVersion < searchSchemaVersion {
		return backfillSearchText(ctx, s.writeDB)
	}
	return nil
}

// searchSchemaVersion is the migration that added transactions.search_text.
const searchSchemaVersion = 3

// backfillSearchText fills search_text of rows that predate migration 0003. It runs in Go because
// SQL cannot do the NFD folding; the update triggers keep the FTS index in sync.
func backfillSearchText(ctx context.Context, db *sql.DB) error {
	type row struct {
		id                 int64
		description, notes string
	}
	rows, err := db.QueryContext(ctx, "SELECT id, description, notes FROM transactions WHERE description <> '' OR notes <> ''")
	if err != nil {
		return fmt.Errorf("backfill search_text: list: %w", err)
	}
	var todo []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.description, &r.notes); err != nil {
			_ = rows.Close()
			return fmt.Errorf("backfill search_text: scan: %w", err)
		}
		todo = append(todo, r)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("backfill search_text: close: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("backfill search_text: iterate: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("backfill search_text: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, r := range todo {
		if _, err := tx.ExecContext(ctx, "UPDATE transactions SET search_text = ? WHERE id = ?",
			textnorm.SearchText(r.description, "", r.notes), r.id); err != nil {
			return fmt.Errorf("backfill search_text: update %d: %w", r.id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("backfill search_text: commit: %w", err)
	}
	return nil
}

func pragmaUserVersion(ctx context.Context, db *sql.DB) (int64, error) {
	var v int64
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		return 0, fmt.Errorf("read pragma user_version: %w", err)
	}
	return v, nil
}

type domainErrorf struct {
	sentinel *domain.Error
	format   string
	args     []any
}

func (e *domainErrorf) Error() string {
	return fmt.Sprintf(e.format, e.args...)
}

func (e *domainErrorf) Is(target error) bool {
	return e.sentinel.Is(target)
}

func collectSchemaDump(ctx context.Context, db *sql.DB) (string, error) {
	const q = `
SELECT type, name, tbl_name, COALESCE(sql, '')
FROM sqlite_master
WHERE name NOT LIKE 'sqlite_%'
ORDER BY type, name`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return "", fmt.Errorf("query sqlite_master: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var b strings.Builder
	for rows.Next() {
		var typ, name, tblName, stmt string
		if err := rows.Scan(&typ, &name, &tblName, &stmt); err != nil {
			return "", fmt.Errorf("scan sqlite_master row: %w", err)
		}
		b.WriteString(typ)
		b.WriteString("|")
		b.WriteString(name)
		b.WriteString("|")
		b.WriteString(tblName)
		b.WriteString("|")
		b.WriteString(strings.TrimSpace(stmt))
		b.WriteString("\n")
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate sqlite_master rows: %w", err)
	}
	return b.String(), nil
}
