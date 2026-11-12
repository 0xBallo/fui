package lists

import (
	"testing"

	"github.com/0xballo/fui/internal/domain"
)

func TestYesNo(t *testing.T) {
	if yesNo(true) != "yes" {
		t.Fatalf("expected yes")
	}
	if yesNo(false) != "no" {
		t.Fatalf("expected no")
	}
}

func TestPtrString(t *testing.T) {
	if got := ptrString(nil); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	v := int64(42)
	if got := ptrString(&v); got != "42" {
		t.Fatalf("expected 42, got %q", got)
	}
}

func TestParseOptionalInt64(t *testing.T) {
	if parseOptionalInt64("") != nil {
		t.Fatalf("expected nil for empty")
	}
	if parseOptionalInt64("abc") != nil {
		t.Fatalf("expected nil for invalid")
	}
	v := parseOptionalInt64("123")
	if v == nil || *v != 123 {
		t.Fatalf("expected 123, got %v", v)
	}
}

func TestSplitList(t *testing.T) {
	if got := splitList(""); got != nil {
		t.Fatalf("expected nil for empty, got %v", got)
	}
	if got := splitList("  "); got != nil {
		t.Fatalf("expected nil for blank, got %v", got)
	}
	got := splitList("a, b ,c")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestIDsToNames(t *testing.T) {
	accts := []domain.Account{{ID: 1, Name: "Checking"}, {ID: 2, Name: "Savings"}}
	if got := idsToNames(nil, accts); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	if got := idsToNames([]domain.AccountID{2, 1}, accts); got != "Savings, Checking" {
		t.Fatalf("expected ordered names, got %q", got)
	}
	if got := idsToNames([]domain.AccountID{99}, accts); got != "" {
		t.Fatalf("expected empty for unknown id, got %q", got)
	}
}

func TestTagIDsToNames(t *testing.T) {
	tags := []domain.Tag{{ID: 1, Name: "business"}}
	if got := tagIDsToNames([]domain.TagID{1}, tags); got != "business" {
		t.Fatalf("expected business, got %q", got)
	}
}

func TestCategoryNameByID(t *testing.T) {
	cats := []domain.Category{{ID: 1, Name: "Groceries"}}
	if got := categoryNameByID(0, cats); got != "" {
		t.Fatalf("expected empty for zero id, got %q", got)
	}
	if got := categoryNameByID(1, cats); got != "Groceries" {
		t.Fatalf("expected Groceries, got %q", got)
	}
	if got := categoryNameByID(99, cats); got != "" {
		t.Fatalf("expected empty for unknown id, got %q", got)
	}
}

func TestPayeeNameByID(t *testing.T) {
	payees := []domain.Payee{{ID: 1, Name: "Landlord"}}
	if got := payeeNameByID(0, payees); got != "" {
		t.Fatalf("expected empty for zero id, got %q", got)
	}
	if got := payeeNameByID(1, payees); got != "Landlord" {
		t.Fatalf("expected Landlord, got %q", got)
	}
	if got := payeeNameByID(99, payees); got != "" {
		t.Fatalf("expected empty for unknown id, got %q", got)
	}
}

func TestAccountNameByID(t *testing.T) {
	accts := []domain.Account{{ID: 1, Name: "Checking"}}
	if got := accountNameByID(1, accts); got != "Checking" {
		t.Fatalf("expected Checking, got %q", got)
	}
	if got := accountNameByID(99, accts); got != "" {
		t.Fatalf("expected empty for unknown id, got %q", got)
	}
}

func TestDefaultKeyMapBindings(t *testing.T) {
	km := DefaultKeyMap()
	if len(km.Bindings()) != 6 {
		t.Fatalf("expected 6 bindings, got %d", len(km.Bindings()))
	}
	if len(km.BindingsWithTest()) != 7 {
		t.Fatalf("expected 7 bindings, got %d", len(km.BindingsWithTest()))
	}
}
