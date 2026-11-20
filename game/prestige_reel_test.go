package game

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func reelGame(t *testing.T) (*Game, *script, *int) {
	t.Helper()
	g, s := headless(t, 2, 1, 1)
	g.startRun()
	n := 0
	old := prestigeGo
	prestigeGo = func(*Game) { n++ }
	t.Cleanup(func() { prestigeGo = old })
	return g, s, &n
}

// 08 P1-X: the cutscene ends by itself, calls startPrestige once, and puts every guest, ghosts too, in the hall.
func TestPrestigeReel(t *testing.T) {
	g, s, n := reelGame(t)
	far := g.arrive(0)
	g.settleIn(far)
	ghost := g.arriveGhost(1)
	g.settleIn(ghost)
	g.player.X, g.player.Y = float64(g.hotel.Archway.X), float64(g.hotel.Archway.Y)
	g.playPrestige()
	for i := 0; g.reel != nil; i++ {
		if i > 2*reelEnd {
			t.Fatal("the cutscene never ended")
		}
		if i == reelGather+10 { // everyone is in the hall once the gathering is over
			for _, gu := range g.guests() {
				if !image.Pt(int(gu.X), int(gu.Y)).In(g.hotel.Lobby) {
					t.Fatalf("guest at %.0f,%.0f is not in the hall %v", gu.X, gu.Y, g.hotel.Lobby)
				}
			}
		}
		s.hold()
		g.Update()
	}
	if *n != 1 {
		t.Fatalf("startPrestige called %d times, want 1", *n)
	}
}

// SPACE skips to the white flash only after the first prestige.
func TestPrestigeReelSkip(t *testing.T) {
	for level, want := range map[int]bool{0: false, 1: true} {
		g, s, _ := reelGame(t)
		g.level = level
		g.playPrestige()
		g.Update()
		s.hold(ebiten.KeySpace)
		g.Update()
		if got := g.reel.t >= reelFlash; got != want {
			t.Errorf("level %d: SPACE skipped to the flash = %v, want %v", level, got, want)
		}
	}
}

// The cutscene's balloons are drawn upright at the speaker's turned-over place: the anchor takes the flip, not the balloon.
func TestFlipY(t *testing.T) {
	for _, c := range []struct{ y, c, want float64 }{{100, 1, 100}, {100, -1, 440}, {100, 0, 270}, {270, -1, 270}} {
		if got := flipY(c.y, 540, c.c); got != c.want {
			t.Errorf("flipY(%v, 540, %v) = %v, want %v", c.y, c.c, got, c.want)
		}
	}
}
