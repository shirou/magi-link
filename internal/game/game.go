package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/shirou/magi_link/internal/spell"
)

// GameState represents the current state of the game
type GameState int

const (
	StateTitle GameState = iota
	StateBattle
	StateGameOver
	StateLevelUp
)

// Game implements ebiten.Game
type Game struct {
	state    GameState
	width    int
	height   int
	battle   *BattleState
	spellReg *spell.Registry
}

// New creates a new Game instance
func New() *Game {
	reg, err := spell.LoadEmbedded()
	if err != nil {
		panic(fmt.Sprintf("failed to load spells: %v", err))
	}
	// Use English locale for debug font compatibility (ASCII only)
	if err := reg.SetLocale("en"); err != nil {
		panic(fmt.Sprintf("failed to set locale: %v", err))
	}

	return &Game{
		state:    StateTitle,
		width:    1280,
		height:   720,
		spellReg: reg,
	}
}

// Update updates the game state
func (g *Game) Update() error {
	switch g.state {
	case StateTitle:
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.battle = NewBattle(g.width, g.height, g.spellReg)
			g.state = StateBattle
		}
	case StateBattle:
		g.battle.Update()
	case StateGameOver:
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			g.battle = NewBattle(g.width, g.height, g.spellReg)
			g.state = StateBattle
		}
	}
	return nil
}

// Draw draws the game screen
func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{20, 20, 30, 255})

	switch g.state {
	case StateTitle:
		ebitenutil.DebugPrint(screen, "Magi Link\n\nPress SPACE to start")
	case StateBattle:
		g.battle.Draw(screen)
	case StateGameOver:
		ebitenutil.DebugPrint(screen, "Game Over\n\nPress R to restart")
	}
}

// Layout returns the logical screen size
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return g.width, g.height
}
