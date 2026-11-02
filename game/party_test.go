package game

import (
	"math/rand/v2"
	"slices"
	"testing"

	"ghosthotel/entities"
)

// D86, D87, D91, D53: a Booking pays Rate × nights at its door, with the Rate locked when booked, and
// keeps the Room Occupied from Booking until then.
func TestBooking(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	if g.book(0) == "" {
		t.Fatal("booked a Room with nobody in the Queue")
	}
	g.walkIn()
	if g.book(0) == "" {
		t.Fatal("booked a Walk-in still on its way to the counter")
	}
	arrive(t, g)
	rate := g.hotel.Rooms[0].Type.Rate
	if why := g.book(0); why != "" {
		t.Fatalf("book: %s", why)
	}
	p := g.parties[0]
	if !g.occupied(0) || g.vacant(0) {
		t.Fatal("a booked Room is not Occupied")
	}
	g.walkIn()
	arrive(t, g)
	if g.book(0) == "" {
		t.Fatal("booked a second Party into an Occupied Room")
	}
	if p.Payout() != rate*p.Nights {
		t.Errorf("payout %d, want Rate %d × %d nights", p.Payout(), rate, p.Nights)
	}
	if day(p.Checkout) != 2 || hour(p.Checkout) < checkoutHour || hour(p.Checkout) >= checkoutHour+2 {
		t.Errorf("a 1-night stay booked Day 1 checks out at %s, want Day 2 10:00–12:00", clockText(p.Checkout))
	}
	g.hotel.Rooms[0].Type.Rate = 999 // the Rate is locked at Booking
	g.fast = true
	for !p.Paid {
		if g.clock > p.Checkout+2*hourFrames {
			t.Fatalf("still unpaid at %s, checkout was %s", clockText(g.clock), clockText(p.Checkout))
		}
		g.step(0, 0)
	}
	if g.money != rate*p.Nights || g.occupied(0) || len(g.floaters) != 1 {
		t.Errorf("after Checkout: money %d (want %d), occupied %v, %d floaters", g.money, rate*p.Nights, g.occupied(0), len(g.floaters))
	}
	if g.clock < p.Checkout {
		t.Errorf("paid at %s, before its checkout %s", clockText(g.clock), clockText(p.Checkout))
	}
}

// D77: Walk-ins follow the hourly curve, and none come while the Queue is full or no Room is Vacant.
func TestWalkIns(t *testing.T) {
	run := func(setup func(g *Game), from, hours int) *Game {
		g := mustNew(t)
		g.startRun()
		g.rng = rand.New(rand.NewPCG(1, 2))
		g.clock = from
		if setup != nil {
			setup(g)
		}
		for range hours * hourFrames {
			g.step(0, 0)
		}
		return g
	}
	if g := run(nil, dayFrames+2*hourFrames, 4); len(g.parties) != 0 {
		t.Errorf("%d Walk-ins between 02:00 and 06:00, want none", len(g.parties))
	}
	if g := run(nil, 14*hourFrames, 2); len(g.parties) == 0 {
		t.Error("no Walk-ins in 2 peak hours")
	}
	full := func(g *Game) {
		for range len(g.hotel.Queue) {
			g.walkIn()
		}
	}
	if g := run(full, 14*hourFrames, 1); len(g.parties) != len(g.hotel.Queue) {
		t.Errorf("%d Parties after a peak hour with a full Queue, want %d", len(g.parties), len(g.hotel.Queue))
	}
	booked := func(g *Game) {
		for i, r := range g.hotel.Rooms {
			guest := entities.NewGuest(r.Stays[0], guestCast[0])
			guest.RoomIndex = i
			g.parties = append(g.parties, &Party{Guests: []*entities.Guest{&guest}, state: staying,
				Booking: Booking{Room: i, Nights: 1, Rate: 1, Checkout: 1 << 30}})
		}
	}
	if g := run(booked, 14*hourFrames, 2); len(g.parties) != len(g.hotel.Rooms) {
		t.Errorf("%d Parties with every Room booked, want only the %d staying", len(g.parties), len(g.hotel.Rooms))
	}
}

// D79: a queued Party gives up when its patience runs out and walks out of the Entrance.
func TestPatience(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	g.clock = 2 * hourFrames // no Walk-ins at night, so only this one
	g.walkIn()
	p := g.parties[0]
	for len(g.parties) > 0 {
		if g.clock > p.patience+hourFrames {
			t.Fatalf("still on the map at %s, patience ran out at %s", clockText(g.clock), clockText(p.patience))
		}
		g.step(0, 0)
	}
	if g.clock < p.patience || len(g.queue) != 0 {
		t.Errorf("left at %s, patience until %s; queue %d", clockText(g.clock), clockText(p.patience), len(g.queue))
	}
}

// D85: at Midnight a Room flips only if it is Vacant and the Concierge's foot point is outside it.
func TestMidnightFlips(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	if len(g.hotel.Rooms) < 3 {
		t.Skip("needs 3 Rooms")
	}
	g.walkIn()
	arrive(t, g)
	if why := g.book(0); why != "" {
		t.Fatal(why)
	}
	in := g.hotel.Rooms[1].Stays[0]
	g.player.X, g.player.Y = float64(in.X), float64(in.Y)
	before := roomTypeNames(g)
	for i := range g.hotel.Rooms {
		if want := i >= 2; g.flips(i) != want {
			t.Errorf("room %d flips = %v, want %v", i, g.flips(i), want)
		}
	}
	g.types = fakeTypes(g, map[string]int{"Z": 10})
	g.midnight()
	if want := []int{2, 3, 4, 5, 6, 7, 8}[:len(g.hotel.Rooms)-2]; !slices.Equal(g.spinRooms, want) {
		t.Errorf("Midnight rolled %v, want %v", g.spinRooms, want)
	}
	for i, n := range roomTypeNames(g) {
		if want := map[bool]string{true: "Z", false: before[i]}[i >= 2]; n != want {
			t.Errorf("room %d is %s after Midnight, want %s", i, n, want)
		}
	}
}

// arrive steps the world until the front queued Party stands at the counter.
func arrive(t *testing.T, g *Game) {
	t.Helper()
	for range 60 * 60 {
		if g.atCounter() {
			return
		}
		g.step(0, 0)
	}
	t.Fatal("the front Walk-in never reached the counter")
}
