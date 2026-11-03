package game

import (
	"image"
	"slices"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// D92, T035: a Room fogs once its whole Party is in with the door shut, fading in over ~0.5 s, and
// clears from the frame the first member opens the door to leave.
func TestFog(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	room := slices.IndexFunc(g.hotel.Rooms, func(r world.Room) bool { return len(r.Stays) == 2 })
	if room < 0 {
		t.Skip("no 2-person Room in this Spin")
	}
	p := pairWalkIn(g)
	arrive(t, g)
	if why := g.book(room); why != "" {
		t.Fatal(why)
	}
	door, shell := &g.hotel.Doors[room], g.hotel.Rooms[room].Shell()
	allIn := func() bool {
		return !slices.ContainsFunc(p.Guests, func(guest *entities.Guest) bool {
			return !image.Pt(int(guest.X), int(guest.Y)).In(shell)
		})
	}
	for range 60 * tps { // checking in: no fog until everyone is in and the door has shut
		if g.step(0, 0); allIn() && door.Open() == 0 {
			break
		}
		if g.fog[room] != 0 {
			t.Fatalf("fog %.2f with the door %.0f%% open, everyone in: %v", g.fog[room], door.Open()*100, allIn())
		}
	}
	if f := g.fog[room]; f <= 0 || f >= 0.5 {
		t.Fatalf("fog %.2f one frame after the door shut, want it just starting to fade in", f)
	}
	for range fogFrames {
		g.step(0, 0)
	}
	if g.fog[room] != 1 {
		t.Fatalf("fog %.2f %d frames after the door shut, want 1", g.fog[room], fogFrames+1)
	}
	p.Checkout = g.clock
	for range 60 * tps {
		if g.step(0, 0); door.Open() > 0 {
			break
		}
		if g.fog[room] != 1 {
			t.Fatalf("fog %.2f before the door opened", g.fog[room])
		}
	}
	if f := g.fog[room]; f >= 1 || f < 0.9 || !allIn() {
		t.Fatalf("fog %.2f as the door opens (everyone in: %v), want it just starting to clear", f, allIn())
	}
	for range fogFrames {
		g.step(0, 0)
	}
	if g.fog[room] != 0 {
		t.Fatalf("fog %.2f %d frames after the door opened, want 0", g.fog[room], fogFrames)
	}
}
