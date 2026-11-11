package game

import (
	"image"
	"math/rand/v2"
	"slices"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// T200: a Guest is messy messyOdds of the time, leaving messyMess dirt a night, else 1. A Party
// leaves its mess for the nights it stayed as it checks out or flees, and its Room books the next Party:
// bookGap after a checkout, fleeGap after a flee.
func TestMess(t *testing.T) {
	g, _, p := settled(t)
	g.rng = rand.New(rand.NewPCG(1, 2))
	messy := 0
	for range 10000 {
		switch g.mess() {
		case messyMess:
			messy++
		case 1:
		default:
			t.Fatal("a mess neither 1 nor messyMess")
		}
	}
	if f := float64(messy) / 10000; f < messyOdds-0.02 || f > messyOdds+0.02 {
		t.Fatalf("%.3f of Guests messy, want about %v", f, messyOdds)
	}
	room := p.Room
	g.setDirtLevel(room, 0)
	p.mess, p.Nights, p.arrived = 2, 3, g.clock-dayFrames-1 // 2 nights begun of 3
	g.checkout(p, g.hotel.Rooms[room])
	if g.dirtLevel(room) != 4 || g.arrivals[room] != (arrival{g.clock, g.clock + bookGap*minuteFrames}) {
		t.Fatalf("checkout after 2 nights at mess 2: dirt %d, next arrival %+v; want 4, in bookGap", g.dirtLevel(room), g.arrivals[room])
	}
	q := g.arrive(room)
	g.setDirtLevel(room, 0)
	q.mess = 1
	g.flee(q)
	if g.dirtLevel(room) != 1 || g.arrivals[room].at != g.clock+fleeGap*minuteFrames {
		t.Fatalf("a flee on the first night: dirt %d, next arrival at %d; want 1, in fleeGap", g.dirtLevel(room), g.arrivals[room].at)
	}
}

// T201: a Room shows enough dirt spots to cover its dirt, past bigFrom each a spotUnit bigger (fewer,
// bigger stains: 10 at the Closed Room's dirtMax), each on its own free floor cell, spiritual about spiritOdds of the
// time; spots stay put, and when dirt falls the spiritual go first, newest first, then the newest normal.
func TestSpots(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	for ecto, want := range map[int]int{0: 0, 1: 1, spotUnit: 1, 2*spotUnit + 1: 3, 4 * spotUnit: 4, 4*spotUnit + 1: 5, dirtMax: 10} {
		g.setDirtLevel(0, ecto)
		g.dirt = nil
		if n := len(g.spots(0)); n != want {
			t.Errorf("dirt %d: %d spots, want %d", ecto, n, want)
		}
	}
	g.setDirtLevel(0, 4*spotUnit)
	got := slices.Clone(g.spots(0))
	free := g.floorCells(0)
	for i, s := range got {
		if !slices.Contains(free, s.cell) || slices.ContainsFunc(got[:i], func(o spot) bool { return o.cell == s.cell }) {
			t.Fatalf("spot %v: not a free floor cell of the Room, or shared", s)
		}
	}
	if !slices.Equal(g.spots(0), got) {
		t.Fatal("the spots moved")
	}
	g.dirt[0] = []spot{{got[0].cell, false, spotUnit}, {got[1].cell, true, spotUnit}, {got[2].cell, false, spotUnit}, {got[3].cell, true, spotUnit}}
	g.setDirtLevel(0, 2*spotUnit)
	if want := []spot{{got[0].cell, false, spotUnit}, {got[2].cell, false, spotUnit}}; !slices.Equal(g.spots(0), want) {
		t.Fatalf("down to 2 spots: %v, want the normal ones %v", g.spots(0), want)
	}
	g.setDirtLevel(0, spotUnit)
	if want := []spot{{got[0].cell, false, spotUnit}}; !slices.Equal(g.spots(0), want) {
		t.Fatalf("down to 1 spot: %v, want the oldest %v", g.spots(0), want)
	}
	spirit := 0
	for room := range 2 {
		g.setDirtLevel(room, dirtMax)
		g.dirt[room], g.dirtN[room] = nil, 0 // 10 spots: 4 × 1, then 2+…+7
		for _, s := range g.spots(room) {
			if s.spirit {
				spirit++
			}
		}
	}
	if spirit < 2 || spirit > 14 { // of 20 at odds 0.35
		t.Fatalf("%d spiritual spots of 20, want about %v", spirit, 20*spiritOdds)
	}
}

// T202: SPACE on a normal spot inside an empty Room cleans it: − its size in dirt; in ghost, harvestRate
// of it banked as Ecto. One TaskHousekeep tap each. Refused with a Guest inside, and on a spiritual spot
// in either dimension.
func TestClean(t *testing.T) {
	g, s, p := pairIn(t, 2, 1, 0)
	g.setDirtLevel(0, 3*spotUnit)
	cellPt := func(c image.Point) image.Point {
		return c.Mul(world.TileSize).Add(image.Pt(world.TileSize/2, world.TileSize/2))
	}
	var free []image.Point // floor cells 2 apart: out of each other's stain reach (20 px)
	for _, c := range g.floorCells(0) {
		if !slices.ContainsFunc(free, func(o image.Point) bool { return abs(o.X-c.X) < 2 && abs(o.Y-c.Y) < 2 }) {
			free = append(free, c)
		}
	}
	g.dirt, g.dirtN = [][]spot{{{free[0], false, spotUnit}, {free[1], true, spotUnit}, {free[2], false, spotUnit}}, nil}, []int{3, 0} // by hand: 2 normal, 1 spiritual
	press(g, s, cellPt(free[0]))
	if g.dirtLevel(0) != 3*spotUnit || len(g.spots(0)) != 3 {
		t.Fatalf("with Guests inside: dirt %d, spots %d; want untouched", g.dirtLevel(0), len(g.spots(0)))
	}
	p.state = out
	press(g, s, cellPt(free[1]))
	g.ghost = true
	press(g, s, cellPt(free[1]))
	g.ghost = false
	if len(g.spots(0)) != 3 {
		t.Fatal("cleaned a spiritual spot")
	}
	press(g, s, cellPt(free[0]))
	if g.dirtLevel(0) != 2*spotUnit || slices.Contains(g.spots(0), spot{free[0], false, spotUnit}) || g.wallet[Ecto] != 0 || g.taps[0][TaskHousekeep] != 1 {
		t.Fatalf("cleaned in real: dirt %d, spots %v, Ecto %v, taps %v; want %d, that spot gone, no Ecto, 1",
			g.dirtLevel(0), g.spots(0), g.wallet[Ecto], g.taps[0][TaskHousekeep], 2*spotUnit)
	}
	g.ghost = true
	press(g, s, cellPt(free[2]))
	if g.dirtLevel(0) != spotUnit || g.wallet[Ecto] != spotUnit*harvestRate || g.taps[0][TaskHousekeep] != 2 {
		t.Fatalf("cleaned in ghost: dirt %d, Ecto %v; want %d and %v banked", g.dirtLevel(0), g.wallet[Ecto], spotUnit, spotUnit*harvestRate)
	}
	press(g, s, cellPt(free[3]))
	if len(g.spots(0)) != 1 || g.taps[0][TaskHousekeep] != 2 {
		t.Fatal("SPACE off the spots cleaned one")
	}
}

// T243: each Room's first Party arrives firstGap minutes after the last one's at Run start; none come while
// every Room is booked.
func TestBookings(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	for g.clock < runStart+firstGap*minuteFrames-1 {
		g.step(0, 0)
	}
	if len(g.parties) != 0 {
		t.Fatalf("%d Parties before the first arrival", len(g.parties))
	}
	for range 2 * minuteFrames {
		g.step(0, 0)
	}
	if len(g.parties) != 1 || g.parties[0].Room != 0 {
		t.Fatalf("at the first arrival: %d Parties, want 1 in Room 0", len(g.parties))
	}
	g, _ = headless(t, 2, 1, 1)
	g.startRun()
	for i, r := range g.hotel.Rooms {
		guest := entities.NewGuest(r.Stays[0], guestCast[0])
		guest.RoomIndex = i
		g.parties = append(g.parties, &Party{Guests: []*entities.Guest{&guest}, state: staying,
			Booking: Booking{Room: i, Nights: 1, Rate: 1, Checkout: 1 << 30}})
	}
	for range 2 * hourFrames {
		g.step(0, 0)
	}
	if len(g.parties) != len(g.hotel.Rooms) {
		t.Fatalf("%d Parties with every Room booked, want only the %d staying", len(g.parties), len(g.hotel.Rooms))
	}
}

// T244: a Party finding its Room's normal stains over dirtLimit walks back to the Lobby and waits; the
// spiritual ones don't count. Cleaned down to the limit, it goes in, each Guest losing dirtComfort per
// spot left of either kind. After patience it walks away unpaid, and the Room books its next Party.
func TestDirtyArrival(t *testing.T) {
	arrive := func() (*Game, *Party) {
		g, _ := headless(t, 2, 1, 1)
		g.startRun()
		free := g.floorCells(0)
		var spots []spot // dirtLimit/spotUnit+1 normal spots, and a spiritual one, by hand
		for i := range dirtLimit/spotUnit + 1 {
			spots = append(spots, spot{free[i], false, spotUnit})
		}
		spots = append(spots, spot{free[len(spots)], true, spotUnit})
		g.dirt, g.dirtN = [][]spot{spots, nil}, []int{len(spots), 0}
		g.setDirtLevel(0, len(spots)*spotUnit)
		p := g.arrive(0)
		for i := 0; p.state != complaining || len(p.Guests[0].Path) > 0; i++ {
			if i > 60*tps || p.state == staying {
				t.Fatalf("no complaint: state %d", p.state)
			}
			g.step(0, 0)
		}
		if at := image.Pt(int(p.Guests[0].X), int(p.Guests[0].Y)); !at.In(g.hotel.Lobby) {
			t.Fatalf("waiting at %v, want in the Lobby %v", at, g.hotel.Lobby)
		}
		return g, p
	}
	g, p := arrive()
	g.clean(0, g.spots(0)[0].cell)
	for i := 0; p.state != staying; i++ {
		if i > 60*tps {
			t.Fatalf("cleaned to the limit, it never settled: state %d", p.state)
		}
		g.step(0, 0)
	}
	if n := len(g.spots(0)); n != dirtLimit/spotUnit+1 {
		t.Fatalf("%d spots left, want %d", n, dirtLimit/spotUnit+1)
	}
	for _, guest := range p.Guests {
		if want := -dirtComfort * len(g.spots(0)); guest.Comfort != want {
			t.Fatalf("settled with %d spots: Comfort %d, want %d", len(g.spots(0)), guest.Comfort, want)
		}
	}
	g, p = arrive()
	from := p.complainAt
	for g.clock < from+patience*minuteFrames+1 {
		g.step(0, 0)
	}
	if !p.Walked || p.state != leaving || p.Occupies(0, Human) || g.arrivals[0].at == 0 {
		t.Fatalf("past patience: walked %v, state %d, occupies %v, next %+v; want gone and Room 0 rebooked", p.Walked, p.state, p.Occupies(0, Human), g.arrivals[0])
	}
}
