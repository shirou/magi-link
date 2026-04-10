package passive

import "github.com/shirou/magi_link/internal/spell"

const PassiveMaxSize = 6

// TriggerResult holds the outcome of a passive trigger
type TriggerResult struct {
	PassiveID string
	Triggered bool
	Effects   []string // description of effects for logging
}

// Passive represents a passive skill
type Passive struct {
	ID          string
	Name        string
	Description string

	// OnTurnEnd is called after all spells resolve with the turn's aggregated stats
	// Returns whether the passive triggered and any resulting effects
	OnTurnEnd func(stats *spell.TurnStats, ctx *PassiveContext) TriggerResult
}

// PassiveContext gives passives access to the game state
type PassiveContext struct {
	CasterID int
	// TODO: add game state access (unit manager, terrain, etc.)
}

// PassiveBook holds the player's active passives
type PassiveBook struct {
	Passives []*Passive
	MaxSize  int
}

func NewPassiveBook() *PassiveBook {
	return &PassiveBook{
		Passives: make([]*Passive, 0, PassiveMaxSize),
		MaxSize:  PassiveMaxSize,
	}
}

// Add adds a passive, returning true if successful
func (pb *PassiveBook) Add(p *Passive) bool {
	if len(pb.Passives) >= pb.MaxSize {
		return false
	}
	pb.Passives = append(pb.Passives, p)
	return true
}

// TriggerAll fires all passives at turn end
func (pb *PassiveBook) TriggerAll(stats *spell.TurnStats, ctx *PassiveContext) []TriggerResult {
	results := make([]TriggerResult, 0, len(pb.Passives))
	for _, p := range pb.Passives {
		if p.OnTurnEnd != nil {
			result := p.OnTurnEnd(stats, ctx)
			results = append(results, result)
		}
	}
	return results
}
