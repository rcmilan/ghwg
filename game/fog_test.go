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
	furnish(t, g, "Twin")
	p := pairWalkIn(g)
	room := p.Room
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
	// a day out (S6 review): the fog clears while the door lets them out, is back once it shuts, and
	// holds while they're away and walking back, until the door opens for them again
	for p.state != staying {
		g.step(0, 0)
	}
	p.state = goingOut
	g.sendParty(p, g.hotel.Exit, g.hotel.Exit)
	away := false
	for range 120 * tps {
		before := g.fog[room]
		g.step(0, 0)
		if p.state == out {
			if away = true; g.fog[room] != 1 {
				t.Fatalf("fog %.2f while the Party is out", g.fog[room])
			}
		}
		if away && door.Open() > 0 {
			if before != 1 {
				t.Fatalf("fog %.2f just before the door opened for the Party coming back", before)
			}
			break
		}
	}
	if !away {
		t.Fatal("the Party never went out")
	}
	for range 60 * tps {
		if g.step(0, 0); p.state == staying && door.Open() == 0 {
			break
		}
	}
	for range fogFrames {
		g.step(0, 0)
	}
	if g.fog[room] != 1 {
		t.Fatalf("fog %.2f once back in with the door shut, want 1", g.fog[room])
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

// Wall leak (2026-11-14) and corner vanish (2026-11-15): from the Corridor, every door shut, nobody gets
// into a Room's Shell, its north wall face and corners included (its top row is blocked, F0): so the
// Concierge walking round a Room never lifts its Fog, and a guest walking round it is never faded by it.
// Inside, the Concierge lifts it and a guest is under it.
func TestFogStopsAtTheWall(t *testing.T) {
	g, _ := headless(t, 3, 3, 1)
	g.startRun()
	g.fog = slices.Repeat([]float64{1}, len(g.hotel.Rooms))
	free := func(p image.Point) bool { return !g.blocked(entities.FootBox(float64(p.X), float64(p.Y))) }
	for room, r := range g.hotel.Rooms {
		g.parties = []*Party{{settled: true, Booking: Booking{Room: room, Realm: Human}}}
		shell := r.Shell()
		area := shell.Inset(-2 * world.TileSize) // the Corridors round it, wall ends and corners
		var todo []image.Point
		seen := map[image.Point]bool{}
		for y := area.Min.Y; y < area.Max.Y; y++ {
			for x := area.Min.X; x < area.Max.X; x++ {
				if p := image.Pt(x, y); !p.In(shell) && free(p) {
					seen[p], todo = true, append(todo, p)
				}
			}
		}
		for len(todo) > 0 {
			p := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			g.player.X, g.player.Y = float64(p.X), float64(p.Y)
			if !g.fogged(room) {
				t.Fatalf("Room %d: the Concierge at %v, come from the Corridor (Room px %v), lifts its Fog", room, p, p.Sub(shell.Min))
			}
			if f := g.fogAt(p); f != 0 {
				t.Fatalf("Room %d: a guest at %v, come from the Corridor (Room px %v), is under its Fog", room, p, p.Sub(shell.Min))
			}
			for _, d := range [...]image.Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				if n := p.Add(d); n.In(area) && !seen[n] && free(n) {
					seen[n], todo = true, append(todo, n)
				}
			}
		}
		in := shell.Min.Add(image.Pt(shell.Dx()/2, shell.Dy()/2))
		if g.player.X, g.player.Y = float64(in.X), float64(in.Y); g.fogged(room) {
			t.Errorf("Room %d: the Concierge inside it does not lift its Fog", room)
		}
		if g.fogAt(in) != 1 {
			t.Errorf("Room %d: a guest inside it is not under its Fog", room)
		}
	}
}
