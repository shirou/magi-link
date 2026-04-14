package hex

import (
	"math"
	"testing"
)

// TestAngleToDirectionMatchesHexGeometry ensures each hex direction's
// geometric position decodes back to its index. Uses HexToScreen-equivalent
// vectors (pointy-top layout) with screen-space (Y-down) deltas.
func TestAngleToDirectionMatchesHexGeometry(t *testing.T) {
	// Positions of each direction hex relative to origin in screen space:
	// dx = sqrt(3)*Q + sqrt(3)/2*R, dy_screen = 3/2*R (screen Y grows down).
	for i, d := range directions {
		dx := math.Sqrt(3)*float64(d.Q) + math.Sqrt(3)/2.0*float64(d.R)
		dy := 3.0 / 2.0 * float64(d.R)
		got := AngleToDirection(dx, dy)
		if got != i {
			t.Errorf("dir %d (Q=%d,R=%d) decoded as %d", i, d.Q, d.R, got)
		}
	}
}
