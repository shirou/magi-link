package game

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/shirou/magi_link/internal/entity"
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/terrain"
)

// BattlePhase represents the current phase of battle.
type BattlePhase int

const (
	PhasePlayerSelect BattlePhase = iota
	PhasePlayerMove
	PhaseEnemyTurn
)

const (
	GridWidth  = 11
	GridHeight = 9
)

// BattleState holds all state for a single battle encounter.
type BattleState struct {
	Grid       *hex.Grid
	TerrainMap *terrain.Map
	Units      []*entity.Unit
	Player     *entity.Unit

	Phase      BattlePhase
	HoverHex   hex.Hex
	HoverValid bool

	SelectedUnit *entity.Unit
	Reachable    map[hex.Hex]int
	MovePath     []hex.Hex
	MoveOrigin   hex.Hex
	HasMoved     bool
	TurnNumber   int
}

// NewBattle creates a new battle with initial setup.
func NewBattle(screenW, screenH int) *BattleState {
	grid := hex.NewGrid(GridWidth, GridHeight, 0, 0, 0)
	padding := 20.0
	grid.FitInRect(padding, padding, float64(screenW)-2*padding, float64(screenH)-2*padding)

	tm := terrain.NewMap(GridWidth, GridHeight)
	// Sample terrain for testing
	tm.Set(hex.OffsetToHex(5, 4), &terrain.Terrain{Type: terrain.TerrainRock, Duration: -1})
	tm.Set(hex.OffsetToHex(6, 3), &terrain.Terrain{Type: terrain.TerrainWoodWall, Duration: -1})
	tm.Set(hex.OffsetToHex(4, 5), &terrain.Terrain{Type: terrain.TerrainWaterPuddle, Duration: -1})
	tm.Set(hex.OffsetToHex(7, 5), &terrain.Terrain{Type: terrain.TerrainLava, Duration: -1})
	tm.Set(hex.OffsetToHex(3, 2), &terrain.Terrain{Type: terrain.TerrainPoisonSwamp, Duration: -1})
	tm.Set(hex.OffsetToHex(8, 4), &terrain.Terrain{Type: terrain.TerrainDirtWall, Duration: -1})
	tm.Set(hex.OffsetToHex(6, 6), &terrain.Terrain{Type: terrain.TerrainThorns, Duration: -1})

	player := entity.NewUnit(1, "Player", hex.OffsetToHex(2, 4), 100, 60)
	player.IsPlayer = true

	enemy1 := entity.NewUnit(2, "Echo", hex.OffsetToHex(8, 2), 30, 0)
	enemy2 := entity.NewUnit(3, "Shard", hex.OffsetToHex(9, 6), 40, 0)
	enemy3 := entity.NewUnit(4, "Chorus", hex.OffsetToHex(7, 7), 25, 0)

	return &BattleState{
		Grid:       grid,
		TerrainMap: tm,
		Units:      []*entity.Unit{player, enemy1, enemy2, enemy3},
		Player:     player,
		Phase:      PhasePlayerSelect,
		TurnNumber: 1,
	}
}

func (b *BattleState) isBlocked(h hex.Hex) bool {
	if !b.TerrainMap.Get(h).IsPassable() {
		return true
	}
	for _, u := range b.Units {
		if !u.IsDead && u.Pos == h {
			return true
		}
	}
	return false
}

func (b *BattleState) unitAt(h hex.Hex) *entity.Unit {
	for _, u := range b.Units {
		if !u.IsDead && u.Pos == h {
			return u
		}
	}
	return nil
}

// Update processes input and updates battle state.
func (b *BattleState) Update() {
	mx, my := ebiten.CursorPosition()
	b.HoverHex = b.Grid.ScreenToHex(float64(mx), float64(my))
	b.HoverValid = b.Grid.InBounds(b.HoverHex)

	switch b.Phase {
	case PhasePlayerSelect:
		b.updatePlayerSelect()
	case PhasePlayerMove:
		b.updatePlayerMove()
	}
}

func (b *BattleState) updatePlayerSelect() {
	// Cancel move with Escape
	if b.HasMoved && inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		b.Player.Pos = b.MoveOrigin
		b.HasMoved = false
		return
	}

	// End turn
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		b.endTurn()
		return
	}

	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || !b.HoverValid {
		return
	}

	// Click on player unit to enter move mode (only if not already moved)
	if !b.HasMoved && b.HoverHex == b.Player.Pos {
		b.SelectedUnit = b.Player
		b.MoveOrigin = b.Player.Pos
		b.Reachable = b.Grid.Reachable(b.Player.Pos, b.Player.MoveRange, b.isBlocked)
		b.Phase = PhasePlayerMove
	}
}

func (b *BattleState) updatePlayerMove() {
	// Cancel with right-click or Escape
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) ||
		inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		b.Reachable = nil
		b.MovePath = nil
		b.SelectedUnit = nil
		b.Phase = PhasePlayerSelect
		return
	}

	// Path preview on hover
	if b.HoverValid {
		if _, ok := b.Reachable[b.HoverHex]; ok && b.HoverHex != b.Player.Pos {
			b.MovePath = b.Grid.FindPath(b.Player.Pos, b.HoverHex, b.isBlocked)
		} else {
			b.MovePath = nil
		}
	} else {
		b.MovePath = nil
	}

	// Confirm move with left-click
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && b.HoverValid {
		if _, ok := b.Reachable[b.HoverHex]; ok && b.HoverHex != b.Player.Pos {
			b.Player.Pos = b.HoverHex
			b.HasMoved = true
			b.Reachable = nil
			b.MovePath = nil
			b.SelectedUnit = nil
			b.Phase = PhasePlayerSelect
		}
	}
}

func (b *BattleState) endTurn() {
	b.HasMoved = false
	b.SelectedUnit = nil
	b.Reachable = nil
	b.MovePath = nil

	b.TerrainMap.Tick()
	for _, u := range b.Units {
		if !u.IsDead {
			u.TickStatuses()
			if u.IsPlayer {
				u.RecoverMana(30)
			}
		}
	}
	b.TurnNumber++
	b.Phase = PhasePlayerSelect
}

// Draw renders the battle scene.
func (b *BattleState) Draw(screen *ebiten.Image) {
	// 1. Grid and terrain
	drawGrid(screen, b.Grid, b.TerrainMap)

	// 2. Reachable hexes
	if b.Reachable != nil {
		for h := range b.Reachable {
			if h != b.Player.Pos {
				drawHexHighlight(screen, b.Grid, h, colorReachable)
			}
		}
	}

	// 3. Path preview
	if b.MovePath != nil {
		for i, h := range b.MovePath {
			if i == 0 {
				continue
			}
			drawHexHighlight(screen, b.Grid, h, colorPath)
		}
	}

	// 4. Hover highlight
	if b.HoverValid {
		drawHexHighlight(screen, b.Grid, b.HoverHex, colorHover)
	}

	// 5. Units
	for _, u := range b.Units {
		if u.IsDead {
			continue
		}
		clr := colorEnemy
		if u.IsPlayer {
			clr = colorPlayer
		}
		drawUnit(screen, b.Grid, u.Pos, clr)
	}

	// 6. Selected unit outline
	if b.SelectedUnit != nil {
		sx, sy := b.Grid.HexToScreen(b.SelectedUnit.Pos)
		drawHexOutline(screen, sx, sy, b.Grid.Size*0.94, 2.5, colorSelected)
	}

	// 7. HUD
	b.drawHUD(screen)
}

func (b *BattleState) drawHUD(screen *ebiten.Image) {
	// Status bar
	info := fmt.Sprintf("Turn %d  |  HP: %d/%d  Mana: %d/%d",
		b.TurnNumber, b.Player.HP, b.Player.MaxHP, b.Player.Mana, b.Player.MaxMana)
	ebitenutil.DebugPrintAt(screen, info, 10, 4)

	// Controls
	var controls string
	switch b.Phase {
	case PhasePlayerSelect:
		if b.HasMoved {
			controls = "Enter/Space: End Turn  |  Esc: Undo Move"
		} else {
			controls = "Click unit to move  |  Enter/Space: End Turn"
		}
	case PhasePlayerMove:
		controls = "Click: Move  |  Esc/Right-click: Cancel"
	}
	ebitenutil.DebugPrintAt(screen, controls, 10, 20)

	// Hover info at bottom
	if b.HoverValid {
		col, row := b.HoverHex.ToOffset()
		hoverInfo := fmt.Sprintf("Hex (%d, %d)", col, row)
		t := b.TerrainMap.Get(b.HoverHex)
		if name, ok := terrainTypeNames[t.Type]; ok {
			hoverInfo += " " + name
		}
		if u := b.unitAt(b.HoverHex); u != nil {
			hoverInfo += fmt.Sprintf("  |  %s HP:%d/%d", u.Name, u.HP, u.MaxHP)
		}
		ebitenutil.DebugPrintAt(screen, hoverInfo, 10, 700)
	}
}

var terrainTypeNames = map[terrain.TerrainType]string{
	terrain.TerrainRock:          "[Rock]",
	terrain.TerrainStone:         "[Stone]",
	terrain.TerrainDirtWall:      "[Dirt Wall]",
	terrain.TerrainWoodWall:      "[Wood Wall]",
	terrain.TerrainGeneratedWall: "[Gen. Wall]",
	terrain.TerrainLava:          "[Lava]",
	terrain.TerrainPoisonSwamp:   "[Poison Swamp]",
	terrain.TerrainThorns:        "[Thorns]",
	terrain.TerrainElectricFloor: "[Electric]",
	terrain.TerrainFireFloor:     "[Fire]",
	terrain.TerrainWaterPuddle:   "[Water]",
	terrain.TerrainCliff:         "[Cliff]",
}
