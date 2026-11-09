package game

import (
	"image"
	"math/rand/v2"
	"slices"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// T200: a Guest is messy messyOdds of the time, leaving messyMess Room Spook a night, else 1. A Party
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
	g.roomSpook[room], p.mess, p.Nights, p.arrived = 0, 2, 3, g.clock-dayFrames-1 // 2 nights begun of 3
	g.checkout(p, g.hotel.Rooms[room])
	if g.roomSpook[room] != 4 || g.arrivals[room] != (arrival{g.clock, g.clock + bookGap*minuteFrames}) {
		t.Fatalf("checkout after 2 nights at mess 2: Room Spook %d, next arrival %+v; want 4, in bookGap", g.roomSpook[room], g.arrivals[room])
	}
	q := g.arrive(room)
	g.roomSpook[room], q.mess = 0, 1
	g.flee(q)
	if g.roomSpook[room] != 1 || g.arrivals[room].at != g.clock+fleeGap*minuteFrames {
		t.Fatalf("a flee on the first night: Room Spook %d, next arrival at %d; want 1, in fleeGap", g.roomSpook[room], g.arrivals[room].at)
	}
}

// T201: a Room shows one dirt spot per spotSpook of Room Spook, rounded up, each on its own free floor
// cell, spiritual about spiritOdds of the time; spots stay put, and when Room Spook falls the spiritual
// go first, newest first, then the newest normal.
func TestSpots(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	for spook, want := range map[int]int{0: 0, 1: 1, spotSpook: 1, 2*spotSpook + 1: 3} {
		g.roomSpook[0], g.dirt = spook, nil
		if n := len(g.spots(0)); n != want {
			t.Errorf("Room Spook %d: %d spots, want %d", spook, n, want)
		}
	}
	g.roomSpook[0] = 4 * spotSpook
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
	g.dirt[0] = []spot{{got[0].cell, false}, {got[1].cell, true}, {got[2].cell, false}, {got[3].cell, true}}
	g.roomSpook[0] = 2 * spotSpook
	if want := []spot{{got[0].cell, false}, {got[2].cell, false}}; !slices.Equal(g.spots(0), want) {
		t.Fatalf("down to 2 spots: %v, want the normal ones %v", g.spots(0), want)
	}
	g.roomSpook[0] = spotSpook
	if want := []spot{{got[0].cell, false}}; !slices.Equal(g.spots(0), want) {
		t.Fatalf("down to 1 spot: %v, want the oldest %v", g.spots(0), want)
	}
	spirit := 0
	for room := range 2 {
		g.roomSpook[room], g.dirt[room], g.dirtN[room] = 20*spotSpook, nil, 0
		for _, s := range g.spots(room) {
			if s.spirit {
				spirit++
			}
		}
	}
	if spirit < 4 || spirit > 24 { // of 40 at odds 0.35
		t.Fatalf("%d spiritual spots of 40, want about %v", spirit, 40*spiritOdds)
	}
}

// T202: SPACE on a normal spot inside an empty Room cleans it: −spotSpook Room Spook; in ghost, harvestRate
// of it banked as Spook. One TaskHousekeep tap each. Refused with a Guest inside, and on a spiritual spot
// in either dimension.
func TestClean(t *testing.T) {
	g, s, p := pairIn(t, 2, 1, 0)
	g.roomSpook[0] = 3 * spotSpook
	cellPt := func(c image.Point) image.Point {
		return c.Mul(world.TileSize).Add(image.Pt(world.TileSize/2, world.TileSize/2))
	}
	free := g.floorCells(0)
	g.dirt, g.dirtN = [][]spot{{{free[0], false}, {free[1], true}, {free[2], false}}, nil}, []int{3, 0} // by hand: 2 normal, 1 spiritual
	press(g, s, cellPt(free[0]))
	if g.roomSpook[0] != 3*spotSpook || len(g.spots(0)) != 3 {
		t.Fatalf("with Guests inside: Room Spook %d, spots %d; want untouched", g.roomSpook[0], len(g.spots(0)))
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
	if g.roomSpook[0] != 2*spotSpook || slices.Contains(g.spots(0), spot{free[0], false}) || g.wallet[Spook] != 0 || g.taps[0][TaskHousekeep] != 1 {
		t.Fatalf("cleaned in real: Room Spook %d, spots %v, Spook %v, taps %v; want %d, that spot gone, no Spook, 1",
			g.roomSpook[0], g.spots(0), g.wallet[Spook], g.taps[0][TaskHousekeep], 2*spotSpook)
	}
	g.ghost = true
	press(g, s, cellPt(free[2]))
	if g.roomSpook[0] != spotSpook || g.wallet[Spook] != spotSpook*harvestRate || g.taps[0][TaskHousekeep] != 2 {
		t.Fatalf("cleaned in ghost: Room Spook %d, Spook %v; want %d and %v banked", g.roomSpook[0], g.wallet[Spook], spotSpook, spotSpook*harvestRate)
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
		var spots []spot // dirtLimit/spotSpook+1 normal spots, and a spiritual one, by hand
		for i := range dirtLimit/spotSpook + 1 {
			spots = append(spots, spot{free[i], false})
		}
		spots = append(spots, spot{free[len(spots)], true})
		g.dirt, g.dirtN = [][]spot{spots, nil}, []int{len(spots), 0}
		g.roomSpook[0] = len(spots) * spotSpook
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
	if n := len(g.spots(0)); n != dirtLimit/spotSpook+1 {
		t.Fatalf("%d spots left, want %d", n, dirtLimit/spotSpook+1)
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
	if !p.Walked || p.state != leaving || p.Occupies(0) || g.arrivals[0].at == 0 {
		t.Fatalf("past patience: walked %v, state %d, occupies %v, next %+v; want gone and Room 0 rebooked", p.Walked, p.state, p.Occupies(0), g.arrivals[0])
	}
}
