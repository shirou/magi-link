package resolve

import (
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/spell"
	"github.com/shirou/magi_link/internal/terrain"
)

// applyStep transforms the running LinkState by applying one spell step.
// This is the core of the bucket-relay resolution model: every spell
// step reads the current (Origins, Targets, Hexes, Flags) tuple and
// returns a new tuple.
func applyStep(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield, count int) LinkState {
	switch s.Shape {
	case spell.ShapeSelf:
		return stepSelf(state, s, input, count)
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
		return stepWeakest(state, s, bf, count)
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

// stepSelf overrides origins to [caster] and adds caster to targets.
func stepSelf(state LinkState, s *spell.SpellDef, input LinkInput, count int) LinkState {
	if degraded(s, count) {
		return state
	}
	state.Origins = []hex.Hex{input.CasterPos}
	state = recordHexes(state, []hex.Hex{input.CasterPos}, nil)
	return state
}

// stepSingle overrides origins to [clicked] and adds it to targets.
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
	state.Origins = []hex.Hex{input.ClickedHex}
	state = recordHexes(state, []hex.Hex{input.ClickedHex}, bf)
	return state
}

// stepLine walks a line from each origin in the chosen direction.
// Each origin is replaced by the line's end hex so subsequent steps
// fire from the impact point (bucket-relay).
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
		walked, endPos := walkLine(origin, dir, effectiveRange, bf, pierce, bounce)
		state = recordHexes(state, walked, bf)
		newOrigins = append(newOrigins, endPos)
	}
	if len(newOrigins) > 0 {
		state.Origins = dedupHexes(newOrigins)
	}

	// Consume the transient flags after the action.
	state.Flags &^= flagPierce
	state.Flags &^= flagBounce
	return state
}

// stepArea adds each origin's radius-r area to targets; origins unchanged.
func stepArea(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield, count int) LinkState {
	effectiveRadius := effectiveValue(s, count, s.Radius)
	if effectiveRadius < 0 {
		return state
	}
	grid := bf.GridBounds()
	for _, origin := range state.Origins {
		// Range check: origin must be within s.Range of caster.
		if input.CasterPos.Distance(origin) > s.Range {
			continue
		}
		var hexes []hex.Hex
		for _, h := range origin.Area(effectiveRadius) {
			if grid.InBounds(h) {
				hexes = append(hexes, h)
			}
		}
		state = recordHexes(state, hexes, bf)
	}
	return state
}

// stepRing adds each origin's ring at radius r to targets; origins unchanged.
func stepRing(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield, count int) LinkState {
	effectiveRadius := effectiveValue(s, count, s.Radius)
	if effectiveRadius <= 0 {
		return state
	}
	grid := bf.GridBounds()
	for _, origin := range state.Origins {
		if input.CasterPos.Distance(origin) > s.Range {
			continue
		}
		var hexes []hex.Hex
		for _, h := range origin.Ring(effectiveRadius) {
			if grid.InBounds(h) {
				hexes = append(hexes, h)
			}
		}
		state = recordHexes(state, hexes, bf)
	}
	return state
}

// stepAdjacent adds the 6 neighbors of each origin to targets; origins unchanged.
func stepAdjacent(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield, count int) LinkState {
	if degraded(s, count) {
		return state
	}
	grid := bf.GridBounds()
	for _, origin := range state.Origins {
		if input.CasterPos.Distance(origin) > s.Range {
			continue
		}
		var hexes []hex.Hex
		for _, h := range origin.Neighbors() {
			if grid.InBounds(h) {
				hexes = append(hexes, h)
			}
		}
		state = recordHexes(state, hexes, bf)
	}
	return state
}

// stepWeakest overrides origins to [weakest enemy pos].
func stepWeakest(state LinkState, s *spell.SpellDef, bf Battlefield, count int) LinkState {
	if count > 1 {
		return state // stacking: always no-op
	}
	var weakest *hex.Hex
	minHP := int(^uint(0) >> 1)
	for _, u := range bf.AllUnits() {
		if u.IsDead || u.IsPlayer {
			continue
		}
		if u.HP < minHP {
			minHP = u.HP
			h := u.Pos
			weakest = &h
		}
	}
	if weakest == nil {
		return state
	}
	state.Origins = []hex.Hex{*weakest}
	state = recordHexes(state, []hex.Hex{*weakest}, bf)
	return state
}

// stepAlly overrides origins to [clicked ally pos] if valid.
func stepAlly(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield, count int) LinkState {
	if degraded(s, count) {
		return state
	}
	if input.CasterPos.Distance(input.ClickedHex) > s.Range {
		return state
	}
	u := bf.UnitAt(input.ClickedHex)
	if u == nil || u.IsDead || !u.IsPlayer {
		return state
	}
	state.Origins = []hex.Hex{input.ClickedHex}
	state = recordHexes(state, []hex.Hex{input.ClickedHex}, bf)
	return state
}

// stepTerrainFilter adds all matching terrain hexes within radius of caster.
// Does NOT change origins (it's a pure target-set modifier).
func stepTerrainFilter(state LinkState, s *spell.SpellDef, input LinkInput, bf Battlefield) LinkState {
	if s.Filter == "" {
		return state
	}
	grid := bf.GridBounds()
	var hexes []hex.Hex
	for _, h := range input.CasterPos.Area(s.Radius) {
		if !grid.InBounds(h) {
			continue
		}
		t := bf.TerrainAt(h)
		if terrainMatchesFilter(t, s.Filter) {
			hexes = append(hexes, h)
		}
	}
	state = recordHexes(state, hexes, bf)
	return state
}

// step3Way expands each origin into three: one step in dir, dir-1, dir+1.
// Does not add the new hexes to targets — they're launch points for the
// next action step.
func step3Way(state LinkState, input LinkInput) LinkState {
	if len(state.Origins) == 0 {
		return state
	}
	dir := normalizeDir(input.Direction)
	offsets := []int{normalizeDir(dir - 1), dir, normalizeDir(dir + 1)}
	var expanded []hex.Hex
	for _, origin := range state.Origins {
		for _, d := range offsets {
			expanded = append(expanded, origin.Direction(d))
		}
	}
	state.Origins = dedupHexes(expanded)
	return state
}

// stepHoming overrides each origin with the nearest living enemy's hex.
// If no enemies exist, origins are unchanged.
func stepHoming(state LinkState, bf Battlefield) LinkState {
	enemies := livingEnemies(bf)
	if len(enemies) == 0 {
		return state
	}
	var newOrigins []hex.Hex
	for _, origin := range state.Origins {
		nearest := nearestHex(origin, enemies)
		newOrigins = append(newOrigins, nearest)
		state = recordHexes(state, []hex.Hex{nearest}, bf)
	}
	if len(newOrigins) > 0 {
		state.Origins = dedupHexes(newOrigins)
	}
	return state
}

// --- Helpers ---

// walkLine walks `maxDist` hexes in direction `dir` starting from `origin`.
// Returns the hexes walked (excluding origin) and the final end position
// (last valid hex, or origin if nothing was walked).
//
// pierce: ignore walls/units, walk full range.
// bounce: on hitting a wall, reflect direction (dir+3) once and continue.
func walkLine(origin hex.Hex, dir, maxDist int, bf Battlefield, pierce, bounce bool) ([]hex.Hex, hex.Hex) {
	var walked []hex.Hex
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
		wall := bf.IsWall(next)
		if wall && !pierce {
			if bounce && !bounced {
				dir = normalizeDir(dir + 3)
				bounced = true
				continue
			}
			break
		}
		walked = append(walked, next)
		cur = next
		endPos = next
		// Without pierce, stop at first occupied hex — but never stop at
		// the walk's own starting hex (in bounce scenarios the walker can
		// return to its origin, which must be traversed not blocked).
		if !pierce && next != origin {
			if u := bf.UnitAt(next); u != nil && !u.IsDead {
				break
			}
		}
	}
	return walked, endPos
}

// recordHexes appends `newHexes` to state.Hexes and state.Targets
// (deduplicated by hex coord). For each hex, if an undead unit exists
// at that hex the target's UnitID is set to that unit's ID.
func recordHexes(state LinkState, newHexes []hex.Hex, bf Battlefield) LinkState {
	for _, h := range newHexes {
		state.Hexes = appendHexUnique(state.Hexes, h)
		t := spell.Target{UnitID: -1, HexQ: h.Q, HexR: h.R}
		if bf != nil {
			if u := bf.UnitAt(h); u != nil && !u.IsDead {
				t.UnitID = u.ID
			}
		}
		state.Targets = appendTargetUnique(state.Targets, t)
	}
	return state
}

func appendTargetUnique(targets []spell.Target, t spell.Target) []spell.Target {
	for _, existing := range targets {
		if existing.HexQ == t.HexQ && existing.HexR == t.HexR {
			return targets
		}
	}
	return append(targets, t)
}

func appendHexUnique(hexes []hex.Hex, h hex.Hex) []hex.Hex {
	for _, existing := range hexes {
		if existing == h {
			return hexes
		}
	}
	return append(hexes, h)
}

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

func livingEnemies(bf Battlefield) []hex.Hex {
	var out []hex.Hex
	for _, u := range bf.AllUnits() {
		if u.IsDead || u.IsPlayer {
			continue
		}
		out = append(out, u.Pos)
	}
	return out
}

func nearestHex(from hex.Hex, candidates []hex.Hex) hex.Hex {
	best := candidates[0]
	bestDist := from.Distance(best)
	for _, c := range candidates[1:] {
		if d := from.Distance(c); d < bestDist {
			best = c
			bestDist = d
		}
	}
	return best
}

// --- Stacking helpers ---

// degraded returns true if same-type stacking makes this spell have no effect.
func degraded(s *spell.SpellDef, count int) bool {
	if count <= 1 {
		return false
	}
	return s.Stacking == "none" || s.Stacking == ""
}

// effectiveValue computes the effective value (radius/range) after stacking.
// First use: full base value. Subsequent uses: +stacking_value per extra.
func effectiveValue(s *spell.SpellDef, count, baseValue int) int {
	if count <= 1 {
		return baseValue
	}
	switch s.Stacking {
	case "increment":
		return baseValue + (count-1)*s.StackingValue
	case "none", "":
		return -1 // signal: no effect
	case "full":
		return baseValue
	}
	return baseValue
}

// --- Terrain filter helper ---

func terrainMatchesFilter(t *terrain.Terrain, filter string) bool {
	return t.TypeName() == filter
}
