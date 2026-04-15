package resolve

import (
	"testing"

	"github.com/shirou/magi_link/internal/entity"
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/spell"
	"github.com/shirou/magi_link/internal/terrain"
)

// --- Damage / Heal ---

func TestActionDamageHitsSingleTarget(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	target := hex.NewHex(2, 0)
	bf.units = []*entity.Unit{entity.NewUnit(10, "E", target, 30, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: target,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			{ID: "fireball", Type: "action", Damage: 5, Element: spell.ElementFire},
		},
	}, bf)

	if result.Damage[10] != 5 {
		t.Errorf("want Damage[10]=5, got %v", result.Damage)
	}
}

func TestActionHealHitsCaster(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	player := entity.NewUnit(1, "Player", caster, 100, 0)
	player.IsPlayer = true
	player.HP = 50
	bf.units = []*entity.Unit{player}

	result := ExecuteLink(LinkInput{
		CasterPos: caster,
		Spells: []*spell.SpellDef{
			{ID: "self", Type: "target", Shape: spell.ShapeSelf},
			{ID: "heal", Type: "action", Heal: 20},
		},
	}, bf)

	if result.Healing[1] != 20 {
		t.Errorf("want Healing[1]=20, got %v", result.Healing)
	}
}

func TestActionDamageAccumulatesAcrossMultipleActions(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	target := hex.NewHex(2, 0)
	bf.units = []*entity.Unit{entity.NewUnit(10, "E", target, 30, 0)}

	fireball := &spell.SpellDef{ID: "fireball", Type: "action", Damage: 3}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: target,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			fireball, fireball, fireball,
		},
	}, bf)

	if result.Damage[10] != 9 {
		t.Errorf("want Damage[10]=9 (3x3), got %v", result.Damage)
	}
}

// --- Status ---

func TestActionAppliesStatus(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	target := hex.NewHex(2, 0)
	bf.units = []*entity.Unit{entity.NewUnit(10, "E", target, 30, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: target,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			{ID: "fb", Type: "action", Damage: 3, Status: "burning", StatusTurns: 2},
		},
	}, bf)

	if len(result.StatusApplied) != 1 {
		t.Fatalf("want 1 status, got %d", len(result.StatusApplied))
	}
	sc := result.StatusApplied[0]
	if sc.UnitID != 10 || sc.Status != entity.StatusBurning || sc.Turns != 2 {
		t.Errorf("bad status: %+v", sc)
	}
}

func TestActionStatusComboBonusDamageAndRemoval(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	target := hex.NewHex(2, 0)
	enemy := entity.NewUnit(10, "E", target, 30, 0)
	enemy.ApplyStatus(entity.StatusPoisoned, 3)
	bf.units = []*entity.Unit{enemy}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: target,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			{
				ID: "fb", Type: "action", Damage: 3,
				StatusCombos: []spell.StatusCombo{
					{If: "poisoned", Remove: true, BonusDamage: 2},
				},
			},
		},
	}, bf)

	if result.Damage[10] != 5 {
		t.Errorf("combo bonus: want 5 (3+2), got %d", result.Damage[10])
	}
	if len(result.StatusRemoved) != 1 || result.StatusRemoved[0].Status != entity.StatusPoisoned {
		t.Errorf("combo remove: got %v", result.StatusRemoved)
	}
}

// --- Explode ---

func TestExplodeFromOriginHitsNeighbors(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	center := hex.NewHex(5, 0)
	neighbor := center.Direction(0)
	bf.units = []*entity.Unit{
		entity.NewUnit(10, "C", center, 30, 0),
		entity.NewUnit(11, "N", neighbor, 30, 0),
	}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: center,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 10},
			{ID: "fireball", Type: "action", Damage: 3, ExplodeRadius: 1},
		},
	}, bf)

	if result.Damage[10] != 3 || result.Damage[11] != 3 {
		t.Errorf("explode: both should take 3, got %v", result.Damage)
	}
}

func TestLinePlusExplodeFiresFromEndPos(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	// No unit on path; line walks full range=3, endPos = caster+3 in dir 0.
	end := caster.Direction(0).Direction(0).Direction(0) // cube (3,0)
	victim := end.Direction(0)                           // cube (4,0)
	bf.units = []*entity.Unit{entity.NewUnit(10, "V", victim, 30, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos: caster,
		Direction: 0,
		Spells: []*spell.SpellDef{
			{ID: "line", Type: "target", Shape: spell.ShapeLine, Range: 3, Stacking: "full"},
			{ID: "fb", Type: "action", Damage: 3, ExplodeRadius: 1},
		},
	}, bf)

	if result.Damage[10] != 3 {
		t.Errorf("explode from line endPos should hit neighbor of endPos, got %v", result.Damage)
	}
}

// --- Terrain ---

func TestActionTerrainCreateUsesField(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	clicked := hex.NewHex(2, 0)

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: clicked,
		Spells: []*spell.SpellDef{
			{ID: "area", Type: "target", Shape: spell.ShapeArea, Range: 5, Radius: 0},
			{ID: "water", Type: "action", TerrainCreate: "water_puddle"},
		},
	}, bf)

	if len(result.TerrainChanges) != 1 {
		t.Fatalf("want 1 terrain change, got %d", len(result.TerrainChanges))
	}
	tc := result.TerrainChanges[0]
	if tc.Pos != clicked || tc.Type != terrain.TerrainWaterPuddle {
		t.Errorf("bad terrain change: %+v", tc)
	}
}

func TestActionTerrainInteractionTransforms(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	clicked := hex.NewHex(2, 0)
	bf.terrMap.Set(clicked, &terrain.Terrain{Type: terrain.TerrainPoisonSwamp, Duration: -1})

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: clicked,
		Spells: []*spell.SpellDef{
			{ID: "area", Type: "target", Shape: spell.ShapeArea, Range: 5, Radius: 0},
			{
				ID: "fb", Type: "action",
				TerrainInteractions: []spell.TerrainInteraction{
					{On: "poison_swamp", Result: "plain"},
				},
			},
		},
	}, bf)

	found := false
	for _, tc := range result.TerrainChanges {
		if tc.Pos == clicked && tc.Type == terrain.TerrainPlain {
			found = true
		}
	}
	if !found {
		t.Errorf("interaction not applied: %+v", result.TerrainChanges)
	}
}

// --- Movement ---

func TestActionPushMovesTargetAway(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	target := hex.NewHex(2, 0)
	bf.units = []*entity.Unit{entity.NewUnit(10, "E", target, 30, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: target,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			{ID: "push", Type: "action", Movement: "push"},
		},
	}, bf)

	if len(result.UnitsMoved) != 1 {
		t.Fatalf("want 1 move, got %d", len(result.UnitsMoved))
	}
	mv := result.UnitsMoved[0]
	if mv.To.Distance(caster) <= target.Distance(caster) {
		t.Errorf("push didn't move away: %+v", mv)
	}
}

func TestActionPullMovesTargetTowardCaster(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	target := hex.NewHex(4, 0)
	bf.units = []*entity.Unit{entity.NewUnit(10, "E", target, 30, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: target,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			{ID: "pull", Type: "action", Movement: "pull"},
		},
	}, bf)

	if len(result.UnitsMoved) != 1 {
		t.Fatalf("want 1 move, got %d", len(result.UnitsMoved))
	}
	mv := result.UnitsMoved[0]
	if mv.To.Distance(caster) >= target.Distance(caster) {
		t.Errorf("pull didn't move closer: %+v", mv)
	}
}

func TestActionPushBlockedByWallsIsSilent(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	target := hex.NewHex(2, 0)
	for i := 0; i < 6; i++ {
		bf.terrMap.Set(target.Direction(i), &terrain.Terrain{Type: terrain.TerrainRock, Duration: -1})
	}
	bf.units = []*entity.Unit{entity.NewUnit(10, "E", target, 30, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: target,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			{ID: "push", Type: "action", Movement: "push"},
		},
	}, bf)

	if len(result.UnitsMoved) != 0 {
		t.Errorf("fully walled: want 0 moves, got %v", result.UnitsMoved)
	}
}

// --- ClearState ---

func TestActionClearStateDropsAccumulated(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(5, 5)
	bf.units = []*entity.Unit{entity.NewUnit(1, "P", caster, 100, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: caster,
		Spells: []*spell.SpellDef{
			{ID: "area", Type: "target", Shape: spell.ShapeArea, Range: 4, Radius: 1},
			{ID: "wall", Type: "action", TerrainCreate: "generated_wall", ClearState: true},
			{ID: "fireball", Type: "action", Damage: 99},
		},
	}, bf)

	if len(result.TerrainChanges) == 0 {
		t.Error("wall should produce terrain changes")
	}
	if len(result.Damage) != 0 {
		t.Errorf("after ClearState, damage should be empty, got %v", result.Damage)
	}
}

// --- Action without a target spell ---

func TestBareActionHitsClickedHex(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	clicked := hex.NewHex(1, 0)
	victim := entity.NewUnit(10, "V", clicked, 100, 0)
	bf.units = []*entity.Unit{victim}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: clicked,
		Spells: []*spell.SpellDef{
			{ID: "fireball", Type: "action", Damage: 7},
		},
	}, bf)

	if result.Damage[10] != 7 {
		t.Errorf("want Damage[10]=7, got %v", result.Damage)
	}
	if len(result.Impacts) == 0 {
		t.Error("want clicked unit recorded as Impact")
	}
}

// --- Registry-driven ---

func TestRegistryFireballFullEffect(t *testing.T) {
	reg, err := spell.LoadEmbedded()
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	fireball := reg.Get("fireball")
	single := reg.Get("single")
	if fireball == nil || single == nil {
		t.Fatal("missing registry spells")
	}

	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	target := hex.NewHex(2, 0)
	bf.units = []*entity.Unit{entity.NewUnit(10, "E", target, 30, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: target,
		Spells:     []*spell.SpellDef{single, fireball},
	}, bf)

	if result.Damage[10] != fireball.Damage {
		t.Errorf("fireball damage: want %d, got %d", fireball.Damage, result.Damage[10])
	}
	if len(result.StatusApplied) == 0 || result.StatusApplied[0].Status != entity.StatusBurning {
		t.Errorf("fireball should apply burning, got %v", result.StatusApplied)
	}
}
