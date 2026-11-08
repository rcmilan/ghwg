package game

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// T130 (D170): SPACE at the Mirror plays a transition; the dimension switches halfway, SPACE waits for
// its end, and the world keeps running through it.
func TestShift(t *testing.T) {
	if _, err := ebiten.NewShader([]byte(transitionSrc)); err != nil {
		t.Fatal(err)
	}
	g, s := headless(t, 2, 1, 1)
	s.hold(ebiten.KeySpace)
	g.Update() // the title
	if g.ghost {
		t.Fatal("a Run starts in the ghost dimension")
	}
	g.player.X, g.player.Y = float64(g.hotel.Mirror.X), float64(g.hotel.Mirror.Y)
	for _, want := range []bool{true, false} {
		s.hold()
		g.Update()
		s.hold(ebiten.KeySpace)
		g.Update()
		sh, clock := g.shifting, g.clock
		if sh == nil || sh.toGhost != want {
			t.Fatalf("SPACE at the Mirror: transition %+v, want one toward ghost %v", sh, want)
		}
		for e := 1; e <= flipFrames; e++ {
			if s.hold(); e%2 == 0 && e < flipFrames {
				s.hold(ebiten.KeySpace) // tapped mid-transition: ignored
			}
			g.Update()
			if half := e >= flipFrames/2; g.ghost != (want == half) {
				t.Fatalf("frame %d of %d: ghost %v", e, flipFrames, g.ghost)
			}
			if p := sh.ghostness(g.tick); want && p < 0 || !want && p > 1 {
				t.Fatalf("ghostness %v", p)
			}
		}
		if g.shifting != nil || g.ghost != want || g.clock == clock {
			t.Fatalf("after the transition: %+v, ghost %v, clock moved %v; want none, %v, true", g.shifting, g.ghost, g.clock != clock, want)
		}
	}
	g.player.X, g.player.Y = float64(g.hotel.Archway.X), float64(g.hotel.Archway.Y)
	s.hold()
	g.Update()
	s.hold(ebiten.KeySpace)
	g.Update()
	if g.shifting != nil {
		t.Fatal("SPACE away from the Mirror started a transition")
	}
}
