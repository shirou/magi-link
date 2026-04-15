package resolve

import (
	"testing"

	"github.com/shirou/magi_link/internal/entity"
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/spell"
	"github.com/shirou/magi_link/internal/terrain"
)

// --- mock Battlefield ---

type mockBF struct {
	units   []*entity.Unit
	terrMap *terrain.Map
	grid    *hex.Grid
}

func newMockBF() *mockBF {
	return &mockBF{
		terrMap: terrain.NewMap(12, 10),
		grid:    hex.NewGrid(12, 10, 30, 0, 0),
	}
}

func (m *mockBF) UnitAt(h hex.Hex) *entity.Unit {
	for _, u := range m.units {
		if !u.IsDead && u.Pos == h {
			return u
		}
	}
	return nil
}
func (m *mockBF) AllUnits() []*entity.Unit             { return m.units }
func (m *mockBF) TerrainAt(h hex.Hex) *terrain.Terrain { return m.terrMap.Get(h) }
func (m *mockBF) GridBounds() *hex.Grid                { return m.grid }
func (m *mockBF) IsWall(h hex.Hex) bool                { return m.terrMap.Get(h).IsWall() }
func (m *mockBF) HasLineOfSight(from, to hex.Hex) bool { return m.terrMap.HasLineOfSight(from, to) }

func hexSet(hexes []hex.Hex) map[hex.Hex]bool {
	s := make(map[hex.Hex]bool, len(hexes))
	for _, h := range hexes {
		s[h] = true
	}
	return s
}

func impactUnitIDs(impacts []Impact) map[int]bool {
	s := make(map[int]bool, len(impacts))
	for _, im := range impacts {
		s[im.UnitID] = true
	}
	return s
}

// --- Shape semantics ---

func TestSelfSetsOriginToCaster(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(2, 4)
	player := entity.NewUnit(1, "Player", caster, 100, 0)
	player.IsPlayer = true
	bf.units = []*entity.Unit{player}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: hex.NewHex(5, 5),
		Spells:     []*spell.SpellDef{{ID: "self", Type: "target", Shape: spell.ShapeSelf}},
	}, bf)

	if !impactUnitIDs(result.Impacts)[1] {
		t.Errorf("self: want caster in Impacts, got %v", result.Impacts)
	}
	if len(result.Field) != 0 {
		t.Errorf("self: Field should be empty, got %v", result.Field)
	}
}

func TestSingleHitsClickedUnit(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	target := hex.NewHex(2, 0)
	bf.units = []*entity.Unit{entity.NewUnit(10, "Echo", target, 30, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: target,
		Spells:     []*spell.SpellDef{{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5}},
	}, bf)

	if !impactUnitIDs(result.Impacts)[10] {
		t.Errorf("single: want unit 10 in Impacts, got %v", result.Impacts)
	}
}

func TestSingleOutOfRangeOnlyKeepsSeedImpact(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	target := hex.NewHex(6, 0)
	bf.units = []*entity.Unit{entity.NewUnit(10, "Echo", target, 30, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: target,
		Spells:     []*spell.SpellDef{{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5}},
	}, bf)

	// seedInitialState auto-targets the clicked unit independent of
	// per-spell range. The UI gates clicks to in-range hexes, so this
	// path is only reached by direct resolve callers (tests, AI).
	if len(result.Impacts) != 1 || result.Impacts[0].UnitID != 10 {
		t.Errorf("out-of-range single: want seed impact only, got %v", result.Impacts)
	}
}

func TestLineStopsAtFirstUnit(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	near := hex.NewHex(2, 0)
	far := hex.NewHex(5, 0)
	bf.units = []*entity.Unit{
		entity.NewUnit(10, "Near", near, 30, 0),
		entity.NewUnit(11, "Far", far, 30, 0),
	}

	result := ExecuteLink(LinkInput{
		CasterPos: caster,
		Direction: 0,
		Spells: []*spell.SpellDef{
			{ID: "line", Type: "target", Shape: spell.ShapeLine, Range: 6, Stacking: "full"},
		},
	}, bf)

	ids := impactUnitIDs(result.Impacts)
	if !ids[10] || ids[11] {
		t.Errorf("line stops at near enemy only, got %v", result.Impacts)
	}
}

func TestPierceLineHitsAllUnitsOnPath(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	bf.units = []*entity.Unit{
		entity.NewUnit(10, "A", hex.NewHex(2, 0), 30, 0),
		entity.NewUnit(11, "B", hex.NewHex(4, 0), 30, 0),
	}

	result := ExecuteLink(LinkInput{
		CasterPos: caster,
		Direction: 0,
		Spells: []*spell.SpellDef{
			{ID: "pierce", Type: "target", Shape: spell.ShapePierce},
			{ID: "line", Type: "target", Shape: spell.ShapeLine, Range: 5, Stacking: "full"},
		},
	}, bf)

	ids := impactUnitIDs(result.Impacts)
	if !ids[10] || !ids[11] {
		t.Errorf("pierce line: want both enemies hit, got %v", result.Impacts)
	}
}

func TestLineEndPosBecomesNextOrigin(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	// No units on line — it walks full range.
	result := ExecuteLink(LinkInput{
		CasterPos: caster,
		Direction: 0,
		Spells: []*spell.SpellDef{
			{ID: "line", Type: "target", Shape: spell.ShapeLine, Range: 3, Stacking: "full"},
			{ID: "dmg", Type: "action", Damage: 5, ExplodeRadius: 0},
		},
	}, bf)

	// Waypoints: 3 path hexes. No Impacts, no damage.
	if len(result.Waypoints) != 3 {
		t.Errorf("want 3 waypoints, got %d", len(result.Waypoints))
	}
	if len(result.Impacts) != 0 {
		t.Errorf("no units; want 0 Impacts, got %v", result.Impacts)
	}
}

func TestAreaMovesOriginToClickedAndFillsField(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 4)
	clicked := hex.NewHex(5, 4) // middle of the 12x10 mock grid
	bf.units = []*entity.Unit{entity.NewUnit(10, "E", clicked, 30, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: clicked,
		Spells: []*spell.SpellDef{
			{ID: "area", Type: "target", Shape: spell.ShapeArea, Range: 10, Radius: 1},
		},
	}, bf)

	if len(result.Field) != 7 {
		t.Errorf("area r=1 field: want 7, got %d", len(result.Field))
	}
	if !hexSet(result.Field)[clicked] {
		t.Errorf("area: clicked hex missing from Field")
	}
}

func TestWeakestOverridesOrigin(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(2, 4)
	player := entity.NewUnit(1, "Player", caster, 100, 60)
	player.IsPlayer = true
	bf.units = []*entity.Unit{
		player,
		entity.NewUnit(10, "Echo", hex.NewHex(5, 3), 30, 0),
		entity.NewUnit(11, "Shard", hex.NewHex(8, 5), 15, 0),
	}

	result := ExecuteLink(LinkInput{
		CasterPos: caster,
		Spells:    []*spell.SpellDef{{ID: "weakest", Type: "target", Shape: spell.ShapeWeakest}},
	}, bf)

	if !impactUnitIDs(result.Impacts)[11] {
		t.Errorf("weakest: want unit 11 (HP=15) in Impacts, got %v", result.Impacts)
	}
}

// --- Bucket-relay across steps ---

func TestAreaRecentersOnClickedEvenAfterLine(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	clicked := hex.NewHex(4, 0)

	// line → area. Area recenters Origins to ClickedHex regardless of
	// line's endPos: area's hex ring is around clicked.
	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: clicked,
		Direction:  0,
		Spells: []*spell.SpellDef{
			{ID: "line", Type: "target", Shape: spell.ShapeLine, Range: 2, Stacking: "full"},
			{ID: "area", Type: "target", Shape: spell.ShapeArea, Range: 10, Radius: 1},
		},
	}, bf)

	if !hexSet(result.Field)[clicked] {
		t.Errorf("area recentered on clicked: want clicked in Field")
	}
}

// --- Action spells consume Impacts ---

func TestActionDamageHitsImpact(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	target := hex.NewHex(2, 0)
	bf.units = []*entity.Unit{entity.NewUnit(10, "E", target, 30, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: target,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			{ID: "dmg", Type: "action", Damage: 7},
		},
	}, bf)

	if result.Damage[10] != 7 {
		t.Errorf("want Damage[10]=7, got %v", result.Damage)
	}
}

func TestExplodeCentersOnOrigin(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	clicked := hex.NewHex(5, 0)
	// Enemy at clicked; enemy one hex further. Fireball radius=1 should
	// catch both (center + neighbor in dir 0).
	bf.units = []*entity.Unit{
		entity.NewUnit(10, "Center", clicked, 30, 0),
		entity.NewUnit(11, "Next", clicked.Direction(0), 30, 0),
	}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: clicked,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 10},
			{ID: "fireball", Type: "action", Damage: 3, ExplodeRadius: 1},
		},
	}, bf)

	if result.Damage[10] != 3 || result.Damage[11] != 3 {
		t.Errorf("explode: want both hit for 3, got %v", result.Damage)
	}
}

// --- Modifier flags ---

func TestPierceFlagBypassesWall(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	wall := caster.Direction(0)
	bf.terrMap.Set(wall, &terrain.Terrain{Type: terrain.TerrainRock, Duration: -1})

	plain := ExecuteLink(LinkInput{
		CasterPos: caster,
		Direction: 0,
		Spells: []*spell.SpellDef{
			{ID: "line", Type: "target", Shape: spell.ShapeLine, Range: 4, Stacking: "full"},
		},
	}, bf)
	if len(plain.Waypoints) != 0 {
		t.Errorf("non-pierce: wall stops line, want 0 waypoints, got %d", len(plain.Waypoints))
	}

	pierced := ExecuteLink(LinkInput{
		CasterPos: caster,
		Direction: 0,
		Spells: []*spell.SpellDef{
			{ID: "pierce", Type: "target", Shape: spell.ShapePierce},
			{ID: "line", Type: "target", Shape: spell.ShapeLine, Range: 4, Stacking: "full"},
		},
	}, bf)
	if len(pierced.Waypoints) != 4 {
		t.Errorf("pierce: want 4 waypoints through wall, got %d", len(pierced.Waypoints))
	}
}

// After a non-explode action, Origins should point at the unit the
// action hit, so a following 3way (etc.) fans out from that hit — not
// from the caster. Regression: `ice → 3way` used to 3-way around the
// caster because Origins stayed at the default [caster].
func TestActionAdvancesOriginsToImpact(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	target := hex.NewHex(4, 0)
	bf.units = []*entity.Unit{entity.NewUnit(10, "E", target, 30, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: target,
		Direction:  0,
		Spells: []*spell.SpellDef{
			{ID: "ice", Type: "action", Damage: 2},
			{ID: "3way", Type: "target", Shape: spell.Shape3Way},
		},
	}, bf)

	// 3way should have seeded Waypoints at the 3 hexes adjacent to the
	// ice-hit target, NOT around the caster.
	set := hexSet(result.Waypoints)
	for _, expected := range []hex.Hex{
		target.Direction(0), target.Direction(5), target.Direction(1),
	} {
		if !set[expected] {
			t.Errorf("3way after action: expected waypoint %v near target, got %v", expected, result.Waypoints)
		}
	}
	for _, notExpected := range caster.Neighbors() {
		if set[notExpected] {
			t.Errorf("3way after action: unexpected waypoint %v near caster", notExpected)
		}
	}
}

// A chain of an explode action followed by a targeting spell must fire
// the explode at the clicked hex, not at the caster's feet. Regression:
// `fireball + single` used to self-damage because the fireball fell back
// to Origins = [caster] when no prior target spell had moved Origins.
func TestExplodeWithFollowingTargetDoesNotHitCaster(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	clicked := hex.NewHex(5, 0)
	player := entity.NewUnit(1, "Player", caster, 100, 0)
	player.IsPlayer = true
	enemy := entity.NewUnit(10, "E", clicked, 30, 0)
	bf.units = []*entity.Unit{player, enemy}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: clicked,
		Spells: []*spell.SpellDef{
			{ID: "fireball", Type: "action", Damage: 5, ExplodeRadius: 1},
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 10},
		},
	}, bf)

	if _, hit := result.Damage[player.ID]; hit {
		t.Errorf("caster must not take damage from fireball aimed at clicked hex, got %v", result.Damage)
	}
	if result.Damage[enemy.ID] != 5 {
		t.Errorf("enemy should take fireball damage, got %v", result.Damage)
	}
}

func TestHomingSelectsNearestEnemy(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	bf.units = []*entity.Unit{
		entity.NewUnit(10, "Near", hex.NewHex(3, 0), 30, 0),
		entity.NewUnit(11, "Far", hex.NewHex(8, 0), 30, 0),
	}

	result := ExecuteLink(LinkInput{
		CasterPos: caster,
		Spells:    []*spell.SpellDef{{ID: "homing", Type: "target", Shape: spell.ShapeHoming}},
	}, bf)

	ids := impactUnitIDs(result.Impacts)
	if !ids[10] || ids[11] {
		t.Errorf("homing: want near only, got %v", result.Impacts)
	}
}

// --- Stacking ---

func TestAreaStackingIncreasesField(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(5, 5)

	area := &spell.SpellDef{
		ID: "area", Type: "target", Shape: spell.ShapeArea,
		Range: 10, Radius: 1,
		Stacking: "increment", StackingField: "radius", StackingValue: 1,
	}

	r1 := ExecuteLink(LinkInput{CasterPos: caster, ClickedHex: caster,
		Spells: []*spell.SpellDef{area}}, bf)
	r2 := ExecuteLink(LinkInput{CasterPos: caster, ClickedHex: caster,
		Spells: []*spell.SpellDef{area, area}}, bf)

	if len(r2.Field) <= len(r1.Field) {
		t.Errorf("stacking: want more Field with 2 stacks (%d) than 1 (%d)",
			len(r2.Field), len(r1.Field))
	}
}

// --- Empty / malformed chains ---

func TestEmptyChain(t *testing.T) {
	bf := newMockBF()
	result := ExecuteLink(LinkInput{CasterPos: hex.NewHex(0, 0)}, bf)

	if len(result.Impacts) != 0 || len(result.Field) != 0 || len(result.Waypoints) != 0 {
		t.Errorf("empty chain: want empty result, got %+v", result)
	}
}

func TestTerrainFilterCollectsMatchingHexes(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(5, 5)

	bf.terrMap.Set(hex.NewHex(5, 4), &terrain.Terrain{Type: terrain.TerrainFireFloor, Duration: -1})
	bf.terrMap.Set(hex.NewHex(6, 5), &terrain.Terrain{Type: terrain.TerrainFireFloor, Duration: -1})
	bf.terrMap.Set(hex.NewHex(3, 3), &terrain.Terrain{Type: terrain.TerrainPoisonSwamp, Duration: -1})

	result := ExecuteLink(LinkInput{
		CasterPos: caster,
		Spells: []*spell.SpellDef{{
			ID: "fire_filter", Type: "target", Shape: spell.ShapeTerrainFilter,
			Filter: "fire_floor", Radius: 5,
		}},
	}, bf)

	if len(result.Field) != 2 {
		t.Errorf("terrain_filter: want 2 Field hexes, got %d", len(result.Field))
	}
}
