package resolve

import (
	"github.com/shirou/magi_link/internal/entity"
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/spell"
	"github.com/shirou/magi_link/internal/terrain"
)

// LinkInput is what the game provides when the player confirms a target.
type LinkInput struct {
	CasterPos  hex.Hex
	ClickedHex hex.Hex           // the hex the player clicked
	Direction  int               // 0-5, hex direction derived from click angle
	Spells     []*spell.SpellDef // the full link's spell list
}

// StatusChange describes a status effect to apply or remove on a unit.
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

// LinkResult holds the outcome of executing a full spell link.
// The resolve package never mutates game state directly; the game
// applies results with animations/logging as needed.
type LinkResult struct {
	Targets []spell.Target
	Hexes   []hex.Hex

	Damage         map[int]int
	Healing        map[int]int
	StatusApplied  []StatusChange
	StatusRemoved  []StatusChange
	TerrainChanges []TerrainChange
	UnitsMoved     []UnitMove
}

// stepFlag is a transient modifier applied to the next action step.
// Flags are consumed (cleared) after the next action-type shape runs.
type stepFlag uint

const (
	flagPierce stepFlag = 1 << iota
	flagBounce
)

// LinkState is the running state of a link execution.
// Each spell step reads and returns a transformed LinkState
// (bucket-relay model).
type LinkState struct {
	Origins []hex.Hex      // current "casting points" for the next action/modifier
	Targets []spell.Target // accumulated hit targets (never removed)
	Hexes   []hex.Hex      // all affected hexes (for VFX/rendering)
	Flags   stepFlag       // transient modifier flags consumed by next action
}

// ExecuteLink runs the full link pipeline and returns the result.
// Target-type spells transform the running LinkState (Phase 1);
// action-type spells consume the current Targets and write diffs
// into the result (Phase 2). The two phases are interleaved in chain
// order so chains like `single → fireball → line → fireball` work
// correctly: the second fireball fires against targets accumulated
// by both the `single` and `line` steps.
func ExecuteLink(input LinkInput, bf Battlefield) LinkResult {
	state := LinkState{Origins: []hex.Hex{input.CasterPos}}
	result := LinkResult{
		Damage:  make(map[int]int),
		Healing: make(map[int]int),
	}

	// If the chain has action spells but no target spell, seed a default
	// single-target at the clicked hex so action-only chains (e.g. fireball
	// alone) fire on the clicked hex. Empty chains stay empty.
	hasTarget, hasAction := classifySpells(input.Spells)
	if hasAction && !hasTarget {
		state = seedDefaultTarget(state, input, bf)
	}

	shapeCounts := make(map[spell.TargetShape]int)

	for _, s := range input.Spells {
		switch {
		case s.IsTarget():
			shapeCounts[s.Shape]++
			state = applyStep(state, s, input, bf, shapeCounts[s.Shape])
		case s.IsAction():
			state = applyAction(state, s, input.CasterPos, bf, &result)
		}
	}

	result.Targets = state.Targets
	result.Hexes = state.Hexes
	return result
}

func classifySpells(spells []*spell.SpellDef) (hasTarget, hasAction bool) {
	for _, s := range spells {
		if s.IsTarget() {
			hasTarget = true
		}
		if s.IsAction() {
			hasAction = true
		}
	}
	return
}

// seedDefaultTarget treats the clicked hex as an implicit single-target so
// action-only chains have something to fire on. The clicked hex must be in
// bounds and within the caster's natural reach (single-spell range of 5).
func seedDefaultTarget(state LinkState, input LinkInput, bf Battlefield) LinkState {
	const defaultReach = 5
	if !bf.GridBounds().InBounds(input.ClickedHex) {
		return state
	}
	if input.CasterPos.Distance(input.ClickedHex) > defaultReach {
		return state
	}
	unitID := -1
	if u := bf.UnitAt(input.ClickedHex); u.IsAlive() {
		unitID = u.ID
	}
	state.Origins = []hex.Hex{input.ClickedHex}
	state.Targets = append(state.Targets, spell.Target{
		UnitID: unitID,
		HexQ:   input.ClickedHex.Q,
		HexR:   input.ClickedHex.R,
	})
	state.Hexes = append(state.Hexes, input.ClickedHex)
	return state
}
