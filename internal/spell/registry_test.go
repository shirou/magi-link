package spell

import (
	"testing"
)

func TestLoadEmbedded(t *testing.T) {
	reg, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded failed: %v", err)
	}

	if reg.Count() != 22 {
		t.Errorf("expected 22 spells, got %d", reg.Count())
		for _, s := range reg.All() {
			t.Logf("  %s (%s)", s.ID, s.Type)
		}
	}
}

func TestTargetSpells(t *testing.T) {
	reg, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded failed: %v", err)
	}

	targets := reg.Targets()
	if len(targets) != 12 {
		t.Errorf("expected 12 target spells, got %d", len(targets))
	}

	expectedTargets := map[string]TargetShape{
		"self":            ShapeSelf,
		"single":          ShapeSingle,
		"line":            ShapeLine,
		"area":            ShapeArea,
		"ring":            ShapeRing,
		"adjacent":        ShapeAdjacent,
		"weakest":         ShapeWeakest,
		"ally":            ShapeAlly,
		"fire_filter":     ShapeTerrainFilter,
		"poison_filter":   ShapeTerrainFilter,
		"electric_filter": ShapeTerrainFilter,
		"thorns_filter":   ShapeTerrainFilter,
	}
	for _, s := range targets {
		expected, ok := expectedTargets[s.ID]
		if !ok {
			t.Errorf("unexpected target spell: %s", s.ID)
			continue
		}
		if s.Shape != expected {
			t.Errorf("spell %s: shape = %q, want %q", s.ID, s.Shape, expected)
		}
	}
}

func TestActionSpells(t *testing.T) {
	reg, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded failed: %v", err)
	}

	actions := reg.Actions()
	if len(actions) != 10 {
		t.Errorf("expected 10 action spells, got %d", len(actions))
	}
}

func TestSpellFields(t *testing.T) {
	reg, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded failed: %v", err)
	}

	tests := []struct {
		id          string
		spellType   string
		costType    string
		baseCost    int
		damage      int
		heal        int
		element     Element
		status      string
		statusTurns int
	}{
		{"fireball", "action", "exponential", 3, 3, 0, ElementFire, "burning", 2},
		{"ice", "action", "exponential", 3, 2, 0, ElementIce, "frozen", 1},
		{"lightning", "action", "exponential", 4, 4, 0, ElementLightning, "electrified", 1},
		{"water", "action", "additive", 2, 1, 0, ElementWater, "wet", 2},
		{"poison", "action", "additive", 2, 1, 0, ElementPoison, "poisoned", 3},
		{"slash", "action", "exponential", 2, 3, 0, ElementPhysical, "bleeding", 2},
		{"heal", "action", "exponential", 3, 0, 3, ElementNone, "", 0},
		{"self", "target", "additive", 0, 0, 0, ElementNone, "", 0},
		{"single", "target", "additive", 1, 0, 0, ElementNone, "", 0},
	}

	for _, tt := range tests {
		s := reg.Get(tt.id)
		if s == nil {
			t.Errorf("spell %q not found", tt.id)
			continue
		}
		if s.Type != tt.spellType {
			t.Errorf("%s: type = %q, want %q", tt.id, s.Type, tt.spellType)
		}
		if s.CostType != tt.costType {
			t.Errorf("%s: cost_type = %q, want %q", tt.id, s.CostType, tt.costType)
		}
		if s.BaseCost != tt.baseCost {
			t.Errorf("%s: base_cost = %d, want %d", tt.id, s.BaseCost, tt.baseCost)
		}
		if s.Damage != tt.damage {
			t.Errorf("%s: damage = %d, want %d", tt.id, s.Damage, tt.damage)
		}
		if s.Heal != tt.heal {
			t.Errorf("%s: heal = %d, want %d", tt.id, s.Heal, tt.heal)
		}
		if s.Element != tt.element {
			t.Errorf("%s: element = %q, want %q", tt.id, s.Element, tt.element)
		}
		if s.Status != tt.status {
			t.Errorf("%s: status = %q, want %q", tt.id, s.Status, tt.status)
		}
		if s.StatusTurns != tt.statusTurns {
			t.Errorf("%s: status_turns = %d, want %d", tt.id, s.StatusTurns, tt.statusTurns)
		}
	}
}

func TestTerrainInteractions(t *testing.T) {
	reg, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded failed: %v", err)
	}

	fb := reg.Get("fireball")
	if fb == nil {
		t.Fatal("fireball not found")
	}
	if len(fb.TerrainInteractions) != 2 {
		t.Fatalf("fireball: expected 2 terrain interactions, got %d", len(fb.TerrainInteractions))
	}

	ice := reg.Get("ice")
	if ice == nil {
		t.Fatal("ice not found")
	}
	if len(ice.TerrainInteractions) != 1 {
		t.Fatalf("ice: expected 1 terrain interaction, got %d", len(ice.TerrainInteractions))
	}
	if ice.TerrainInteractions[0].On != "lava" || ice.TerrainInteractions[0].Result != "plain" {
		t.Errorf("ice terrain interaction: got on=%q result=%q", ice.TerrainInteractions[0].On, ice.TerrainInteractions[0].Result)
	}
}

func TestStatusCombos(t *testing.T) {
	reg, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded failed: %v", err)
	}

	lightning := reg.Get("lightning")
	if lightning == nil {
		t.Fatal("lightning not found")
	}
	if len(lightning.StatusCombos) != 1 {
		t.Fatalf("lightning: expected 1 status combo, got %d", len(lightning.StatusCombos))
	}
	combo := lightning.StatusCombos[0]
	if combo.If != "wet" {
		t.Errorf("lightning combo: if = %q, want %q", combo.If, "wet")
	}
	if combo.BonusDamage != 4 {
		t.Errorf("lightning combo: bonus_damage = %d, want %d", combo.BonusDamage, 4)
	}
	if !combo.Remove {
		t.Error("lightning combo: remove should be true")
	}
}

func TestChainTotalCost(t *testing.T) {
	reg, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded failed: %v", err)
	}

	tests := []struct {
		name     string
		spellIDs []string
		want     int
	}{
		{
			"single + fireball",
			[]string{"single", "fireball"},
			1 + 3, // 4
		},
		{
			"self + heal",
			[]string{"self", "heal"},
			0 + 3, // 3
		},
		{
			"area x2 (exponential)",
			[]string{"area", "area"},
			3 + 6, // 9
		},
		{
			"line x3 (additive)",
			[]string{"line", "line", "line"},
			2 + 3 + 4, // 9
		},
		{
			"area + water + area + lightning",
			[]string{"area", "water", "area", "lightning"},
			3 + 2 + 6 + 4, // 15
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := &Chain{}
			for _, id := range tt.spellIDs {
				s := reg.Get(id)
				if s == nil {
					t.Fatalf("spell %q not found", id)
				}
				chain.Slots = append(chain.Slots, &SpellSlot{Spell: s})
			}
			got := chain.TotalCost()
			if got != tt.want {
				t.Errorf("TotalCost = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestUtilitySpells(t *testing.T) {
	reg, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded failed: %v", err)
	}

	push := reg.Get("push")
	if push == nil {
		t.Fatal("push not found")
	}
	if push.Movement != "push" {
		t.Errorf("push: movement = %q, want %q", push.Movement, "push")
	}

	pull := reg.Get("pull")
	if pull == nil {
		t.Fatal("pull not found")
	}
	if pull.Movement != "pull" {
		t.Errorf("pull: movement = %q, want %q", pull.Movement, "pull")
	}

	wall := reg.Get("wall")
	if wall == nil {
		t.Fatal("wall not found")
	}
	if wall.TerrainCreate != "generated_wall" {
		t.Errorf("wall: terrain_create = %q, want %q", wall.TerrainCreate, "generated_wall")
	}
	if !wall.ClearTargets {
		t.Error("wall: clear_targets should be true")
	}
}

// --- Locale tests ---

func TestDefaultLocaleJapanese(t *testing.T) {
	reg, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded failed: %v", err)
	}

	tests := []struct {
		id   string
		name string
	}{
		{"fireball", "ファイアボール"},
		{"ice", "アイスランス"},
		{"self", "セルフ"},
		{"heal", "ヒール"},
	}

	for _, tt := range tests {
		text := reg.Text(tt.id)
		if text.Name != tt.name {
			t.Errorf("ja text %s: name = %q, want %q", tt.id, text.Name, tt.name)
		}
		if text.Description == "" {
			t.Errorf("ja text %s: description is empty", tt.id)
		}
	}
}

func TestSwitchLocaleEnglish(t *testing.T) {
	reg, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded failed: %v", err)
	}

	if err := reg.SetLocale("en"); err != nil {
		t.Fatalf("SetLocale(en) failed: %v", err)
	}

	tests := []struct {
		id   string
		name string
	}{
		{"fireball", "Fireball"},
		{"ice", "Ice Lance"},
		{"self", "Self"},
		{"heal", "Heal"},
	}

	for _, tt := range tests {
		text := reg.Text(tt.id)
		if text.Name != tt.name {
			t.Errorf("en text %s: name = %q, want %q", tt.id, text.Name, tt.name)
		}
		if text.Description == "" {
			t.Errorf("en text %s: description is empty", tt.id)
		}
	}
}

func TestLocaleFallback(t *testing.T) {
	reg, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded failed: %v", err)
	}

	if err := reg.SetLocale("en"); err != nil {
		t.Fatalf("SetLocale(en) failed: %v", err)
	}

	// "nonexistent" is not in any locale -> should fallback to ID
	text := reg.Text("nonexistent")
	if text.Name != "nonexistent" {
		t.Errorf("fallback: name = %q, want %q", text.Name, "nonexistent")
	}
}

func TestExponentialCostOverflowProtection(t *testing.T) {
	s := &SpellDef{
		ID:       "test",
		BaseCost: 3,
		CostType: "exponential",
	}

	// 40th use: 3 * 2^39 would overflow int32 without protection
	cost := SlotCost(s, 40)
	if cost <= 0 {
		t.Errorf("SlotCost for n=40 should be positive (capped), got %d", cost)
	}
	if cost > maxManaCost {
		t.Errorf("SlotCost for n=40 should be <= maxManaCost, got %d", cost)
	}
}

func TestChainMaxSlots(t *testing.T) {
	chain := &Chain{}
	s := &SpellDef{ID: "test", BaseCost: 0, CostType: "additive"}

	for i := 0; i < ChainMaxSlots; i++ {
		if !chain.CanAdd() {
			t.Fatalf("CanAdd() should be true at slot %d", i)
		}
		chain.Slots = append(chain.Slots, &SpellSlot{Spell: s})
	}

	if chain.CanAdd() {
		t.Error("CanAdd() should be false at max capacity")
	}
}

func TestSlotCostsMatchTotalCost(t *testing.T) {
	reg, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded failed: %v", err)
	}

	chain := &Chain{}
	for _, id := range []string{"area", "fireball", "area", "fireball", "fireball"} {
		chain.Slots = append(chain.Slots, &SpellSlot{Spell: reg.Get(id)})
	}

	costs := chain.SlotCosts()
	sum := 0
	for _, c := range costs {
		sum += c
	}
	if sum != chain.TotalCost() {
		t.Errorf("sum of SlotCosts (%d) != TotalCost (%d)", sum, chain.TotalCost())
	}
}

func TestLocaleInvalidLang(t *testing.T) {
	_, err := LoadLocale("../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal in lang, got nil")
	}

	_, err = LoadLocale("")
	if err == nil {
		t.Error("expected error for empty lang, got nil")
	}
}

func TestAllSpellsHaveLocaleText(t *testing.T) {
	reg, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded failed: %v", err)
	}

	for _, lang := range []string{"ja", "en"} {
		if err := reg.SetLocale(lang); err != nil {
			t.Fatalf("SetLocale(%s) failed: %v", lang, err)
		}
		for _, s := range reg.All() {
			text := reg.Text(s.ID)
			if text.Name == s.ID {
				t.Errorf("[%s] spell %q: missing localized name (got ID as fallback)", lang, s.ID)
			}
			if text.Description == "" {
				t.Errorf("[%s] spell %q: missing localized description", lang, s.ID)
			}
		}
	}
}
