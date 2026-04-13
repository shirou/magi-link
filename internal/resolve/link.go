package resolve

import (
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/spell"
)

// LinkInput is what the game provides when the player confirms a target.
type LinkInput struct {
	CasterPos  hex.Hex
	ClickedHex hex.Hex           // the hex the player clicked
	Direction  int               // 0-5, hex direction derived from click angle
	Spells     []*spell.SpellDef // the full link's spell list
}

// StatusChange describes a status effect to apply or remove on a unit.
// Status names are strings so the resolve package does not depend on
// the entity status enum; the game layer parses with entity.ParseStatus.
type StatusChange struct {
	UnitID int
	Status string
	Turns  int
}

// TerrainChange describes a terrain mutation at a given hex.
// Type is a terrain type name; the game layer parses with terrain.ParseTerrainType.
type TerrainChange struct {
	Pos  hex.Hex
	Type string
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
	// Phase 1: Target resolution
	Targets []spell.Target
	Hexes   []hex.Hex // all selected hexes (for rendering/VFX)

	// Phase 2: Action execution
	Damage         map[int]int // unitID → total damage
	Healing        map[int]int // unitID → total healing
	StatusApplied  []StatusChange
	StatusRemoved  []StatusChange
	TerrainChanges []TerrainChange
	UnitsMoved     []UnitMove

	// Phase 3: Turn-end resolution (future, separate call)
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

	// Same-shape count for optional stacking behaviors.
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

	result.Targets = state.Targets
	result.Hexes = state.Hexes
	return result
}
