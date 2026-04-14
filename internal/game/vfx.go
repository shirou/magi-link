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

// HexFlash briefly highlights one hex in the given color.
type HexFlash struct {
	Pos     hex.Hex
	Color   color.RGBA
	Elapsed float64
}

const hexFlashDuration = 0.3

func NewHexFlash(pos hex.Hex, c color.RGBA) *HexFlash {
	return &HexFlash{Pos: pos, Color: c}
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
	drawHexHighlight(screen, grid, h.Pos, c)
}

// --- UnitTween ---

// UnitTween animates a unit from one hex to another. It does not draw the unit
// itself (the battle draws units separately); it blocks the queue to signal the
// move is in progress. Update-only.
type UnitTween struct {
	From, To hex.Hex
	Elapsed  float64
}

const unitTweenDuration = 0.2

func NewUnitTween(from, to hex.Hex) *UnitTween {
	return &UnitTween{From: from, To: to}
}

func (t *UnitTween) Update(dt float64) bool {
	t.Elapsed += dt
	return t.Elapsed >= unitTweenDuration
}

func (t *UnitTween) Draw(screen *ebiten.Image, grid *hex.Grid) {
	// No-op: unit rendering happens in the main draw pass. The tween's role
	// is to stall the queue so the player perceives the move as separate from
	// neighbouring events.
}

