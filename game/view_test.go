package game

import (
	"image"
	"slices"
	"testing"

	"shift/world"
)

// T052: the View shows what the Terminal and the screen show, and no more.
func TestView(t *testing.T) {
	g, _ := headless(t, 3, 3, 1)
	g.startRun()
	p := g.walkIn()
	p.Wish = "Twin"
	for len(p.Guests[0].Path) > 0 {
		g.step(0, 0)
	}
	v := g.view()
	if len(v.Counter) != 1 || v.Counter[0].Size != len(p.Guests) || v.Counter[0].Wish != "" || v.Counter[0].Nights != 0 {
		t.Fatalf("Counter with the Terminal shut = %+v, want only its size %d", v.Counter, len(p.Guests))
	}
	g.openTerminal()
	if v = g.view(); v.Counter[0].Wish != "Twin" || v.Counter[0].Nights != p.Nights || !v.Terminal.Open {
		t.Fatalf("Counter with the Terminal open = %+v, want the list's Wish and Nights", v.Counter)
	}
	g.state = StatePlay
	room := slices.IndexFunc(g.hotel.Rooms, func(r world.Room) bool { return len(r.Stays) >= len(p.Guests) })
	if why := g.book(room); why != "" {
		t.Fatal(why)
	}
	if v = g.view(); v.Rooms[room].Vacant || len(v.Counter) > 0 {
		t.Fatalf("after booking Room %d: %+v, counter %+v; want it Occupied and nobody at the counter", room, v.Rooms[room], v.Counter)
	}
	for i := 0; g.fog[room] < 1; i++ {
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
