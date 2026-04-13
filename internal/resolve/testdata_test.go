package resolve

// TOML-driven test case format for link (bucket-relay) resolution.
//
// Each file in testdata/*.toml defines a single test case. The test
// TestResolveFromTestdata discovers all files in testdata/ and runs
// them as subtests. This allows adding new test cases without writing
// any Go code.
//
// ============================================================
// Schema
// ============================================================
//
//   name        = "short test name"          # required
//   description = "what this test verifies"  # optional
//
//   [battlefield]
//   grid_width  = 12
//   grid_height = 10
//
//   [[battlefield.units]]
//   id     = 1           # unit ID (unique per case)
//   name   = "Player"    # display name
//   pos    = [2, 4]      # [col, row] offset coords
//   hp     = 100
//   player = true        # true = player/ally, false = enemy
//
//   [[battlefield.terrain]]
//   pos  = [5, 5]
//   type = "rock"        # terrain type name (see terrain.TypeName)
//
//   [input]
//   caster    = [2, 4]   # caster position in offset coords
//   clicked   = [4, 4]   # clicked hex in offset coords
//   direction = 0        # hex direction 0-5 (only used by line spells)
//
//   # Spells to add to the link. Use `ref` to reference a spell from
//   # the loaded registry (spells.toml), or provide inline fields for
//   # custom edge-case definitions.
//   [[input.spells]]
//   ref = "single"
//
//   [[input.spells]]
//   id    = "custom_line"
//   type  = "target"
//   shape = "line"
//   range = 3
//
//   [expected]
//   hex_count      = 1           # optional: exact count of result hexes
//   hex_count_min  = 3           # optional: minimum (inclusive)
//   hex_count_max  = 10          # optional: maximum (inclusive)
//   target_count   = 2           # optional: exact count of targets
//   contains_hexes = [[4, 4]]    # must contain these offset coords
//   excludes_hexes = [[0, 0]]    # must NOT contain these offset coords
//   contains_units = [10]        # must contain these unit IDs
//   excludes_units = [1]         # must NOT contain these unit IDs
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

// --- TOML schema types ---

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
	// Ref references a spell loaded from spells.toml. If empty,
	// the other fields define an inline spell.
	Ref string `toml:"ref"`

	// Inline spell definition (used when Ref is empty)
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
}

type expectedSpec struct {
	HexCount      *int    `toml:"hex_count"`
	HexCountMin   *int    `toml:"hex_count_min"`
	HexCountMax   *int    `toml:"hex_count_max"`
	TargetCount   *int    `toml:"target_count"`
	ContainsHexes [][]int `toml:"contains_hexes"`
	ExcludesHexes [][]int `toml:"excludes_hexes"`
	ContainsUnits []int   `toml:"contains_units"`
	ExcludesUnits []int   `toml:"excludes_units"`
}

// --- Loader / builder ---

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
		})
	}
	return spells, nil
}

// --- Runner ---

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

	caster := offsetPair(tc.Input.Caster)
	clicked := offsetPair(tc.Input.Clicked)

	input := LinkInput{
		CasterPos:  caster,
		ClickedHex: clicked,
		Direction:  tc.Input.Direction,
		Spells:     spells,
	}

	result := ExecuteLink(input, bf)
	assertExpected(t, tc.Expected, result.Targets, result.Hexes)
}

func offsetPair(p []int) hex.Hex {
	if len(p) != 2 {
		return hex.NewHex(0, 0)
	}
	return hex.OffsetToHex(p[0], p[1])
}

func assertExpected(t *testing.T, e expectedSpec, targets []spell.Target, hexes []hex.Hex) {
	t.Helper()

	if e.HexCount != nil && len(hexes) != *e.HexCount {
		t.Errorf("hex_count = %d, want %d", len(hexes), *e.HexCount)
	}
	if e.HexCountMin != nil && len(hexes) < *e.HexCountMin {
		t.Errorf("hex_count = %d, want >= %d", len(hexes), *e.HexCountMin)
	}
	if e.HexCountMax != nil && len(hexes) > *e.HexCountMax {
		t.Errorf("hex_count = %d, want <= %d", len(hexes), *e.HexCountMax)
	}
	if e.TargetCount != nil && len(targets) != *e.TargetCount {
		t.Errorf("target_count = %d, want %d", len(targets), *e.TargetCount)
	}

	hexSet := make(map[hex.Hex]bool, len(hexes))
	for _, h := range hexes {
		hexSet[h] = true
	}
	for _, ch := range e.ContainsHexes {
		if len(ch) != 2 {
			continue
		}
		h := hex.OffsetToHex(ch[0], ch[1])
		if !hexSet[h] {
			t.Errorf("expected hex [%d, %d] not found in result", ch[0], ch[1])
		}
	}
	for _, eh := range e.ExcludesHexes {
		if len(eh) != 2 {
			continue
		}
		h := hex.OffsetToHex(eh[0], eh[1])
		if hexSet[h] {
			t.Errorf("unexpected hex [%d, %d] found in result", eh[0], eh[1])
		}
	}

	unitSet := make(map[int]bool)
	for _, tg := range targets {
		if tg.UnitID != -1 {
			unitSet[tg.UnitID] = true
		}
	}
	for _, uid := range e.ContainsUnits {
		if !unitSet[uid] {
			t.Errorf("expected unit ID %d not found in targets", uid)
		}
	}
	for _, uid := range e.ExcludesUnits {
		if unitSet[uid] {
			t.Errorf("unexpected unit ID %d found in targets", uid)
		}
	}
}

// --- Entry point: discover & run all testdata files ---

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
