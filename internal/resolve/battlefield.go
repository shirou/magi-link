package resolve

import (
	"github.com/shirou/magi_link/internal/entity"
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/terrain"
)

// Battlefield abstracts the game state needed for link resolution.
// game.BattleState satisfies this interface.
//
// UnitAt may return nil (no living unit at the hex); resolve helpers
// treat nil as "no unit". TerrainAt must never return nil — callers
// rely on being able to chain `.IsPassable()` / `.TypeName()` directly.
type Battlefield interface {
	UnitAt(h hex.Hex) *entity.Unit
	AllUnits() []*entity.Unit
	TerrainAt(h hex.Hex) *terrain.Terrain
	GridBounds() *hex.Grid
	IsWall(h hex.Hex) bool
	HasLineOfSight(from, to hex.Hex) bool
}
