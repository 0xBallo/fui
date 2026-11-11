package memstore_test

import (
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

func TestContract(t *testing.T) {
	memstore.RunContract(t, func(*testing.T) app.Store { return memstore.New() })
}
