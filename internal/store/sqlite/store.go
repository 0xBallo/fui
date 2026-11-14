package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"sync"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/backup"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"

	_ "modernc.org/sqlite" // registers the pure-Go SQLite driver for database/sql
)

const (
	sqliteDriverName           = "sqlite"
	defaultReadPoolSize        = 4
	defaultBusyTimeoutMS       = 5000
	currentSchemaVersion int64 = 15
)

// Options configures opening a SQLite store.
type Options struct {
	Path         string
	BackupDir    string
	BackupKeep   int
	ReadPoolSize int
	Clock        app.Clock
}

// Store is the SQLite adapter implementing app.Store, app.Backuper, and reporting.Source.
type Store struct {
	path       string
	backupDir  string
	backupKeep int
	clock      app.Clock

	mu      sync.RWMutex
	writeDB *sql.DB
	readDB  *sql.DB
}

var (
	_ app.Store        = (*Store)(nil)
	_ app.Backuper     = (*Store)(nil)
	_ reporting.Source = (*Store)(nil)
)

// Open initializes a SQLite store, applies migrations, and returns the ready adapter.
func Open(ctx context.Context, opts Options) (*Store, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("open sqlite store: missing path: %w", domain.ErrMissingField)
	}
	if opts.BackupDir == "" {
		return nil, fmt.Errorf("open sqlite store: missing backup dir: %w", domain.ErrMissingField)
	}
	if opts.BackupKeep <= 0 {
		return nil, fmt.Errorf("open sqlite store: invalid backup.keep=%d: %w", opts.BackupKeep, domain.ErrInvalidValue)
	}
	if opts.ReadPoolSize <= 0 {
		opts.ReadPoolSize = defaultReadPoolSize
	}
	if opts.Clock == nil {
		system := clock.NewSystem()
		opts.Clock = system
	}

	existing, err := ensureDBFile(opts.Path)
	if err != nil {
		return nil, err
	}
	if err := backup.EnsureDir(opts.BackupDir); err != nil {
		return nil, fmt.Errorf("open sqlite store: ensure backup dir: %w", err)
	}

	writeDB, err := openDB(opts.Path, 1, true)
	if err != nil {
		return nil, fmt.Errorf("open sqlite store: open write db: %w", mapSQLError(err))
	}

	opened := &Store{
		path:       opts.Path,
		backupDir:  opts.BackupDir,
		backupKeep: opts.BackupKeep,
		clock:      opts.Clock,
		writeDB:    writeDB,
	}
	if err := opened.migrate(ctx, existing); err != nil {
		_ = writeDB.Close()
		return nil, err
	}

	readDB, err := openDB(opts.Path, opts.ReadPoolSize, false)
	if err != nil {
		_ = writeDB.Close()
		return nil, fmt.Errorf("open sqlite store: open read db: %w", mapSQLError(err))
	}
	opened.readDB = readDB

	return opened, nil
}

// Close releases all DB resources.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error
	if s.readDB != nil {
		errs = append(errs, s.readDB.Close())
		s.readDB = nil
	}
	if s.writeDB != nil {
		errs = append(errs, s.writeDB.Close())
		s.writeDB = nil
	}
	return errors.Join(errs...)
}

// Read executes fn in a read-only transaction.
func (s *Store) Read(ctx context.Context, fn func(app.Repos) error) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("sqlite read: %w", err)
	}

	s.mu.RLock()
	db := s.readDB
	s.mu.RUnlock()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("sqlite read: begin tx: %w", mapSQLError(err))
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(newRepos(tx, true)); err != nil {
		return err
	}
	return nil
}

// WithTx executes fn in one SQL transaction.
func (s *Store) WithTx(ctx context.Context, fn func(app.Repos) error) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("sqlite withtx: %w", err)
	}

	s.mu.RLock()
	db := s.writeDB
	s.mu.RUnlock()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite withtx: begin tx: %w", mapSQLError(err))
	}

	if err := fn(newRepos(tx, false)); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("sqlite withtx: commit: %w", mapSQLError(err))
	}
	return nil
}

func openDB(path string, maxOpen int, immediateTxLock bool) (*sql.DB, error) {
	dsn := sqliteDSN(path, immediateTxLock)
	db, err := sql.Open(sqliteDriverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db %q: %w", path, err)
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxOpen)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite db %q: %w", path, err)
	}
	return db, nil
}

func sqliteDSN(path string, immediateTxLock bool) string {
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(ON)")
	values.Add("_pragma", "journal_mode(WAL)")
	values.Add("_pragma", "synchronous(FULL)")
	values.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", defaultBusyTimeoutMS))
	if immediateTxLock {
		values.Set("_txlock", "immediate")
	}

	u := url.URL{
		Scheme:   "file",
		Path:     path,
		RawQuery: values.Encode(),
	}
	return u.String()
}

func newQueries(tx *sql.Tx) *gen.Queries {
	return gen.New(tx)
}
