package resolve

import (
	"github.com/shirou/magi_link/internal/entity"
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/spell"
	"github.com/shirou/magi_link/internal/terrain"
)

// applyAction consumes the current LinkState and writes damage / heal /
// status / terrain / movement diffs into result. It returns the state
// (possibly mutated by ExplodeRadius expansion and ClearState).
//
//   - ExplodeRadius > 0 expands around each explode center (Origins if
//     explicitly set, else the clicked hex — so bare "fireball" fires
//     at the click instead of the caster's feet), adding to Field and
//     any living units within to Impacts.
//   - Damage / heal / status / combos / movement apply to Impacts only.
//   - TerrainCreate / TerrainInteractions apply to Field only.
//   - After the action, Origins moves to wherever the action "happened"
//     so a following targeting step (e.g. 3way) operates on the hit
//     location, not the caster.
func applyAction(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield, result *LinkResult) LinkState {
	centers := explodeCenters(state, input)
	if s.ExplodeRadius > 0 {
		grid := bf.GridBounds()
		for _, center := range centers {
			for _, h := range center.Area(s.ExplodeRadius) {
				if !grid.InBounds(h) {
					continue
				}
				state = addField(state, h)
				if u := bf.UnitAt(h); u.IsAlive() {
					state = addImpact(state, u.ID, h)
				}
			}
		}
	}

	for _, im := range state.Impacts {
		if u := bf.UnitAt(im.Pos); u.IsAlive() {
			applyUnitEffects(s, u, result)
		}
	}

	for _, h := range state.Field {
		applyHexEffects(s, h, bf, result)
	}

	if s.Movement != "" {
		applyMovement(state.Impacts, s.Movement, input.CasterPos, bf, result)
	}

	if s.ClearState {
		state.Impacts = nil
		state.Field = nil
		state.Waypoints = nil
	}

	return advanceOriginsAfterAction(state, s, centers)
}

// explodeCenters picks where an action's explode fires from. Prefers
// Origins when a target spell has aimed the chain, otherwise falls back
// to the clicked hex so bare action-only chains (e.g. a raw fireball
// inside a `single+area+fireball` where the two targeting spells have
// both failed) never self-detonate on the caster.
func explodeCenters(state LinkState, input LinkInput) []hex.Hex {
	if state.originsSet {
		return state.Origins
	}
	return []hex.Hex{input.ClickedHex}
}

// advanceOriginsAfterAction moves Origins to the hexes the action just
// affected so a following targeting step (e.g. 3way, line) operates
// on the hit location. Explode actions advance to the explode centers;
// non-explode actions advance to their Impact positions. Actions that
// did nothing observable leave Origins untouched.
func advanceOriginsAfterAction(state LinkState, s *spell.SpellDef, centers []hex.Hex) LinkState {
	if s.ClearState {
		return state
	}
	if s.ExplodeRadius > 0 {
		return setOrigins(state, centers...)
	}
	if len(state.Impacts) > 0 {
		positions := make([]hex.Hex, 0, len(state.Impacts))
		for _, im := range state.Impacts {
			positions = append(positions, im.Pos)
		}
		return setOrigins(state, dedupHexes(positions)...)
	}
	return state
}

// applyUnitEffects writes damage / heal / status changes for a single
// living unit into result.
func applyUnitEffects(s *spell.SpellDef, u *entity.Unit, result *LinkResult) {
	if s.Damage > 0 {
		if result.Damage == nil {
			result.Damage = make(map[int]int)
		}
		dmg := s.Damage + applyStatusCombos(s.StatusCombos, u, result)
		result.Damage[u.ID] += dmg
	}
	if s.Heal > 0 {
		if result.Healing == nil {
			result.Healing = make(map[int]int)
		}
		result.Healing[u.ID] += s.Heal
	}
	if s.Status != "" {
		if st := entity.ParseStatus(s.Status); st != entity.StatusNone {
			result.StatusApplied = append(result.StatusApplied, StatusChange{
				UnitID: u.ID,
				Status: st,
				Turns:  s.StatusTurns,
			})
		}
	}
}

// applyHexEffects writes terrain changes for a hex.
func applyHexEffects(s *spell.SpellDef, h hex.Hex, bf Battlefield, result *LinkResult) {
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

// applyStatusCombos returns the bonus damage for combos matching u's
// existing statuses, and records configured removals / applications.
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

// applyMovement pushes / pulls each Impact unit one hex relative to the
// caster. Blocked / occupied / claimed destinations are silently skipped.
func applyMovement(impacts []Impact, movement string, casterPos hex.Hex, bf Battlefield, result *LinkResult) {
	claimed := make(map[hex.Hex]bool)
	for _, im := range impacts {
		u := bf.UnitAt(im.Pos)
		if !u.IsAlive() {
			continue
		}
		var dir int
		switch movement {
		case "push":
			dir = directionRelative(im.Pos, casterPos, true)
		case "pull":
			dir = directionRelative(im.Pos, casterPos, false)
		default:
			continue
		}
		to := im.Pos.Direction(dir)
		if claimed[to] || !canMoveTo(to, bf) {
			continue
		}
		claimed[to] = true
		result.UnitsMoved = append(result.UnitsMoved, UnitMove{
			UnitID: u.ID,
			From:   im.Pos,
			To:     to,
		})
	}
}

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

func canMoveTo(h hex.Hex, bf Battlefield) bool {
	if !bf.GridBounds().InBounds(h) {
		return false
	}
	if !bf.TerrainAt(h).IsPassable() {
		return false
	}
	return !bf.UnitAt(h).IsAlive()
}
