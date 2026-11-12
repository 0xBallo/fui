package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/0xballo/fui/internal/domain"
)

// TagDTO is the --json shape of one tag.
type TagDTO struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func tagDTO(t domain.Tag) TagDTO { return TagDTO{ID: int64(t.ID), Name: t.Name} }

func tagRows(tags []domain.Tag) Rows {
	t := Rows{Header: []string{"NAME"}}
	for _, tg := range tags {
		t.Body = append(t.Body, []string{tg.Name})
	}
	return t
}

// WriteTagList renders a list of tags.
func WriteTagList(w io.Writer, f Format, tags []domain.Tag) error {
	if f == JSON {
		dtos := make([]TagDTO, len(tags))
		for i, tg := range tags {
			dtos[i] = tagDTO(tg)
		}
		return WriteJSON(w, dtos)
	}
	if f == CSV {
		return WriteCSV(w, tagRows(tags))
	}
	return WriteTable(w, tagRows(tags), false)
}

// WriteTag renders one tag.
func WriteTag(w io.Writer, f Format, t domain.Tag) error {
	if f == JSON {
		return WriteJSON(w, tagDTO(t))
	}
	return WriteTagList(w, f, []domain.Tag{t})
}

// TxTagsDTO is the --json shape of a transaction's tag set after `tx tag`/`tx untag`.
type TxTagsDTO struct {
	ID   int64    `json:"id"`
	Tags []string `json:"tags"`
}

func txTagsRows(id int64, tags []string) Rows {
	return Rows{Header: []string{"ID", "TAGS"}, Body: [][]string{{fmt.Sprint(id), strings.Join(tags, ",")}}}
}

// WriteTxTags renders the resulting tag set of one transaction.
func WriteTxTags(w io.Writer, f Format, id int64, tags []string) error {
	if tags == nil {
		tags = []string{}
	}
	if f == JSON {
		return WriteJSON(w, TxTagsDTO{ID: id, Tags: tags})
	}
	if f == CSV {
		return WriteCSV(w, txTagsRows(id, tags))
	}
	return WriteTable(w, txTagsRows(id, tags), false)
}
