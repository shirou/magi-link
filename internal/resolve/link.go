package resolve

import (
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/spell"
)

// LinkInput is what the game provides when the player confirms a target.
type LinkInput struct {
	CasterPos  hex.Hex
	ClickedHex hex.Hex         // the hex the player clicked (ignored for auto-target shapes)
	Direction  int             // 0-5, hex direction for line spells (derived from click angle)
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

// ExecuteLink runs the full link pipeline and returns the result.
// Currently only Phase 1 (target resolution) is implemented.
func ExecuteLink(input LinkInput, bf Battlefield) LinkResult {
	targets, hexes := ResolveTargets(input, bf)
	return LinkResult{
		Targets: targets,
		Hexes:   hexes,
	}
}
