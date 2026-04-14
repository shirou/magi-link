package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/shirou/magi_link/internal/entity"
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/resolve"
	"github.com/shirou/magi_link/internal/spell"
	"github.com/shirou/magi_link/internal/terrain"
)

// BattlePhase represents the current phase of battle.
type BattlePhase int

const (
	PhasePlayerSelect BattlePhase = iota
	PhasePlayerMove
	PhaseChainTarget // player built a chain, now picking a hex to cast on
	PhaseEnemyTurn
)

// BattleOutcome indicates whether the battle has ended.
type BattleOutcome int

const (
	OutcomeNone BattleOutcome = iota
	OutcomeVictory
	OutcomeDefeat
)

const (
	GridWidth  = 12
	GridHeight = 10
)

// vfxDt is the fixed time step fed to the VFX queue per Update tick.
// Ebiten calls Update at ~60 Hz.
const vfxDt = 1.0 / 60.0

var (
	colorChainTargetHex = color.RGBA{200, 160, 40, 100}
	colorCastPreview    = color.RGBA{230, 180, 70, 110}
	colorFlashHit       = color.RGBA{255, 220, 120, 220}
	colorFlashHeal      = color.RGBA{120, 240, 140, 200}
	colorProjectile     = color.RGBA{255, 240, 180, 255}
	colorDead           = color.RGBA{90, 90, 90, 255}
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

	SelectedUnit   *entity.Unit
	Reachable      map[hex.Hex]int
	MovePath       []hex.Hex
	MoveOrigin     hex.Hex
	HasMoved       bool
	TurnNumber     int
	EnemyReachable map[hex.Hex]int

	// Spell system
	SpellReg  *spell.Registry
	Book      *spell.SpellBook
	Chain     spell.Chain
	TurnStats *spell.TurnStats

	// Spell UI state
	HoverBookIdx    int
	HoverChainIdx   int
	HoverCast       bool
	cachedChainCost int // cached per frame to avoid recomputing TotalCost()

	// VFX playback and turn state.
	VFX            VFXQueue
	Outcome        BattleOutcome
	enemyTurnQueue []*entity.Unit

	// CastPreview holds the hexes that the current chain would affect if
	// cast at HoverHex. Recomputed each frame during PhaseChainTarget.
	CastPreview []hex.Hex
}

// NewBattle creates a new battle with initial setup.
func NewBattle(screenW, screenH int, reg *spell.Registry) *BattleState {
	grid := hex.NewGrid(GridWidth, GridHeight, 0, 0, 0)
	// Fit grid into upper area, leaving room for spell panel at bottom
	gw := float64(screenW) * 0.70
	gridTop := 30.0
	gridBottom := float64(screenH) - float64(panelH) - 16
	gh := gridBottom - gridTop
	ox := (float64(screenW) - gw) / 2
	grid.FitInRect(ox, gridTop, gw, gh)

	tm := terrain.NewMap(GridWidth, GridHeight)
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
	enemy1.MeleeDamage = 6
	enemy2 := entity.NewUnit(3, "Shard", hex.OffsetToHex(9, 6), 40, 0)
	enemy2.MeleeDamage = 10
	enemy3 := entity.NewUnit(4, "Chorus", hex.OffsetToHex(7, 7), 25, 0)
	enemy3.MeleeDamage = 4

	// Initialize spellbook with starter spells. More variety than the minimum
	// so combo discovery has room during self-playtest.
	book := spell.NewSpellBook()
	starterSpells := []string{
		"fireball", "ice", "lightning", "water", "poison",
		"slash", "heal", "push",
		"single", "self", "line", "area", "3way", "pierce",
	}
	for _, id := range starterSpells {
		if s := reg.Get(id); s != nil {
			book.Add(s)
		}
	}

	return &BattleState{
		Grid:       grid,
		TerrainMap: tm,
		Units:      []*entity.Unit{player, enemy1, enemy2, enemy3},
		Player:     player,
		Phase:      PhasePlayerSelect,
		TurnNumber: 1,

		SpellReg:      reg,
		Book:          book,
		TurnStats:     spell.NewTurnStats(),
		HoverBookIdx:  -1,
		HoverChainIdx: -1,
	}
}

// --- resolve.Battlefield interface implementation ---

func (b *BattleState) UnitAt(h hex.Hex) *entity.Unit  { return b.unitAt(h) }
func (b *BattleState) AllUnits() []*entity.Unit        { return b.Units }
func (b *BattleState) TerrainAt(h hex.Hex) *terrain.Terrain { return b.TerrainMap.Get(h) }
func (b *BattleState) GridBounds() *hex.Grid           { return b.Grid }
func (b *BattleState) IsWall(h hex.Hex) bool           { return b.TerrainMap.Get(h).IsWall() }
func (b *BattleState) HasLineOfSight(from, to hex.Hex) bool {
	return b.TerrainMap.HasLineOfSight(from, to)
}

// --- internal helpers ---

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
	// VFX is authoritative for input blocking: while events are playing,
	// state mutations have already happened but the player hasn't seen them
	// yet, so accepting input would let them act on invisible state.
	b.VFX.Update(vfxDt)
	if !b.VFX.IsIdle() {
		return
	}

	b.checkOutcome()
	if b.Outcome != OutcomeNone {
		return
	}

	if b.Phase == PhaseEnemyTurn {
		b.tickEnemyTurn()
		return
	}

	mx, my := ebiten.CursorPosition()

	// Spell UI input (only during select phase — not during targeting)
	if b.Phase != PhaseChainTarget {
		b.updateSpellUI()
	}

	// Hex hover — only outside the panel area
	if !isInSpellPanel(my) {
		b.HoverHex = b.Grid.ScreenToHex(float64(mx), float64(my))
		b.HoverValid = b.Grid.InBounds(b.HoverHex)
	} else {
		b.HoverValid = false
	}

	// Enemy hover reachable (not during chain targeting)
	b.EnemyReachable = nil
	if b.HoverValid && b.Phase != PhaseChainTarget {
		if u := b.unitAt(b.HoverHex); u != nil && !u.IsPlayer {
			b.EnemyReachable = b.Grid.Reachable(u.Pos, u.MoveRange, b.isBlocked)
		}
	}

	switch b.Phase {
	case PhasePlayerSelect:
		b.updatePlayerSelect()
	case PhasePlayerMove:
		b.updatePlayerMove()
	case PhaseChainTarget:
		b.updateChainTarget()
	}
}

// checkOutcome sets b.Outcome if the battle has been decided.
func (b *BattleState) checkOutcome() {
	if b.Outcome != OutcomeNone {
		return
	}
	if !b.Player.IsAlive() {
		b.Outcome = OutcomeDefeat
		return
	}
	for _, u := range b.Units {
		if !u.IsPlayer && u.IsAlive() {
			return
		}
	}
	b.Outcome = OutcomeVictory
}

// tickEnemyTurn processes one enemy from the queue. When empty, the turn
// returns to the player. Dead units are skipped silently.
func (b *BattleState) tickEnemyTurn() {
	for len(b.enemyTurnQueue) > 0 {
		u := b.enemyTurnQueue[0]
		b.enemyTurnQueue = b.enemyTurnQueue[1:]
		if !u.IsAlive() {
			continue
		}
		act := DecideEnemyAction(u, b)
		b.executeEnemyAction(u, act)
		return
	}
	b.Phase = PhasePlayerSelect
	b.refillPlayerTurn()
}

// executeEnemyAction applies an enemy decision and enqueues its VFX.
// Melee damage routes through applyLinkResult so it shares Phase 2 with
// spells.
func (b *BattleState) executeEnemyAction(u *entity.Unit, act EnemyAction) {
	switch act.Kind {
	case ActionMove:
		from := u.Pos
		u.Pos = act.Move
		b.VFX.Push(NewUnitTween(u.ID, from, act.Move, unitColor(u)))
	case ActionMelee:
		if act.Target == nil || !act.Target.IsAlive() {
			return
		}
		b.applyLinkResult(resolve.LinkResult{
			Damage: map[int]int{act.Target.ID: u.MeleeDamage},
			Hexes:  []hex.Hex{act.Target.Pos},
		})
	case ActionWait:
		// Nothing; the turn simply advances.
	}
}

// refillPlayerTurn runs the end-of-enemy-turn housekeeping that starts
// the player's next turn.
func (b *BattleState) refillPlayerTurn() {
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
	b.HasMoved = false
	b.TurnStats = spell.NewTurnStats()
}

func (b *BattleState) updatePlayerSelect() {
	_, my := ebiten.CursorPosition()

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

	// Skip hex clicks if in spell panel
	if isInSpellPanel(my) {
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

// updateChainTarget handles hex selection after the player presses Cast.
func (b *BattleState) updateChainTarget() {
	b.refreshCastPreview()

	// Cancel with right-click or Escape → back to select (chain preserved)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) ||
		inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		b.Phase = PhasePlayerSelect
		b.CastPreview = nil
		return
	}

	// Confirm target with left-click on a valid hex. Ignore clicks that would
	// produce no effect (preview empty — e.g. target spell's range not met);
	// otherwise the player loses mana with no visible outcome.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && b.HoverValid {
		if len(b.CastPreview) == 0 {
			return
		}
		b.executeLinkAt(b.HoverHex)
		b.Phase = PhasePlayerSelect
		b.CastPreview = nil
	}
}

// refreshCastPreview dry-runs the current chain at the hover hex so the
// player can see the affected hexes before committing the cast. resolve is
// pure (no bf mutation), so this is safe to call every frame.
func (b *BattleState) refreshCastPreview() {
	if !b.HoverValid || len(b.Chain.Slots) == 0 {
		b.CastPreview = nil
		return
	}

	spells := make([]*spell.SpellDef, 0, len(b.Chain.Slots))
	for _, slot := range b.Chain.Slots {
		if slot.Spell != nil {
			spells = append(spells, slot.Spell)
		}
	}
	cx, cy := b.Grid.HexToScreen(b.Player.Pos)
	tx, ty := b.Grid.HexToScreen(b.HoverHex)
	dir := hex.AngleToDirection(tx-cx, ty-cy)

	result := resolve.ExecuteLink(resolve.LinkInput{
		CasterPos:  b.Player.Pos,
		ClickedHex: b.HoverHex,
		Direction:  dir,
		Spells:     spells,
	}, b)
	b.CastPreview = result.Hexes
}

// executeLinkAt runs the resolve pipeline and applies the result.
func (b *BattleState) executeLinkAt(target hex.Hex) {
	b.Player.UseMana(b.cachedChainCost)

	// Build spell list from chain slots
	spells := make([]*spell.SpellDef, 0, len(b.Chain.Slots))
	for _, slot := range b.Chain.Slots {
		if slot.Spell != nil {
			spells = append(spells, slot.Spell)
			b.TurnStats.SpellsUsed[slot.Spell.ID]++
		}
	}

	// Compute direction from caster to clicked hex
	cx, cy := b.Grid.HexToScreen(b.Player.Pos)
	tx, ty := b.Grid.HexToScreen(target)
	dir := hex.AngleToDirection(tx-cx, ty-cy)

	result := resolve.ExecuteLink(resolve.LinkInput{
		CasterPos:  b.Player.Pos,
		ClickedHex: target,
		Direction:  dir,
		Spells:     spells,
	}, b)

	// Projectile plays first so the impact flash/pop that applyLinkResult
	// enqueues fires right after the bolt arrives. Aim at the farthest
	// affected hex so line spells follow the line instead of stopping at
	// the clicked hex.
	end := projectileEndpoint(b.Player.Pos, target, result.Hexes)
	b.VFX.Push(NewProjectile(b.Player.Pos, end, colorProjectile))
	b.applyLinkResult(result)

	b.Chain.Slots = b.Chain.Slots[:0]
	b.cachedChainCost = 0
}

// applyLinkResult mutates the battlefield from a LinkResult's diff and
// enqueues the corresponding VFX. State mutation is immediate; VFX plays
// back over several frames while input is blocked.
//
// Dead-check on heal / status / move is intentional: earlier damage
// in the same result may have killed the unit.
func (b *BattleState) applyLinkResult(result resolve.LinkResult) {
	if len(result.Hexes) > 0 {
		b.VFX.Push(NewHexFlash(result.Hexes, colorFlashHit))
	}

	for unitID, dmg := range result.Damage {
		u := b.unitByID(unitID)
		if !u.IsAlive() {
			continue
		}
		pos := u.Pos
		actual := u.TakeDamage(dmg)
		b.TurnStats.DamageDealt += actual
		b.TurnStats.UnitsAttacked[unitID] = true
		if actual > 0 {
			b.VFX.Push(NewDamagePop(pos, actual))
		}
	}
	for unitID, heal := range result.Healing {
		u := b.unitByID(unitID)
		if !u.IsAlive() {
			continue
		}
		actual := u.Heal(heal)
		b.TurnStats.HealingDone += actual
		if actual > 0 {
			b.VFX.Push(NewHealPop(u.Pos, actual))
			b.VFX.Push(NewHexFlash([]hex.Hex{u.Pos}, colorFlashHeal))
		}
	}
	for _, sc := range result.StatusApplied {
		u := b.unitByID(sc.UnitID)
		if !u.IsAlive() {
			continue
		}
		u.ApplyStatus(sc.Status, sc.Turns)
		b.TurnStats.StatusesApplied++
	}
	for _, sc := range result.StatusRemoved {
		// Removing a burn from a corpse is harmless; don't filter on IsDead.
		if u := b.unitByID(sc.UnitID); u != nil {
			delete(u.Statuses, sc.Status)
		}
	}
	for _, tc := range result.TerrainChanges {
		b.TerrainMap.Set(tc.Pos, &terrain.Terrain{Type: tc.Type, Duration: -1})
	}
	for _, mv := range result.UnitsMoved {
		u := b.unitByID(mv.UnitID)
		if !u.IsAlive() {
			continue
		}
		u.Pos = mv.To
		b.VFX.Push(NewUnitTween(u.ID, mv.From, mv.To, unitColor(u)))
	}
}

// unitByID returns the unit with the given ID, or nil if not found.
func (b *BattleState) unitByID(id int) *entity.Unit {
	for _, u := range b.Units {
		if u.ID == id {
			return u
		}
	}
	return nil
}

// endTurn hands control to the enemy. The per-turn housekeeping
// (terrain tick, status tick, mana recovery, turn counter) runs when the
// enemy queue finishes — see refillPlayerTurn.
func (b *BattleState) endTurn() {
	b.SelectedUnit = nil
	b.Reachable = nil
	b.MovePath = nil
	b.Chain.Slots = nil

	b.enemyTurnQueue = b.enemyTurnQueue[:0]
	for _, u := range b.Units {
		if !u.IsPlayer && u.IsAlive() {
			b.enemyTurnQueue = append(b.enemyTurnQueue, u)
		}
	}
	b.Phase = PhaseEnemyTurn
}

// Draw renders the battle scene.
func (b *BattleState) Draw(screen *ebiten.Image) {
	// 1. Grid and terrain
	drawGrid(screen, b.Grid, b.TerrainMap)

	// 2. Enemy hover reachable
	if b.EnemyReachable != nil {
		for h := range b.EnemyReachable {
			drawHexHighlight(screen, b.Grid, h, colorEnemyReachable)
		}
	}

	// 3. Reachable hexes (player)
	if b.Reachable != nil {
		for h := range b.Reachable {
			if h != b.Player.Pos {
				drawHexHighlight(screen, b.Grid, h, colorReachable)
			}
		}
	}

	// 4. Path preview
	if b.MovePath != nil {
		for i, h := range b.MovePath {
			if i == 0 {
				continue
			}
			drawHexHighlight(screen, b.Grid, h, colorPath)
		}
	}

	// 5. Chain target preview (affected hexes) + hover highlight
	if b.Phase == PhaseChainTarget {
		for _, h := range b.CastPreview {
			drawHexHighlight(screen, b.Grid, h, colorCastPreview)
		}
		if b.HoverValid {
			drawHexHighlight(screen, b.Grid, b.HoverHex, colorChainTargetHex)
		}
	} else if b.HoverValid {
		drawHexHighlight(screen, b.Grid, b.HoverHex, colorHover)
	}

	// 6. Units (dead first so living draw on top). Skip the unit currently
	//    being animated by a UnitTween so it doesn't double-render.
	tweenedID := b.VFX.TweenHidesUnitID()
	for _, u := range b.Units {
		if !u.IsDead {
			continue
		}
		drawUnit(screen, b.Grid, u.Pos, colorDead)
	}
	for _, u := range b.Units {
		if u.IsDead || u.ID == tweenedID {
			continue
		}
		drawUnit(screen, b.Grid, u.Pos, unitColor(u))
		drawStatusIcons(screen, b.Grid, u)
	}

	// 7. Selected unit outline
	if b.SelectedUnit != nil {
		sx, sy := b.Grid.HexToScreen(b.SelectedUnit.Pos)
		drawHexOutline(screen, sx, sy, b.Grid.Size*0.94, 2.5, colorSelected)
	}

	// 8. VFX layer (on top of units, under the panel).
	b.VFX.Draw(screen, b.Grid)

	// 9. Spell panel
	b.drawSpellPanel(screen)

	// 10. HUD (on top)
	b.drawHUD(screen)
}

// projectileEndpoint picks where the bolt should land: the affected hex
// farthest from the caster, tie-breaking by proximity to the clicked hex.
// Falls back to clicked when hexes is empty.
func projectileEndpoint(caster, clicked hex.Hex, hexes []hex.Hex) hex.Hex {
	end := clicked
	bestDist := -1
	bestTieBreak := 0
	for _, h := range hexes {
		d := caster.Distance(h)
		if d < bestDist {
			continue
		}
		tie := -h.Distance(clicked) // closer to clicked wins tie
		if d > bestDist || tie > bestTieBreak {
			bestDist = d
			bestTieBreak = tie
			end = h
		}
	}
	return end
}

func unitColor(u *entity.Unit) color.RGBA {
	if u.IsPlayer {
		return colorPlayer
	}
	return colorEnemy
}

// statusDisplayOrder fixes the order of status letters so the badge strip
// doesn't flicker frame-to-frame (Go's map iteration is randomized).
var statusDisplayOrder = []struct {
	Status entity.StatusEffect
	Letter byte
}{
	{entity.StatusBurning, 'B'},
	{entity.StatusFrozen, 'F'},
	{entity.StatusPoisoned, 'P'},
	{entity.StatusBleeding, 'b'},
	{entity.StatusWet, 'W'},
	{entity.StatusElectrified, 'E'},
}

func drawStatusIcons(screen *ebiten.Image, grid *hex.Grid, u *entity.Unit) {
	if len(u.Statuses) == 0 {
		return
	}
	letters := make([]byte, 0, len(u.Statuses))
	for _, entry := range statusDisplayOrder {
		if u.HasStatus(entry.Status) {
			letters = append(letters, entry.Letter)
		}
	}
	if len(letters) == 0 {
		return
	}
	sx, sy := grid.HexToScreen(u.Pos)
	ebitenutil.DebugPrintAt(screen, string(letters), int(sx)-len(letters)*3, int(sy)+int(grid.Size*0.4))
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
	case PhaseChainTarget:
		controls = "Click hex to cast chain  |  Esc/Right-click: Cancel"
	}
	ebitenutil.DebugPrintAt(screen, controls, 10, 20)

	// Hover info
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
		ebitenutil.DebugPrintAt(screen, hoverInfo, 10, panelY-18)
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
