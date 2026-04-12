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
func (m *mockBF) AllUnits() []*entity.Unit               { return m.units }
func (m *mockBF) TerrainAt(h hex.Hex) *terrain.Terrain    { return m.terrMap.Get(h) }
func (m *mockBF) GridBounds() *hex.Grid                   { return m.grid }
func (m *mockBF) IsWall(h hex.Hex) bool                   { return m.terrMap.Get(h).IsWall() }
func (m *mockBF) HasLineOfSight(from, to hex.Hex) bool    { return m.terrMap.HasLineOfSight(from, to) }

// --- Tests ---

func TestResolveSelf(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(2, 4)

	input := LinkInput{
		CasterPos:  caster,
		ClickedHex: hex.NewHex(5, 5), // ignored for self
		Spells:     []*spell.SpellDef{{ID: "self", Type: "target", Shape: "self"}},
	}
	targets, hexes := ResolveTargets(input, bf)

	if len(hexes) != 1 || hexes[0] != caster {
		t.Errorf("self: expected caster hex, got %v", hexes)
	}
	if len(targets) != 1 {
		t.Errorf("self: expected 1 target, got %d", len(targets))
	}
}

func TestResolveSelfStacking(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(2, 4)

	input := LinkInput{
		CasterPos: caster,
		Spells: []*spell.SpellDef{
			{ID: "self", Type: "target", Shape: "self", Stacking: "none"},
			{ID: "self", Type: "target", Shape: "self", Stacking: "none"},
		},
	}
	_, hexes := ResolveTargets(input, bf)

	if len(hexes) != 1 {
		t.Errorf("self+self stacking: expected 1 hex (no duplicate), got %d", len(hexes))
	}
}

func TestResolveSingle(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	target := hex.NewHex(2, 0)

	enemy := entity.NewUnit(10, "Echo", target, 30, 0)
	bf.units = []*entity.Unit{enemy}

	input := LinkInput{
		CasterPos:  caster,
		ClickedHex: target,
		Spells:     []*spell.SpellDef{{ID: "single", Type: "target", Shape: "single", Range: 5}},
	}
	targets, hexes := ResolveTargets(input, bf)

	if len(hexes) != 1 || hexes[0] != target {
		t.Errorf("single: expected target hex, got %v", hexes)
	}
	if len(targets) != 1 || targets[0].UnitID != 10 {
		t.Errorf("single: expected unit 10, got %v", targets)
	}
}

func TestResolveSingleOutOfRange(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	target := hex.NewHex(6, 0) // distance 6, range 5

	input := LinkInput{
		CasterPos:  caster,
		ClickedHex: target,
		Spells:     []*spell.SpellDef{{ID: "single", Type: "target", Shape: "single", Range: 5}},
	}
	_, hexes := ResolveTargets(input, bf)

	if len(hexes) != 0 {
		t.Errorf("single out of range: expected 0 hexes, got %d", len(hexes))
	}
}

func TestResolveArea(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(2, 4)
	center := hex.NewHex(4, 4)

	input := LinkInput{
		CasterPos:  caster,
		ClickedHex: center,
		Spells:     []*spell.SpellDef{{ID: "area", Type: "target", Shape: "area", Range: 4, Radius: 1}},
	}
	_, hexes := ResolveTargets(input, bf)

	// Radius 1 area = center + 6 neighbors = 7 (minus any out of bounds)
	if len(hexes) < 4 {
		t.Errorf("area r=1: expected at least 4 hexes, got %d", len(hexes))
	}
}

func TestResolveAreaStacking(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(5, 5)
	center := hex.NewHex(5, 5)

	area1 := &spell.SpellDef{ID: "area", Type: "target", Shape: "area", Range: 4, Radius: 1,
		Stacking: "increment", StackingField: "radius", StackingValue: 1}

	input1 := LinkInput{CasterPos: caster, ClickedHex: center, Spells: []*spell.SpellDef{area1}}
	_, hexes1 := ResolveTargets(input1, bf)

	// Two areas: second should use radius 2 (1 + 1 increment)
	input2 := LinkInput{CasterPos: caster, ClickedHex: center, Spells: []*spell.SpellDef{area1, area1}}
	_, hexes2 := ResolveTargets(input2, bf)

	if len(hexes2) <= len(hexes1) {
		t.Errorf("area stacking: expected more hexes with 2 areas (%d) than 1 (%d)", len(hexes2), len(hexes1))
	}
}

func TestResolveLine(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(2, 4)

	input := LinkInput{
		CasterPos: caster,
		Direction: 0, // direction 0: +Q
		Spells:    []*spell.SpellDef{{ID: "line", Type: "target", Shape: "line", Range: 4}},
	}
	_, hexes := ResolveTargets(input, bf)

	if len(hexes) == 0 {
		t.Error("line: expected at least 1 hex")
	}
	// All hexes should be in direction 0 from caster
	for _, h := range hexes {
		if h.R != caster.R {
			t.Errorf("line dir 0: hex %v has different R from caster %v", h, caster)
		}
	}
}

func TestResolveLineStopsAtWall(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(2, 4)

	// Place a wall 2 steps in direction 0
	wallHex := caster.Direction(0).Direction(0) // 2 steps
	bf.terrMap.Set(wallHex, &terrain.Terrain{Type: terrain.TerrainRock, Duration: -1})

	input := LinkInput{
		CasterPos: caster,
		Direction: 0,
		Spells:    []*spell.SpellDef{{ID: "line", Type: "target", Shape: "line", Range: 6}},
	}
	_, hexes := ResolveTargets(input, bf)

	// Should have 1 hex (the one before the wall), not 6
	if len(hexes) != 1 {
		t.Errorf("line wall: expected 1 hex before wall, got %d", len(hexes))
	}
}

func TestResolveWeakest(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(2, 4)

	e1 := entity.NewUnit(10, "Echo", hex.NewHex(5, 3), 30, 0)
	e2 := entity.NewUnit(11, "Shard", hex.NewHex(8, 5), 15, 0)
	player := entity.NewUnit(1, "Player", caster, 100, 60)
	player.IsPlayer = true
	bf.units = []*entity.Unit{player, e1, e2}

	input := LinkInput{
		CasterPos: caster,
		Spells:    []*spell.SpellDef{{ID: "weakest", Type: "target", Shape: "weakest"}},
	}
	targets, _ := ResolveTargets(input, bf)

	if len(targets) != 1 {
		t.Fatalf("weakest: expected 1 target, got %d", len(targets))
	}
	if targets[0].UnitID != 11 {
		t.Errorf("weakest: expected unit 11 (HP=15), got unit %d", targets[0].UnitID)
	}
}

func TestResolveWeakestNoEnemies(t *testing.T) {
	bf := newMockBF()
	player := entity.NewUnit(1, "Player", hex.NewHex(2, 4), 100, 60)
	player.IsPlayer = true
	bf.units = []*entity.Unit{player}

	input := LinkInput{
		CasterPos: player.Pos,
		Spells:    []*spell.SpellDef{{ID: "weakest", Type: "target", Shape: "weakest"}},
	}
	targets, _ := ResolveTargets(input, bf)

	if len(targets) != 0 {
		t.Errorf("weakest no enemies: expected 0, got %d", len(targets))
	}
}

func TestResolveTerrainFilter(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(5, 5)

	// Place fire terrain nearby
	bf.terrMap.Set(hex.NewHex(5, 4), &terrain.Terrain{Type: terrain.TerrainFireFloor, Duration: -1})
	bf.terrMap.Set(hex.NewHex(6, 5), &terrain.Terrain{Type: terrain.TerrainFireFloor, Duration: -1})
	bf.terrMap.Set(hex.NewHex(3, 3), &terrain.Terrain{Type: terrain.TerrainPoisonSwamp, Duration: -1}) // not fire

	input := LinkInput{
		CasterPos: caster,
		Spells: []*spell.SpellDef{{
			ID: "fire_filter", Type: "target", Shape: "terrain_filter",
			Filter: "fire_floor", Radius: 5,
		}},
	}
	_, hexes := ResolveTargets(input, bf)

	if len(hexes) != 2 {
		t.Errorf("fire filter: expected 2 fire hexes, got %d", len(hexes))
	}
}

func TestResolveCrossTypeFullEffect(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(5, 5)
	clicked := hex.NewHex(5, 5)

	// area + line (cross-type): both should get full effect
	area := &spell.SpellDef{ID: "area", Type: "target", Shape: "area", Range: 4, Radius: 1,
		Stacking: "increment", StackingField: "radius", StackingValue: 1}
	line := &spell.SpellDef{ID: "line", Type: "target", Shape: "line", Range: 4,
		Stacking: "increment", StackingField: "range", StackingValue: 1}

	input := LinkInput{CasterPos: caster, ClickedHex: clicked, Direction: 0,
		Spells: []*spell.SpellDef{area, line}}
	_, hexes := ResolveTargets(input, bf)

	// Should have area hexes + line hexes (both full effect)
	if len(hexes) < 5 {
		t.Errorf("cross-type: expected at least 5 hexes (area+line), got %d", len(hexes))
	}
}

func TestResolveActionSpellsSkipped(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(2, 4)

	input := LinkInput{
		CasterPos:  caster,
		ClickedHex: hex.NewHex(3, 4),
		Spells: []*spell.SpellDef{
			{ID: "fireball", Type: "action"},
			{ID: "single", Type: "target", Shape: "single", Range: 5},
			{ID: "heal", Type: "action"},
		},
	}
	_, hexes := ResolveTargets(input, bf)

	// Only the single target spell should produce a hex
	if len(hexes) != 1 {
		t.Errorf("action skip: expected 1 hex from single, got %d", len(hexes))
	}
}

func TestResolveEmptyChain(t *testing.T) {
	bf := newMockBF()
	input := LinkInput{CasterPos: hex.NewHex(0, 0)}
	targets, hexes := ResolveTargets(input, bf)

	if len(targets) != 0 || len(hexes) != 0 {
		t.Error("empty chain should produce no targets")
	}
}
