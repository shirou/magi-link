package resolve

import (
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/spell"
	"github.com/shirou/magi_link/internal/terrain"
)

// ResolveTargets processes all target-type spells in the link sequentially,
// building the targets list. Action spells are skipped (handled in Phase 2).
func ResolveTargets(input LinkInput, bf Battlefield) ([]spell.Target, []hex.Hex) {
	var targets []spell.Target
	var allHexes []hex.Hex

	shapeCounts := make(map[spell.TargetShape]int)

	for _, s := range input.Spells {
		if !s.IsTarget() {
			continue
		}

		shapeCounts[s.Shape]++
		count := shapeCounts[s.Shape]

		newHexes := resolveShape(s, input, bf, count)
		allHexes = appendHexesUnique(allHexes, newHexes)

		for _, h := range newHexes {
			t := spell.Target{UnitID: -1, HexQ: h.Q, HexR: h.R}
			if u := bf.UnitAt(h); u != nil && !u.IsDead {
				t.UnitID = u.ID
			}
			targets = appendTargetUnique(targets, t)
		}
	}

	return targets, allHexes
}

func resolveShape(s *spell.SpellDef, input LinkInput, bf Battlefield, sameTypeCount int) []hex.Hex {
	switch s.Shape {
	case spell.ShapeSelf:
		return resolveSelf(s, input.CasterPos, sameTypeCount)
	case spell.ShapeSingle:
		return resolveSingle(s, input.CasterPos, input.ClickedHex, bf, sameTypeCount)
	case spell.ShapeLine:
		return resolveLine(s, input.CasterPos, input.Direction, bf, sameTypeCount)
	case spell.ShapeArea:
		return resolveArea(s, input.CasterPos, input.ClickedHex, bf, sameTypeCount)
	case spell.ShapeRing:
		return resolveRing(s, input.CasterPos, input.ClickedHex, bf, sameTypeCount)
	case spell.ShapeAdjacent:
		return resolveAdjacent(s, input.CasterPos, input.ClickedHex, bf, sameTypeCount)
	case spell.ShapeWeakest:
		return resolveWeakest(input.CasterPos, bf, sameTypeCount)
	case spell.ShapeAlly:
		return resolveAlly(s, input.CasterPos, input.ClickedHex, bf, sameTypeCount)
	case spell.ShapeTerrainFilter:
		return resolveTerrainFilter(s, input.CasterPos, bf)
	}
	return nil
}

// --- Shape resolvers ---

func resolveSelf(s *spell.SpellDef, casterPos hex.Hex, count int) []hex.Hex {
	if degraded(s, count) {
		return nil
	}
	return []hex.Hex{casterPos}
}

func resolveSingle(s *spell.SpellDef, casterPos, clicked hex.Hex, bf Battlefield, count int) []hex.Hex {
	if degraded(s, count) {
		return nil
	}
	if casterPos.Distance(clicked) > s.Range {
		return nil
	}
	if !bf.GridBounds().InBounds(clicked) {
		return nil
	}
	return []hex.Hex{clicked}
}

func resolveLine(s *spell.SpellDef, casterPos hex.Hex, dir int, bf Battlefield, count int) []hex.Hex {
	effectiveRange := effectiveValue(s, count, s.Range)
	if effectiveRange <= 0 {
		return nil
	}
	return lineInDirection(casterPos, dir, effectiveRange, bf)
}

func resolveArea(s *spell.SpellDef, casterPos, clicked hex.Hex, bf Battlefield, count int) []hex.Hex {
	if casterPos.Distance(clicked) > s.Range {
		return nil
	}
	effectiveRadius := effectiveValue(s, count, s.Radius)
	if effectiveRadius < 0 {
		return nil
	}
	grid := bf.GridBounds()
	var hexes []hex.Hex
	for _, h := range clicked.Area(effectiveRadius) {
		if grid.InBounds(h) {
			hexes = append(hexes, h)
		}
	}
	return hexes
}

func resolveRing(s *spell.SpellDef, casterPos, clicked hex.Hex, bf Battlefield, count int) []hex.Hex {
	if casterPos.Distance(clicked) > s.Range {
		return nil
	}
	effectiveRadius := effectiveValue(s, count, s.Radius)
	if effectiveRadius <= 0 {
		return nil
	}
	grid := bf.GridBounds()
	var hexes []hex.Hex
	for _, h := range clicked.Ring(effectiveRadius) {
		if grid.InBounds(h) {
			hexes = append(hexes, h)
		}
	}
	return hexes
}

func resolveAdjacent(s *spell.SpellDef, casterPos, clicked hex.Hex, bf Battlefield, count int) []hex.Hex {
	if degraded(s, count) {
		return nil
	}
	if casterPos.Distance(clicked) > s.Range {
		return nil
	}
	grid := bf.GridBounds()
	var hexes []hex.Hex
	for _, h := range clicked.Neighbors() {
		if grid.InBounds(h) {
			hexes = append(hexes, h)
		}
	}
	return hexes
}

func resolveWeakest(casterPos hex.Hex, bf Battlefield, count int) []hex.Hex {
	if count > 1 {
		return nil // stacking: always none for weakest
	}
	var weakest *hex.Hex
	minHP := int(^uint(0) >> 1) // max int
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
		return nil
	}
	return []hex.Hex{*weakest}
}

func resolveAlly(s *spell.SpellDef, casterPos, clicked hex.Hex, bf Battlefield, count int) []hex.Hex {
	if degraded(s, count) {
		return nil
	}
	if casterPos.Distance(clicked) > s.Range {
		return nil
	}
	u := bf.UnitAt(clicked)
	if u == nil || u.IsDead || !u.IsPlayer {
		return nil
	}
	return []hex.Hex{clicked}
}

func resolveTerrainFilter(s *spell.SpellDef, casterPos hex.Hex, bf Battlefield) []hex.Hex {
	if s.Filter == "" {
		return nil
	}
	grid := bf.GridBounds()
	var hexes []hex.Hex
	for _, h := range casterPos.Area(s.Radius) {
		if !grid.InBounds(h) {
			continue
		}
		t := bf.TerrainAt(h)
		if terrainMatchesFilter(t, s.Filter) {
			hexes = append(hexes, h)
		}
	}
	return hexes
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

// --- Line helper ---

func lineInDirection(origin hex.Hex, dir int, maxDist int, bf Battlefield) []hex.Hex {
	dir = ((dir % 6) + 6) % 6
	var hexes []hex.Hex
	cur := origin
	for i := 0; i < maxDist; i++ {
		cur = cur.Direction(dir)
		if !bf.GridBounds().InBounds(cur) {
			break
		}
		if bf.IsWall(cur) {
			break
		}
		hexes = append(hexes, cur)
	}
	return hexes
}

// --- Terrain filter helper ---

func terrainMatchesFilter(t *terrain.Terrain, filter string) bool {
	return t.TypeName() == filter
}

// --- Dedup helpers ---

func appendTargetUnique(targets []spell.Target, t spell.Target) []spell.Target {
	for _, existing := range targets {
		if existing.HexQ == t.HexQ && existing.HexR == t.HexR {
			return targets
		}
	}
	return append(targets, t)
}

func appendHexesUnique(hexes []hex.Hex, news []hex.Hex) []hex.Hex {
	for _, h := range news {
		found := false
		for _, existing := range hexes {
			if existing == h {
				found = true
				break
			}
		}
		if !found {
			hexes = append(hexes, h)
		}
	}
	return hexes
}
