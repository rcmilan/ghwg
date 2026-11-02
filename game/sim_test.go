package game

import (
	"image"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// Headless Shifts (no rendering, no input): every Guest must reach its Stay Spot and then the street
// within a frame budget, even with the Concierge pinned where Guests have to pass (D22 anti-lock).
// Left alone, Guests must never need the stuck fallback: they sidestep each other instead.
func TestSimShift(t *testing.T) {
	const budget = 2400 // frames per phase; a free run needs ~1300 for Arrivals and ~1760 for Departures
	cases := []struct {
		name string
		pin  func(g *Game) image.Point // where the Concierge is held every frame; nil = left alone
	}{
		{"free", nil},
		{"concierge pinned in the Lobby Archway", func(g *Game) image.Point { return image.Pt(256, 384) }},
		{"concierge pinned on room 101's route, in the corridor", func(g *Game) image.Point {
			r := g.hotel.Rooms[0]
			path := g.hotel.Path(g.hotel.Spawn, r.Stay, 0)
			for i, p := range path {
				if p.Div(world.TileSize) == r.Door.Add(image.Pt(0, 1)) {
					return path[i-3]
				}
			}
			panic("route misses the door")
		}},
		{"concierge pinned in room 101's doorway", func(g *Game) image.Point {
			d := g.hotel.Doors[0].Rect
			return d.Min.Add(d.Max).Div(2)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g, err := New()
			if err != nil {
				t.Fatal(err)
			}
			g.startShift(false)
			for i := range g.confirmed {
				g.confirmed[i] = true
			}
			g.beginArrivals()
			frame := func() {
				if c.pin != nil {
					p := c.pin(g)
					g.player.X, g.player.Y = float64(p.X), float64(p.Y)
				}
				g.step(0, 0)
				for _, guest := range g.guests {
					if c.pin == nil && guest.Phasing {
						t.Fatalf("tick %d: guest of room %d got stuck at (%.0f,%.0f) with nobody in the way",
							g.tick, guest.RoomIndex, guest.X, guest.Y)
					}
				}
			}
			for f := 0; g.state == StateArrivals; f++ {
				if f == budget {
					t.Fatalf("Arrivals still running after %d frames: %s", budget, where(g.guests))
				}
				frame()
			}
			for g.state == StateDay {
				frame()
			}
			for f := 0; g.state == StateDepartures; f++ {
				if f == budget {
					t.Fatalf("Departures still running after %d frames: %s", budget, where(g.guests))
				}
				frame()
			}
			if g.state != StateBooking || len(g.guests) != 0 {
				t.Fatalf("Shift ended in state %d with %d guests left", g.state, len(g.guests))
			}
		})
	}
}

func where(gs []*entities.Guest) string {
	s := ""
	for _, g := range gs {
		s += " " + image.Pt(int(g.X), int(g.Y)).String()
	}
	return "guests at" + s
}
