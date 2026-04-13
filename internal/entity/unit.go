package entity

import "github.com/shirou/magi_link/internal/hex"

// StatusEffect represents a status condition
type StatusEffect int

const (
	StatusNone StatusEffect = iota
	StatusBurning
	StatusFrozen
	StatusPoisoned
	StatusBleeding
	StatusWet
	StatusElectrified
)

// Unit represents any unit on the battlefield (player, enemy, ally)
type Unit struct {
	ID       int
	Name     string
	Pos      hex.Hex
	HP       int
	MaxHP    int
	Mana     int
	MaxMana  int
	MoveRange int

	Statuses map[StatusEffect]int // status -> remaining turns

	IsPlayer bool
	IsDead   bool
}

func NewUnit(id int, name string, pos hex.Hex, hp, mana int) *Unit {
	return &Unit{
		ID:        id,
		Name:      name,
		Pos:       pos,
		HP:        hp,
		MaxHP:     hp,
		Mana:      mana,
		MaxMana:   mana,
		MoveRange: 3,
		Statuses:  make(map[StatusEffect]int),
	}
}

// TakeDamage applies damage and returns actual damage dealt
func (u *Unit) TakeDamage(amount int) int {
	if amount <= 0 {
		return 0
	}
	actual := min(u.HP, amount)
	u.HP -= actual
	if u.HP <= 0 {
		u.HP = 0
		u.IsDead = true
	}
	return actual
}

// Heal restores HP and returns actual amount healed
func (u *Unit) Heal(amount int) int {
	if amount <= 0 {
		return 0
	}
	actual := min(u.MaxHP-u.HP, amount)
	u.HP += actual
	return actual
}

// ApplyStatus applies a status effect for a given number of turns
func (u *Unit) ApplyStatus(s StatusEffect, turns int) {
	if current, ok := u.Statuses[s]; ok {
		if turns > current {
			u.Statuses[s] = turns
		}
	} else {
		u.Statuses[s] = turns
	}
}

// HasStatus returns true if the unit has the given status
func (u *Unit) HasStatus(s StatusEffect) bool {
	turns, ok := u.Statuses[s]
	return ok && turns > 0
}

// IsAlive returns true if u is a non-nil, living unit. Safe to call on nil.
func (u *Unit) IsAlive() bool {
	return u != nil && !u.IsDead
}

// statusNames maps every StatusEffect to its TOML/UI string name.
var statusNames = map[StatusEffect]string{
	StatusBurning:     "burning",
	StatusFrozen:      "frozen",
	StatusPoisoned:    "poisoned",
	StatusBleeding:    "bleeding",
	StatusWet:         "wet",
	StatusElectrified: "electrified",
}

// ParseStatus converts a status name string to a StatusEffect.
// Returns StatusNone for unknown or empty names.
func ParseStatus(name string) StatusEffect {
	for s, n := range statusNames {
		if n == name {
			return s
		}
	}
	return StatusNone
}

// TickStatuses decrements status durations at turn end
func (u *Unit) TickStatuses() {
	for s, turns := range u.Statuses {
		if turns <= 1 {
			delete(u.Statuses, s)
		} else {
			u.Statuses[s] = turns - 1
		}
	}
}

// UseMana attempts to use mana, returns false if insufficient
func (u *Unit) UseMana(amount int) bool {
	if u.Mana < amount {
		return false
	}
	u.Mana -= amount
	return true
}

// RecoverMana recovers mana up to max
func (u *Unit) RecoverMana(amount int) {
	u.Mana = min(u.MaxMana, u.Mana+amount)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
