package spell

import "math"

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

// maxManaCost caps any single slot cost to prevent integer overflow.
const maxManaCost = 1 << 30 // ~1 billion

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

// ChainMaxSlots is the maximum number of slots in a chain.
// Mana is the primary limiter, but this prevents degenerate cases
// (e.g., spamming cost-0 spells).
const ChainMaxSlots = 20

// SpellSlot is one slot in the chain.
type SpellSlot struct {
	Spell *SpellDef
}

// CanAdd returns true if another spell can be added to the chain.
func (c *Chain) CanAdd() bool {
	return len(c.Slots) < ChainMaxSlots
}

// TotalCost returns the mana cost of the entire chain.
func (c *Chain) TotalCost() int {
	total := 0
	for _, cost := range c.SlotCosts() {
		total += cost
		if total > maxManaCost {
			return maxManaCost
		}
	}
	return total
}

// SlotCosts returns the individual mana cost of each slot in a single pass.
// This avoids O(n²) when drawing per-slot costs.
func (c *Chain) SlotCosts() []int {
	costs := make([]int, len(c.Slots))
	counts := make(map[string]int, len(c.Slots))
	for i, slot := range c.Slots {
		if slot.Spell == nil {
			continue
		}
		counts[slot.Spell.ID]++
		costs[i] = SlotCost(slot.Spell, counts[slot.Spell.ID])
	}
	return costs
}

// SlotCost computes the mana cost for a spell used for the n-th time in a chain.
func SlotCost(s *SpellDef, n int) int {
	switch s.ParseCostType() {
	case CostTypeExponential:
		if n <= 0 {
			return s.BaseCost
		}
		// Use bit shift with overflow guard: cost = baseCost * 2^(n-1)
		shift := n - 1
		if shift >= 31 || s.BaseCost > maxManaCost>>(shift) {
			return maxManaCost
		}
		return min(s.BaseCost<<shift, maxManaCost)
	case CostTypeAdditive:
		cost := s.BaseCost + (n - 1)
		if cost > maxManaCost {
			return maxManaCost
		}
		return cost
	}
	return s.BaseCost
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

func init() {
	// Ensure maxManaCost doesn't exceed int range on 32-bit platforms.
	if maxManaCost > math.MaxInt32 {
		panic("maxManaCost exceeds int32 range")
	}
}
