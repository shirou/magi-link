package resolve

// TOML-driven link test cases.
//
// Each file in testdata/*.toml defines one case. TestResolveFromTestdata
// discovers them and runs each as a subtest — no Go code per case.
//
// ============================================================
// Schema
// ============================================================
//
//   name        = "short name"              # required
//   description = "what this verifies"      # optional
//
//   [battlefield]
//   grid_width  = 12
//   grid_height = 10
//
//   [[battlefield.units]]
//   id     = 1
//   name   = "Player"
//   pos    = [2, 4]        # [col, row] offset coords
//   hp     = 100
//   player = true
//
//   [[battlefield.terrain]]
//   pos  = [5, 5]
//   type = "rock"
//
//   [input]
//   caster    = [2, 4]
//   clicked   = [4, 4]
//   direction = 0
//
//   [[input.spells]]
//   ref = "single"             # reference registry spell by ID
//
//   [[input.spells]]
//   id    = "custom_line"      # or an inline definition
//   type  = "target"
//   shape = "line"
//   range = 3
//
//   [expected]
//   # Impacts: unit-level damage / heal / status recipients
//   impact_count     = 2
//   contains_impacts = [10, 11]   # unit IDs
//   excludes_impacts = [1]
//
//   # Field: hex-level "spell area" (terrain + VFX derivation)
//   field_count     = 7
//   field_count_min = 1
//   field_count_max = 19
//   contains_field  = [[4, 4]]
//   excludes_field  = [[0, 0]]
//
//   # Waypoints: projectile trajectory hexes (no effect)
//   waypoint_count     = 3
//   waypoint_count_min = 1
//   waypoint_count_max = 5
//   contains_waypoints = [[3, 4]]
//
// ============================================================

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/shirou/magi_link/internal/entity"
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/spell"
	"github.com/shirou/magi_link/internal/terrain"
)

type testCase struct {
	Name        string          `toml:"name"`
	Description string          `toml:"description"`
	Battlefield battlefieldSpec `toml:"battlefield"`
	Input       inputSpec       `toml:"input"`
	Expected    expectedSpec    `toml:"expected"`
}

type battlefieldSpec struct {
	GridWidth  int           `toml:"grid_width"`
	GridHeight int           `toml:"grid_height"`
	Units      []unitSpec    `toml:"units"`
	Terrain    []terrainSpec `toml:"terrain"`
}

type unitSpec struct {
	ID     int    `toml:"id"`
	Name   string `toml:"name"`
	Pos    []int  `toml:"pos"`
	HP     int    `toml:"hp"`
	Player bool   `toml:"player"`
}

type terrainSpec struct {
	Pos  []int  `toml:"pos"`
	Type string `toml:"type"`
}

type inputSpec struct {
	Caster    []int       `toml:"caster"`
	Clicked   []int       `toml:"clicked"`
	Direction int         `toml:"direction"`
	Spells    []spellSpec `toml:"spells"`
}

type spellSpec struct {
	Ref string `toml:"ref"`

	ID            string            `toml:"id"`
	Type          string            `toml:"type"`
	Shape         spell.TargetShape `toml:"shape"`
	Range         int               `toml:"range"`
	Radius        int               `toml:"radius"`
	Pierce        bool              `toml:"pierce"`
	Filter        string            `toml:"filter"`
	Stacking      string            `toml:"stacking"`
	StackingField string            `toml:"stacking_field"`
	StackingValue int               `toml:"stacking_value"`
	ExplodeRadius int               `toml:"explode_radius"`
	Damage        int               `toml:"damage"`
	Heal          int               `toml:"heal"`
}

type expectedSpec struct {
	ImpactCount     *int    `toml:"impact_count"`
	ImpactCountMin  *int    `toml:"impact_count_min"`
	ImpactCountMax  *int    `toml:"impact_count_max"`
	ContainsImpacts []int   `toml:"contains_impacts"`
	ExcludesImpacts []int   `toml:"excludes_impacts"`

	FieldCount    *int    `toml:"field_count"`
	FieldCountMin *int    `toml:"field_count_min"`
	FieldCountMax *int    `toml:"field_count_max"`
	ContainsField [][]int `toml:"contains_field"`
	ExcludesField [][]int `toml:"excludes_field"`

	WaypointCount    *int    `toml:"waypoint_count"`
	WaypointCountMin *int    `toml:"waypoint_count_min"`
	WaypointCountMax *int    `toml:"waypoint_count_max"`
	ContainsWaypoints [][]int `toml:"contains_waypoints"`
	ExcludesWaypoints [][]int `toml:"excludes_waypoints"`
}

func loadTestCase(path string) (*testCase, error) {
	var tc testCase
	if _, err := toml.DecodeFile(path, &tc); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return &tc, nil
}

func buildTestBattlefield(spec battlefieldSpec) *mockBF {
	w, h := spec.GridWidth, spec.GridHeight
	if w == 0 {
		w = 12
	}
	if h == 0 {
		h = 10
	}
	bf := &mockBF{
		terrMap: terrain.NewMap(w, h),
		grid:    hex.NewGrid(w, h, 30, 0, 0),
	}
	for _, u := range spec.Units {
		if len(u.Pos) != 2 {
			continue
		}
		pos := hex.OffsetToHex(u.Pos[0], u.Pos[1])
		unit := entity.NewUnit(u.ID, u.Name, pos, u.HP, 0)
		unit.IsPlayer = u.Player
		bf.units = append(bf.units, unit)
	}
	for _, t := range spec.Terrain {
		if len(t.Pos) != 2 {
			continue
		}
		pos := hex.OffsetToHex(t.Pos[0], t.Pos[1])
		bf.terrMap.Set(pos, &terrain.Terrain{
			Type:     terrain.ParseTerrainType(t.Type),
			Duration: -1,
		})
	}
	return bf
}

func buildTestSpells(reg *spell.Registry, specs []spellSpec) ([]*spell.SpellDef, error) {
	spells := make([]*spell.SpellDef, 0, len(specs))
	for i, s := range specs {
		if s.Ref != "" {
			def := reg.Get(s.Ref)
			if def == nil {
				return nil, fmt.Errorf("spell[%d]: unknown ref %q", i, s.Ref)
			}
			spells = append(spells, def)
			continue
		}
		spells = append(spells, &spell.SpellDef{
			ID:            s.ID,
			Type:          s.Type,
			Shape:         s.Shape,
			Range:         s.Range,
			Radius:        s.Radius,
			Pierce:        s.Pierce,
			Filter:        s.Filter,
			Stacking:      s.Stacking,
			StackingField: s.StackingField,
			StackingValue: s.StackingValue,
			ExplodeRadius: s.ExplodeRadius,
			Damage:        s.Damage,
			Heal:          s.Heal,
		})
	}
	return spells, nil
}

func runTestdataCase(t *testing.T, reg *spell.Registry, path string) {
	t.Helper()

	tc, err := loadTestCase(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}

	bf := buildTestBattlefield(tc.Battlefield)
	spells, err := buildTestSpells(reg, tc.Input.Spells)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}

	input := LinkInput{
		CasterPos:  offsetPair(tc.Input.Caster),
		ClickedHex: offsetPair(tc.Input.Clicked),
		Direction:  tc.Input.Direction,
		Spells:     spells,
	}

	result := ExecuteLink(input, bf)
	assertExpected(t, tc.Expected, result)
}

func offsetPair(p []int) hex.Hex {
	if len(p) != 2 {
		return hex.NewHex(0, 0)
	}
	return hex.OffsetToHex(p[0], p[1])
}

func assertExpected(t *testing.T, e expectedSpec, result LinkResult) {
	t.Helper()

	// Impact counts + contains/excludes
	checkCount(t, "impact", len(result.Impacts), e.ImpactCount, e.ImpactCountMin, e.ImpactCountMax)
	impactSet := make(map[int]bool, len(result.Impacts))
	for _, im := range result.Impacts {
		impactSet[im.UnitID] = true
	}
	for _, id := range e.ContainsImpacts {
		if !impactSet[id] {
			t.Errorf("expected impact unit ID %d not found", id)
		}
	}
	for _, id := range e.ExcludesImpacts {
		if impactSet[id] {
			t.Errorf("unexpected impact unit ID %d found", id)
		}
	}

	// Field counts + contains/excludes
	checkCount(t, "field", len(result.Field), e.FieldCount, e.FieldCountMin, e.FieldCountMax)
	checkHexes(t, "field", result.Field, e.ContainsField, e.ExcludesField)

	// Waypoints counts + contains/excludes
	checkCount(t, "waypoint", len(result.Waypoints), e.WaypointCount, e.WaypointCountMin, e.WaypointCountMax)
	checkHexes(t, "waypoint", result.Waypoints, e.ContainsWaypoints, e.ExcludesWaypoints)
}

func checkCount(t *testing.T, label string, got int, exact, min, max *int) {
	t.Helper()
	if exact != nil && got != *exact {
		t.Errorf("%s_count = %d, want %d", label, got, *exact)
	}
	if min != nil && got < *min {
		t.Errorf("%s_count = %d, want >= %d", label, got, *min)
	}
	if max != nil && got > *max {
		t.Errorf("%s_count = %d, want <= %d", label, got, *max)
	}
}

func checkHexes(t *testing.T, label string, hexes []hex.Hex, contains, excludes [][]int) {
	t.Helper()
	set := make(map[hex.Hex]bool, len(hexes))
	for _, h := range hexes {
		set[h] = true
	}
	for _, p := range contains {
		if len(p) != 2 {
			t.Fatalf("malformed %s hex entry %v (want [col, row])", label, p)
		}
		h := hex.OffsetToHex(p[0], p[1])
		if !set[h] {
			t.Errorf("expected %s hex [%d, %d] not found", label, p[0], p[1])
		}
	}
	for _, p := range excludes {
		if len(p) != 2 {
			t.Fatalf("malformed %s hex entry %v (want [col, row])", label, p)
		}
		h := hex.OffsetToHex(p[0], p[1])
		if set[h] {
			t.Errorf("unexpected %s hex [%d, %d] found", label, p[0], p[1])
		}
	}
}

// TestResolveFromTestdata discovers all *.toml files in testdata/ and
// runs each as a subtest. New cases can be added by dropping files in
// testdata/ — no Go code changes required.
func TestResolveFromTestdata(t *testing.T) {
	reg, err := spell.LoadEmbedded()
	if err != nil {
		t.Fatalf("load spell registry: %v", err)
	}

	files, err := filepath.Glob("testdata/*.toml")
	if err != nil {
		t.Fatalf("glob testdata: %v", err)
	}
	if len(files) == 0 {
		t.Skip("no testdata files found")
	}

	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".toml")
		t.Run(name, func(t *testing.T) {
			runTestdataCase(t, reg, f)
		})
	}
}
