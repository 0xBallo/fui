// Package module defines the static module registry and operations on enabled-module sets.
package module

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ID is a module identifier.
type ID string

// Known module identifiers.
const (
	Core        ID = "core"
	Budget      ID = "budget"
	Recurring   ID = "recurring"
	Loans       ID = "loans"
	Investments ID = "investments"
	Crypto      ID = "crypto"
	Assets      ID = "assets"
	Analytics   ID = "analytics"
	Simulations ID = "simulations"
	AI          ID = "ai"
)

type entry struct {
	deps      []ID
	defaultOn bool
	expense   bool
}

// ErrDisabled marks failures caused by a disabled module.
var ErrDisabled = errors.New("module disabled")

// Definition is a read-only view of one module registry entry.
type Definition struct {
	ID             ID
	Dependencies   []ID
	DefaultEnabled bool
	InExpense      bool
}

// Definitions returns every module definition in registry order.
func Definitions() []Definition {
	ids := allIDs()
	out := make([]Definition, 0, len(ids))
	for _, id := range ids {
		e, _ := registryEntry(id)
		out = append(out, Definition{
			ID:             id,
			Dependencies:   slices.Clone(e.deps),
			DefaultEnabled: e.defaultOn,
			InExpense:      e.expense,
		})
	}
	return out
}

// Set is an immutable enabled-module set.
type Set struct {
	ids []ID
}

func newSet(ids []ID) Set {
	uniq := make(map[ID]struct{}, len(ids))
	out := make([]ID, 0, len(ids))
	for _, id := range ids {
		if _, seen := uniq[id]; seen {
			continue
		}
		uniq[id] = struct{}{}
		out = append(out, id)
	}
	slices.Sort(out)
	return Set{ids: out}
}

// IDs returns sorted IDs of this set.
func (s Set) IDs() []ID {
	return slices.Clone(s.ids)
}

// Has reports whether id is enabled.
func (s Set) Has(id ID) bool {
	_, ok := slices.BinarySearch(s.ids, id)
	return ok
}

// Require returns ErrDisabled when id is not enabled.
func (s Set) Require(id ID) error {
	if s.Has(id) {
		return nil
	}
	return fmt.Errorf("%w: module %q is disabled; run fui module enable %s", ErrDisabled, id, id)
}

// Default returns the default enabled set.
func Default() Set {
	ids := allIDs()
	out := make([]ID, 0, len(ids))
	for _, id := range ids {
		e, _ := registryEntry(id)
		if e.defaultOn {
			out = append(out, id)
		}
	}
	return newSet(out)
}

// Preset returns the preset enabled set.
func Preset(name string) (Set, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "expense":
		ids := allIDs()
		out := make([]ID, 0, len(ids))
		for _, id := range ids {
			e, _ := registryEntry(id)
			if e.expense {
				out = append(out, id)
			}
		}
		return newSet(out), nil
	case "full":
		return newSet(allIDs()), nil
	default:
		return Set{}, fmt.Errorf("unknown module preset %q", name)
	}
}

// Validate validates IDs and returns a normalized set.
func Validate(ids []ID) (Set, error) {
	s := newSet(ids)
	for _, id := range s.ids {
		if _, ok := registryEntry(id); !ok {
			return Set{}, fmt.Errorf("unknown module %q", id)
		}
	}
	if !s.Has(Core) {
		return Set{}, fmt.Errorf("module %q is required", Core)
	}
	for _, id := range s.ids {
		e, _ := registryEntry(id)
		for _, dep := range e.deps {
			if !s.Has(dep) {
				return Set{}, fmt.Errorf("module %q requires %q", id, dep)
			}
		}
	}
	return s, nil
}

// Enable enables id and all its dependencies.
func Enable(s Set, id ID) (Set, []ID, error) {
	if _, ok := registryEntry(id); !ok {
		return s, nil, fmt.Errorf("unknown module %q", id)
	}
	addedMap := make(map[ID]struct{})
	addDependenciesAndSelf(s, addedMap, id)
	added := make([]ID, 0, len(addedMap))
	next := slices.Clone(s.ids)
	for mid := range addedMap {
		if !s.Has(mid) {
			next = append(next, mid)
			added = append(added, mid)
		}
	}
	slices.Sort(added)
	return newSet(next), added, nil
}

func addDependenciesAndSelf(s Set, added map[ID]struct{}, id ID) {
	e, _ := registryEntry(id)
	for _, dep := range e.deps {
		addDependenciesAndSelf(s, added, dep)
	}
	if s.Has(id) {
		return
	}
	added[id] = struct{}{}
}

// Disable disables id and every active transitive dependent.
func Disable(s Set, id ID) (Set, []ID, error) {
	if id == Core {
		return s, nil, fmt.Errorf("module %q cannot be disabled", Core)
	}
	if _, ok := registryEntry(id); !ok {
		return s, nil, fmt.Errorf("unknown module %q", id)
	}
	if !s.Has(id) {
		return s, nil, nil
	}
	dependents := collectDependents(s, id)
	toRemove := append(slices.Clone(dependents), id)
	next := removeIDs(s.ids, toRemove)
	return newSet(next), dependents, nil
}

func collectDependents(s Set, id ID) []ID {
	found := make(map[ID]struct{})
	var visit func(ID)
	visit = func(target ID) {
		for _, candidate := range s.ids {
			if candidate == target {
				continue
			}
			if _, seen := found[candidate]; seen {
				continue
			}
			e, _ := registryEntry(candidate)
			if slices.Contains(e.deps, target) {
				found[candidate] = struct{}{}
				visit(candidate)
			}
		}
	}
	visit(id)
	out := make([]ID, 0, len(found))
	for dep := range found {
		out = append(out, dep)
	}
	slices.Sort(out)
	return out
}

func removeIDs(ids, toRemove []ID) []ID {
	rm := make(map[ID]struct{}, len(toRemove))
	for _, id := range toRemove {
		rm[id] = struct{}{}
	}
	out := make([]ID, 0, len(ids))
	for _, id := range ids {
		if _, drop := rm[id]; drop {
			continue
		}
		out = append(out, id)
	}
	return out
}

func allIDs() []ID {
	return []ID{
		Core, Budget, Recurring, Loans, Investments, Crypto, Assets, Analytics, Simulations, AI,
	}
}

func registryEntry(id ID) (entry, bool) {
	switch id {
	case Core:
		return entry{defaultOn: true, expense: true}, true
	case Budget:
		return entry{defaultOn: true, expense: true}, true
	case Recurring:
		return entry{defaultOn: true, expense: true}, true
	case Loans:
		return entry{deps: []ID{Recurring}}, true
	case Investments:
		return entry{}, true
	case Crypto:
		return entry{deps: []ID{Investments}}, true
	case Assets:
		return entry{}, true
	case Analytics:
		return entry{defaultOn: true}, true
	case Simulations:
		return entry{deps: []ID{Recurring}}, true
	case AI:
		return entry{deps: []ID{Simulations}}, true
	default:
		return entry{}, false
	}
}
