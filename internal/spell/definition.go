package spell

// TargetShape defines how a target spell selects hexes.
type TargetShape string

const (
	ShapeSelf     TargetShape = "self"
	ShapeSingle   TargetShape = "single"
	ShapeLine     TargetShape = "line"
	ShapeArea     TargetShape = "area"
	ShapeRing     TargetShape = "ring"
	ShapeAdjacent TargetShape = "adjacent"
)

// Element represents an elemental affinity.
type Element string

const (
	ElementNone      Element = ""
	ElementFire      Element = "fire"
	ElementIce       Element = "ice"
	ElementLightning Element = "lightning"
	ElementWater     Element = "water"
	ElementPoison    Element = "poison"
	ElementPhysical  Element = "physical"
)

// TerrainInteraction describes how a spell interacts with terrain.
type TerrainInteraction struct {
	On     string `toml:"on"`     // terrain type that triggers the interaction
	Result string `toml:"result"` // resulting terrain type
}

// StatusCombo describes bonus effects when target already has a status.
type StatusCombo struct {
	If          string `toml:"if"`           // existing status on target
	Remove      bool   `toml:"remove"`       // remove the existing status
	BonusDamage int    `toml:"bonus_damage"` // extra damage dealt
	Apply       string `toml:"apply"`        // new status to apply instead
	ApplyTurns  int    `toml:"apply_turns"`  // duration of the new status
}

// SpellDef is a data-driven spell definition loaded from TOML.
// All fields use simple types so new spells can be added without code changes.
type SpellDef struct {
	ID          string // set from the TOML map key
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Type        string `toml:"type"`      // "target" | "action"
	CostType    string `toml:"cost_type"` // "exponential" | "additive"
	BaseCost    int    `toml:"base_cost"`

	// --- Target spell fields ---
	Shape  TargetShape `toml:"shape"`
	Range  int         `toml:"range"`
	Radius int         `toml:"radius"`

	// --- Action spell fields ---
	Damage      int     `toml:"damage"`
	Heal        int     `toml:"heal"`
	Element     Element `toml:"element"`
	Status      string  `toml:"status"`       // status effect to apply
	StatusTurns int     `toml:"status_turns"`  // duration of the status
	Movement    string  `toml:"movement"`      // "push" | "pull"

	// --- Terrain ---
	TerrainCreate string `toml:"terrain_create"` // terrain type to create on target hex
	ClearTargets  bool   `toml:"clear_targets"`  // clear target list after execution

	// --- Interactions ---
	TerrainInteractions []TerrainInteraction `toml:"terrain_interactions"`
	StatusCombos        []StatusCombo        `toml:"status_combos"`
}

// IsTarget returns true if this spell is a target-type spell.
func (d *SpellDef) IsTarget() bool {
	return d.Type == "target"
}

// IsAction returns true if this spell is an action-type spell.
func (d *SpellDef) IsAction() bool {
	return d.Type == "action"
}

// ParseSpellType converts the string type to SpellType.
func (d *SpellDef) ParseSpellType() SpellType {
	if d.Type == "action" {
		return SpellTypeAction
	}
	return SpellTypeTarget
}

// ParseCostType converts the string cost_type to CostType.
func (d *SpellDef) ParseCostType() CostType {
	if d.CostType == "additive" {
		return CostTypeAdditive
	}
	return CostTypeExponential
}
