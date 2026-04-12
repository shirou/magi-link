package spell

// SpellType represents the type of spell.
type SpellType int

const (
	SpellTypeTarget SpellType = iota
	SpellTypeAction
)

// CostType represents how cost scales with repeated use.
type CostType int

const (
	CostTypeExponential CostType = iota // cost × 2^(n-1)
	CostTypeAdditive                    // cost + (n-1)
)

// Target represents something that can be targeted.
type Target struct {
	UnitID     int // -1 if terrain target
	HexQ, HexR int
}

// CastContext holds the state during a spell chain execution.
type CastContext struct {
	CasterID  int
	Targets   []Target
	TurnStats *TurnStats
}

// TurnStats tracks aggregated values for passive skill triggers.
type TurnStats struct {
	UnitsAttacked   map[int]bool   // unitID -> attacked this turn
	SpellsUsed      map[string]int // spellID -> count
	StatusesApplied int
	DamageDealt     int
	DamageReceived  int
	HealingDone     int
}

func NewTurnStats() *TurnStats {
	return &TurnStats{
		UnitsAttacked: make(map[int]bool),
		SpellsUsed:    make(map[string]int),
	}
}

// Chain represents a sequence of spells to cast.
type Chain struct {
	Slots []*SpellSlot
}

// SpellSlot is one slot in the chain.
type SpellSlot struct {
	Spell *SpellDef
}

// TotalCost returns the mana cost of the entire chain.
func (c *Chain) TotalCost() int {
	counts := make(map[string]int)
	total := 0
	for _, slot := range c.Slots {
		if slot.Spell == nil {
			continue
		}
		counts[slot.Spell.ID]++
		n := counts[slot.Spell.ID]
		total += slotCost(slot.Spell, n)
	}
	return total
}

func slotCost(s *SpellDef, n int) int {
	switch s.ParseCostType() {
	case CostTypeExponential:
		cost := s.BaseCost
		for i := 1; i < n; i++ {
			cost *= 2
		}
		return cost
	case CostTypeAdditive:
		return s.BaseCost + (n - 1)
	}
	return s.BaseCost
}

// SpellBook holds the player's available spells.
type SpellBook struct {
	Spells  []*SpellDef
	MaxSize int
}

const SpellBookMaxSize = 12

func NewSpellBook() *SpellBook {
	return &SpellBook{
		Spells:  make([]*SpellDef, 0, SpellBookMaxSize),
		MaxSize: SpellBookMaxSize,
	}
}

// Add adds a spell, returning true if successful.
func (sb *SpellBook) Add(s *SpellDef) bool {
	if len(sb.Spells) >= sb.MaxSize {
		return false
	}
	sb.Spells = append(sb.Spells, s)
	return true
}

// Remove removes a spell by index.
func (sb *SpellBook) Remove(index int) {
	if index < 0 || index >= len(sb.Spells) {
		return
	}
	sb.Spells = append(sb.Spells[:index], sb.Spells[index+1:]...)
}
