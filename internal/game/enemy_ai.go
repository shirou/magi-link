package game

import (
	"github.com/shirou/magi_link/internal/entity"
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/resolve"
)

// ActionKind is the decision of enemy AI for a single turn.
type ActionKind int

const (
	ActionWait ActionKind = iota
	ActionMove
	ActionMelee
)

// EnemyAction is what the AI decided the enemy should do this turn.
type EnemyAction struct {
	Kind   ActionKind
	Move   hex.Hex      // valid when Kind == ActionMove
	Target *entity.Unit // valid when Kind == ActionMelee
}

// DecideEnemyAction returns the action self should take this turn.
//
// Rules:
//   - If a living player unit is adjacent, melee it.
//   - Otherwise BFS-reachable hexes (within self.MoveRange) and pick the
//     one closest to the player. Move there.
//   - If no reachable hex makes progress (or there is no player), wait.
//
// The function is pure: it reads bf but mutates nothing.
func DecideEnemyAction(self *entity.Unit, bf resolve.Battlefield) EnemyAction {
	player := findPlayer(bf)
	if player == nil {
		return EnemyAction{Kind: ActionWait}
	}

	if self.Pos.Distance(player.Pos) <= 1 {
		return EnemyAction{Kind: ActionMelee, Target: player}
	}

	grid := bf.GridBounds()
	blocked := func(h hex.Hex) bool {
		if bf.IsWall(h) {
			return true
		}
		u := bf.UnitAt(h)
		return u != nil && u.ID != self.ID
	}

	reachable := grid.Reachable(self.Pos, self.MoveRange, blocked)

	best := self.Pos
	bestDist := self.Pos.Distance(player.Pos)
	for h := range reachable {
		if h == self.Pos {
			continue
		}
		d := h.Distance(player.Pos)
		if d < bestDist {
			best = h
			bestDist = d
		}
	}

	if best == self.Pos {
		return EnemyAction{Kind: ActionWait}
	}
	return EnemyAction{Kind: ActionMove, Move: best}
}

func findPlayer(bf resolve.Battlefield) *entity.Unit {
	for _, u := range bf.AllUnits() {
		if u.IsPlayer && u.IsAlive() {
			return u
		}
	}
	return nil
}
