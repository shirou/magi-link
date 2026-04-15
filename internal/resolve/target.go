package resolve

import (
	"github.com/shirou/magi_link/internal/entity"
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/spell"
)

// applyStep transforms the running LinkState by applying one target-type
// spell. Each shape knows which of Origins / Impacts / Field / Waypoints
// it writes to (see README of this package's design).
func applyStep(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield, count int) LinkState {
	switch s.Shape {
	case spell.ShapeSelf:
		return stepSelf(state, s, input, bf, count)
	case spell.ShapeSingle:
		return stepSingle(state, s, input, bf, count)
	case spell.ShapeLine:
		return stepLine(state, s, input, bf, count)
	case spell.ShapeArea:
		return stepArea(state, s, input, bf, count)
	case spell.ShapeRing:
		return stepRing(state, s, input, bf, count)
	case spell.ShapeAdjacent:
		return stepAdjacent(state, s, input, bf, count)
	case spell.ShapeWeakest:
		return stepWeakest(state, bf, count)
	case spell.ShapeAlly:
		return stepAlly(state, s, input, bf, count)
	case spell.ShapeTerrainFilter:
		return stepTerrainFilter(state, s, input, bf)
	case spell.Shape3Way:
		return step3Way(state, input)
	case spell.ShapePierce:
		state.Flags |= flagPierce
		return state
	case spell.ShapeHoming:
		return stepHoming(state, bf)
	case spell.ShapeBounce:
		state.Flags |= flagBounce
		return state
	}
	return state
}

// --- Shape steps ---

func stepSelf(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield, count int) LinkState {
	if degraded(s, count) {
		return state
	}
	state = setOrigins(state, input.CasterPos)
	if u := bf.UnitAt(input.CasterPos); u.IsAlive() {
		state = addImpact(state, u.ID, input.CasterPos)
	}
	return state
}

func stepSingle(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield, count int) LinkState {
	if degraded(s, count) {
		return state
	}
	if input.CasterPos.Distance(input.ClickedHex) > s.Range {
		return state
	}
	if !bf.GridBounds().InBounds(input.ClickedHex) {
		return state
	}
	state = setOrigins(state, input.ClickedHex)
	if u := bf.UnitAt(input.ClickedHex); u.IsAlive() {
		state = addImpact(state, u.ID, input.ClickedHex)
	}
	return state
}

// stepLine walks a line from each origin in the current direction. Without
// pierce, it stops at the first hit unit and records that unit as the sole
// Impact from the walk. With pierce, every unit on the path becomes an
// Impact. Walked hexes (excluding hit hexes) go into Waypoints; the final
// position becomes the new Origin so subsequent steps fire from impact.
func stepLine(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield, count int) LinkState {
	effectiveRange := effectiveValue(s, count, s.Range)
	if effectiveRange <= 0 {
		return state
	}

	pierce := s.Pierce || state.Flags&flagPierce != 0
	bounce := state.Flags&flagBounce != 0
	dir := normalizeDir(input.Direction)

	var newOrigins []hex.Hex
	for _, origin := range state.Origins {
		waypoints, endPos, hits := walkLine(origin, dir, effectiveRange, bf, pierce, bounce)
		for _, h := range waypoints {
			state = addWaypoint(state, h)
		}
		for _, u := range hits {
			state = addImpact(state, u.ID, u.Pos)
		}
		newOrigins = append(newOrigins, endPos)
	}
	if len(newOrigins) > 0 {
		state = setOrigins(state, dedupHexes(newOrigins)...)
	}

	state.Flags &^= flagPierce
	state.Flags &^= flagBounce
	return state
}

func stepArea(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield, count int) LinkState {
	r := effectiveValue(s, count, s.Radius)
	if r < 0 {
		return state
	}
	return fillAreaFromClicked(state, s, input, bf, input.ClickedHex.Area(r))
}

func stepRing(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield, count int) LinkState {
	r := effectiveValue(s, count, s.Radius)
	if r <= 0 {
		return state
	}
	return fillAreaFromClicked(state, s, input, bf, input.ClickedHex.Ring(r))
}

func stepAdjacent(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield, count int) LinkState {
	if degraded(s, count) {
		return state
	}
	return fillAreaFromClicked(state, s, input, bf, input.ClickedHex.Neighbors())
}

// fillAreaFromClicked implements the common area-shape flow: recenter
// Origins on the clicked hex, then record each hex in `hexes` as Field
// and any living unit there as an Impact. Caller-provided hex lists may
// contain out-of-bounds entries; they're filtered here.
func fillAreaFromClicked(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield, hexes []hex.Hex) LinkState {
	if input.CasterPos.Distance(input.ClickedHex) > s.Range {
		return state
	}
	grid := bf.GridBounds()
	if !grid.InBounds(input.ClickedHex) {
		return state
	}
	state = setOrigins(state, input.ClickedHex)
	for _, h := range hexes {
		if !grid.InBounds(h) {
			continue
		}
		state = addField(state, h)
		if u := bf.UnitAt(h); u.IsAlive() {
			state = addImpact(state, u.ID, h)
		}
	}
	return state
}

func stepWeakest(state LinkState, bf Battlefield, count int) LinkState {
	if count > 1 {
		return state
	}
	var weakest *entity.Unit
	for _, u := range livingEnemies(bf) {
		if weakest == nil || u.HP < weakest.HP {
			weakest = u
		}
	}
	if weakest == nil {
		return state
	}
	state = setOrigins(state, weakest.Pos)
	return addImpact(state, weakest.ID, weakest.Pos)
}

func stepAlly(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield, count int) LinkState {
	if degraded(s, count) {
		return state
	}
	if input.CasterPos.Distance(input.ClickedHex) > s.Range {
		return state
	}
	u := bf.UnitAt(input.ClickedHex)
	if !u.IsAlive() || !u.IsPlayer {
		return state
	}
	state = setOrigins(state, input.ClickedHex)
	state = addImpact(state, u.ID, input.ClickedHex)
	return state
}

// stepTerrainFilter adds all matching-terrain hexes within radius of the
// caster to Field. Does not touch Origins or Impacts: callers pair it with
// a terrain action to transform the matched hexes.
func stepTerrainFilter(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield) LinkState {
	if s.Filter == "" {
		return state
	}
	grid := bf.GridBounds()
	for _, h := range input.CasterPos.Area(s.Radius) {
		if !grid.InBounds(h) {
			continue
		}
		if bf.TerrainAt(h).TypeName() == s.Filter {
			state = addField(state, h)
		}
	}
	return state
}

// step3Way expands each origin into three: dir-1, dir, dir+1. The new
// launch points also enter Waypoints so the preview can show the spread.
func step3Way(state LinkState, input LinkInput) LinkState {
	if len(state.Origins) == 0 {
		return state
	}
	dir := normalizeDir(input.Direction)
	offsets := []int{normalizeDir(dir - 1), dir, normalizeDir(dir + 1)}
	var expanded []hex.Hex
	for _, origin := range state.Origins {
		for _, d := range offsets {
			h := origin.Direction(d)
			expanded = append(expanded, h)
			state = addWaypoint(state, h)
		}
	}
	state = setOrigins(state, dedupHexes(expanded)...)
	return state
}

// stepHoming overrides each origin with the nearest living enemy's hex
// and adds that enemy to Impacts. If no enemies exist, origins are unchanged.
func stepHoming(state LinkState, bf Battlefield) LinkState {
	enemies := livingEnemies(bf)
	if len(enemies) == 0 {
		return state
	}
	var newOrigins []hex.Hex
	for _, origin := range state.Origins {
		nearest := nearestEnemy(origin, enemies)
		newOrigins = append(newOrigins, nearest.Pos)
		state = addImpact(state, nearest.ID, nearest.Pos)
	}
	if len(newOrigins) > 0 {
		state = setOrigins(state, dedupHexes(newOrigins)...)
	}
	return state
}

// --- Line walking ---

// walkLine walks up to maxDist hexes from origin in direction dir.
// Returns:
//   - waypoints: hexes the projectile passes through (excludes hit-unit hexes)
//   - endPos: final position (last traversed hex, or origin if blocked at step 0)
//   - hits: units struck along the way (first only, unless pierce)
//
// Non-pierce stops at the first living unit (that unit is the hit, and its
// hex is NOT added to waypoints — it's the "landing" hex, handled by Impacts).
// Pierce continues through units and walls up to maxDist.
// Bounce reflects once off a wall and keeps walking.
func walkLine(origin hex.Hex, dir, maxDist int, bf Battlefield, pierce, bounce bool) ([]hex.Hex, hex.Hex, []*entity.Unit) {
	var waypoints []hex.Hex
	var hits []*entity.Unit
	grid := bf.GridBounds()
	cur := origin
	endPos := origin
	bounced := false

	for i := 0; i < maxDist; i++ {
		next := cur.Direction(dir)
		if !grid.InBounds(next) {
			if bounce && !bounced {
				dir = normalizeDir(dir + 3)
				bounced = true
				continue
			}
			break
		}
		if bf.IsWall(next) && !pierce {
			if bounce && !bounced {
				dir = normalizeDir(dir + 3)
				bounced = true
				continue
			}
			break
		}

		// Occupancy check. A unit on `next` is a hit. Non-pierce stops here
		// with `next` treated as the landing hex (NOT in waypoints). Pierce
		// records the hit and keeps walking, adding the hex to waypoints.
		u := bf.UnitAt(next)
		if u.IsAlive() && next != origin {
			hits = append(hits, u)
			if !pierce {
				endPos = next
				break
			}
		}

		waypoints = append(waypoints, next)
		cur = next
		endPos = next
	}
	return waypoints, endPos, hits
}

// --- Helpers ---

func dedupHexes(hexes []hex.Hex) []hex.Hex {
	seen := make(map[hex.Hex]bool, len(hexes))
	out := make([]hex.Hex, 0, len(hexes))
	for _, h := range hexes {
		if seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	return out
}

func normalizeDir(dir int) int {
	return ((dir % 6) + 6) % 6
}

func livingEnemies(bf Battlefield) []*entity.Unit {
	var out []*entity.Unit
	for _, u := range bf.AllUnits() {
		if u.IsDead || u.IsPlayer {
			continue
		}
		out = append(out, u)
	}
	return out
}

func nearestEnemy(from hex.Hex, enemies []*entity.Unit) *entity.Unit {
	best := enemies[0]
	bestDist := from.Distance(best.Pos)
	for _, u := range enemies[1:] {
		if d := from.Distance(u.Pos); d < bestDist {
			best = u
			bestDist = d
		}
	}
	return best
}

// --- Stacking ---

func degraded(s *spell.SpellDef, count int) bool {
	if count <= 1 {
		return false
	}
	return s.Stacking == "none" || s.Stacking == ""
}

func effectiveValue(s *spell.SpellDef, count, baseValue int) int {
	if count <= 1 {
		return baseValue
	}
	switch s.Stacking {
	case "increment":
		return baseValue + (count-1)*s.StackingValue
	case "none", "":
		return -1
	case "full":
		return baseValue
	}
	return baseValue
}

