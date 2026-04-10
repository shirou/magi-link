package spell

// SpellType represents the type of spell
type SpellType int

const (
	SpellTypeTarget SpellType = iota
	SpellTypeAction
)

// CostType represents how cost scales with repeated use
type CostType int

const (
	CostTypeExponential CostType = iota // cost × 2^(n-1)
	CostTypeAdditive                    // cost + (n-1)
)

// Target represents something that can be targeted
type Target struct {
	UnitID  int // -1 if terrain target
	HexQ, HexR int
}

// Spell represents a spell in the spellbook
type Spell struct {
	ID       string
	Name     string
	Type     SpellType
	CostType CostType
	BaseCost int

	// Execute applies the spell to the current targets and returns new targets
	Execute func(ctx *CastContext) []Target
}

// CastContext holds the state during a spell chain
type CastContext struct {
	CasterID int
	Targets  []Target
	TurnStats *TurnStats
}

// TurnStats tracks aggregated values for passive skill triggers
type TurnStats struct {
	UnitsAttacked     map[int]bool // unitID -> attacked this turn
	SpellsUsed        map[string]int // spellID -> count
	StatusesApplied   int
	DamageDealt       int
	DamageReceived    int
	HealingDone       int
}

func NewTurnStats() *TurnStats {
	return &TurnStats{
		UnitsAttacked: make(map[int]bool),
		SpellsUsed:    make(map[string]int),
	}
}

// Chain represents a sequence of spells to cast
type Chain struct {
	Slots []*SpellSlot
}

// SpellSlot is one slot in the chain
type SpellSlot struct {
	Spell     *Spell
	UseCount  int // how many times this spell has been used this chain
}

// TotalCost returns the mana cost of the entire chain
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

func slotCost(s *Spell, n int) int {
	switch s.CostType {
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

// Execute runs the chain and returns TurnStats
func (c *Chain) Execute(ctx *CastContext) {
	targets := ctx.Targets

	for _, slot := range c.Slots {
		if slot.Spell == nil {
			continue
		}
		ctx.TurnStats.SpellsUsed[slot.Spell.ID]++
		ctx.Targets = targets
		targets = slot.Spell.Execute(ctx)
	}
}

// SpellBook holds the player's available spells
type SpellBook struct {
	Spells []*Spell
	MaxSize int
}

const SpellBookMaxSize = 12

func NewSpellBook() *SpellBook {
	return &SpellBook{
		Spells:  make([]*Spell, 0, SpellBookMaxSize),
		MaxSize: SpellBookMaxSize,
	}
}

// Add adds a spell, returning true if successful
func (sb *SpellBook) Add(s *Spell) bool {
	if len(sb.Spells) >= sb.MaxSize {
		return false
	}
	sb.Spells = append(sb.Spells, s)
	return true
}

// Remove removes a spell by index
func (sb *SpellBook) Remove(index int) {
	if index < 0 || index >= len(sb.Spells) {
		return
	}
	sb.Spells = append(sb.Spells[:index], sb.Spells[index+1:]...)
}
