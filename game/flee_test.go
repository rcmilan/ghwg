package game

import (
	"math"
	"testing"

	"ghosthotel/world"
)

// T114, D164: a fleeing Party hops in place, every Guest lifted mid-hop, then runs out at 3× speed.
func TestFleeHop(t *testing.T) {
	g, _, p := settled(t)
	g.flee(p)
	if !g.vacant(p.Room) {
		t.Fatal("the Room isn't Vacant at once")
	}
	type pos struct{ x, y float64 }
	at := map[int]pos{}
	for i, guest := range p.Guests {
		at[i] = pos{guest.X, guest.Y}
	}
	for f := range hopFrames {
		g.step(0, 0)
		for i, guest := range p.Guests {
			if (pos{guest.X, guest.Y}) != at[i] {
				t.Fatalf("Guest %d moved %d frames into the hop", i, f+1)
			}
			if f == hopFrames/2 && g.lift(guest) < hopHeight/2 {
				t.Fatalf("Guest %d lifted %.1f mid-hop, want about %v", i, g.lift(guest), hopHeight)
			}
		}
	}
	lead := p.Guests[0]
	x, y := lead.X, lead.Y
	g.step(0, 0)
	g.step(0, 0)
	if d := math.Hypot(lead.X-x, lead.Y-y) / 2; d <= 2*departSpeed || g.lift(lead) != 0 {
		t.Fatalf("after the hop: %.2f px a frame, lift %.1f; want more than %v and 0", d, g.lift(lead), 2*departSpeed)
	}
}

// T115, D165: a Guest is heard near the Concierge; in real, not from inside a Room nor through a wall.
func TestHears(t *testing.T) {
	g, _, p := settled(t)
	guest, r := p.Guests[0], g.hotel.Rooms[p.Room]
	for _, c := range []struct {
		name          string
		x, y          int // the Guest
		dx            int // the Concierge, this far right of it
		real, inGhost bool
	}{
		{"near, outside", center(r.Spot()).X, center(r.Spot()).Y, 2 * world.TileSize, true, true},
		{"near, inside", center(r.Shell()).X, center(r.Shell()).Y, 2 * world.TileSize, false, true},
		{"far", center(r.Spot()).X, center(r.Spot()).Y, (hearRange + 1) * world.TileSize, false, false},
		{"behind a wall", center(r.Spot()).X, center(r.Spot()).Y, 0, false, true}, // the Concierge in the Lobby, below the corridor's wall
	} {
		guest.X, guest.Y = float64(c.x), float64(c.y)
		g.player.X, g.player.Y = float64(c.x+c.dx), float64(c.y)
		if c.dx == 0 {
			g.player.Y += 5 * world.TileSize
		}
		for _, ghost := range []bool{false, true} {
			g.ghost = ghost
			if want := c.real && !ghost || c.inGhost && ghost; g.hears(guest) != want {
				t.Errorf("%s, ghost %v: heard %v, want %v", c.name, ghost, !want, want)
			}
		}
	}
}
