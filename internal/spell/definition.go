package spell

// TargetShape defines how a target spell selects hexes.
type TargetShape string

const (
	ShapeSelf          TargetShape = "self"
	ShapeSingle        TargetShape = "single"
	ShapeLine          TargetShape = "line"
	ShapeArea          TargetShape = "area"
	ShapeRing          TargetShape = "ring"
	ShapeAdjacent      TargetShape = "adjacent"
	ShapeWeakest       TargetShape = "weakest"
	ShapeAlly          TargetShape = "ally"
	ShapeTerrainFilter TargetShape = "terrain_filter"

	// Modifier shapes (bucket-relay model): these transform the running
	// LinkState without directly adding to Targets, or set transient
	// flags that alter the next action step.
	Shape3Way   TargetShape = "3way"   // origin transformer: 1 → 3 adjacent origins
	ShapePierce TargetShape = "pierce" // flag: next line walks through obstacles
	ShapeHoming TargetShape = "homing" // origin override: snap to nearest enemy
	ShapeBounce TargetShape = "bounce" // flag: next line reflects off walls
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
// Localized text (name, description) is stored separately in Locale.
type SpellDef struct {
	ID       string // set from the TOML map key
	Type     string `toml:"type"`      // "target" | "action"
	CostType string `toml:"cost_type"` // "exponential" | "additive"
	BaseCost int    `toml:"base_cost"`

	// --- Target spell fields ---
	Shape  TargetShape `toml:"shape"`
	Range  int         `toml:"range"`
	Radius int         `toml:"radius"`
	Pierce bool        `toml:"pierce"` // line spells: walk through walls/units up to max range

	// --- Action spell fields ---
	Damage        int     `toml:"damage"`
	Heal          int     `toml:"heal"`
	Element       Element `toml:"element"`
	Status        string  `toml:"status"`        // status effect to apply
	StatusTurns   int     `toml:"status_turns"`  // duration of the status
	Movement      string  `toml:"movement"`      // "push" | "pull"
	ExplodeRadius int     `toml:"explode_radius"` // >0 = spell explodes on hit with this radius

	// --- Terrain ---
	TerrainCreate string `toml:"terrain_create"` // terrain type to create on target hex
	ClearState    bool   `toml:"clear_state"`    // drop accumulated Impacts/Field/Waypoints after this action

	// --- Target: terrain filter ---
	Filter string `toml:"filter"` // terrain type name to match (for terrain_filter shape)

	// --- Same-type stacking ---
	Stacking      string `toml:"stacking"`       // "increment" | "none" | "full" (default: "none")
	StackingField string `toml:"stacking_field"`  // which field to increment: "radius" | "range"
	StackingValue int    `toml:"stacking_value"`  // increment per extra same-shape use

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
