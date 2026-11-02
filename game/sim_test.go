package game

import (
	"fmt"
	"image"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// Headless Shifts (no rendering, no input): every Guest must reach its Stay Spot and then the street
// within a frame budget, even with the Concierge pinned where Guests have to pass (D22 anti-lock).
// Left alone, Guests must never need the stuck fallback: they sidestep each other instead. Runs on every
// grid the hotel supports (D54), not just the shipped one.
func TestSimShift(t *testing.T) {
	cases := []struct {
		name string
		pin  func(g *Game) image.Point // where the Concierge is held every frame; nil = left alone
	}{
		{"free", nil},
		{"concierge pinned in the Lobby Archway", func(g *Game) image.Point { return g.hotel.Archway }},
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
	for cols := 2; cols <= 3; cols++ {
		for rows := 1; rows <= 3; rows++ {
			for _, c := range cases {
				t.Run(fmt.Sprintf("%dx%d/%s", cols, rows, c.name), func(t *testing.T) { simShift(t, cols, rows, c.pin) })
			}
		}
	}
}

// simShift plays one Shift on a cols x rows hotel; pin holds the Concierge (nil: left alone).
func simShift(t *testing.T, cols, rows int, pin func(g *Game) image.Point) {
	// frames per phase, scaled by the Guest count. A free run's max frames over 10 runs (Arrivals / Departures),
	// with the Room lanes (D62): 2x1 857/1016, 2x2 1217/1647, 2x3 1577/2278, 3x1 1253/1371, 3x2 1793/2182,
	// 3x3 2333/2993.
	budget := 800 * cols * rows
	g, err := newGame(cols, rows)
	if err != nil {
		t.Fatal(err)
	}
	g.startShift(false)
	for i := range g.confirmed {
		g.confirmed[i] = true
	}
	g.beginArrivals()
	frame := func() {
		if pin != nil {
			p := pin(g)
			g.player.X, g.player.Y = float64(p.X), float64(p.Y)
		}
		g.step(0, 0)
		for _, guest := range g.guests {
			if pin == nil && guest.Phasing {
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
}

func mustNew(t *testing.T) *Game {
	t.Helper()
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func where(gs []*entities.Guest) string {
	s := ""
	for _, g := range gs {
		s += " " + image.Pt(int(g.X), int(g.Y)).String()
	}
	return "guests at" + s
}
