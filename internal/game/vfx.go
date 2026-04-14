package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/shirou/magi_link/internal/hex"
)

// BattleEvent is one VFX animation that runs for a bounded duration.
// Update ticks time forward and returns true once the event is finished.
// Draw renders the event at its current state.
type BattleEvent interface {
	Update(dt float64) bool
	Draw(screen *ebiten.Image, grid *hex.Grid)
}

// VFXQueue plays battle events sequentially. Only the head event ticks and draws.
type VFXQueue struct {
	events []BattleEvent
}

func (q *VFXQueue) Push(e BattleEvent) {
	q.events = append(q.events, e)
}

func (q *VFXQueue) Update(dt float64) {
	if len(q.events) == 0 {
		return
	}
	if q.events[0].Update(dt) {
		q.events = q.events[1:]
	}
}

func (q *VFXQueue) Draw(screen *ebiten.Image, grid *hex.Grid) {
	if len(q.events) == 0 {
		return
	}
	q.events[0].Draw(screen, grid)
}

func (q *VFXQueue) IsIdle() bool {
	return len(q.events) == 0
}

// --- FloatingNumber ---

// FloatingNumber pops a number above a hex and fades upward.
type FloatingNumber struct {
	Pos     hex.Hex
	Text    string
	Color   color.RGBA
	Elapsed float64
}

const floatingNumberDuration = 0.6

func NewDamagePop(pos hex.Hex, amount int) *FloatingNumber {
	return &FloatingNumber{
		Pos:   pos,
		Text:  fmt.Sprintf("-%d", amount),
		Color: color.RGBA{255, 90, 90, 255},
	}
}

func NewHealPop(pos hex.Hex, amount int) *FloatingNumber {
	return &FloatingNumber{
		Pos:   pos,
		Text:  fmt.Sprintf("+%d", amount),
		Color: color.RGBA{120, 230, 120, 255},
	}
}

func (f *FloatingNumber) Update(dt float64) bool {
	f.Elapsed += dt
	return f.Elapsed >= floatingNumberDuration
}

func (f *FloatingNumber) Draw(screen *ebiten.Image, grid *hex.Grid) {
	sx, sy := grid.HexToScreen(f.Pos)
	t := f.Elapsed / floatingNumberDuration
	if t > 1 {
		t = 1
	}
	rise := 26.0 * t
	alpha := uint8(255 * (1 - t*t))
	c := f.Color
	c.A = alpha
	drawColoredText(screen, f.Text, int(sx)-len(f.Text)*3, int(sy)-int(rise)-8, c)
}

// --- HexFlash ---

// HexFlash briefly highlights one or more hexes in the given color. When an
// action lands on multiple hexes (area / line / ring) they all flash as a
// single event so the player sees one synchronised impact rather than a
// sequential ripple.
type HexFlash struct {
	Positions []hex.Hex
	Color     color.RGBA
	Elapsed   float64
}

const hexFlashDuration = 0.35

func NewHexFlash(positions []hex.Hex, c color.RGBA) *HexFlash {
	return &HexFlash{Positions: positions, Color: c}
}

func (h *HexFlash) Update(dt float64) bool {
	h.Elapsed += dt
	return h.Elapsed >= hexFlashDuration
}

func (h *HexFlash) Draw(screen *ebiten.Image, grid *hex.Grid) {
	t := h.Elapsed / hexFlashDuration
	if t > 1 {
		t = 1
	}
	// Fade from full alpha to 0 over the duration.
	alpha := uint8(float64(h.Color.A) * (1 - t))
	c := h.Color
	c.A = alpha
	for _, p := range h.Positions {
		drawHexHighlight(screen, grid, p, c)
	}
}

// --- UnitTween ---

// UnitTween animates a unit's circle from one hex to another. The main draw
// pass must skip drawing unit UnitID while the tween is the head event so
// the moving unit doesn't double-render at its (already updated) Pos.
type UnitTween struct {
	UnitID   int
	From, To hex.Hex
	Color    color.RGBA
	Elapsed  float64
}

const unitTweenDuration = 0.2

func NewUnitTween(unitID int, from, to hex.Hex, clr color.RGBA) *UnitTween {
	return &UnitTween{UnitID: unitID, From: from, To: to, Color: clr}
}

func (t *UnitTween) Update(dt float64) bool {
	t.Elapsed += dt
	return t.Elapsed >= unitTweenDuration
}

func (t *UnitTween) Draw(screen *ebiten.Image, grid *hex.Grid) {
	p := t.Elapsed / unitTweenDuration
	if p > 1 {
		p = 1
	}
	fx, fy := grid.HexToScreen(t.From)
	tx, ty := grid.HexToScreen(t.To)
	x := fx + (tx-fx)*p
	y := fy + (ty-fy)*p
	drawCircle(screen, x, y, grid.Size*0.35, t.Color)
}

// TweenHidesUnitID returns the unit ID that the head VFX is currently
// animating (so the battle draw can skip it), or -1 if none.
func (q *VFXQueue) TweenHidesUnitID() int {
	if len(q.events) == 0 {
		return -1
	}
	if tw, ok := q.events[0].(*UnitTween); ok {
		return tw.UnitID
	}
	return -1
}

