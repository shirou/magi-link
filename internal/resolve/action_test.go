package resolve

import (
	"testing"

	"github.com/shirou/magi_link/internal/entity"
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/spell"
	"github.com/shirou/magi_link/internal/terrain"
)

// --- Damage / Heal ---

func TestActionDamageHitsTargetUnit(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	targetPos := hex.NewHex(2, 0)
	enemy := entity.NewUnit(10, "Echo", targetPos, 30, 0)
	bf.units = []*entity.Unit{enemy}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: targetPos,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			{ID: "fireball", Type: "action", Damage: 5, Element: spell.ElementFire},
		},
	}, bf)

	if result.Damage[10] != 5 {
		t.Errorf("want Damage[10]=5, got %v", result.Damage)
	}
}

func TestActionHealHitsTargetUnit(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	ally := entity.NewUnit(1, "Player", caster, 100, 0)
	ally.IsPlayer = true
	ally.HP = 50
	bf.units = []*entity.Unit{ally}

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
	targetPos := hex.NewHex(2, 0)
	bf.units = []*entity.Unit{entity.NewUnit(10, "Echo", targetPos, 30, 0)}

	fireball := &spell.SpellDef{ID: "fireball", Type: "action", Damage: 3}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: targetPos,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			fireball,
			fireball,
			fireball,
		},
	}, bf)

	if result.Damage[10] != 9 {
		t.Errorf("want Damage[10]=9 (3×3), got %v", result.Damage)
	}
}

// --- Status application ---

func TestActionAppliesStatus(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	targetPos := hex.NewHex(2, 0)
	bf.units = []*entity.Unit{entity.NewUnit(10, "Echo", targetPos, 30, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: targetPos,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			{ID: "fireball", Type: "action", Damage: 3, Status: "burning", StatusTurns: 2},
		},
	}, bf)

	if len(result.StatusApplied) != 1 {
		t.Fatalf("want 1 status applied, got %d", len(result.StatusApplied))
	}
	sc := result.StatusApplied[0]
	if sc.UnitID != 10 || sc.Status != entity.StatusBurning || sc.Turns != 2 {
		t.Errorf("unexpected status change: %+v", sc)
	}
}

// --- Status combos ---

func TestActionStatusComboBonusDamageAndRemoval(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	targetPos := hex.NewHex(2, 0)
	enemy := entity.NewUnit(10, "Echo", targetPos, 30, 0)
	enemy.ApplyStatus(entity.StatusPoisoned, 3)
	bf.units = []*entity.Unit{enemy}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: targetPos,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			{
				ID: "fireball", Type: "action", Damage: 3,
				StatusCombos: []spell.StatusCombo{
					{If: "poisoned", Remove: true, BonusDamage: 2},
				},
			},
		},
	}, bf)

	if result.Damage[10] != 5 {
		t.Errorf("combo bonus damage: want 5 (3+2), got %d", result.Damage[10])
	}
	if len(result.StatusRemoved) != 1 || result.StatusRemoved[0].Status != entity.StatusPoisoned {
		t.Errorf("combo status removal: got %v", result.StatusRemoved)
	}
}

// --- Explode radius ---

func TestActionExplodeRadiusHitsNeighbors(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	center := hex.NewHex(5, 0)
	neighbor := center.Direction(0) // (6, 0)
	bf.units = []*entity.Unit{
		entity.NewUnit(10, "Center", center, 30, 0),
		entity.NewUnit(11, "Neighbor", neighbor, 30, 0),
	}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: center,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 10},
			{ID: "fireball", Type: "action", Damage: 3, ExplodeRadius: 1},
		},
	}, bf)

	if result.Damage[10] != 3 {
		t.Errorf("center hit: want 3, got %d", result.Damage[10])
	}
	if result.Damage[11] != 3 {
		t.Errorf("explode neighbor: want 3, got %d", result.Damage[11])
	}
}

// --- Terrain creation ---

func TestActionTerrainCreate(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	pos := hex.NewHex(2, 0)

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: pos,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			{ID: "water", Type: "action", TerrainCreate: "water_puddle"},
		},
	}, bf)

	if len(result.TerrainChanges) != 1 {
		t.Fatalf("want 1 terrain change, got %d", len(result.TerrainChanges))
	}
	tc := result.TerrainChanges[0]
	if tc.Pos != pos || tc.Type != terrain.TerrainWaterPuddle {
		t.Errorf("unexpected terrain change: %+v", tc)
	}
}

// --- Terrain interactions ---

func TestActionTerrainInteractionTransformsExisting(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	pos := hex.NewHex(2, 0)
	bf.terrMap.Set(pos, &terrain.Terrain{Type: terrain.TerrainPoisonSwamp, Duration: -1})

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: pos,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			{
				ID: "fireball", Type: "action", Damage: 3,
				TerrainInteractions: []spell.TerrainInteraction{
					{On: "poison_swamp", Result: "plain"},
				},
			},
		},
	}, bf)

	found := false
	for _, tc := range result.TerrainChanges {
		if tc.Pos == pos && tc.Type == terrain.TerrainPlain {
			found = true
		}
	}
	if !found {
		t.Errorf("poison_swamp → plain interaction not found: %+v", result.TerrainChanges)
	}
}

// --- Movement ---

func TestActionPushMovesTargetAwayFromCaster(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	targetPos := hex.NewHex(2, 0)
	bf.units = []*entity.Unit{entity.NewUnit(10, "Echo", targetPos, 30, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: targetPos,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			{ID: "push", Type: "action", Movement: "push"},
		},
	}, bf)

	if len(result.UnitsMoved) != 1 {
		t.Fatalf("want 1 move, got %d", len(result.UnitsMoved))
	}
	mv := result.UnitsMoved[0]
	if mv.UnitID != 10 || mv.From != targetPos {
		t.Errorf("unexpected move: %+v", mv)
	}
	if mv.To.Distance(caster) <= targetPos.Distance(caster) {
		t.Errorf("push should increase distance from caster: from %v (d=%d) to %v (d=%d)",
			mv.From, targetPos.Distance(caster), mv.To, mv.To.Distance(caster))
	}
}

func TestActionPullMovesTargetTowardCaster(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	targetPos := hex.NewHex(4, 0)
	bf.units = []*entity.Unit{entity.NewUnit(10, "Echo", targetPos, 30, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: targetPos,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			{ID: "pull", Type: "action", Movement: "pull"},
		},
	}, bf)

	if len(result.UnitsMoved) != 1 {
		t.Fatalf("want 1 move, got %d", len(result.UnitsMoved))
	}
	mv := result.UnitsMoved[0]
	if mv.To.Distance(caster) >= targetPos.Distance(caster) {
		t.Errorf("pull should decrease distance from caster: from %v (d=%d) to %v (d=%d)",
			mv.From, targetPos.Distance(caster), mv.To, mv.To.Distance(caster))
	}
}

func TestActionPushBlockedByWallIsSilent(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	targetPos := hex.NewHex(2, 0)
	// Block every direction the target might move into by flooding with walls.
	for i := 0; i < 6; i++ {
		bf.terrMap.Set(targetPos.Direction(i), &terrain.Terrain{Type: terrain.TerrainRock, Duration: -1})
	}
	bf.units = []*entity.Unit{entity.NewUnit(10, "Echo", targetPos, 30, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: targetPos,
		Spells: []*spell.SpellDef{
			{ID: "single", Type: "target", Shape: spell.ShapeSingle, Range: 5},
			{ID: "push", Type: "action", Movement: "push"},
		},
	}, bf)

	if len(result.UnitsMoved) != 0 {
		t.Errorf("completely walled-in push should produce no move, got %v", result.UnitsMoved)
	}
}

// --- Clear targets ---

func TestActionClearTargetsDropsAccumulatedList(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(5, 5)

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: caster,
		Spells: []*spell.SpellDef{
			{ID: "self", Type: "target", Shape: spell.ShapeSelf},
			{ID: "area", Type: "target", Shape: spell.ShapeArea, Range: 4, Radius: 1},
			{ID: "wall", Type: "action", TerrainCreate: "generated_wall", ClearTargets: true},
			// After clear_targets, nothing should remain to be targeted.
			{ID: "fireball", Type: "action", Damage: 99},
		},
	}, bf)

	// The wall should have written terrain changes for every area hex.
	if len(result.TerrainChanges) == 0 {
		t.Error("wall action should produce terrain changes")
	}
	// After ClearTargets, the fireball should hit nothing.
	if len(result.Damage) != 0 {
		t.Errorf("after ClearTargets, subsequent damage should be empty, got %v", result.Damage)
	}
}

// --- Action spells alone (no target spells) ---

// An action spell with no preceding target spell falls back to an implicit
// single-target at the clicked hex. Users expect "fireball on enemy" to
// just work without first adding a `single` to the chain.
func TestBareActionHitsClickedHex(t *testing.T) {
	bf := newMockBF()
	caster := hex.NewHex(0, 0)
	clicked := hex.NewHex(1, 0)
	victim := entity.NewUnit(10, "V", clicked, 100, 0)
	bf.units = append(bf.units, victim)

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: clicked,
		Spells: []*spell.SpellDef{
			{ID: "fireball", Type: "action", Damage: 7},
		},
	}, bf)

	if result.Damage[victim.ID] != 7 {
		t.Errorf("want 7 damage on clicked unit, got %v", result.Damage)
	}
	if len(result.Hexes) == 0 {
		t.Error("want clicked hex reported for VFX")
	}
}

// --- Registry-driven: full fireball spell ---

func TestRegistryFireballAppliesFullEffect(t *testing.T) {
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
	targetPos := hex.NewHex(2, 0)
	bf.units = []*entity.Unit{entity.NewUnit(10, "Echo", targetPos, 30, 0)}

	result := ExecuteLink(LinkInput{
		CasterPos:  caster,
		ClickedHex: targetPos,
		Spells:     []*spell.SpellDef{single, fireball},
	}, bf)

	if result.Damage[10] != fireball.Damage {
		t.Errorf("fireball base damage: want %d, got %d", fireball.Damage, result.Damage[10])
	}
	if len(result.StatusApplied) != 1 || result.StatusApplied[0].Status != entity.StatusBurning {
		t.Errorf("fireball should apply burning, got %v", result.StatusApplied)
	}
}
