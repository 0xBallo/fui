package app

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// AddTagRequest creates a tag.
type AddTagRequest struct {
	Name string
}

// AddTagPlan is the plan to create Tag.
type AddTagPlan struct {
	Plan
	Tag domain.Tag
}

// PlanAddTag validates the request and plans the creation.
func (s *Service) PlanAddTag(ctx context.Context, req AddTagRequest) (AddTagPlan, error) {
	tg, err := domain.NewTag(req.Name)
	if err != nil {
		return AddTagPlan{}, fmt.Errorf("tag: %w", err)
	}
	err = s.store.Read(ctx, func(r Repos) error {
		tags, err := r.Tags().List(ctx)
		if err != nil {
			return fmt.Errorf("list tags: %w", err)
		}
		return checkTagName(tags, tg.Name, 0)
	})
	if err != nil {
		return AddTagPlan{}, err
	}
	ch := Change{Op: OpCreate, Entity: EntityTag, Summary: fmt.Sprintf("add tag %q", tg.Name), Fields: map[string]any{"name": tg.Name}}
	return AddTagPlan{Plan: Plan{Changes: []Change{ch}}, Tag: tg}, nil
}

// ApplyAddTag creates the tag and returns it with its ID.
func (s *Service) ApplyAddTag(ctx context.Context, p AddTagPlan) (domain.Tag, error) {
	var out domain.Tag
	err := s.apply(ctx, p.Bulk, func(r Repos) (err error) {
		out, err = r.Tags().Insert(ctx, p.Tag)
		return err
	})
	return out, err
}

// ListTags returns every tag.
func (s *Service) ListTags(ctx context.Context) ([]domain.Tag, error) {
	var out []domain.Tag
	err := s.store.Read(ctx, func(r Repos) (err error) {
		out, err = r.Tags().List(ctx)
		return err
	})
	return out, err
}

// RenameTagRequest renames the tag named by Tag.
type RenameTagRequest struct {
	Tag     string
	NewName string
}

// RenameTagPlan is the plan to store Tag.
type RenameTagPlan struct {
	Plan
	Tag domain.Tag
}

// PlanRenameTag plans the rename.
func (s *Service) PlanRenameTag(ctx context.Context, req RenameTagRequest) (RenameTagPlan, error) {
	var out RenameTagPlan
	err := s.store.Read(ctx, func(r Repos) error {
		tags, err := r.Tags().List(ctx)
		if err != nil {
			return fmt.Errorf("list tags: %w", err)
		}
		cur, err := pickTag(tags, req.Tag)
		if err != nil {
			return err
		}
		renamed, err := domain.NewTag(req.NewName)
		if err != nil {
			return fmt.Errorf("tag: %w", err)
		}
		renamed.ID = cur.ID
		if err := checkTagName(tags, renamed.Name, cur.ID); err != nil {
			return err
		}
		ch := Change{
			Op: OpUpdate, Entity: EntityTag,
			Summary: fmt.Sprintf("rename tag %q to %q", cur.Name, renamed.Name),
			Fields:  map[string]any{"id": int64(cur.ID), "name": renamed.Name, "previous_name": cur.Name},
		}
		out = RenameTagPlan{Plan: Plan{Changes: []Change{ch}}, Tag: renamed}
		return nil
	})
	return out, err
}

// ApplyRenameTag stores the renamed tag.
func (s *Service) ApplyRenameTag(ctx context.Context, p RenameTagPlan) (domain.Tag, error) {
	err := s.apply(ctx, p.Bulk, func(r Repos) error {
		if err := r.Tags().Update(ctx, p.Tag); err != nil {
			return fmt.Errorf("rename tag %d: %w", p.Tag.ID, err)
		}
		return nil
	})
	return p.Tag, err
}

// RemoveTagRequest removes Tag, detaching it from every transaction that carries it.
type RemoveTagRequest struct {
	Tag string
}

// RemoveTagPlan deletes Tag; Detached is how many transactions carry it (0 when none). It is
// Bulk when any transaction is affected, so Apply takes a backup first.
type RemoveTagPlan struct {
	Plan
	Tag      domain.Tag
	Detached int
}

// PlanRemoveTag plans the removal.
func (s *Service) PlanRemoveTag(ctx context.Context, req RemoveTagRequest) (RemoveTagPlan, error) {
	var out RemoveTagPlan
	err := s.store.Read(ctx, func(r Repos) error {
		tags, err := r.Tags().List(ctx)
		if err != nil {
			return fmt.Errorf("list tags: %w", err)
		}
		tg, err := pickTag(tags, req.Tag)
		if err != nil {
			return err
		}
		used, err := countTaggedTx(ctx, r, tg.ID)
		if err != nil {
			return err
		}
		out = RemoveTagPlan{Tag: tg, Detached: used, Plan: Plan{Bulk: used > 0}}
		summary := fmt.Sprintf("delete tag %q", tg.Name)
		if used > 0 {
			summary = fmt.Sprintf("delete tag %q, detaching it from %d transactions", tg.Name, used)
		}
		out.Changes = append(out.Changes, Change{
			Op: OpDelete, Entity: EntityTag, Summary: summary,
			Fields: map[string]any{"id": int64(tg.ID), "name": tg.Name, "detached": used},
		})
		return nil
	})
	return out, err
}

// ApplyRemoveTag deletes the tag, detaching it from every transaction.
func (s *Service) ApplyRemoveTag(ctx context.Context, p RemoveTagPlan) error {
	return s.apply(ctx, p.Bulk, func(r Repos) error {
		if err := r.Tags().Delete(ctx, p.Tag.ID); err != nil {
			return fmt.Errorf("delete tag %q: %w", p.Tag.Name, err)
		}
		return nil
	})
}

// MergeTagRequest retags every transaction tagged Source with Target (Source is detached from
// them) and removes Source.
type MergeTagRequest struct {
	Source string
	Target string
}

// MergeTagPlan merges Source into Target. Moved is how many transactions were retagged.
type MergeTagPlan struct {
	Plan
	Source domain.Tag
	Target domain.Tag
	Moved  int
}

// PlanMergeTag plans the merge.
func (s *Service) PlanMergeTag(ctx context.Context, req MergeTagRequest) (MergeTagPlan, error) {
	var out MergeTagPlan
	err := s.store.Read(ctx, func(r Repos) error {
		tags, err := r.Tags().List(ctx)
		if err != nil {
			return fmt.Errorf("list tags: %w", err)
		}
		src, err := pickTag(tags, req.Source)
		if err != nil {
			return err
		}
		dst, err := pickTag(tags, req.Target)
		if err != nil {
			return err
		}
		if src.ID == dst.ID {
			return newError(domain.ErrInvalidValue, map[string]any{"field": "target", "value": req.Target},
				"cannot merge tag %q into itself", src.Name)
		}
		used, err := countTaggedTx(ctx, r, src.ID)
		if err != nil {
			return err
		}
		out = MergeTagPlan{Source: src, Target: dst, Moved: used, Plan: Plan{Bulk: used > 0}}
		out.Changes = append(out.Changes, Change{
			Op: OpUpdate, Entity: EntityTag,
			Summary: fmt.Sprintf("retag %d transactions from %q to %q", used, src.Name, dst.Name),
			Fields:  map[string]any{"source": src.Name, "target": dst.Name, "count": used},
		}, Change{
			Op: OpDelete, Entity: EntityTag, Summary: fmt.Sprintf("delete tag %q", src.Name),
			Fields: map[string]any{"id": int64(src.ID), "name": src.Name},
		})
		return nil
	})
	return out, err
}

// ApplyMergeTag retags every transaction tagged Source with Target, then deletes Source.
func (s *Service) ApplyMergeTag(ctx context.Context, p MergeTagPlan) error {
	return s.apply(ctx, p.Bulk, func(r Repos) error {
		f, err := domain.NewTxFilter(domain.TxFilterSpec{TagIDs: []domain.TagID{p.Source.ID}})
		if err != nil {
			return err
		}
		txs, err := r.Transactions().List(ctx, f, nil, 0)
		if err != nil {
			return fmt.Errorf("list transactions tagged %q: %w", p.Source.Name, err)
		}
		for _, tx := range txs {
			cur, err := r.Tags().TagsOf(ctx, tx.ID)
			if err != nil {
				return fmt.Errorf("tags of transaction %d: %w", tx.ID, err)
			}
			if err := r.Tags().SetTags(ctx, tx.ID, retagIDs(cur, p.Source.ID, p.Target.ID)); err != nil {
				return fmt.Errorf("retag transaction %d: %w", tx.ID, err)
			}
		}
		if err := r.Tags().Delete(ctx, p.Source.ID); err != nil {
			return fmt.Errorf("delete tag %q: %w", p.Source.Name, err)
		}
		return nil
	})
}

// TagTransactionRequest adds Add to the tags transaction ID already carries.
type TagTransactionRequest struct {
	ID  domain.TransactionID
	Add []string
}

// TagTransactionPlan is the plan to tag ID with the resolved tag ids.
type TagTransactionPlan struct {
	Plan
	ID   domain.TransactionID
	Tags []domain.TagID
}

// PlanTagTransaction plans adding Add to the transaction's current tags.
func (s *Service) PlanTagTransaction(ctx context.Context, req TagTransactionRequest) (TagTransactionPlan, error) {
	var out TagTransactionPlan
	err := s.store.Read(ctx, func(r Repos) error {
		cur, names, err := currentTagIDsAndNames(ctx, r, req.ID)
		if err != nil {
			return err
		}
		tags, err := r.Tags().List(ctx)
		if err != nil {
			return fmt.Errorf("list tags: %w", err)
		}
		added := make([]string, 0, len(req.Add))
		for _, ref := range req.Add {
			tg, err := pickTag(tags, ref)
			if err != nil {
				return err
			}
			if !containsTagID(cur, tg.ID) {
				cur = append(cur, tg.ID)
				added = append(added, tg.Name)
			}
		}
		out = TagTransactionPlan{ID: req.ID, Tags: cur}
		if len(added) > 0 {
			out.Changes = append(out.Changes, Change{
				Op: OpUpdate, Entity: EntityTransaction,
				Summary: fmt.Sprintf("tag transaction %d with %v (currently %v)", req.ID, added, names),
				Fields:  map[string]any{"id": int64(req.ID), "added": added},
			})
		}
		return nil
	})
	return out, err
}

// ApplyTagTransaction stores the resolved tag set.
func (s *Service) ApplyTagTransaction(ctx context.Context, p TagTransactionPlan) error {
	return s.apply(ctx, p.Bulk, func(r Repos) error {
		return r.Tags().SetTags(ctx, p.ID, p.Tags)
	})
}

// UntagTransactionRequest removes Remove from the tags transaction ID carries.
type UntagTransactionRequest struct {
	ID     domain.TransactionID
	Remove []string
}

// PlanUntagTransaction plans removing Remove from the transaction's current tags.
func (s *Service) PlanUntagTransaction(ctx context.Context, req UntagTransactionRequest) (TagTransactionPlan, error) {
	var out TagTransactionPlan
	err := s.store.Read(ctx, func(r Repos) error {
		cur, _, err := currentTagIDsAndNames(ctx, r, req.ID)
		if err != nil {
			return err
		}
		tags, err := r.Tags().List(ctx)
		if err != nil {
			return fmt.Errorf("list tags: %w", err)
		}
		removed := make([]string, 0, len(req.Remove))
		for _, ref := range req.Remove {
			tg, err := pickTag(tags, ref)
			if err != nil {
				return err
			}
			if containsTagID(cur, tg.ID) {
				cur = removeTagID(cur, tg.ID)
				removed = append(removed, tg.Name)
			}
		}
		out = TagTransactionPlan{ID: req.ID, Tags: cur}
		if len(removed) > 0 {
			out.Changes = append(out.Changes, Change{
				Op: OpUpdate, Entity: EntityTransaction,
				Summary: fmt.Sprintf("untag transaction %d of %v", req.ID, removed),
				Fields:  map[string]any{"id": int64(req.ID), "removed": removed},
			})
		}
		return nil
	})
	return out, err
}

// ApplyUntagTransaction applies a plan made by PlanUntagTransaction or PlanTagTransaction.
func (s *Service) ApplyUntagTransaction(ctx context.Context, p TagTransactionPlan) error {
	return s.ApplyTagTransaction(ctx, p)
}

// currentTagIDsAndNames fails with ErrNotFound when the transaction does not exist.
func currentTagIDsAndNames(ctx context.Context, r Repos, id domain.TransactionID) ([]domain.TagID, []string, error) {
	if _, err := r.Transactions().Get(ctx, id); err != nil {
		return nil, nil, fmt.Errorf("transaction %d: %w", id, err)
	}
	cur, err := r.Tags().TagsOf(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("tags of transaction %d: %w", id, err)
	}
	ids := make([]domain.TagID, len(cur))
	names := make([]string, len(cur))
	for i, tg := range cur {
		ids[i], names[i] = tg.ID, tg.Name
	}
	return ids, names, nil
}

// countTaggedTx counts the transactions tagged id.
func countTaggedTx(ctx context.Context, r Repos, id domain.TagID) (int, error) {
	f, err := domain.NewTxFilter(domain.TxFilterSpec{TagIDs: []domain.TagID{id}})
	if err != nil {
		return 0, err
	}
	txs, err := r.Transactions().List(ctx, f, nil, 0)
	if err != nil {
		return 0, fmt.Errorf("count transactions tagged %d: %w", id, err)
	}
	return len(txs), nil
}

// retagIDs returns ids with every occurrence of from replaced by to (deduplicated).
func retagIDs(tags []domain.Tag, from, to domain.TagID) []domain.TagID {
	out := make([]domain.TagID, 0, len(tags))
	hasTo := false
	for _, tg := range tags {
		if tg.ID == to {
			hasTo = true
		}
	}
	for _, tg := range tags {
		if tg.ID == from {
			if !hasTo {
				out = append(out, to)
				hasTo = true
			}
			continue
		}
		out = append(out, tg.ID)
	}
	return out
}

func containsTagID(ids []domain.TagID, id domain.TagID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func removeTagID(ids []domain.TagID, id domain.TagID) []domain.TagID {
	out := make([]domain.TagID, 0, len(ids))
	for _, x := range ids {
		if x != id {
			out = append(out, x)
		}
	}
	return out
}

// checkTagName fails with ErrAlreadyExists when a tag other than self has the same key.
func checkTagName(tags []domain.Tag, name string, self domain.TagID) error {
	key := domain.Tag{Name: name}.Key()
	for _, tg := range tags {
		if tg.ID != self && tg.Key() == key {
			return newError(ErrAlreadyExists, map[string]any{"entity": EntityTag, "name": tg.Name},
				"a tag named %q already exists", tg.Name)
		}
	}
	return nil
}
