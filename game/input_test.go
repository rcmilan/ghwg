package game

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func headless(t *testing.T, cols, rows int, seed uint64) (*Game, *script) {
	t.Helper()
	g, err := newWorld(cols, rows, seed)
	if err != nil {
		t.Fatal(err)
	}
	g.openAtStart = true // most tests are about something else than buying Rooms (owned_test.go)
	return g, g.in.(*script)
}

// T049: the game reads a script like a keyboard.
func TestScriptInput(t *testing.T) {
	g, s := headless(t, 3, 3, 1)
	s.hold(ebiten.KeySpace)
	g.Update()
	if g.state != StatePlay {
		t.Fatalf("SPACE on the title: state %d, want a Run", g.state)
	}
	g.player.X, g.player.Y = float64(g.hotel.Archway.X), float64(g.hotel.Archway.Y) // open floor
	x := g.player.X
	for range 10 {
		s.hold(ebiten.KeyD)
		g.Update()
	}
	if got := g.player.X - x; got != 10*conciergeSpeed {
		t.Fatalf("D held 10 frames moved %.1f px, want %.1f", got, 10*conciergeSpeed)
	}
	s.hold(ebiten.KeySpace)
	if !s.JustPressed(ebiten.KeySpace) {
		t.Fatal("a fresh press isn't just pressed")
	}
	s.hold(ebiten.KeySpace)
	if s.JustPressed(ebiten.KeySpace) || !s.Pressed(ebiten.KeySpace) {
		t.Fatal("a held key reads as just pressed")
	}
}
