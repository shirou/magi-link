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

// LinkResult holds the outcome of executing a full spell link.
// The resolve package never mutates game state directly; the game
// applies results with animations/logging as needed.
type LinkResult struct {
	// Phase 1: Target resolution
	Targets []spell.Target
	Hexes   []hex.Hex // all selected hexes (for rendering/VFX)

	// Phase 2: Action execution (future)
	// Damage         map[int]int
	// Healing        map[int]int
	// StatusApplied  []StatusChange
	// StatusRemoved  []StatusChange
	// TerrainChanges []TerrainChange
	// UnitsMoved     []UnitMove

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
// Currently only Phase 1 (target resolution) is implemented.
func ExecuteLink(input LinkInput, bf Battlefield) LinkResult {
	state := LinkState{Origins: []hex.Hex{input.CasterPos}}

	// Same-shape count for optional stacking behaviors.
	shapeCounts := make(map[spell.TargetShape]int)

	for _, s := range input.Spells {
		if !s.IsTarget() {
			// Action spells are handled in Phase 2.
			continue
		}
		shapeCounts[s.Shape]++
		state = applyStep(state, s, input, bf, shapeCounts[s.Shape])
	}

	return LinkResult{
		Targets: state.Targets,
		Hexes:   state.Hexes,
	}
}
