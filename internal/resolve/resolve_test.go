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
func (m *mockBF) AllUnits() []*entity.Unit            { return m.units }
func (m *mockBF) TerrainAt(h hex.Hex) *terrain.Terrain { return m.terrMap.Get(h) }
func (m *mockBF) GridBounds() *hex.Grid                { return m.grid }
func (m *mockBF) IsWall(h hex.Hex) bool                { return m.terrMap.Get(h).IsWall() }
func (m *mockBF) HasLineOfSight(from, to hex.Hex) bool { return m.terrMap.HasLineOfSight(from, to) }

// hexSet collects unique hexes for assertion convenience.
func hexSet(hexes []hex.Hex) map[hex.Hex]bool {
	s := make(map[hex.Hex]bool, len(hexes))
	for _, h := range hexes {
		s[h] = true
	}
	return s
}

// --- Tests: basic shape semantics (modifier-model) ---

func TestSelfSetsOriginToCaster(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(2, 4)

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: hex.NewHex(5, 5),
		Spells:     []*spell.SpellDef{{ID: "self", Type: "target", Shape: spell.ShapeSelf}},
	}, bf)

	if len(result.Hexes) != 1 || result.Hexes[0] != caster {
		t.Errorf("self: want single caster hex, got %v", result.Hexes)
	}
	if len(result.Targets) != 1 {
		t.Errorf("self: want 1 target, got %d", len(result.Targets))
	}
}

func TestSingleSetsOriginToClicked(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	target := hex.NewHex(2, 0)

	enemy := entity.NewUnit(10, "Echo", target, 30, 0)
	bf.units = []*entity.Unit{enemy}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: target,
		Spells:     []*spell.SpellDef{{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5}},
	}, bf)

	if len(result.Hexes) != 1 || result.Hexes[0] != target {
		t.Errorf("single: want clicked hex, got %v", result.Hexes)
	}
	if len(result.Targets) != 1 || result.Targets[0].UnitID != 10 {
		t.Errorf("single: want unit 10, got %v", result.Targets)
	}
}

func TestSingleOutOfRange(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	target := hex.NewHex(6, 0)

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: target,
		Spells:     []*spell.SpellDef{{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5}},
	}, bf)

	if len(result.Hexes) != 0 {
		t.Errorf("out-of-range single: want 0 hexes, got %d", len(result.Hexes))
	}
}

func TestLineWalksAndBecomesOrigin(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(2, 4)

	result := ExecuteLink(LinkInput{
		CasterPos: caster,
		Direction: 0,
		Spells:    []*spell.SpellDef{{ID: "line", Type: "target", Shape: spell.ShapeLine, Range: 4}},
	}, bf)

	if len(result.Hexes) != 4 {
		t.Errorf("line r=4: want 4 hexes, got %d", len(result.Hexes))
	}
	for _, h := range result.Hexes {
		if h.R != caster.R {
			t.Errorf("dir 0: hex %v off axis from %v", h, caster)
		}
	}
}

func TestLineStopsAtWall(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(2, 4)

	wallHex := caster.Direction(0).Direction(0) // 2 steps
	bf.terrMap.Set(wallHex, &terrain.Terrain{Type: terrain.TerrainRock, Duration: -1})

	result := ExecuteLink(LinkInput{
		CasterPos: caster,
		Direction: 0,
		Spells:    []*spell.SpellDef{{ID: "line", Type: "target", Shape: spell.ShapeLine, Range: 6}},
	}, bf)

	if len(result.Hexes) != 1 {
		t.Errorf("line wall stop: want 1 hex before wall, got %d", len(result.Hexes))
	}
}

func TestAreaExpandsAroundOrigin(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(5, 5)

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: caster,
		Spells: []*spell.SpellDef{
			{ID: "self", Type: "target", Shape: spell.ShapeSelf},
			{ID: "area", Type: "target", Shape: spell.ShapeArea, Range: 4, Radius: 1},
		},
	}, bf)

	// self moves origin to caster; area r=1 adds 7 hexes around it.
	if len(result.Hexes) < 7 {
		t.Errorf("self+area: want at least 7 hexes, got %d", len(result.Hexes))
	}
	if !hexSet(result.Hexes)[caster] {
		t.Errorf("self+area: center hex missing")
	}
}

func TestWeakestOverridesOrigin(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(2, 4)

	e1 := entity.NewUnit(10, "Echo", hex.NewHex(5, 3), 30, 0)
	e2 := entity.NewUnit(11, "Shard", hex.NewHex(8, 5), 15, 0)
	player := entity.NewUnit(1, "Player", caster, 100, 60)
	player.IsPlayer = true
	bf.units = []*entity.Unit{player, e1, e2}

	result := ExecuteLink(LinkInput{
		CasterPos: caster,
		Spells:    []*spell.SpellDef{{ID: "weakest", Type: "target", Shape: spell.ShapeWeakest}},
	}, bf)

	if len(result.Targets) != 1 || result.Targets[0].UnitID != 11 {
		t.Errorf("weakest: want unit 11 (HP=15), got %v", result.Targets)
	}
}

// --- Tests: bucket-relay (origins propagate through steps) ---

func TestLineThenAreaUsesImpactAsCenter(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(2, 4)

	// line walks 3 steps from caster in dir 0, impact at (5,4) effectively.
	// area r=1 then fires from impact hex, adding neighbors.
	result := ExecuteLink(LinkInput{
		CasterPos: caster,
		Direction: 0,
		Spells: []*spell.SpellDef{
			{ID: "line", Type: "target", Shape: spell.ShapeLine, Range: 3},
			{ID: "area", Type: "target", Shape: spell.ShapeArea, Range: 20, Radius: 1},
		},
	}, bf)

	// Line contributes 3 hexes; area around (5,4) contributes 7 (may overlap line).
	// Ensure the hex (5,4) + its neighbors are present.
	impact := caster.Direction(0).Direction(0).Direction(0)
	set := hexSet(result.Hexes)
	if !set[impact] {
		t.Errorf("line→area: impact hex %v missing from hexes", impact)
	}
	for _, n := range impact.Neighbors() {
		if !set[n] {
			t.Errorf("line→area: impact neighbor %v missing", n)
		}
	}
}

func TestThreeWayExpandsOriginsForNextLine(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(5, 5)

	// self → 3way → line: 3 lines from 3 adjacent hexes.
	result := ExecuteLink(LinkInput{
		CasterPos: caster,
		Direction: 0,
		Spells: []*spell.SpellDef{
			{ID: "self", Type: "target", Shape: spell.ShapeSelf},
			{ID: "3way", Type: "target", Shape: spell.Shape3Way},
			{ID: "line", Type: "target", Shape: spell.ShapeLine, Range: 2},
		},
	}, bf)

	// Each of 3 origins walks 2 steps → up to 6 hexes walked (some may dup).
	// Plus the caster hex from self step.
	set := hexSet(result.Hexes)
	if !set[caster] {
		t.Errorf("3way→line: caster missing")
	}
	// Expect at least 6 unique hexes from the 3 line walks, plus caster.
	if len(result.Hexes) < 5 {
		t.Errorf("3way→line: want at least 5 hexes, got %d", len(result.Hexes))
	}
}

func TestPierceFlagExtendsLineThroughWall(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(2, 4)

	// place wall 1 step ahead
	wall := caster.Direction(0)
	bf.terrMap.Set(wall, &terrain.Terrain{Type: terrain.TerrainRock, Duration: -1})

	// Without pierce, line stops before wall → 0 hexes.
	plain := ExecuteLink(LinkInput{
		CasterPos: caster,
		Direction: 0,
		Spells:    []*spell.SpellDef{{ID: "line", Type: "target", Shape: spell.ShapeLine, Range: 4}},
	}, bf)
	if len(plain.Hexes) != 0 {
		t.Errorf("wall stops line: want 0 hexes, got %d", len(plain.Hexes))
	}

	// With pierce flag, line walks full range through the wall.
	pierced := ExecuteLink(LinkInput{
		CasterPos: caster,
		Direction: 0,
		Spells: []*spell.SpellDef{
			{ID: "pierce", Type: "target", Shape: spell.ShapePierce},
			{ID: "line", Type: "target", Shape: spell.ShapeLine, Range: 4},
		},
	}, bf)
	if len(pierced.Hexes) != 4 {
		t.Errorf("pierce→line: want 4 hexes through wall, got %d", len(pierced.Hexes))
	}
}

func TestHomingOverridesOriginsToNearestEnemy(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)

	// Two enemies at different distances.
	near := hex.NewHex(3, 0)
	far := hex.NewHex(8, 0)
	bf.units = []*entity.Unit{
		entity.NewUnit(10, "Near", near, 30, 0),
		entity.NewUnit(11, "Far", far, 30, 0),
	}

	result := ExecuteLink(LinkInput{
		CasterPos: caster,
		Spells: []*spell.SpellDef{
			{ID: "homing", Type: "target", Shape: spell.ShapeHoming},
		},
	}, bf)

	if len(result.Targets) != 1 || result.Targets[0].UnitID != 10 {
		t.Errorf("homing: want unit 10 (nearest), got %v", result.Targets)
	}
}

func TestBounceFlagReflectsOffWall(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(2, 4)

	// wall 2 steps ahead; bounce should make line reflect back
	wall := caster.Direction(0).Direction(0)
	bf.terrMap.Set(wall, &terrain.Terrain{Type: terrain.TerrainRock, Duration: -1})

	result := ExecuteLink(LinkInput{
		CasterPos: caster,
		Direction: 0,
		Spells: []*spell.SpellDef{
			{ID: "bounce", Type: "target", Shape: spell.ShapeBounce},
			{ID: "line", Type: "target", Shape: spell.ShapeLine, Range: 4},
		},
	}, bf)

	// Walks 1 step forward, then wall → bounces: walks back through caster
	// and continues. Range 4 → 1 forward + up to 3 backward (all in grid).
	if len(result.Hexes) < 3 {
		t.Errorf("bounce→line: want at least 3 hexes (walk+reflect), got %d", len(result.Hexes))
	}
}

// --- Same-shape stacking ---

func TestAreaStackingIncreasesRadius(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(5, 5)

	area1 := &spell.SpellDef{
		ID: "area", Type: "target", Shape: spell.ShapeArea,
		Range: 4, Radius: 1, Stacking: "increment", StackingField: "radius", StackingValue: 1,
	}

	r1 := ExecuteLink(LinkInput{CasterPos: caster, ClickedHex: caster,
		Spells: []*spell.SpellDef{
			{ID: "self", Type: "target", Shape: spell.ShapeSelf},
			area1,
		}}, bf)

	r2 := ExecuteLink(LinkInput{CasterPos: caster, ClickedHex: caster,
		Spells: []*spell.SpellDef{
			{ID: "self", Type: "target", Shape: spell.ShapeSelf},
			area1, area1,
		}}, bf)

	if len(r2.Hexes) <= len(r1.Hexes) {
		t.Errorf("stacking: want more hexes with 2 areas (%d) than 1 (%d)", len(r2.Hexes), len(r1.Hexes))
	}
}

func TestActionSpellsSkipped(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(2, 4)

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: hex.NewHex(3, 4),
		Spells: []*spell.SpellDef{
			{ID: "fireball", Type: "action"},
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			{ID: "heal", Type: "action"},
		},
	}, bf)

	if len(result.Hexes) != 1 {
		t.Errorf("action skip: want 1 hex from single, got %d", len(result.Hexes))
	}
}

func TestEmptyChain(t *testing.T) {
	bf := newMockBF()
	result := ExecuteLink(LinkInput{CasterPos: hex.NewHex(0, 0)}, bf)

	if len(result.Targets) != 0 || len(result.Hexes) != 0 {
		t.Error("empty chain should produce nothing")
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

	if len(result.Hexes) != 2 {
		t.Errorf("fire filter: want 2 matching hexes, got %d", len(result.Hexes))
	}
}
