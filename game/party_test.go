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

// D88, T017: each Party draws 1–3 nights, Homebody or Tourist 50/50, and jittered times; the first
// Walk-in of a Run stays 1 night, so a Room frees up for Day 2's Spin.
func TestScheduleDraw(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	g.rng = rand.New(rand.NewPCG(3, 4))
	nights, tourists := map[int]int{}, 0
	const n = 400
	for i := range n {
		g.queue = nil // keep a lane free
		g.walkIn()
		p := g.parties[len(g.parties)-1]
		if i == 0 && p.Nights != 1 {
			t.Errorf("the first Walk-in of a Run stays %d nights, want 1", p.Nights)
		}
		nights[p.Nights]++
		if p.Routine == tourist {
			tourists++
		}
		for _, w := range []struct {
			name     string
			at, from int
			span     int
		}{{"out", p.outAt, outHour, outJitter}, {"back", p.backAt, backHour, backJitter}, {"checkout", p.checkoutAt, checkoutHour, checkoutJitter}} {
			if w.at < w.from*hourFrames || w.at >= w.from*hourFrames+w.span {
				t.Fatalf("%s time %s, want %02d:00 + under %d min", w.name, clockText(w.at), w.from, w.span/minuteFrames)
			}
		}
	}
	if len(nights) != 3 || nights[1] == 0 || nights[2] == 0 || nights[3] == 0 {
		t.Errorf("nights drawn %v, want 1–3", nights)
	}
	if tourists < n*2/5 || tourists > n*3/5 {
		t.Errorf("%d Tourists in %d, want about half", tourists, n)
	}
}

// D88: a Tourist is out between its out and back times on every day of its routine but the checkout
// day; booked after 10:00, its routine starts the next day.
func TestTouristDays(t *testing.T) {
	at := func(d, h, m int) int { return (d-1)*dayFrames + h*hourFrames + m*minuteFrames }
	b := Booking{Nights: 3, Routine: tourist, outAt: 9 * hourFrames, backAt: 20 * hourFrames, checkoutAt: 11 * hourFrames}
	b.schedule(at(1, 15, 0))
	if b.Checkout != at(4, 11, 0) {
		t.Errorf("checkout %s, want DAY 4  11:00", clockText(b.Checkout))
	}
	for _, c := range []struct {
		at  int
		out bool
	}{
		{at(1, 16, 0), false}, {at(2, 8, 59), false}, {at(2, 9, 0), true}, {at(2, 19, 59), true},
		{at(2, 20, 0), false}, {at(3, 12, 0), true}, {at(4, 9, 30), false}, {at(4, 12, 0), false},
	} {
		if b.Out(c.at) != c.out {
			t.Errorf("Out at %s = %v, want %v", clockText(c.at), !c.out, c.out)
		}
	}
	early := b
	early.schedule(at(1, 8, 30))
	if !early.Out(at(1, 9, 30)) {
		t.Error("a Tourist booked at 08:30 isn't out at 09:30 the same day")
	}
	home := b
	home.Routine = homebody
	if home.Out(at(2, 12, 0)) {
		t.Error("a Homebody went out")
	}
}

// D91: the Concierge can't get through an Occupied Room's doorway, open or not, so it can't tailgate.
func TestConciergeKeptOut(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	door := g.hotel.Doors[0].Rect
	if g.conciergeBlocked(door) != g.blocked(door) { // a shut door blocks anyone
		t.Fatal("a Vacant Room's doorway is barred to the Concierge alone")
	}
	g.walkIn()
	arrive(t, g)
	if why := g.book(0); why != "" {
		t.Fatal(why)
	}
	if !g.conciergeBlocked(door) {
		t.Error("the Concierge can enter an Occupied Room's doorway")
	}
}
