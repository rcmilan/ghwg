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
	if v.Front == nil || v.Front.Size != len(p.Guests) || v.Front.Wish != "" || v.Front.Nights != 0 {
		t.Fatalf("Front with the Terminal shut = %+v, want only its size %d", v.Front, len(p.Guests))
	}
	g.openTerminal()
	if v = g.view(); v.Front.Wish != "Twin" || v.Front.Nights != p.Nights || !v.Terminal.Open {
		t.Fatalf("Front with the Terminal open = %+v, want the card's Wish and Nights", v.Front)
	}
	g.state = StatePlay
	room := slices.IndexFunc(g.hotel.Rooms, func(r world.Room) bool { return len(r.Stays) >= len(p.Guests) })
	if why := g.book(room); why != "" {
		t.Fatal(why)
	}
	if v = g.view(); v.Rooms[room].Vacant || v.Front != nil {
		t.Fatalf("after booking Room %d: %+v, front %+v; want it Occupied and nobody at the counter", room, v.Rooms[room], v.Front)
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
