package resolve

import (
	"slices"

	"github.com/shirou/magi_link/internal/entity"
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/spell"
	"github.com/shirou/magi_link/internal/terrain"
)

// LinkInput is what the game provides when the player confirms a cast.
type LinkInput struct {
	CasterPos  hex.Hex
	ClickedHex hex.Hex
	Direction  int // 0-5, hex direction derived from click angle
	Spells     []*spell.SpellDef
}

// Impact is a unit-level target: something that will receive damage, heal,
// status, or movement from an action spell. Purely unit-based; a hex with
// no unit is never an Impact.
type Impact struct {
	UnitID int
	Pos    hex.Hex
}

// StatusChange describes a status effect applied to / removed from a unit.
type StatusChange struct {
	UnitID int
	Status entity.StatusEffect
	Turns  int
}

// TerrainChange describes a terrain mutation at a given hex.
type TerrainChange struct {
	Pos  hex.Hex
	Type terrain.TerrainType
}

// UnitMove describes a unit position change.
type UnitMove struct {
	UnitID int
	From   hex.Hex
	To     hex.Hex
}

// LinkResult is the side-effect-free output of a link cast.
// The game applies it over time with animations.
type LinkResult struct {
	Impacts   []Impact  // units affected by damage / heal / status / movement
	Field     []hex.Hex // hexes that form the spell's area (terrain + VFX derivation)
	Waypoints []hex.Hex // hexes traversed by projectiles (VFX only, no effect)

	Damage         map[int]int
	Healing        map[int]int
	StatusApplied  []StatusChange
	StatusRemoved  []StatusChange
	TerrainChanges []TerrainChange
	UnitsMoved     []UnitMove
}

// stepFlag is a transient modifier set by modifier spells (pierce, bounce)
// and consumed by the next line-type spell.
type stepFlag uint

const (
	flagPierce stepFlag = 1 << iota
	flagBounce
)

// LinkState is the running state threaded through every step of a link.
// The four hex/unit sets carry distinct semantics:
//
//   - Origins   : next step's launch point (projectile / explosion center)
//   - Impacts   : units that receive damage / heal / status / movement
//   - Field     : area hexes the spell covers (terrain changes + VFX derivation)
//   - Waypoints : hexes a projectile passes through (VFX only)
//
// originsSet tracks whether any spell has explicitly moved Origins away
// from the default [caster]. This lets explode-style actions fall back
// to ClickedHex when no target spell has refined the aim yet.
type LinkState struct {
	Origins   []hex.Hex
	Impacts   []Impact
	Field     []hex.Hex
	Waypoints []hex.Hex
	Flags     stepFlag

	originsSet bool
}

// ExecuteLink runs a link and returns its side-effect-free result.
func ExecuteLink(input LinkInput, bf Battlefield) LinkResult {
	state := LinkState{Origins: []hex.Hex{input.CasterPos}}
	state = seedInitialState(state, input, bf)

	// Damage/Healing maps are lazily allocated the first time an action
	// spell touches a unit; target-only chains (e.g. cast previews) skip
	// the allocation entirely.
	result := LinkResult{}

	shapeCounts := make(map[spell.TargetShape]int)
	for _, s := range input.Spells {
		switch {
		case s.IsTarget():
			shapeCounts[s.Shape]++
			state = applyStep(state, s, input, bf, shapeCounts[s.Shape])
		case s.IsAction():
			state = applyAction(state, s, input, bf, &result)
		}
	}

	result.Impacts = state.Impacts
	result.Field = state.Field
	result.Waypoints = state.Waypoints
	return result
}

// seedInitialState picks up any unit at the clicked hex as the initial
// Impact (Spellmasons-style auto-target). For action-only chains (no
// target spell present) it also moves Origins to the clicked hex so
// explode-style actions fire at the click, not at the caster's feet.
//
// Target spells are responsible for their own Origins semantics: line
// walks from the current Origins (= caster by default), area recenters
// Origins to clicked, etc.
func seedInitialState(state LinkState, input LinkInput, bf Battlefield) LinkState {
	if !bf.GridBounds().InBounds(input.ClickedHex) {
		return state
	}
	if u := bf.UnitAt(input.ClickedHex); u.IsAlive() {
		state = addImpact(state, u.ID, input.ClickedHex)
	}
	if !slices.ContainsFunc(input.Spells, (*spell.SpellDef).IsTarget) {
		state = setOrigins(state, input.ClickedHex)
	}
	return state
}

// --- State mutation helpers ---

// setOrigins replaces Origins with the given hexes and flags them as
// explicitly set. Use this whenever a spell step aims the chain
// somewhere — explodes and line launches both rely on originsSet.
func setOrigins(state LinkState, hexes ...hex.Hex) LinkState {
	state.Origins = hexes
	state.originsSet = true
	return state
}

func addImpact(state LinkState, unitID int, pos hex.Hex) LinkState {
	for _, im := range state.Impacts {
		if im.UnitID == unitID {
			return state
		}
	}
	state.Impacts = append(state.Impacts, Impact{UnitID: unitID, Pos: pos})
	return state
}

func addField(state LinkState, h hex.Hex) LinkState {
	if slices.Contains(state.Field, h) {
		return state
	}
	state.Field = append(state.Field, h)
	return state
}

func addWaypoint(state LinkState, h hex.Hex) LinkState {
	if slices.Contains(state.Waypoints, h) {
		return state
	}
	state.Waypoints = append(state.Waypoints, h)
	return state
}
