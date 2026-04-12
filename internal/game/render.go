package game

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/shirou/magi_link/internal/hex"
	"github.com/shirou/magi_link/internal/terrain"
)

var whiteImage *ebiten.Image

func init() {
	whiteImage = ebiten.NewImage(3, 3)
	whiteImage.Fill(color.White)
}

// Color palette
var (
	colorHexFill   = color.RGBA{30, 32, 42, 255}
	colorHexStroke = color.RGBA{55, 60, 80, 255}
	colorHover     = color.RGBA{70, 75, 100, 120}
	colorReachable = color.RGBA{40, 90, 160, 100}
	colorPath      = color.RGBA{80, 150, 230, 150}
	colorSelected  = color.RGBA{230, 210, 80, 255}
	colorPlayer    = color.RGBA{80, 180, 255, 255}
	colorEnemy     = color.RGBA{220, 70, 70, 255}
)

var terrainColors = map[terrain.TerrainType]color.RGBA{
	terrain.TerrainRock:          {60, 55, 50, 255},
	terrain.TerrainStone:         {80, 75, 70, 255},
	terrain.TerrainDirtWall:      {100, 75, 45, 255},
	terrain.TerrainWoodWall:      {120, 85, 50, 255},
	terrain.TerrainGeneratedWall: {90, 90, 120, 255},
	terrain.TerrainLava:          {200, 60, 20, 255},
	terrain.TerrainPoisonSwamp:   {50, 100, 40, 255},
	terrain.TerrainThorns:        {60, 120, 50, 255},
	terrain.TerrainElectricFloor: {200, 200, 60, 255},
	terrain.TerrainFireFloor:     {200, 100, 30, 255},
	terrain.TerrainWaterPuddle:   {40, 80, 160, 255},
	terrain.TerrainCliff:         {40, 35, 30, 255},
}

func getTerrainColor(t terrain.TerrainType) color.RGBA {
	if c, ok := terrainColors[t]; ok {
		return c
	}
	return colorHexFill
}

// hexVertices returns the 6 vertices of a pointy-top hexagon centered at (cx, cy).
func hexVertices(cx, cy, size float64) [6][2]float64 {
	var v [6][2]float64
	for i := 0; i < 6; i++ {
		angle := math.Pi/3.0*float64(i) + math.Pi/6.0
		v[i][0] = cx + size*math.Cos(angle)
		v[i][1] = cy + size*math.Sin(angle)
	}
	return v
}

func hexPath(cx, cy, size float64) vector.Path {
	verts := hexVertices(cx, cy, size)
	var p vector.Path
	p.MoveTo(float32(verts[0][0]), float32(verts[0][1]))
	for i := 1; i < 6; i++ {
		p.LineTo(float32(verts[i][0]), float32(verts[i][1]))
	}
	p.Close()
	return p
}

func drawTrianglesColored(screen *ebiten.Image, vs []ebiten.Vertex, is []uint16, clr color.RGBA) {
	cr := float32(clr.R) / 0xff
	cg := float32(clr.G) / 0xff
	cb := float32(clr.B) / 0xff
	ca := float32(clr.A) / 0xff
	for i := range vs {
		vs[i].SrcX = 1
		vs[i].SrcY = 1
		vs[i].ColorR = cr
		vs[i].ColorG = cg
		vs[i].ColorB = cb
		vs[i].ColorA = ca
	}
	screen.DrawTriangles(vs, is, whiteImage, &ebiten.DrawTrianglesOptions{AntiAlias: true})
}

func drawFilledHex(screen *ebiten.Image, cx, cy, size float64, clr color.RGBA) {
	p := hexPath(cx, cy, size)
	vs, is := p.AppendVerticesAndIndicesForFilling(nil, nil)
	drawTrianglesColored(screen, vs, is, clr)
}

func drawHexOutline(screen *ebiten.Image, cx, cy, size, width float64, clr color.RGBA) {
	p := hexPath(cx, cy, size)
	vs, is := p.AppendVerticesAndIndicesForStroke(nil, nil, &vector.StrokeOptions{
		Width:    float32(width),
		LineJoin: vector.LineJoinRound,
	})
	drawTrianglesColored(screen, vs, is, clr)
}

func drawCircle(screen *ebiten.Image, cx, cy, radius float64, clr color.RGBA) {
	const segments = 24
	var p vector.Path
	for i := 0; i < segments; i++ {
		angle := 2 * math.Pi * float64(i) / segments
		x := float32(cx + radius*math.Cos(angle))
		y := float32(cy + radius*math.Sin(angle))
		if i == 0 {
			p.MoveTo(x, y)
		} else {
			p.LineTo(x, y)
		}
	}
	p.Close()
	vs, is := p.AppendVerticesAndIndicesForFilling(nil, nil)
	drawTrianglesColored(screen, vs, is, clr)
}

// drawGrid renders the entire hex grid with terrain colors.
func drawGrid(screen *ebiten.Image, grid *hex.Grid, tm *terrain.Map) {
	hexSize := grid.Size * 0.94
	for _, h := range grid.AllHexes() {
		sx, sy := grid.HexToScreen(h)
		t := tm.Get(h)
		drawFilledHex(screen, sx, sy, hexSize, getTerrainColor(t.Type))
		drawHexOutline(screen, sx, sy, hexSize, 1.5, colorHexStroke)
	}
}

// drawHexHighlight draws a colored overlay on a hex.
func drawHexHighlight(screen *ebiten.Image, grid *hex.Grid, h hex.Hex, clr color.RGBA) {
	sx, sy := grid.HexToScreen(h)
	drawFilledHex(screen, sx, sy, grid.Size*0.88, clr)
}

// drawUnit draws a unit as a filled circle on its hex.
func drawUnit(screen *ebiten.Image, grid *hex.Grid, h hex.Hex, clr color.RGBA) {
	sx, sy := grid.HexToScreen(h)
	drawCircle(screen, sx, sy, grid.Size*0.35, clr)
}
