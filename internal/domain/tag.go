package domain

import "strings"

// Tag is a free label transversal to categories, in a many-to-many relation
// with transactions. Two tags with the same Key are the same tag regardless
// of letter case.
type Tag struct {
	ID   TagID
	Name string
}

// NewTag validates and returns a tag with the name trimmed. A tag name must
// not be empty and must not contain spaces or commas, which are reserved as
// separators in the CLI (`-t a,b` / repeated `-t`).
func NewTag(name string) (Tag, error) {
	n, err := requireName("name", name)
	if err != nil {
		return Tag{}, err
	}
	if strings.ContainsAny(n, " ,") {
		return Tag{}, invalidValue("name", name)
	}
	return Tag{Name: n}, nil
}

// Key returns the case-insensitive identity of the tag, so "Vacanze" and
// "vacanze" collide.
func (t Tag) Key() string { return strings.ToLower(t.Name) }
