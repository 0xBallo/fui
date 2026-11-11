package sqlite

import (
	"errors"
	"fmt"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"

	msqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	// CodeDBLocked is returned when SQLite cannot acquire locks in time.
	CodeDBLocked domain.Code = "db_locked"
	// CodeDBCorrupted is returned when a database file fails integrity checks.
	CodeDBCorrupted domain.Code = "db_corrupted"
	// CodeMigrationFailed is returned when schema migration fails.
	CodeMigrationFailed domain.Code = "migration_failed"
	// CodeDBNewerThanBinary is returned when the DB schema is newer than this binary.
	CodeDBNewerThanBinary domain.Code = "db_newer_than_binary"
)

var (
	errDBLocked          = &domain.Error{Code: CodeDBLocked, Message: "database is locked"}
	errDBCorrupted       = &domain.Error{Code: CodeDBCorrupted, Message: "database is corrupted"}
	errMigrationFailed   = &domain.Error{Code: CodeMigrationFailed, Message: "database migration failed"}
	errDBNewerThanBinary = &domain.Error{Code: CodeDBNewerThanBinary, Message: "database schema is newer than this binary"}
	errReadOnly          = errors.New("sqlite: write in read-only transaction")
)

func mapSQLError(err error) error {
	if err == nil {
		return nil
	}
	var sqlErr *msqlite.Error
	if !errors.As(err, &sqlErr) {
		return err
	}

	switch sqlErr.Code() {
	case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
		return &domain.Error{
			Code:    errDBLocked.Code,
			Message: fmt.Sprintf("database busy or locked: %v", err),
		}
	case sqlite3.SQLITE_CORRUPT, sqlite3.SQLITE_NOTADB:
		return &domain.Error{
			Code:    errDBCorrupted.Code,
			Message: fmt.Sprintf("database file is invalid: %v", err),
		}
	case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
		return app.ErrAlreadyExists
	case sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
		return app.ErrInUse
	case sqlite3.SQLITE_CONSTRAINT:
		if strings.Contains(strings.ToUpper(err.Error()), "FOREIGN KEY") {
			return app.ErrInUse
		}
		return app.ErrAlreadyExists
	default:
		if sqlErr.Code()&0xFF == sqlite3.SQLITE_CONSTRAINT {
			if strings.Contains(strings.ToUpper(err.Error()), "FOREIGN KEY") {
				return app.ErrInUse
			}
			return app.ErrAlreadyExists
		}
		return err
	}
}
