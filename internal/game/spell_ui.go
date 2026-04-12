package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/shirou/magi_link/internal/spell"
)

// Spell panel layout constants
const (
	panelY    = 580
	panelH    = 140
	slotW     = 74
	slotH     = 28
	slotGap   = 4
	slotsX    = 95
	labelX    = 10
	bookRowY  = panelY + 8
	chainRowY = panelY + 46
	castRowY  = panelY + 84
	castBtnW  = 72
	arrowW    = 14 // width reserved for ">" between chain slots
)

// Spell panel colors
var (
	colorPanelBG    = color.RGBA{18, 18, 28, 230}
	colorPanelLine  = color.RGBA{50, 55, 75, 255}
	colorSlotTarget = color.RGBA{30, 60, 110, 255}
	colorSlotAction = color.RGBA{100, 35, 35, 255}
	colorSlotHovT   = color.RGBA{45, 80, 140, 255}
	colorSlotHovA   = color.RGBA{130, 50, 50, 255}
	colorChainSlotC = color.RGBA{42, 45, 60, 255}
	colorChainHovC  = color.RGBA{62, 65, 85, 255}
	colorCastOK     = color.RGBA{30, 105, 45, 255}
	colorCastHov    = color.RGBA{45, 135, 60, 255}
	colorCastNo     = color.RGBA{65, 30, 30, 255}
	colorManaWarn   = color.RGBA{230, 55, 55, 255}
)

// Pre-allocated image for colored text rendering to avoid per-frame GPU allocation.
var warnTextImg *ebiten.Image

func init() {
	// "Not enough mana!" = 16 chars × 6px = 96px wide, 16px tall
	warnTextImg = ebiten.NewImage(96, 16)
}

// updateSpellUI handles input for the spell panel.
func (b *BattleState) updateSpellUI() {
	mx, my := ebiten.CursorPosition()

	b.HoverBookIdx = -1
	b.HoverChainIdx = -1
	b.HoverCast = false

	// Check spellbook slot hover
	for i := range b.Book.Spells {
		sx := int(slotsX) + i*(slotW+slotGap)
		if mx >= sx && mx < sx+slotW && my >= bookRowY && my < bookRowY+slotH {
			b.HoverBookIdx = i
			break
		}
	}

	// Check chain slot hover
	cx := int(slotsX)
	for i := range b.Chain.Slots {
		if i > 0 {
			cx += arrowW
		}
		if mx >= cx && mx < cx+slotW && my >= chainRowY && my < chainRowY+slotH {
			b.HoverChainIdx = i
			break
		}
		cx += slotW + slotGap
	}

	// Check cast button hover
	castX := int(slotsX)
	if mx >= castX && mx < castX+castBtnW && my >= castRowY && my < castRowY+slotH {
		b.HoverCast = true
	}

	// Cache chain cost for this frame (avoids recomputing in canCast + Draw)
	b.cachedChainCost = b.Chain.TotalCost()

	// Handle left-click
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if b.HoverBookIdx >= 0 && b.HoverBookIdx < len(b.Book.Spells) {
			if b.Chain.CanAdd() {
				s := b.Book.Spells[b.HoverBookIdx]
				b.Chain.Slots = append(b.Chain.Slots, &spell.SpellSlot{Spell: s})
				b.cachedChainCost = b.Chain.TotalCost()
			}
		} else if b.HoverChainIdx >= 0 && b.HoverChainIdx < len(b.Chain.Slots) {
			idx := b.HoverChainIdx
			b.Chain.Slots = append(b.Chain.Slots[:idx], b.Chain.Slots[idx+1:]...)
			b.HoverChainIdx = -1
			b.cachedChainCost = b.Chain.TotalCost()
		} else if b.HoverCast && b.canCast() {
			b.castChain()
		}
	}

	// Keyboard shortcuts
	if inpututil.IsKeyJustPressed(ebiten.KeyX) && len(b.Chain.Slots) > 0 {
		b.Chain.Slots = b.Chain.Slots[:0]
		b.cachedChainCost = 0
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyC) && b.canCast() {
		b.castChain()
	}
}

// canCast returns true if the chain has spells and mana is sufficient.
func (b *BattleState) canCast() bool {
	return len(b.Chain.Slots) > 0 && b.cachedChainCost <= b.Player.Mana
}

// castChain executes the spell chain and deducts mana.
func (b *BattleState) castChain() {
	b.Player.UseMana(b.cachedChainCost)

	// Record spells used for passive triggers
	for _, slot := range b.Chain.Slots {
		if slot.Spell != nil {
			b.TurnStats.SpellsUsed[slot.Spell.ID]++
		}
	}

	b.Chain.Slots = b.Chain.Slots[:0]
	b.cachedChainCost = 0
}

// isInSpellPanel returns true if the y coordinate is in the spell panel area.
func isInSpellPanel(my int) bool {
	return my >= panelY
}

// drawSpellPanel renders the entire spell panel (spellbook, chain, cast).
func (b *BattleState) drawSpellPanel(screen *ebiten.Image) {
	sw := screen.Bounds().Dx()

	// Panel background
	vector.DrawFilledRect(screen, 0, panelY, float32(sw), panelH, colorPanelBG, false)
	// Top border line
	vector.DrawFilledRect(screen, 0, panelY, float32(sw), 2, colorPanelLine, false)

	b.drawBookRow(screen)
	b.drawChainRow(screen)
	b.drawCastRow(screen, sw)
}

func (b *BattleState) drawBookRow(screen *ebiten.Image) {
	ebitenutil.DebugPrintAt(screen, "Book:", int(labelX), bookRowY+6)

	for i, s := range b.Book.Spells {
		sx := float32(slotsX + i*(slotW+slotGap))

		// Determine slot color based on spell type and hover
		var bg color.RGBA
		if s.IsTarget() {
			bg = colorSlotTarget
			if b.HoverBookIdx == i {
				bg = colorSlotHovT
			}
		} else {
			bg = colorSlotAction
			if b.HoverBookIdx == i {
				bg = colorSlotHovA
			}
		}

		vector.DrawFilledRect(screen, sx, bookRowY, slotW, slotH, bg, false)

		// Spell name
		text := b.SpellReg.Text(s.ID)
		name := truncate(text.Name, 9)
		ebitenutil.DebugPrintAt(screen, name, int(sx)+3, bookRowY+2)

		// Base cost in corner
		costStr := fmt.Sprintf("%d", s.BaseCost)
		ebitenutil.DebugPrintAt(screen, costStr, int(sx)+slotW-len(costStr)*6-3, bookRowY+16)
	}
}

func (b *BattleState) drawChainRow(screen *ebiten.Image) {
	ebitenutil.DebugPrintAt(screen, "Chain:", int(labelX), chainRowY+6)

	if len(b.Chain.Slots) == 0 {
		ebitenutil.DebugPrintAt(screen, "(click spells above to build chain)", int(slotsX), chainRowY+6)
		return
	}

	// Compute all slot costs in a single O(n) pass
	costs := b.Chain.SlotCosts()

	cx := float32(slotsX)
	for i, slot := range b.Chain.Slots {
		// Draw arrow between slots
		if i > 0 {
			ebitenutil.DebugPrintAt(screen, ">", int(cx)+3, chainRowY+6)
			cx += arrowW
		}

		bg := colorChainSlotC
		if b.HoverChainIdx == i {
			bg = colorChainHovC
		}
		vector.DrawFilledRect(screen, cx, chainRowY, slotW, slotH, bg, false)

		text := b.SpellReg.Text(slot.Spell.ID)
		name := truncate(text.Name, 9)
		ebitenutil.DebugPrintAt(screen, name, int(cx)+3, chainRowY+2)

		// Show individual cost (from pre-computed array)
		costStr := fmt.Sprintf("%d", costs[i])
		ebitenutil.DebugPrintAt(screen, costStr, int(cx)+slotW-len(costStr)*6-3, chainRowY+16)

		cx += slotW + slotGap
	}
}

func (b *BattleState) drawCastRow(screen *ebiten.Image, screenW int) {
	totalCost := b.cachedChainCost
	hasMana := totalCost <= b.Player.Mana
	hasChain := len(b.Chain.Slots) > 0

	// Cast button
	castX := float32(slotsX)
	var btnColor color.RGBA
	if !hasChain || !hasMana {
		btnColor = colorCastNo
	} else if b.HoverCast {
		btnColor = colorCastHov
	} else {
		btnColor = colorCastOK
	}
	vector.DrawFilledRect(screen, castX, castRowY, castBtnW, slotH, btnColor, false)

	castLabel := "Cast"
	if hasChain {
		castLabel = fmt.Sprintf("Cast(%d)", totalCost)
	}
	ebitenutil.DebugPrintAt(screen, castLabel, int(castX)+3, castRowY+6)

	// Mana display
	manaX := int(castX) + castBtnW + 16
	manaStr := fmt.Sprintf("Mana: %d/%d", b.Player.Mana, b.Player.MaxMana)
	ebitenutil.DebugPrintAt(screen, manaStr, manaX, castRowY+6)

	// Mana warning (uses pre-allocated image instead of per-frame allocation)
	if hasChain && !hasMana {
		warnX := manaX + len(manaStr)*6 + 12
		drawWarnText(screen, "Not enough mana!", warnX, castRowY+6)
	}

	// Controls hint
	hintStr := "C:Cast  X:Clear"
	ebitenutil.DebugPrintAt(screen, hintStr, screenW-len(hintStr)*6-16, castRowY+6)
}

// drawWarnText draws warning text in red using the pre-allocated image.
func drawWarnText(screen *ebiten.Image, text string, x, y int) {
	warnTextImg.Clear()
	ebitenutil.DebugPrintAt(warnTextImg, text, 0, 0)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.Scale(
		float32(colorManaWarn.R)/255.0,
		float32(colorManaWarn.G)/255.0,
		float32(colorManaWarn.B)/255.0,
		float32(colorManaWarn.A)/255.0,
	)
	screen.DrawImage(warnTextImg, op)
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "."
}
