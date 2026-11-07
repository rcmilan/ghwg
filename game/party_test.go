package game

import (
	"image"
	"math"
	"math/rand/v2"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// D139, D140, D86, D87, D91, D53: a Walk-in arrives booked into a Room it fits, which is Occupied from
// then until it pays Rate × nights at its door, at the Rate locked on arrival. Once every Room is
// Occupied, nobody else can come.
func TestArrival(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	p := g.walkIn()
	if p == nil || p.Room < 0 || g.vacant(p.Room) || p.state != checkingIn {
		t.Fatalf("a Walk-in on a Vacant hotel: %+v; want it booked, its Room Occupied, walking in", p)
	}
	p.Routine = homebody // no day out: on the ×10 clock below, a Tourist's walk back can outlast the budget
	room := p.Room
	rate := g.hotel.Rooms[room].Type.Rate
	if p.Rate != rate || len(g.hotel.Rooms[room].Stays) < len(p.Guests) {
		t.Errorf("a Party of %d in a Room for %d at $%d/night, want the Room's $%d", len(p.Guests), len(g.hotel.Rooms[room].Stays), p.Rate, rate)
	}
	if day(p.Checkout) != 1+p.Nights || hour(p.Checkout) < checkoutHour || hour(p.Checkout) >= checkoutHour+2 {
		t.Errorf("a %d-night stay arriving Day 1 checks out at %s, want Day %d 10:00–12:00", p.Nights, clockText(p.Checkout), 1+p.Nights)
	}
	for range len(g.hotel.Rooms) - 1 {
		if g.walkIn() == nil {
			t.Fatal("no Walk-in with a Room still Vacant")
		}
	}
	if q := g.walkIn(); q != nil {
		t.Fatalf("a Walk-in booked into Room %d with every Room Occupied", q.Room)
	}
	g.hotel.Rooms[room].Type.Rate = 999 // the Rate is locked on arrival
	g.fast = true
	for !p.Paid {
		if g.clock > p.Checkout+4*hourFrames { // ×10 clock: the walk from the bed to the door takes ~2 game hours
			t.Fatalf("still unpaid at %s, checkout was %s: %s", clockText(g.clock), clockText(p.Checkout), where(p.Guests))
		}
		g.step(0, 0)
	}
	if g.money < rate*p.Nights || p.Occupies(room) {
		t.Errorf("after Checkout: money %d (want at least %d), Room still held by the Party", g.money, rate*p.Nights)
	}
	if g.clock < p.Checkout {
		t.Errorf("paid at %s, before its checkout %s", clockText(g.clock), clockText(p.Checkout))
	}
}

// D77, D140: Walk-ins follow the hourly curve, and none come while no Room is Vacant.
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

// furnish gives every Room the Standard Room Type with this bed layout.
func furnish(t *testing.T, g *Game, beds string) {
	t.Helper()
	for _, rt := range g.types {
		if rt.Name == "Standard" && rt.Beds == beds {
			next := make([]world.RoomType, len(g.hotel.Rooms))
			for i := range next {
				next[i] = rt
			}
			g.hotel.Furnish(next)
			return
		}
	}
	t.Fatalf("no Standard %s", beds)
}

// D88, T017, D77: each Party draws 1–3 nights, Homebody or Tourist 50/50, and jittered times; 40% are
// pairs when a 2-person Room is free.
func TestScheduleDraw(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	furnish(t, g, "Twin")
	g.rng = rand.New(rand.NewPCG(3, 4))
	nights, tourists, pairs := map[int]int{}, 0, 0
	const n = 400
	for range n {
		g.parties = nil // keep the Rooms Vacant
		p := g.walkIn()
		nights[p.Nights]++
		if p.Routine == tourist {
			tourists++
		}
		switch {
		case len(p.Guests) > 2:
			t.Fatalf("a Party of %d", len(p.Guests))
		case len(p.Guests) == 2:
			pairs++
			if p.Guests[1].Leader != p.Guests[0] {
				t.Fatal("a pair's second guest doesn't trail the first")
			}
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
	if pairs < n*3/10 || pairs > n/2 { // D77: 40% pairs
		t.Errorf("%d pairs in %d, want ~40%%", pairs, n)
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
	p := g.walkIn()
	if door = g.hotel.Doors[p.Room].Rect; !g.conciergeBlocked(door) {
		t.Error("the Concierge can enter an Occupied Room's doorway")
	}
}

// pairWalkIn walks in Parties until one is a pair, at night so no other Walk-in joins it. It needs a
// 2-person Room (furnish).
func pairWalkIn(g *Game) *Party {
	g.clock = 2 * hourFrames
	for {
		g.parties = nil
		if p := g.walkIn(); len(p.Guests) == 2 {
			return p
		}
	}
}

// D80, D159, T030: a pair is booked into a 2-person Room at its full Rate; with none free it comes solo.
func TestPartyFit(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	var single, double world.RoomType
	for _, rt := range g.types {
		switch {
		case rt.Name == "Standard" && rt.Beds == "Single":
			single = rt
		case rt.Name == "Standard" && rt.Beds == "Double":
			double = rt
		}
	}
	g.hotel.Furnish([]world.RoomType{single, double})
	pair := pairWalkIn(g)
	if pair.Room != 1 || pair.Rate != double.Rate {
		t.Fatalf("a pair booked into Room %d at $%d, want the Double (1) at $%d", pair.Room, pair.Rate, double.Rate)
	}
	for range 50 {
		p := g.walkIn()
		if len(p.Guests) != 1 || p.Room != 0 {
			t.Fatalf("a Party of %d into Room %d with only the Single free, want solo into 0", len(p.Guests), p.Room)
		}
		g.parties = g.parties[:1]
	}
}

// D90, T032: a pair walks as one, the follower a tile or so behind its leader, and splits to its own
// Stay Spot in the Room; checking out, both leave the Room before it pays.
func TestPairFollows(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	furnish(t, g, "Twin")
	p := pairWalkIn(g)
	room := p.Room
	lead, f := p.Guests[0], p.Guests[1]
	gap := func() float64 { return math.Hypot(f.X-lead.X, f.Y-lead.Y) }
	walk := func(what string, until func() bool) {
		t.Helper()
		for range 60 * tps {
			if until() {
				return
			}
			g.step(0, 0)
			if len(lead.Path) > 0 && len(f.Path) > 0 && gap() > 3*trailGap {
				t.Fatalf("%s: the follower is %.0f px behind its leader", what, gap())
			}
			if lead.Phasing || f.Phasing {
				t.Fatalf("%s: a mate phased: %s", what, where(p.Guests))
			}
		}
		t.Fatalf("%s: still walking after 60 s: %s", what, where(p.Guests))
	}
	walk("checking in", func() bool { return p.state == staying })
	r := g.hotel.Rooms[room]
	for i, guest := range p.Guests {
		if at := image.Pt(int(guest.X), int(guest.Y)); at != r.Stays[i] {
			t.Errorf("member %d stands at %v, want its Stay Spot %v", i, at, r.Stays[i])
		}
	}
	p.Checkout = g.clock
	walk("checking out", func() bool { return p.Paid })
	if !p.outside(r) {
		t.Errorf("paid with a member still in the Room: %s", where(p.Guests))
	}
}

// S5 review: a follower that slipped into the Room ahead of its leader while checking in must not hold the
// leader out (the wait is for leaving the Room only).
func TestFollowerAhead(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	room := g.freeRoom(2)
	if room < 0 {
		t.Skip("no 2-Guest Room on this seed")
	}
	r := g.hotel.Rooms[room]
	lead := entities.NewGuest(g.hotel.Spawn, "Adam")
	mate := entities.NewGuest(r.Stays[1], "Alex")
	mate.Leader, lead.RoomIndex, mate.RoomIndex = &lead, room, room
	p := &Party{Guests: []*entities.Guest{&lead, &mate}, Booking: Booking{Room: room, Nights: 1}}
	p.schedule(g.clock)
	g.parties = append(g.parties, p)
	g.checkIn(p)
	mate.SetPath(nil) // already in, ahead of its leader
	for i := 0; p.state != staying; i++ {
		if i > 60*tps {
			t.Fatalf("the leader never got in: %s", where(p.Guests))
		}
		g.step(0, 0)
	}
}
