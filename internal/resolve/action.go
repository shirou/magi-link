package resolve

import (
	"github.com/shirou/magi_link/internal/entity"
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/spell"
)

// applyAction processes one action-type spell step against the current
// LinkState. It reads state.Targets and writes diffs into result.
// Returns the (possibly modified) state — e.g. when ClearTargets is set.
func applyAction(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield, result *LinkResult) LinkState {
	// Damage / heal / status / terrain affect every target hex
	// (and their explode-radius neighbors).
	hitHexes := expandHitHexes(state.Targets, s.ExplodeRadius, bf)
	for _, h := range hitHexes {
		applyHit(s, h, bf, result)
	}

	// Movement is per-target only (not affected by explode radius).
	if s.Movement != "" {
		applyMovement(state, s, input, bf, result)
	}

	if s.ClearTargets {
		state.Targets = nil
	}
	return state
}

// expandHitHexes returns the set of hexes affected by the action,
// including explode_radius neighbors. Deduplicated and in-bounds.
func expandHitHexes(targets []spell.Target, explodeRadius int, bf Battlefield) []hex.Hex {
	seen := make(map[hex.Hex]bool)
	var out []hex.Hex
	grid := bf.GridBounds()
	add := func(h hex.Hex) {
		if seen[h] || !grid.InBounds(h) {
			return
		}
		seen[h] = true
		out = append(out, h)
	}
	for _, t := range targets {
		center := hex.NewHex(t.HexQ, t.HexR)
		add(center)
		if explodeRadius > 0 {
			for _, h := range center.Area(explodeRadius) {
				add(h)
			}
		}
	}
	return out
}

// applyHit writes the action's damage/heal/status/terrain effects for
// a single hex into result. Unit-targeted effects are no-ops if the
// hex has no living unit; terrain effects apply to the hex regardless.
func applyHit(s *spell.SpellDef, h hex.Hex, bf Battlefield, result *LinkResult) {
	u := bf.UnitAt(h)
	hasUnit := u != nil && !u.IsDead

	// Damage (with status-combo bonus)
	if s.Damage > 0 && hasUnit {
		dmg := s.Damage
		for _, combo := range s.StatusCombos {
			if combo.If == "" {
				continue
			}
			existing := entity.ParseStatus(combo.If)
			if existing == entity.StatusNone || !u.HasStatus(existing) {
				continue
			}
			dmg += combo.BonusDamage
			if combo.Remove {
				result.StatusRemoved = append(result.StatusRemoved, StatusChange{
					UnitID: u.ID,
					Status: combo.If,
				})
			}
			if combo.Apply != "" {
				result.StatusApplied = append(result.StatusApplied, StatusChange{
					UnitID: u.ID,
					Status: combo.Apply,
					Turns:  combo.ApplyTurns,
				})
			}
		}
		result.Damage[u.ID] += dmg
	}

	// Heal
	if s.Heal > 0 && hasUnit {
		result.Healing[u.ID] += s.Heal
	}

	// Primary status application (independent of combos)
	if s.Status != "" && hasUnit {
		result.StatusApplied = append(result.StatusApplied, StatusChange{
			UnitID: u.ID,
			Status: s.Status,
			Turns:  s.StatusTurns,
		})
	}

	// Terrain creation (applied to the hex regardless of unit presence)
	if s.TerrainCreate != "" {
		result.TerrainChanges = append(result.TerrainChanges, TerrainChange{
			Pos:  h,
			Type: s.TerrainCreate,
		})
	}

	// Terrain interactions (transform existing terrain into something else)
	if len(s.TerrainInteractions) > 0 {
		if t := bf.TerrainAt(h); t != nil {
			name := t.TypeName()
			for _, ti := range s.TerrainInteractions {
				if ti.On == name {
					result.TerrainChanges = append(result.TerrainChanges, TerrainChange{
						Pos:  h,
						Type: ti.Result,
					})
					break
				}
			}
		}
	}
}

// applyMovement computes push/pull moves for each target and appends
// valid moves to result.UnitsMoved. Blocked destinations (wall,
// out-of-bounds, occupied) cause the move to be skipped silently.
func applyMovement(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield, result *LinkResult) {
	for _, t := range state.Targets {
		pos := hex.NewHex(t.HexQ, t.HexR)
		u := bf.UnitAt(pos)
		if u == nil || u.IsDead {
			continue
		}
		var dir int
		switch s.Movement {
		case "push":
			dir = directionAwayFrom(pos, input.CasterPos)
		case "pull":
			dir = directionToward(pos, input.CasterPos)
		default:
			continue
		}
		to := pos.Direction(dir)
		if !canMoveTo(to, bf) {
			continue
		}
		result.UnitsMoved = append(result.UnitsMoved, UnitMove{
			UnitID: u.ID,
			From:   pos,
			To:     to,
		})
	}
}

// directionAwayFrom returns the hex direction (0-5) from `from` that
// maximises distance from `source`.
func directionAwayFrom(from, source hex.Hex) int {
	bestDir := 0
	bestDist := from.Distance(source)
	for i := 0; i < 6; i++ {
		d := from.Direction(i).Distance(source)
		if d > bestDist {
			bestDist = d
			bestDir = i
		}
	}
	return bestDir
}

// directionToward returns the hex direction (0-5) from `from` that
// minimises distance to `goal`.
func directionToward(from, goal hex.Hex) int {
	bestDir := 0
	bestDist := from.Distance(goal)
	for i := 0; i < 6; i++ {
		d := from.Direction(i).Distance(goal)
		if d < bestDist {
			bestDist = d
			bestDir = i
		}
	}
	return bestDir
}

// canMoveTo reports whether a unit can legally be placed at h.
func canMoveTo(h hex.Hex, bf Battlefield) bool {
	if !bf.GridBounds().InBounds(h) {
		return false
	}
	if bf.IsWall(h) {
		return false
	}
	if u := bf.UnitAt(h); u != nil && !u.IsDead {
		return false
	}
	return true
}
