package game

import (
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/shirou/magi_link/internal/hex"
)

// fakeEvent records how many times Update was called and finishes after n ticks.
type fakeEvent struct {
	ticks  int
	limit  int
	draws  int
}

func (f *fakeEvent) Update(dt float64) bool {
	f.ticks++
	return f.ticks >= f.limit
}

func (f *fakeEvent) Draw(screen *ebiten.Image, grid *hex.Grid) {
	f.draws++
}

func TestVFXQueueIdleWhenEmpty(t *testing.T) {
	var q VFXQueue
	if !q.IsIdle() {
		t.Fatal("empty queue should be idle")
	}
}

func TestVFXQueuePlaysHeadOnly(t *testing.T) {
	var q VFXQueue
	a := &fakeEvent{limit: 2}
	b := &fakeEvent{limit: 1}
	q.Push(a)
	q.Push(b)

	q.Update(0.1) // a ticks once
	if a.ticks != 1 || b.ticks != 0 {
		t.Fatalf("only head should tick: a=%d b=%d", a.ticks, b.ticks)
	}
	if q.IsIdle() {
		t.Fatal("queue not idle while events remain")
	}

	q.Update(0.1) // a finishes and is popped
	if a.ticks != 2 {
		t.Fatalf("a should have ticked twice: got %d", a.ticks)
	}
	if len(q.events) != 1 || q.events[0] != b {
		t.Fatalf("b should be head: len=%d", len(q.events))
	}

	q.Update(0.1) // b finishes
	if !q.IsIdle() {
		t.Fatal("queue should be idle after all events finish")
	}
}

func TestFloatingNumberFinishes(t *testing.T) {
	f := NewDamagePop(hex.NewHex(0, 0), 10)
	done := false
	for i := 0; i < 100 && !done; i++ {
		done = f.Update(0.05)
	}
	if !done {
		t.Fatal("FloatingNumber should finish within bounded time")
	}
}

func TestHexFlashFinishes(t *testing.T) {
	h := NewHexFlash([]hex.Hex{hex.NewHex(0, 0)}, color.RGBA{255, 0, 0, 200})
	done := false
	for i := 0; i < 100 && !done; i++ {
		done = h.Update(0.05)
	}
	if !done {
		t.Fatal("HexFlash should finish within bounded time")
	}
}

func TestUnitTweenFinishes(t *testing.T) {
	tw := NewUnitTween(1, hex.NewHex(0, 0), hex.NewHex(1, 0), color.RGBA{80, 180, 255, 255})
	done := false
	for i := 0; i < 100 && !done; i++ {
		done = tw.Update(0.05)
	}
	if !done {
		t.Fatal("UnitTween should finish within bounded time")
	}
}
