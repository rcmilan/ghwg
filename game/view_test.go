package game

import (
	"image"
	"slices"
	"testing"

	"ghosthotel/world"
)

// T052: the View shows what the Terminal and the screen show, and no more.
func TestView(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	p := g.walkIn()
	room := p.Room
	if v := g.view(); v.Rooms[room].Vacant {
		t.Fatalf("Room %d after a Walk-in booked into it: %+v, want it Occupied", room, v.Rooms[room])
	}
	g.openTerminal()
	if v := g.view(); !v.Terminal.Open {
		t.Fatal("the View doesn't show the Terminal open")
	}
	g.state = StatePlay
	for i := 0; len(g.fog) == 0 || g.fog[room] < 1; i++ {
		if i > 60*tps {
			t.Fatal("the Room never fogged")
		}
		g.step(0, 0)
	}
	cell := image.Pt(int(p.Guests[0].X), int(p.Guests[0].Y)).Div(world.TileSize)
	if slices.Contains(g.view().Guests, cell) {
		t.Fatalf("a guest in a fogged Room shows at %v", cell)
	}
}
