package lists

import (
	"strings"

	"github.com/0xballo/fui/internal/domain"
)

// idsToNames resolves account ids against accts and joins the matching names with ", ".
func idsToNames(ids []domain.AccountID, accts []domain.Account) string {
	var names []string
	for _, id := range ids {
		for _, a := range accts {
			if a.ID == id {
				names = append(names, a.Name)
				break
			}
		}
	}
	return strings.Join(names, ", ")
}

// tagIDsToNames resolves tag ids against tags and joins the matching names with ", ".
func tagIDsToNames(ids []domain.TagID, tags []domain.Tag) string {
	var names []string
	for _, id := range ids {
		for _, t := range tags {
			if t.ID == id {
				names = append(names, t.Name)
				break
			}
		}
	}
	return strings.Join(names, ", ")
}

// categoryNameByID returns the name of id among cats, or "" when unset or not found.
func categoryNameByID(id domain.CategoryID, cats []domain.Category) string {
	if id == 0 {
		return ""
	}
	for _, c := range cats {
		if c.ID == id {
			return c.Name
		}
	}
	return ""
}

// payeeNameByID returns the name of id among payees, or "" when unset or not found.
func payeeNameByID(id domain.PayeeID, payees []domain.Payee) string {
	if id == 0 {
		return ""
	}
	for _, p := range payees {
		if p.ID == id {
			return p.Name
		}
	}
	return ""
}
