package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
)

func newTestStore(t *testing.T) (*Store, *clock.Fixed) {
	t.Helper()

	baseTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	clk := clock.NewFixed(baseTime)

	root := t.TempDir()
	store, err := Open(context.Background(), Options{
		Path:         filepath.Join(root, "data", "fui.db"),
		BackupDir:    filepath.Join(root, "data", "backups"),
		BackupKeep:   10,
		ReadPoolSize: 4,
		Clock:        clk,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Fatalf("Close: %v", closeErr)
		}
	})
	return store, clk
}

func mustCurrency(t *testing.T, code string) domain.Currency {
	t.Helper()
	c, err := domain.ParseCurrency(code)
	if err != nil {
		t.Fatalf("ParseCurrency(%q): %v", code, err)
	}
	return c
}

func mustDate(t *testing.T, s string) domain.Date {
	t.Helper()
	d, err := domain.ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", s, err)
	}
	return d
}

func mustMoney(t *testing.T, minor int64, currency domain.Currency) domain.Money {
	t.Helper()
	m, err := domain.NewMoney(minor, currency)
	if err != nil {
		t.Fatalf("NewMoney(%d, %s): %v", minor, currency, err)
	}
	return m
}
