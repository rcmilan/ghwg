package game

import (
	"fmt"
	"image"
	"math/rand/v2"
	"testing"

	"ghosthotel/entities"
)

// TestSimClock (D100, first cut) runs whole days headless on every grid the hotel supports (D54): the
// Concierge books the front Party into the first Room that fits as soon as it stands in its Queue slot.
// It fails on a softlock (a Party walking one leg for too long), on money banked without a Checkout or
// twice for one, on a Booking that never pays, and on a first night with no Spin (the hotel filled up
// at once: rampDays).
func TestSimClock(t *testing.T) {
	for cols := 2; cols <= 3; cols++ {
		for rows := 1; rows <= 3; rows++ {
			t.Run(fmt.Sprintf("%dx%d", cols, rows), func(t *testing.T) { simClock(t, cols, rows, 2) })
		}
	}
}

func simClock(t *testing.T, cols, rows, days int) {
	const legBudget = 60 * tps // the longest walk, Street to the farthest Stay Spot on 3×3, is ~25 s
	g, err := newGame(cols, rows)
	if err != nil {
		t.Fatal(err)
	}
	g.rng = rand.New(rand.NewPCG(uint64(cols), uint64(rows)))
	g.startRun()
	type leg struct {
		state      partyState
		slot, from int // from: the tick this leg began
	}
	legs := map[*Party]leg{}
	var booked []*Party
	counted := map[*Party]bool{}
	for end := g.clock + days*dayFrames; g.clock < end; {
		if len(g.queue) > 0 && len(g.queue[0].Guests[0].Path) == 0 {
			for i := range g.hotel.Rooms {
				if p := g.queue[0]; g.book(i) == "" {
					booked = append(booked, p)
					break
				}
			}
		}
		money := g.money
		g.step(0, 0)
		if g.clock == dayFrames && !g.spinning() {
			t.Fatal("no Spin on the first night: every Room was booked")
		}
		banked := 0
		for _, p := range booked {
			if p.Paid && !counted[p] {
				counted[p], banked = true, banked+p.Payout()
			}
		}
		if g.money-money != banked {
			t.Fatalf("%s: money went up %d, but Checkouts paid %d", clockText(g.clock), g.money-money, banked)
		}
		for _, p := range g.parties {
			l, ok := legs[p]
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
	t.Logf("%d Bookings, %d paid, $%d banked", len(booked), len(counted), g.money)
	if len(booked) == 0 {
		t.Fatal("no Bookings in the whole run")
	}
	for _, p := range booked {
		if !p.Paid && g.clock > p.Checkout+2*hourFrames {
			t.Errorf("room %d's Booking never paid: checkout was %s, now %s", p.Room, clockText(p.Checkout), clockText(g.clock))
		}
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
