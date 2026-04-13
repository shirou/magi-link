package terrain

import "github.com/shirou/magi_link/internal/hex"

// TerrainType represents the type of terrain on a hex
type TerrainType int

const (
	TerrainPlain TerrainType = iota
	// Walls (block movement and line of sight)
	TerrainRock
	TerrainStone
	TerrainDirtWall
	TerrainWoodWall
	TerrainGeneratedWall
	// Damage terrain (passable but hazardous)
	TerrainLava
	TerrainPoisonSwamp
	TerrainThorns
	TerrainElectricFloor
	TerrainFireFloor
	TerrainWaterPuddle
	// Special
	TerrainCliff
)

// terrainTypeNames maps TerrainType to its string identifier.
// Used for TOML filter matching and display.
var terrainTypeNames = map[TerrainType]string{
	TerrainPlain:         "plain",
	TerrainRock:          "rock",
	TerrainStone:         "stone",
	TerrainDirtWall:      "dirt_wall",
	TerrainWoodWall:      "wood_wall",
	TerrainGeneratedWall: "generated_wall",
	TerrainLava:          "lava",
	TerrainPoisonSwamp:   "poison_swamp",
	TerrainThorns:        "thorns",
	TerrainElectricFloor: "electric_floor",
	TerrainFireFloor:     "fire_floor",
	TerrainWaterPuddle:   "water_puddle",
	TerrainCliff:         "cliff",
}

// Terrain represents the state of a single hex cell
type Terrain struct {
	Type       TerrainType
	Duration   int // -1 = permanent, 0 = expired, >0 = turns remaining
}

// TypeName returns the string identifier for this terrain's type.
func (t *Terrain) TypeName() string {
	if name, ok := terrainTypeNames[t.Type]; ok {
		return name
	}
	return "plain"
}

// ParseTerrainType converts a string identifier back to TerrainType.
// Returns TerrainPlain if the name is unknown.
func ParseTerrainType(name string) TerrainType {
	for t, n := range terrainTypeNames {
		if n == name {
			return t
		}
	}
	return TerrainPlain
}

// IsWall returns true if the terrain blocks movement and LoS
func (t *Terrain) IsWall() bool {
	switch t.Type {
	case TerrainRock, TerrainStone, TerrainDirtWall, TerrainWoodWall, TerrainGeneratedWall:
		return true
	}
	return false
}

// IsPassable returns true if units can move through this terrain
func (t *Terrain) IsPassable() bool {
	return !t.IsWall() && t.Type != TerrainCliff
}

// IsDestructible returns true if the terrain can be destroyed
func (t *Terrain) IsDestructible() bool {
	switch t.Type {
	case TerrainDirtWall, TerrainWoodWall, TerrainGeneratedWall:
		return true
	}
	return false
}

// Map holds the terrain state for all hexes
type Map struct {
	grid    map[hex.Hex]*Terrain
	width   int
	height  int
}

func NewMap(width, height int) *Map {
	return &Map{
		grid:   make(map[hex.Hex]*Terrain),
		width:  width,
		height: height,
	}
}

// Get returns the terrain at a hex, defaulting to plain
func (m *Map) Get(h hex.Hex) *Terrain {
	if t, ok := m.grid[h]; ok {
		return t
	}
	return &Terrain{Type: TerrainPlain, Duration: -1}
}

// Set sets the terrain at a hex
func (m *Map) Set(h hex.Hex, t *Terrain) {
	m.grid[h] = t
}

// Tick decrements terrain durations and removes expired ones
func (m *Map) Tick() {
	for h, t := range m.grid {
		if t.Duration == 0 {
			delete(m.grid, h)
		} else if t.Duration > 0 {
			t.Duration--
		}
	}
}

// HasLineOfSight returns true if there are no walls between two hexes.
// Adjacent hexes (distance <= 1) always have line of sight regardless of walls.
func (m *Map) HasLineOfSight(from, to hex.Hex) bool {
	if from.Distance(to) <= 1 {
		return true
	}
	line := from.LineTo(to)
	for _, h := range line[1 : len(line)-1] { // exclude endpoints
		if m.Get(h).IsWall() {
			return false
		}
	}
	return true
}

// Interact handles terrain interaction when a spell hits a terrain hex
// Returns the resulting terrain type after interaction
func (m *Map) Interact(h hex.Hex, spellID string) *Terrain {
	current := m.Get(h)
	switch {
	case current.Type == TerrainLava && spellID == "ice":
		return &Terrain{Type: TerrainPlain, Duration: -1}
	case current.Type == TerrainWaterPuddle && spellID == "lightning":
		// Handled by spell logic (chain propagation)
		return current
	case current.Type == TerrainPoisonSwamp && spellID == "fireball":
		// Explosion handled by spell logic
		return &Terrain{Type: TerrainPlain, Duration: -1}
	case current.Type == TerrainFireFloor && spellID == "water":
		return &Terrain{Type: TerrainPlain, Duration: -1}
	case current.Type == TerrainThorns && spellID == "fireball":
		return &Terrain{Type: TerrainPlain, Duration: -1}
	}
	return current
}
