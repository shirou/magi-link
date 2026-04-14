package game

import (
	"testing"

	"github.com/shirou/magi_link/internal/entity"
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/terrain"
)

// aiTestBF is a minimal Battlefield for AI tests.
type aiTestBF struct {
	grid  *hex.Grid
	tm    *terrain.Map
	units []*entity.Unit
}

func (b *aiTestBF) UnitAt(h hex.Hex) *entity.Unit {
	for _, u := range b.units {
		if u.IsAlive() && u.Pos == h {
			return u
		}
	}
	return nil
}
func (b *aiTestBF) AllUnits() []*entity.Unit            { return b.units }
func (b *aiTestBF) TerrainAt(h hex.Hex) *terrain.Terrain { return b.tm.Get(h) }
func (b *aiTestBF) GridBounds() *hex.Grid               { return b.grid }
func (b *aiTestBF) IsWall(h hex.Hex) bool               { return b.tm.Get(h).IsWall() }
func (b *aiTestBF) HasLineOfSight(from, to hex.Hex) bool {
	return b.tm.HasLineOfSight(from, to)
}

func newAITestBF(w, h int) *aiTestBF {
	return &aiTestBF{
		grid: hex.NewGrid(w, h, 20, 0, 0),
		tm:   terrain.NewMap(w, h),
	}
}

func newEnemy(id int, pos hex.Hex) *entity.Unit {
	u := entity.NewUnit(id, "E", pos, 20, 0)
	u.MoveRange = 3
	return u
}

func newTestPlayer(pos hex.Hex) *entity.Unit {
	u := entity.NewUnit(1, "P", pos, 100, 0)
	u.IsPlayer = true
	return u
}

func TestAIMeleesWhenAdjacent(t *testing.T) {
	bf := newAITestBF(10, 10)
	player := newTestPlayer(hex.OffsetToHex(3, 3))
	enemy := newEnemy(2, hex.OffsetToHex(4, 3))
	bf.units = []*entity.Unit{player, enemy}

	act := DecideEnemyAction(enemy, bf)
	if act.Kind != ActionMelee {
		t.Fatalf("expected ActionMelee, got %v", act.Kind)
	}
	if act.Target != player {
		t.Fatal("melee target should be player")
	}
}

func TestAIMovesCloserWhenFar(t *testing.T) {
	bf := newAITestBF(10, 10)
	player := newTestPlayer(hex.OffsetToHex(1, 3))
	enemy := newEnemy(2, hex.OffsetToHex(8, 3))
	bf.units = []*entity.Unit{player, enemy}

	act := DecideEnemyAction(enemy, bf)
	if act.Kind != ActionMove {
		t.Fatalf("expected ActionMove, got %v", act.Kind)
	}
	oldDist := enemy.Pos.Distance(player.Pos)
	newDist := act.Move.Distance(player.Pos)
	if newDist >= oldDist {
		t.Fatalf("enemy should move closer: %d -> %d", oldDist, newDist)
	}
}

func TestAIWaitsWhenPlayerDead(t *testing.T) {
	bf := newAITestBF(10, 10)
	player := newTestPlayer(hex.OffsetToHex(1, 3))
	player.IsDead = true
	enemy := newEnemy(2, hex.OffsetToHex(8, 3))
	bf.units = []*entity.Unit{player, enemy}

	act := DecideEnemyAction(enemy, bf)
	if act.Kind != ActionWait {
		t.Fatalf("expected ActionWait when no living player, got %v", act.Kind)
	}
}

func TestAIWaitsWhenFullyBlocked(t *testing.T) {
	bf := newAITestBF(10, 10)
	player := newTestPlayer(hex.OffsetToHex(5, 5))
	enemy := newEnemy(2, hex.OffsetToHex(0, 0))
	// Surround enemy with walls so no neighbor is reachable.
	for _, n := range enemy.Pos.Neighbors() {
		if bf.grid.InBounds(n) {
			bf.tm.Set(n, &terrain.Terrain{Type: terrain.TerrainRock, Duration: -1})
		}
	}
	bf.units = []*entity.Unit{player, enemy}

	act := DecideEnemyAction(enemy, bf)
	if act.Kind != ActionWait {
		t.Fatalf("expected ActionWait when boxed in, got %v", act.Kind)
	}
}

func TestAIBlockedByOtherEnemy(t *testing.T) {
	bf := newAITestBF(10, 10)
	player := newTestPlayer(hex.OffsetToHex(5, 3))
	enemy := newEnemy(2, hex.OffsetToHex(8, 3))
	// Another enemy in between — should still move through free hexes.
	other := newEnemy(3, hex.OffsetToHex(7, 3))
	bf.units = []*entity.Unit{player, enemy, other}

	act := DecideEnemyAction(enemy, bf)
	if act.Kind != ActionMove {
		t.Fatalf("expected ActionMove, got %v", act.Kind)
	}
	if act.Move == other.Pos {
		t.Fatal("enemy should not move onto another unit")
	}
}
