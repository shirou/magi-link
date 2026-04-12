package spell

import (
	"embed"
	"fmt"
	"sort"

	"github.com/BurntSushi/toml"
)

//go:embed spells.toml
var embeddedSpells embed.FS

// spellsFile is the top-level TOML structure.
type spellsFile struct {
	Spells map[string]SpellDef `toml:"spells"`
}

// Registry holds all loaded spell definitions and provides lookups.
type Registry struct {
	defs map[string]*SpellDef
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{defs: make(map[string]*SpellDef)}
}

// LoadEmbedded loads spell definitions from the embedded spells.toml.
func LoadEmbedded() (*Registry, error) {
	data, err := embeddedSpells.ReadFile("spells.toml")
	if err != nil {
		return nil, fmt.Errorf("read embedded spells.toml: %w", err)
	}
	return Load(data)
}

// Load parses TOML bytes into a Registry.
func Load(data []byte) (*Registry, error) {
	var f spellsFile
	if err := toml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse spells.toml: %w", err)
	}

	r := NewRegistry()
	for id, def := range f.Spells {
		d := def // copy
		d.ID = id
		r.defs[id] = &d
	}
	return r, nil
}

// Get returns a spell definition by ID, or nil if not found.
func (r *Registry) Get(id string) *SpellDef {
	return r.defs[id]
}

// All returns all spell definitions sorted by ID.
func (r *Registry) All() []*SpellDef {
	out := make([]*SpellDef, 0, len(r.defs))
	for _, d := range r.defs {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Targets returns all target-type spell definitions.
func (r *Registry) Targets() []*SpellDef {
	var out []*SpellDef
	for _, d := range r.defs {
		if d.IsTarget() {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Actions returns all action-type spell definitions.
func (r *Registry) Actions() []*SpellDef {
	var out []*SpellDef
	for _, d := range r.defs {
		if d.IsAction() {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Count returns the total number of spells.
func (r *Registry) Count() int {
	return len(r.defs)
}
