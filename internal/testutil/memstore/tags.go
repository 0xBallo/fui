package memstore

import (
	"context"
	"slices"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

type tags struct{ *repos }

func (t tags) checkName(tag domain.Tag) error {
	for _, o := range t.st.Tags {
		if o.ID != tag.ID && strings.EqualFold(o.Name, tag.Name) {
			return exists("tag", tag.Name)
		}
	}
	return nil
}

func (t tags) Insert(_ context.Context, tag domain.Tag) (domain.Tag, error) {
	if err := t.writable(); err != nil {
		return domain.Tag{}, err
	}
	tag.ID = 0
	if err := t.checkName(tag); err != nil {
		return domain.Tag{}, err
	}
	tag.ID = domain.TagID(t.st.nextID("tags"))
	t.st.Tags[tag.ID] = tag
	return tag, nil
}

func (t tags) Update(_ context.Context, tag domain.Tag) error {
	if err := t.writable(); err != nil {
		return err
	}
	if _, ok := t.st.Tags[tag.ID]; !ok {
		return notFound("tag", tag.ID)
	}
	if err := t.checkName(tag); err != nil {
		return err
	}
	t.st.Tags[tag.ID] = tag
	return nil
}

func (t tags) Delete(_ context.Context, id domain.TagID) error {
	if err := t.writable(); err != nil {
		return err
	}
	if _, ok := t.st.Tags[id]; !ok {
		return notFound("tag", id)
	}
	delete(t.st.Tags, id)
	for tx, ids := range t.st.TxTags {
		if slices.Contains(ids, id) {
			t.st.TxTags[tx] = slices.DeleteFunc(slices.Clone(ids), func(o domain.TagID) bool { return o == id })
		}
	}
	return nil
}

func (t tags) Get(_ context.Context, id domain.TagID) (domain.Tag, error) {
	tag, ok := t.st.Tags[id]
	if !ok {
		return domain.Tag{}, notFound("tag", id)
	}
	return tag, nil
}

func (t tags) List(context.Context) ([]domain.Tag, error) {
	return sortedValues(t.st.Tags), nil
}

func (t tags) TagsOf(_ context.Context, tx domain.TransactionID) ([]domain.Tag, error) {
	out := []domain.Tag{}
	for _, id := range t.st.TxTags[tx] {
		out = append(out, t.st.Tags[id])
	}
	slices.SortFunc(out, func(a, b domain.Tag) int { return int(a.ID - b.ID) })
	return out, nil
}

func (t tags) SetTags(_ context.Context, tx domain.TransactionID, ids []domain.TagID) error {
	if err := t.writable(); err != nil {
		return err
	}
	if _, ok := t.st.Txs[tx]; !ok {
		return notFound("transaction", tx)
	}
	var set []domain.TagID
	for _, id := range ids {
		if _, ok := t.st.Tags[id]; !ok {
			return notFound("tag", id)
		}
		if !slices.Contains(set, id) {
			set = append(set, id)
		}
	}
	if len(set) == 0 {
		delete(t.st.TxTags, tx)
		return nil
	}
	t.st.TxTags[tx] = set
	return nil
}

var _ app.TagRepo = tags{}
