package game

import (
	"image"
	"math"
	"slices"
	"testing"

	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// matchRun: a Run of cols Rooms in a row, the Mirror owned, every Room open, and nobody coming on their own.
func matchRun(t *testing.T, cols int) (*Game, *script) {
	t.Helper()
	g, s := headless(t, cols, 1, 1)
	g.startRun()
	g.own(Mirror)
	g.arrivals, g.ghostArrivals = make([]arrival, cols), make([]arrival, cols)
	return g, s
}

// settledHuman: a homebody of the Nerve settled in the Room for 3 nights.
func settledHuman(t *testing.T, g *Game, room, n int) *Party {
	t.Helper()
	p := g.arrive(room)
	p.Guests, p.Routine, p.Nights = p.Guests[:1], homebody, 3
	p.schedule(g.clock)
	setNerve(p, n)
	g.settleIn(p)
	return p
}

// ghostOf: a ghost of the Phantasm settled in the Room for 3 nights.
func ghostOf(g *Game, room, phantasm int) *Party {
	q := g.arriveGhost(room)
	q.Guests, q.Nights = q.Guests[:1], 3
	q.schedule(g.clock)
	q.Guests[0].Phantasm = phantasm
	g.settleIn(q)
	return q
}

// standIn puts the Concierge just inside the Room's door.
func standIn(g *Game, room int) {
	at := g.hotel.Map.Center(g.hotel.Rooms[room].Door.X, g.hotel.Rooms[room].Door.Y-1)
	g.player.X, g.player.Y = float64(at.X), float64(at.Y)
}

// T319: every human has a Nerve, which sets its FleeAt; a ghost fits a human when its Phantasm is within the
// Nerve. A fitting pair stays through every haunt of its ghost; a bad one flees.
func TestNerve(t *testing.T) {
	g, _ := matchRun(t, 2)
	seen := map[int]bool{}
	for range 200 {
		p := g.arrive(0)
		n := nerve(p.Guests[0])
		if n < nerveMin || n > nerveMax || p.Guests[0].FleeAt != n*nerveFear {
			t.Fatalf("Nerve %d, FleeAt %d; want %d..%d and Nerve × %d", n, p.Guests[0].FleeAt, nerveMin, nerveMax, nerveFear)
		}
		seen[n] = true
		g.parties = nil
	}
	if len(seen) < (nerveMax-nerveMin)/2 {
		t.Fatalf("200 humans had %d Nerves between them, want a spread", len(seen))
	}
	for _, c := range []struct {
		phantasm, nerve int
		flees           bool
	}{{8, 8, false}, {3, 10, false}, {12, 2, true}, {9, 4, true}} {
		g, _ := matchRun(t, 2)
		h := settledHuman(t, g, 0, c.nerve)
		gh := ghostOf(g, 0, c.phantasm)
		if fits(h.Guests[0], gh.Guests[0]) == c.flees {
			t.Fatalf("Phantasm %d, Nerve %d: fits %v", c.phantasm, c.nerve, !c.flees)
		}
		for i := 0; slices.Contains(g.parties, gh) && !h.Fled; i++ {
			if i > (c.phantasm+1)*ghostHaunt*minuteFrames {
				t.Fatalf("Phantasm %d: the ghost never passed on (%d haunts)", c.phantasm, gh.Guests[0].Haunted)
			}
			g.step(0, 0)
		}
		if h.Fled != c.flees {
			t.Errorf("Phantasm %d against Nerve %d: fled %v (Fear %d of %d), want %v", c.phantasm, c.nerve, h.Fled, h.Guests[0].Fear, h.Guests[0].FleeAt, c.flees)
		}
	}
}

// T320: an arriving human Party takes the vacant, owned, open Room it fits best (beds enough): the strongest
// ghost within its Nerve, else the weakest beyond it. The Room it leaves takes over that Room's booking.
func TestBookByFit(t *testing.T) {
	g, _ := matchRun(t, 3)
	ghostOf(g, 0, 9)
	ghostOf(g, 1, 4) // Room 2 has none
	for _, c := range []struct{ nerve, size, want int }{{5, 1, 1}, {9, 1, 0}, {12, 1, 0}, {3, 1, 2}, {5, 2, -1}} {
		if got := g.fitRoom(c.nerve, c.size); got != c.want {
			t.Errorf("Nerve %d, %d guests: Room %d, want %d", c.nerve, c.size, got, c.want)
		}
	}
	settledHuman(t, g, 2, 12)
	if got := g.fitRoom(3, 1); got != 1 {
		t.Errorf("Nerve 3 with Room 2 taken: Room %d, want 1, the closest bad fit", got)
	}
	g.roomOwned = []bool{true, false, true}
	if got := g.fitRoom(3, 1); got != 0 {
		t.Errorf("Nerve 3 with Room 1 boarded: Room %d, want 0", got)
	}
	g.roomOwned = nil
	g.parties = slices.DeleteFunc(g.parties, func(p *Party) bool { return p.Realm == Human })
	for g.clock%minuteFrames != minuteFrames-1 {
		g.clock++
	}
	later := arrival{g.clock, g.clock + 60*minuteFrames}
	g.arrivals = []arrival{{g.clock, g.clock + 1}, later, {}}
	want := map[int]int{} // by Nerve: where its Party should go, all three Rooms vacant
	for n := 1; n <= nerveMax; n++ {
		want[n] = g.fitRoom(n, 1)
	}
	g.step(0, 0)
	p := g.humanIn(0)
	for _, r := range []int{1, 2} {
		if p == nil {
			p = g.humanIn(r)
		}
	}
	if p == nil {
		t.Fatal("nobody arrived")
	}
	if n := nerve(p.Guests[0]); p.Room != want[n] {
		t.Fatalf("a Party of Nerve %d took Room %d, want %d", n, p.Room, want[n])
	}
	if g.arrivals[p.Room] != (arrival{}) || !slices.Contains(g.arrivals, later) {
		t.Errorf("Room %d taken on Room 101's booking: bookings %v; want the Room taken booked no more, the later booking kept", p.Room, g.arrivals)
	}
}

// T321: the hotel view reads the fit: a Room with a human and a ghost Party is a good or a bad pair.
func TestPairing(t *testing.T) {
	g, _ := matchRun(t, 3)
	settledHuman(t, g, 0, 5)
	ghostOf(g, 0, 4)
	settledHuman(t, g, 1, 5)
	ghostOf(g, 1, 9)
	settledHuman(t, g, 2, 5)
	for room, want := range []pairing{goodPair, badPair, unpaired} {
		if got := g.pairing(room); got != want {
			t.Errorf("Room %d: %v, want %v", room, got, want)
		}
	}
	if _, ok := g.pairInk(2); ok {
		t.Error("a Room with no ghost is tinted")
	}
	if c, ok := g.pairInk(1); !ok || c.R < 200 || c.G > 80 {
		t.Errorf("a bad pair's tint %v, want red", c)
	}
	// live test 2: the ink takes the Room's walls' top band, top and bottom, and never the Room's middle
	g.props = g.sortedProps()
	shell := g.hotel.Rooms[1].Shell()
	mid := shell.Inset(shell.Dy() / 3)
	var top, bottom bool
	for _, p := range g.props {
		r := world.BandRect(g.ts.Tiles[p.id].Name).Add(g.hotel.Map.Origin(p.cx, p.cy))
		if p.wood == nil || !r.In(shell) { // as tintWood picks them
			continue
		}
		top, bottom = top || r.Max.Y <= mid.Min.Y, bottom || r.Min.Y >= mid.Max.Y
		if !r.In(shell) || r.Overlaps(mid) {
			t.Errorf("tinted band %v: want it in the shell %v, clear of its middle %v", r, shell, mid)
		}
	}
	if !top || !bottom {
		t.Errorf("tinted band on the top wall %v, on the bottom wall %v; want both", top, bottom)
	}
	// its motes stay in the shell and rise
	for tick := range int64(3 * moteLoop) {
		for _, m := range motes(shell, 1, tick) {
			if !(image.Rectangle{m.at, m.at.Add(image.Pt(m.size, m.size))}).In(shell) {
				t.Fatalf("frame %d: a mote at %v, out of the shell %v", tick, m.at, shell)
			}
		}
	}
	if a, b := motes(shell, 1, 0)[0], motes(shell, 1, moteLoop/2)[0]; b.at.Y >= a.at.Y || b.at.X != a.at.X {
		t.Errorf("a mote from %v to %v in half a loop, want straight up", a.at, b.at)
	}
}

// Live test 1: a human Party in a bad pair pays badRent of its hourly rent; a good pair the whole.
func TestBadPairRent(t *testing.T) {
	g, _ := matchRun(t, 2)
	bad, good := settledHuman(t, g, 0, 2), settledHuman(t, g, 1, 12)
	ghostOf(g, 0, 9).Guests[0].Haunt = 100 * ghostHaunt // one scare as it comes in, not enough to flee
	ghostOf(g, 1, 9).Guests[0].Haunt = 100 * ghostHaunt
	for range 6 * hourFrames {
		g.step(0, 0)
	}
	if want := good.rent * badRent; good.rent == 0 || math.Abs(float64(bad.rent-want)) > 2 {
		t.Errorf("6 hours: the bad pair paid $%v, the good one $%v; want about $%v", bad.rent, good.rent, want)
	}
}

// Live test 1: how far the talk slows the world and how heavy its blur grow with how far the ghost's Phantasm
// is beyond the guest's Nerve, fixed as it starts: a guest that fits gets the mildest.
func TestTalkDepth(t *testing.T) {
	var last float64
	for i, c := range []struct{ nerve, phantasm int }{{8, 4}, {nerveMin, nerveMin + 1}, {nerveMin, nerveMin + 3}, {nerveMin, phantasmMax}, {2, phantasmMax}} {
		g, _ := matchRun(t, 2)
		settledHuman(t, g, 0, c.nerve)
		ghostOf(g, 0, c.phantasm)
		standIn(g, 0)
		g.scale = 0.5
		g.startTalk(g.talker())
		if g.parley == nil {
			t.Fatalf("Nerve %d, Phantasm %d: no talk", c.nerve, c.phantasm)
		}
		d := g.parley.depth
		switch {
		case i == 0 && (d != 0 || g.scale != 0.5*talkSlow[0]):
			t.Errorf("a guest that fits: depth %v, scale %v; want 0, %v", d, g.scale, 0.5*talkSlow[0])
		case i >= 3 && (d != 1 || g.scale != 0.5*talkSlow[1]):
			t.Errorf("Nerve %d, Phantasm %d: depth %v, scale %v; want the worst, %v", c.nerve, c.phantasm, d, g.scale, 0.5*talkSlow[1])
		case i > 0 && i < 3 && !(d > last && d < 1):
			t.Errorf("Nerve %d, Phantasm %d: depth %v after %v; want deeper, short of the worst", c.nerve, c.phantasm, d, last)
		}
		last = d
		g.scale = 0.1 // whatever happened since, it ends at the scale it found
		for range talkIn {
			g.updateParley()
		}
		g.endTalk()
		if g.scale != 0.5 {
			t.Errorf("after the talk: scale %v, want 0.5", g.scale)
		}
	}
}

// Live test 1: the talk shows only the offers the wallet can pay (menus show only what applies), and W/S
// walks those alone; the plain ask is free, always there.
func TestOffersShown(t *testing.T) {
	g, s := matchRun(t, 2)
	p := settledHuman(t, g, 0, 2)
	ghostOf(g, 0, 9)
	standIn(g, 0)
	disc := g.offerCost(p, 1, discount)
	for _, c := range []struct {
		money Num
		want  []int
	}{{0, []int{plainAsk}}, {disc, []int{plainAsk, discount}}, {1000, []int{plainAsk, discount, onTheHouse}}} {
		g.wallet[Money] = c.money
		if got := g.offers(p, 1); !slices.Equal(got, c.want) {
			t.Errorf("$%v: offers %v, want %v", c.money, got, c.want)
		}
	}
	g.wallet[Money] = 0
	keys(g, s, ebiten.KeySpace, ebiten.KeyS, ebiten.KeyW, ebiten.KeyS)
	if g.state != StateTalk || g.parley.pick != plainAsk {
		t.Fatalf("broke, W/S: state %d, pick %d; want the talk on the plain ask, the only one", g.state, g.parley.pick)
	}
}

// T322: TALK. In a Room with its human Party in, and some Room it fits, TALK shows on the HUD's right; SPACE
// starts the talk: the world slows by talkSlow (the worst pair here: Nerve 2 against Phantasm 9), the camera
// closes in on the two on real frames, and leaving gives the world the scale it had.
func TestTalkCue(t *testing.T) {
	g, s := matchRun(t, 2)
	p := settledHuman(t, g, 0, 2)
	ghostOf(g, 0, 9)
	standIn(g, 0)
	if g.talker() != p {
		t.Fatal("no TALK in a bad pair's Room with a free Room it fits")
	}
	for range slideFrames {
		g.updateHUD()
	}
	if g.talkCue.in != 1 {
		t.Fatalf("the cue slid in %.2f, want 1", g.talkCue.in)
	}
	g.ghost = true
	if g.talker() != nil {
		t.Error("TALK in the ghost Dimension: the Concierge can't talk to humans from there")
	}
	g.ghost = false
	g.scale = 0.5 // whatever it was, the talk gives it back
	keys(g, s, ebiten.KeySpace)
	if g.state != StateTalk || g.parley.p != p || !slices.Equal(g.parley.targets, []int{1}) {
		t.Fatalf("SPACE: state %d, want the talk with Room 101's Party about Room 102", g.state)
	}
	clock, x, slow := g.clock, p.Guests[0].X, 0.5*talkSlow[1]
	p.Guests[0].SetPath(g.route(p.Guests[0], g.hotel.Rooms[0].Spot().Min)) // on its way somewhere: it stops to listen
	const frames = 4 * talkIn
	for range frames {
		s.hold()
		g.Update()
	}
	if want := int(frames * slow); g.clock != clock+want || g.scale != slow {
		t.Errorf("%d frames of talk ran the Clock %d at scale %v, want %d at %v", frames, g.clock-clock, g.scale, want, slow)
	}
	if p.Guests[0].X != x {
		t.Error("the guest walked off while spoken to")
	}
	if g.talkZoom() <= 1 {
		t.Errorf("zoom %.2f once in, want closer than play", g.talkZoom())
	}
	keys(g, s, ebiten.KeyEscape)
	if g.state != StatePlay || g.scale != 0.5 {
		t.Fatalf("ESC: state %d at scale %v, want play at 0.5", g.state, g.scale)
	}
	g.scale = 1
	ghostOf(g, 1, 5) // now no Room fits a Nerve of 2
	if g.talker() != nil {
		t.Fatal("TALK with no Room to offer")
	}
	keys(g, s, ebiten.KeySpace)
	if g.state != StateView {
		t.Errorf("SPACE with nothing to talk about: state %d, want the hotel view", g.state)
	}
}

// T323: offers. Per target Room: a plain ask (free), a discount (Money), on the house (a night's Rate, sure);
// the odds rise with the guest's Fear and the target's Mood. A refusal costs the Room Mood and ends the talk, and
// TALK hides for that guest for snubFrames (live test 1). An occupied target swaps the two Parties, priced for both. A yes: both walk to their new Rooms, the
// rent runs on at the new Rate, Mood stays with the Room. Two SPACE presses make an offer.
func TestOffers(t *testing.T) {
	g, s := matchRun(t, 3)
	p := settledHuman(t, g, 0, 2)
	ghostOf(g, 0, 9)
	rate := Num(g.hotel.Rooms[1].Type.Rate)
	if c := [3]Num{g.offerCost(p, 1, plainAsk), g.offerCost(p, 1, discount), g.offerCost(p, 1, onTheHouse)}; c[0] != 0 || c[1] <= 0 || c[1] >= c[2] || c[2] != rate {
		t.Fatalf("prices %v, want free, some, and a night's Rate %v", c, rate)
	}
	low := [3]float64{g.offerOdds(p, 1, plainAsk), g.offerOdds(p, 1, discount), g.offerOdds(p, 1, onTheHouse)}
	if !(low[0] < low[1] && low[1] < low[2] && low[2] == 1) {
		t.Fatalf("odds %v, want plain < discount < on the house = 1", low)
	}
	p.Guests[0].Fear = p.Guests[0].FleeAt - 1
	if g.offerOdds(p, 1, plainAsk) <= low[0] {
		t.Error("a scared guest is no likelier to move")
	}
	p.Guests[0].Fear = 0
	g.mood[1] = moodMax
	if g.offerOdds(p, 1, plainAsk) <= low[0] {
		t.Error("a nicer target Room is no likelier")
	}
	g.mood[1] = moodMin + 1 // as unlikely as it gets: a refusal soon
	q := settledHuman(t, g, 2, 12)
	if g.offerCost(p, 2, onTheHouse) != rate+Num(g.hotel.Rooms[0].Type.Rate) {
		t.Errorf("a swap on the house costs %v, want both Rates", g.offerCost(p, 2, onTheHouse))
	}
	standIn(g, 0)
	g.wallet[Money] = 1000
	keys(g, s, ebiten.KeySpace) // the talk
	mood := g.mood[0]
	keys(g, s, ebiten.KeySpace) // the plain ask, picked first, for Room 102
	if g.state != StatePlay || g.mood[0] != mood-refuseMood || p.Room != 0 {
		t.Fatalf("a plain ask at odds %.2f: state %d, Mood %d→%d, Room %d; want it refused (another seed if not), -%d Mood, the talk over", g.offerOdds(p, 1, plainAsk), g.state, mood, g.mood[0], p.Room, refuseMood)
	}
	refusedAt := g.clock
	g.clock = p.snubbed - 1 // live test 1: no TALK for that guest for about a game hour
	if g.talker() != nil || p.snubbed-refusedAt < snubFrames-tps {
		t.Fatalf("just refused: TALK %v, snubbed until %d; want none for %d frames", g.talker() != nil, p.snubbed, snubFrames)
	}
	g.clock, g.mood[0] = p.snubbed, mood // the Mood back up: the refusal's low Mood shows stains, which SPACE cleans first
	if g.talker() != p {
		t.Fatal("TALK not back an hour after the refusal")
	}
	keys(g, s, ebiten.KeySpace) // the talk again
	for g.parley.targets[g.parley.to] != 2 {
		keys(g, s, ebiten.KeyD)
	}
	swap := g.offerCost(p, 2, onTheHouse)
	keys(g, s, ebiten.KeyS, ebiten.KeyS, ebiten.KeySpace) // on the house, the swap
	moods := slices.Clone(g.mood)
	if g.state != StatePlay || p.Room != 2 || q.Room != 0 || g.offered != swap {
		t.Fatalf("on the house: state %d, Rooms %d and %d, Money %v; want both moved, paid", g.state, p.Room, q.Room, g.wallet[Money])
	}
	for i := 0; !g.inRoom(p) || !g.inRoom(q); i++ {
		if i > 60*tps {
			t.Fatalf("they never got there: states %d and %d", p.state, q.state)
		}
		g.step(0, 0)
	}
	if !slices.Equal(g.mood, moods) {
		t.Errorf("Moods %v, were %v: they stay with the Rooms", g.mood, moods)
	}
	if p.Rate != g.hotel.Rooms[2].Type.Rate || q.Rate != g.hotel.Rooms[0].Type.Rate || p.Guests[0].RoomIndex != 2 || q.Guests[0].RoomIndex != 0 {
		t.Errorf("Rates %d and %d, doors %d and %d; want the new Rooms'", p.Rate, q.Rate, p.Guests[0].RoomIndex, q.Guests[0].RoomIndex)
	}
}

// T324: the upgrade. Matchmaking (HOTEL, Ectoplasm) goes on sale with a housekeeper hired; owned, an idle
// housekeeper walks to a red Room and makes the cheapest sure offer.
func TestMatchmaking(t *testing.T) {
	g, _ := matchRun(t, 3)
	i := row(t, Matchmaking)
	if it := shop[i]; it.Tab != tabHotel || it.Cost[Ecto] == 0 || it.Cost[Money] != 0 || g.unlocked(Matchmaking) {
		t.Fatalf("Matchmaking: tab %d, cost %v, owned %v; want a HOTEL unlock for Ectoplasm, not owned", it.Tab, it.Cost, g.unlocked(Matchmaking))
	}
	if g.onSale(i) {
		t.Fatal("on sale with no housekeeper")
	}
	g.wallet[Money] = 10 * keeperWage
	g.hire()
	if !g.onSale(i) {
		t.Fatal("not on sale with a housekeeper")
	}
	p := settledHuman(t, g, 0, 2)
	ghostOf(g, 0, 9).Guests[0].Haunt = 100 * ghostHaunt // one scare as it comes in, not enough to flee
	settledHuman(t, g, 1, 12)                           // a swap would cost both Rates: Room 2, empty, is cheaper
	for range 10 * tps {
		g.step(0, 0)
	}
	if p.Room != 0 {
		t.Fatal("without the upgrade a housekeeper moved a Party")
	}
	g.own(Matchmaking)
	for f := 0; p.Room == 0; f++ {
		if f > 90*tps {
			t.Fatal("no housekeeper fixed the red Room")
		}
		g.step(0, 0)
	}
	if p.Room != 2 || g.offered != Num(g.hotel.Rooms[2].Type.Rate) {
		t.Fatalf("moved to Room %d for $%v; want Room 103 on the house, $%d", p.Room, g.offered, g.hotel.Rooms[2].Type.Rate)
	}
}

// The talk's close-in and blur compiles.
func TestFocusShader(t *testing.T) {
	if _, err := ebiten.NewShader([]byte(focusSrc)); err != nil {
		t.Fatal(err)
	}
}
