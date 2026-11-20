package game

import (
	"fmt"
	"image"
	"reflect"
	"testing"

	"ghosthotel/world"
)

// TestSimClock (D100, D109) runs whole days headless on every grid the hotel supports (D54), played by
// keys alone: the Concierge stays at the Terminal or holds a spot where guests must pass (02's anti-lock
// cases) while Parties arrive booked (D139). Any invariant failure fails it (D108), as do a run with no
// Booking and, across the grids, no Tourist outing or pair.
func TestSimClock(t *testing.T) {
	cases := []struct {
		name string
		days int
		at   func(h *world.Hotel) image.Point // where the Concierge holds between Bookings
	}{
		{"at the Terminal", 4, atTerminal},
		{"in the Lobby Archway", 2, func(h *world.Hotel) image.Point { return h.Archway }},
		{"on the first Room's route, in the Corridor", 2, func(h *world.Hotel) image.Point {
			r := h.Rooms[0]
			path := h.Path(h.Spawn, r.Stays[0], 0)
			for i, p := range path {
				if p.Div(world.TileSize) == r.Door.Add(image.Pt(0, 1)) {
					return path[i-3]
				}
			}
			panic("route misses the door")
		}},
		{"in the first Room's doorway", 2, func(h *world.Hotel) image.Point {
			d := h.Doors[0].Rect
			return d.Min.Add(d.Max).Div(2)
		}},
	}
	outings, pairs := 0, 0
	for cols := 2; cols <= 3; cols++ {
		for rows := 1; rows <= 3; rows++ {
			for _, c := range cases {
				t.Run(fmt.Sprintf("%dx%d/%s", cols, rows, c.name), func(t *testing.T) {
					r := RunSim(Config{Cols: cols, Rows: rows, Days: c.days, Seed: uint64(cols*10 + rows),
						Strategy: stand{c.at}, React: 30, KeyGap: 6, Open: true, Beds: "Twin"}) // Twin: pairs walk in too
					t.Logf("%+v", r)
					for _, f := range r.Failures {
						t.Errorf("%s: %s", f.Clock, f.Reason)
					}
					if r.Bookings == 0 {
						t.Error("no Bookings in the whole run")
					}
					outings, pairs = outings+r.Outings, pairs+r.Pairs
				})
			}
		}
	}
	if outings == 0 || pairs == 0 { // one grid's seed can draw neither
		t.Errorf("%d Tourist outings and %d pairs booked across the grids, want both", outings, pairs)
	}
}

// T055: with Parties booked before they arrive (D139), even never's money is its paid Bookings.
func TestRunSim(t *testing.T) {
	r := RunSim(Config{Cols: 2, Rows: 1, Days: 4, Seed: 1, Strategy: Strategies["never"], React: 30, KeyGap: 6, Open: true})
	if r.Bookings == 0 || r.Paid == 0 || sum(r.MoneyByDay) == 0 || len(r.Failures) > 0 || r.Sold == 0 || r.Available != 4*2 {
		t.Fatalf("never: %+v; want paid Bookings, money, Room-nights sold of 8, no failures", r)
	}
}

// T056: an invariant broken mid-Run is one Failure naming it, not a panic.
func TestSimInvariant(t *testing.T) {
	r := RunSim(Config{Cols: 2, Rows: 1, Days: 1, Seed: 1, Strategy: &thief{}})
	if len(r.Failures) != 1 || r.Failures[0].Reason != "money went up 50, but Guests paid 0 (x1.00) and Priests cost 0" || !r.Cheat {
		t.Fatalf("%+v, want the one money failure on a Cheat row", r)
	}
}

// thief banks $50 out of thin air, once, mid-Run.
type thief struct{ done bool }

func (s *thief) Act(View, *Pilot) {}
func (s *thief) Cheat(g *Game) {
	if g.clock > runStart+hourFrames && !s.done {
		s.done, g.wallet[Money] = true, g.wallet[Money]+50
	}
}

// T058: a Run is a function of its Config.
func TestSimDeterministic(t *testing.T) {
	c := Config{Cols: 2, Rows: 1, Days: 3, Seed: 42, Strategy: Strategies["never"], React: 30, KeyGap: 6, Open: true}
	if a, b := RunSim(c), RunSim(c); !reflect.DeepEqual(a, b) {
		t.Fatalf("same seed, different Runs:\n%+v\n%+v", a, b)
	}
}

func sum(xs []int) (n int) {
	for _, x := range xs {
		n += x
	}
	return n
}

func mustNew(t *testing.T) *Game {
	t.Helper()
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	g.openAtStart = true // see headless
	return g
}

// W3.5 P1FIX-SIM: `goals` buys the Fathers' unlocks and calls one when a Room's dread is high, and puts K on his
// retainer once Ecto piles up.
// Whether one Run piles up the 2 days of retainer is chaotic (W4: upgrades and quirks each moved seed 2's course), so
// the retainer is asked of the first of a few seeds that gets there; every Run checked must call Priests and stay clean.
func TestGoalsCallsPriests(t *testing.T) {
	for seed := uint64(1); seed <= 6; seed++ {
		r := RunSim(Config{Cols: 2, Rows: 1, Days: 8, Seed: seed, Name: "goals", Strategy: Strategies["goals"], React: 30, KeyGap: 6})
		t.Logf("%+v", r)
		for _, f := range r.Failures {
			t.Errorf("seed %d %s: %s", seed, f.Clock, f.Reason)
		}
		if r.Priests == 0 || r.PriestEcto < unlockKEcto+priestWeak {
			t.Errorf("seed %d: %d Priests called for %d Ectoplasm, want some", seed, r.Priests, r.PriestEcto)
		}
		if r.RetainDays > 0 {
			return
		}
	}
	t.Error("Father K was never put on his retainer in seeds 1-6")
}
