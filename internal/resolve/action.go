package resolve

import (
	"github.com/shirou/magi_link/internal/entity"
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/spell"
	"github.com/shirou/magi_link/internal/terrain"
)

// applyAction processes one action-type spell step against the current
// LinkState. It reads state.Targets and writes diffs into result.
// Returns the (possibly modified) state — e.g. when ClearTargets is set.
func applyAction(state LinkState, s *spell.SpellDef, casterPos hex.Hex, bf Battlefield, result *LinkResult) LinkState {
	hitHexes := expandHitHexes(state.Targets, s.ExplodeRadius, bf)
	for _, h := range hitHexes {
		applyHit(s, h, bf, result)
	}
	// Expose the full hit set (including explode_radius expansion) to
	// state.Hexes so VFX and cast preview cover the actual area of effect.
	state = recordHexes(state, hitHexes, nil)

	// Movement uses the pre-expansion target list and caster position
	// to derive push/pull direction.
	if s.Movement != "" {
		applyMovement(state.Targets, s.Movement, casterPos, bf, result)
	}

	if s.ClearTargets {
		state.Targets = nil
	}
	return state
}

// expandHitHexes returns the set of hexes affected by the action,
// including explode_radius neighbors. Deduplicated and in-bounds.
func expandHitHexes(targets []spell.Target, explodeRadius int, bf Battlefield) []hex.Hex {
	grid := bf.GridBounds()
	raw := make([]hex.Hex, 0, len(targets))
	for _, t := range targets {
		center := hex.NewHex(t.HexQ, t.HexR)
		if grid.InBounds(center) {
			raw = append(raw, center)
		}
		if explodeRadius > 0 {
			for _, h := range center.Area(explodeRadius) {
				if grid.InBounds(h) {
					raw = append(raw, h)
				}
			}
		}
	}
	return dedupHexes(raw)
}

// applyHit writes the action's damage / heal / status / terrain effects
// for a single hex into result. Unit-targeted effects no-op when the
// hex has no living unit; terrain effects apply regardless.
func applyHit(s *spell.SpellDef, h hex.Hex, bf Battlefield, result *LinkResult) {
	u := bf.UnitAt(h)

	if s.Damage > 0 && u.IsAlive() {
		dmg := s.Damage + applyStatusCombos(s.StatusCombos, u, result)
		result.Damage[u.ID] += dmg
	}

	if s.Heal > 0 && u.IsAlive() {
		result.Healing[u.ID] += s.Heal
	}

	if s.Status != "" && u.IsAlive() {
		if st := entity.ParseStatus(s.Status); st != entity.StatusNone {
			result.StatusApplied = append(result.StatusApplied, StatusChange{
				UnitID: u.ID,
				Status: st,
				Turns:  s.StatusTurns,
			})
		}
	}

	if s.TerrainCreate != "" {
		result.TerrainChanges = append(result.TerrainChanges, TerrainChange{
			Pos:  h,
			Type: terrain.ParseTerrainType(s.TerrainCreate),
		})
	}

	if len(s.TerrainInteractions) > 0 {
		name := bf.TerrainAt(h).TypeName()
		for _, ti := range s.TerrainInteractions {
			if ti.On == name {
				result.TerrainChanges = append(result.TerrainChanges, TerrainChange{
					Pos:  h,
					Type: terrain.ParseTerrainType(ti.Result),
				})
				break
			}
		}
	}
}

// applyStatusCombos checks the spell's status combos against the unit's
// current statuses. For each matching combo it records the configured
// status removal/application into result and returns the accumulated
// bonus damage.
func applyStatusCombos(combos []spell.StatusCombo, u *entity.Unit, result *LinkResult) int {
	var bonus int
	for _, combo := range combos {
		existing := entity.ParseStatus(combo.If)
		if existing == entity.StatusNone || !u.HasStatus(existing) {
			continue
		}
		bonus += combo.BonusDamage
		if combo.Remove {
			result.StatusRemoved = append(result.StatusRemoved, StatusChange{
				UnitID: u.ID,
				Status: existing,
			})
		}
		if combo.Apply != "" {
			if applied := entity.ParseStatus(combo.Apply); applied != entity.StatusNone {
				result.StatusApplied = append(result.StatusApplied, StatusChange{
					UnitID: u.ID,
					Status: applied,
					Turns:  combo.ApplyTurns,
				})
			}
		}
	}
	return bonus
}

// applyMovement computes push/pull moves for each target and appends
// valid moves to result.UnitsMoved. Destinations that are blocked, out
// of bounds, occupied by a living unit, or already claimed by an
// earlier move in the same action are silently skipped.
func applyMovement(targets []spell.Target, movement string, casterPos hex.Hex, bf Battlefield, result *LinkResult) {
	claimed := make(map[hex.Hex]bool)
	for _, t := range targets {
		pos := hex.NewHex(t.HexQ, t.HexR)
		u := bf.UnitAt(pos)
		if !u.IsAlive() {
			continue
		}
		var dir int
		switch movement {
		case "push":
			dir = directionRelative(pos, casterPos, true)
		case "pull":
			dir = directionRelative(pos, casterPos, false)
		default:
			continue
		}
		to := pos.Direction(dir)
		if claimed[to] || !canMoveTo(to, bf) {
			continue
		}
		claimed[to] = true
		result.UnitsMoved = append(result.UnitsMoved, UnitMove{
			UnitID: u.ID,
			From:   pos,
			To:     to,
		})
	}
}

// directionRelative returns the hex direction (0-5) from `from` that
// either maximises (away=true) or minimises (away=false) distance from `ref`.
func directionRelative(from, ref hex.Hex, away bool) int {
	bestDir := 0
	bestDist := from.Distance(ref)
	for i := 0; i < 6; i++ {
		d := from.Direction(i).Distance(ref)
		if (away && d > bestDist) || (!away && d < bestDist) {
			bestDist = d
			bestDir = i
		}
	}
	return bestDir
}

// canMoveTo reports whether a unit can legally be placed at h.
// Blocks walls, cliffs, out-of-bounds, and hexes occupied by a living unit.
func canMoveTo(h hex.Hex, bf Battlefield) bool {
	if !bf.GridBounds().InBounds(h) {
		return false
	}
	if !bf.TerrainAt(h).IsPassable() {
		return false
	}
	return !bf.UnitAt(h).IsAlive()
}
