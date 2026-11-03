package game

import (
	"fmt"
	"image"
	"math/rand/v2"
	"slices"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// TestSimClock (D100) runs whole days headless on every grid the hotel supports (D54): the Concierge books
// the front Party into the first Room that fits as soon as it stands in its Queue slot, and is either
// left alone or pinned where guests must pass (02's anti-lock cases). It fails on a softlock (a Party
// walking one leg for too long), on any phasing while the Concierge stands at the Terminal (D89: keep-right,
// yielding and paths around the Concierge must suffice), on money banked without a Checkout or twice for one, on a Booking that
// never pays or pays on another day than its last (D88), on a Midnight that rolls a Room it shouldn't
// (D85), on a run where no Tourist goes out, and on a first night with no Spin (rampDays).
func TestSimClock(t *testing.T) {
	cases := []struct {
		name string
		days int
		pin  func(g *Game) image.Point // where the Concierge is held every frame
	}{
		{"at the Terminal", 4, func(g *Game) image.Point { return g.hotel.ConciergeStart }}, // where it books; held, as the guests' pushes would drift an idle one into the Corridors
		{"pinned in the Lobby Archway", 2, func(g *Game) image.Point { return g.hotel.Archway }},
		{"pinned on the first Room's route, in the Corridor", 2, func(g *Game) image.Point {
			r := g.hotel.Rooms[0]
			path := g.hotel.Path(g.hotel.Spawn, r.Stays[0], 0)
			for i, p := range path {
				if p.Div(world.TileSize) == r.Door.Add(image.Pt(0, 1)) {
					return path[i-3]
				}
			}
			panic("route misses the door")
		}},
		{"pinned in the first Room's doorway", 2, func(g *Game) image.Point {
			d := g.hotel.Doors[0].Rect
			return d.Min.Add(d.Max).Div(2)
		}},
	}
	outings := 0
	for cols := 2; cols <= 3; cols++ {
		for rows := 1; rows <= 3; rows++ {
			for i, c := range cases {
				t.Run(fmt.Sprintf("%dx%d/%s", cols, rows, c.name), func(t *testing.T) { outings += simClock(t, cols, rows, c.days, c.pin, i == 0) })
			}
		}
	}
	if outings == 0 {
		t.Error("no Tourist went out on any grid")
	}
}

// simClock runs one grid for days and returns the Tourist outings. pin holds the Concierge; strict fails
// on any phasing, for a pin out of the guests' way.
func simClock(t *testing.T, cols, rows, days int, pin func(g *Game) image.Point, strict bool) int {
	const legBudget = 60 * tps // the longest walk, Street to the farthest Stay Spot on 3×3, is ~25 s
	g, err := newGame(cols, rows)
	if err != nil {
		t.Fatal(err)
	}
	g.rng = rand.New(rand.NewPCG(uint64(cols), uint64(rows)))
	g.startRun()
	pinAt := pin(g) // the hotel's cells never move, so neither does the pin
	type leg struct {
		state      partyState
		slot, from int // from: the tick this leg began
	}
	legs := map[*Party]leg{}
	var booked []*Party
	counted := map[*Party]bool{}
	nights, outings := map[int]int{}, 0
	for end := g.clock + days*dayFrames; g.clock < end; {
		if len(g.queue) > 0 && len(g.queue[0].Guests[0].Path) == 0 {
			for i := range g.hotel.Rooms {
				if p := g.queue[0]; g.book(i) == "" {
					booked = append(booked, p)
					break
				}
			}
		}
		g.player.X, g.player.Y = float64(pinAt.X), float64(pinAt.Y)
		var eligible []int // the Rooms this frame's Midnight may roll
		midnight := g.clock%dayFrames == dayFrames-1 && !g.spinning()
		if midnight {
			for i := range g.hotel.Rooms {
				if g.flips(i) {
					eligible = append(eligible, i)
				}
			}
		}
		money := g.money
		g.step(0, 0)
		if midnight && !slices.Equal(g.spinRooms, eligible) && (g.spinning() || len(eligible) > 0) {
			t.Fatalf("%s: Midnight rolled Rooms %v, want the eligible %v", clockText(g.clock), g.spinRooms, eligible)
		}
		if g.clock == dayFrames && !g.spinning() {
			t.Fatal("no Spin on the first night: every Room was booked")
		}
		for _, guest := range g.guests() {
			if strict && guest.Phasing {
				for _, p := range g.parties {
					t.Logf("party room %d state %d slot %d %s path %v", p.Room, p.state, p.slot, where(p.Guests), p.Guests[0].Path[:min(3, len(p.Guests[0].Path))])
				}
				t.Fatalf("%s: a guest of room %d phased at %v with the Concierge at the Terminal",
					clockText(g.clock), guest.RoomIndex, image.Pt(int(guest.X), int(guest.Y)))
			}
		}
		banked := 0
		for _, p := range booked {
			if p.Paid && !counted[p] {
				counted[p], banked = true, banked+p.Payout()
				nights[p.Nights]++
				if day(g.clock) != day(p.Checkout) {
					t.Errorf("a %d-night Booking paid on %s, its checkout is %s", p.Nights, clockText(g.clock), clockText(p.Checkout))
				}
			}
		}
		if g.money-money != banked {
			t.Fatalf("%s: money went up %d, but Checkouts paid %d", clockText(g.clock), g.money-money, banked)
		}
		for _, p := range g.parties {
			l, ok := legs[p]
			if p.state == out && l.state != out {
				outings++
			}
			if !ok || l.state != p.state || l.slot != p.slot {
				legs[p] = leg{p.state, p.slot, g.tick}
				continue
			}
			if walking := p.state != staying && len(p.Guests[0].Path) > 0; walking && g.tick-l.from > legBudget {
				t.Fatalf("%s: a Party (state %d, room %d) has walked one leg for %d frames: %s",
					clockText(g.clock), p.state, p.Room, g.tick-l.from, where(p.Guests))
			}
		}
	}
	t.Logf("%d Bookings, %d paid (by nights %v), %d Tourist outings, $%d banked", len(booked), len(counted), nights, outings, g.money)
	if len(booked) == 0 {
		t.Fatal("no Bookings in the whole run")
	}
	for _, p := range booked {
		if !p.Paid && g.clock > p.Checkout+2*hourFrames {
			t.Errorf("room %d's Booking never paid: checkout was %s, now %s", p.Room, clockText(p.Checkout), clockText(g.clock))
		}
	}
	return outings
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
