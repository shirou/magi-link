package hex

import "math"

// Hex represents a hex cell in cube coordinates
type Hex struct {
	Q, R, S int
}

// NewHex creates a new Hex. Q + R + S must equal 0.
func NewHex(q, r int) Hex {
	return Hex{Q: q, R: r, S: -q - r}
}

// Add adds two hexes
func (h Hex) Add(other Hex) Hex {
	return Hex{Q: h.Q + other.Q, R: h.R + other.R, S: h.S + other.S}
}

// Subtract subtracts two hexes
func (h Hex) Subtract(other Hex) Hex {
	return Hex{Q: h.Q - other.Q, R: h.R - other.R, S: h.S - other.S}
}

// Length returns the distance from the origin
func (h Hex) Length() int {
	return (abs(h.Q) + abs(h.R) + abs(h.S)) / 2
}

// Distance returns the distance between two hexes
func (h Hex) Distance(other Hex) int {
	return h.Subtract(other).Length()
}

// Neighbors returns the 6 neighboring hexes
func (h Hex) Neighbors() []Hex {
	neighbors := make([]Hex, 6)
	for i, d := range directions {
		neighbors[i] = h.Add(d)
	}
	return neighbors
}

// Direction returns the hex in a given direction (0-5)
func (h Hex) Direction(dir int) Hex {
	return h.Add(directions[dir%6])
}

// LineTo returns all hexes in a straight line to target
func (h Hex) LineTo(target Hex) []Hex {
	n := h.Distance(target)
	if n == 0 {
		return []Hex{h}
	}
	results := make([]Hex, 0, n+1)
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		results = append(results, hexLerp(h, target, t))
	}
	return results
}

// Ring returns all hexes at exactly radius distance
func (h Hex) Ring(radius int) []Hex {
	if radius == 0 {
		return []Hex{h}
	}
	results := make([]Hex, 0, 6*radius)
	cur := h.Add(Hex{Q: directions[4].Q * radius, R: directions[4].R * radius, S: directions[4].S * radius})
	for i := 0; i < 6; i++ {
		for j := 0; j < radius; j++ {
			results = append(results, cur)
			cur = cur.Add(directions[i])
		}
	}
	return results
}

// Area returns all hexes within radius distance
func (h Hex) Area(radius int) []Hex {
	results := make([]Hex, 0)
	for q := -radius; q <= radius; q++ {
		r1 := max(-radius, -q-radius)
		r2 := min(radius, -q+radius)
		for r := r1; r <= r2; r++ {
			results = append(results, h.Add(NewHex(q, r)))
		}
	}
	return results
}

// ToPixel converts hex to pixel coordinates (pointy-top layout)
func (h Hex) ToPixel(size float64) (float64, float64) {
	x := size * (math.Sqrt(3)*float64(h.Q) + math.Sqrt(3)/2.0*float64(h.R))
	y := size * (3.0 / 2.0 * float64(h.R))
	return x, y
}

// FromPixel converts pixel to nearest hex (pointy-top layout)
func FromPixel(x, y, size float64) Hex {
	q := (math.Sqrt(3)/3.0*x - 1.0/3.0*y) / size
	r := (2.0 / 3.0 * y) / size
	return hexRound(q, r, -q-r)
}

// Grid represents the hex grid
type Grid struct {
	Width  int
	Height int
	Size   float64 // hex size in pixels
	OffX   float64 // screen offset X
	OffY   float64 // screen offset Y
}

// NewGrid creates a new grid
func NewGrid(width, height int, size, offX, offY float64) *Grid {
	return &Grid{
		Width:  width,
		Height: height,
		Size:   size,
		OffX:   offX,
		OffY:   offY,
	}
}

// InBounds checks if a hex is within grid bounds
func (g *Grid) InBounds(h Hex) bool {
	// Convert to offset coords for bounds check
	col := h.Q + (h.R-(h.R&1))/2
	row := h.R
	return col >= 0 && col < g.Width && row >= 0 && row < g.Height
}

// HexToScreen converts hex to screen pixel position
func (g *Grid) HexToScreen(h Hex) (float64, float64) {
	x, y := h.ToPixel(g.Size)
	return x + g.OffX, y + g.OffY
}

// ScreenToHex converts screen pixel position to nearest hex
func (g *Grid) ScreenToHex(x, y float64) Hex {
	return FromPixel(x-g.OffX, y-g.OffY, g.Size)
}

// --- helpers ---

var directions = []Hex{
	{1, 0, -1}, {1, -1, 0}, {0, -1, 1},
	{-1, 0, 1}, {-1, 1, 0}, {0, 1, -1},
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func hexRound(q, r, s float64) Hex {
	rq := math.Round(q)
	rr := math.Round(r)
	rs := math.Round(s)
	dq := math.Abs(rq - q)
	dr := math.Abs(rr - r)
	ds := math.Abs(rs - s)
	if dq > dr && dq > ds {
		rq = -rr - rs
	} else if dr > ds {
		rr = -rq - rs
	} else {
		rs = -rq - rr
	}
	return Hex{Q: int(rq), R: int(rr), S: int(rs)}
}

func hexLerp(a, b Hex, t float64) Hex {
	return hexRound(
		float64(a.Q)+float64(b.Q-a.Q)*t,
		float64(a.R)+float64(b.R-a.R)*t,
		float64(a.S)+float64(b.S-a.S)*t,
	)
}

// OffsetToHex converts odd-r offset coordinates to cube coordinates.
func OffsetToHex(col, row int) Hex {
	q := col - (row-(row&1))/2
	r := row
	return NewHex(q, r)
}

// ToOffset converts cube coordinates to odd-r offset coordinates.
func (h Hex) ToOffset() (col, row int) {
	col = h.Q + (h.R-(h.R&1))/2
	row = h.R
	return
}

// AllHexes returns all hexes within grid bounds in row-major order.
func (g *Grid) AllHexes() []Hex {
	hexes := make([]Hex, 0, g.Width*g.Height)
	for row := 0; row < g.Height; row++ {
		for col := 0; col < g.Width; col++ {
			hexes = append(hexes, OffsetToHex(col, row))
		}
	}
	return hexes
}

// FitInRect adjusts Size, OffX, OffY to fit and center the grid within a rectangle.
func (g *Grid) FitInRect(rx, ry, rw, rh float64) {
	allHexes := g.AllHexes()
	if len(allHexes) == 0 {
		return
	}

	var minX, maxX, minY, maxY float64
	for i, h := range allHexes {
		x, y := h.ToPixel(1.0)
		if i == 0 {
			minX, maxX, minY, maxY = x, x, y, y
		} else {
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}

	// Pointy-top hex extent from center: sqrt(3)/2 horizontally, 1.0 vertically
	unitW := (maxX - minX) + math.Sqrt(3)
	unitH := (maxY - minY) + 2.0

	g.Size = math.Min(rw/unitW, rh/unitH)
	centerX := (minX + maxX) / 2.0 * g.Size
	centerY := (minY + maxY) / 2.0 * g.Size

	g.OffX = rx + rw/2.0 - centerX
	g.OffY = ry + rh/2.0 - centerY
}

// Reachable returns all hexes reachable within moveRange steps via BFS.
// blocked reports whether a hex is impassable. Returns a map of hex to distance.
func (g *Grid) Reachable(start Hex, moveRange int, blocked func(Hex) bool) map[Hex]int {
	visited := map[Hex]int{start: 0}
	frontier := []Hex{start}

	for dist := 0; dist < moveRange; dist++ {
		var next []Hex
		for _, h := range frontier {
			for _, n := range h.Neighbors() {
				if _, seen := visited[n]; seen {
					continue
				}
				if !g.InBounds(n) {
					continue
				}
				if blocked != nil && blocked(n) {
					continue
				}
				visited[n] = dist + 1
				next = append(next, n)
			}
		}
		frontier = next
	}

	return visited
}

// FindPath returns the shortest path from start to goal using A*.
// Returns nil if no path found.
func (g *Grid) FindPath(start, goal Hex, blocked func(Hex) bool) []Hex {
	if start == goal {
		return []Hex{start}
	}

	type node struct {
		hex      Hex
		priority int
	}

	cameFrom := map[Hex]Hex{}
	cost := map[Hex]int{start: 0}
	frontier := []node{{start, 0}}

	for len(frontier) > 0 {
		best := 0
		for i := 1; i < len(frontier); i++ {
			if frontier[i].priority < frontier[best].priority {
				best = i
			}
		}
		cur := frontier[best].hex
		frontier[best] = frontier[len(frontier)-1]
		frontier = frontier[:len(frontier)-1]

		if cur == goal {
			path := []Hex{goal}
			h := goal
			for h != start {
				h = cameFrom[h]
				path = append(path, h)
			}
			for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
				path[i], path[j] = path[j], path[i]
			}
			return path
		}

		for _, next := range cur.Neighbors() {
			if !g.InBounds(next) {
				continue
			}
			if blocked != nil && blocked(next) {
				continue
			}
			newCost := cost[cur] + 1
			if old, ok := cost[next]; !ok || newCost < old {
				cost[next] = newCost
				frontier = append(frontier, node{next, newCost + next.Distance(goal)})
				cameFrom[next] = cur
			}
		}
	}

	return nil
}
