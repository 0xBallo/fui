package sqlite

import (
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

func TestStoreContract(t *testing.T) {
	memstore.RunContract(t, func(t *testing.T) app.Store {
		store, _ := newTestStore(t)
		return store
	})
}
