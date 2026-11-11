package render

import (
	"io"

	"github.com/0xballo/fui/internal/domain"
)

// CategoryDTO is the --json shape of one category.
type CategoryDTO struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

func categoryDTO(c domain.Category) CategoryDTO {
	return CategoryDTO{ID: int64(c.ID), Name: c.Name, Kind: string(c.Kind)}
}

func categoryRows(cats []domain.Category) Rows {
	t := Rows{Header: []string{"NAME", "KIND"}}
	for _, c := range cats {
		t.Body = append(t.Body, []string{c.Name, string(c.Kind)})
	}
	return t
}

// WriteCategoryList renders a list of categories.
func WriteCategoryList(w io.Writer, f Format, cats []domain.Category) error {
	if f == JSON {
		dtos := make([]CategoryDTO, len(cats))
		for i, c := range cats {
			dtos[i] = categoryDTO(c)
		}
		return WriteJSON(w, dtos)
	}
	if f == CSV {
		return WriteCSV(w, categoryRows(cats))
	}
	return WriteTable(w, categoryRows(cats), false)
}

// WriteCategory renders one category.
func WriteCategory(w io.Writer, f Format, c domain.Category) error {
	if f == JSON {
		return WriteJSON(w, categoryDTO(c))
	}
	return WriteCategoryList(w, f, []domain.Category{c})
}
