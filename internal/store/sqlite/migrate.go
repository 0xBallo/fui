package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xballo/fui/internal/backup"
	"github.com/0xballo/fui/internal/domain"

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

	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("open sqlite store: configure goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, s.writeDB, "migrations"); err != nil {
		return &domainErrorf{
			sentinel: errMigrationFailed,
			format:   "migrate schema: %v",
			args:     []any{err},
		}
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
