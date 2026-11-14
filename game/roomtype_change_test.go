package game

import (
	"slices"
	"sort"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// order buys a Room's change to a bed layout through the store: the layout picked, the cart, the checkout.
// "" or what the store said.
func order(t *testing.T, g *Game, room int, beds string) string {
	t.Helper()
	g.termBeds = slices.IndexFunc(bedKinds[:], func(b bedKind) bool { return b.Beds == beds })
	if why := g.addToCart(row(t, rowChange), room); why != "" {
		return why
	}
	return g.placeOrder()
}

func rateOf(t *testing.T, g *Game, beds string) int {
	t.Helper()
	rt, ok := world.TypeOf(g.types, beds)
	if !ok {
		t.Fatalf("no Standard %s", beds)
	}
	return rt.Rate
}

// T276: every Room opens a Standard Single, whatever the seed. Twin, then Double, are HOTEL unlocks behind
// the Mirror; only an unlocked layout can be picked for a change.
func TestRoomsOpenSingle(t *testing.T) {
	for seed := uint64(1); seed <= 5; seed++ {
		g, _ := headless(t, 3, 2, seed)
		g.startRun()
		for _, r := range g.hotel.Rooms {
			if r.Type.Name != "Standard" || r.Type.Beds != "Single" || !r.Mirror.Empty() {
				t.Fatalf("seed %d: Room %s opens %s %s, mirror %v", seed, r.Number, r.Type.Name, r.Type.Beds, r.Mirror)
			}
		}
	}
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.wallet[Money] = 500
	twin, double := row(t, BedsTwin), row(t, BedsDouble)
	if g.onSale(twin) || g.onSale(double) || shop[twin].Tab != tabHotel || shop[double].Tab != tabHotel {
		t.Fatal("Twin or Double is on sale before the Mirror, or not a HOTEL unlock")
	}
	if g.pickBeds(1); g.termBeds != 0 {
		t.Errorf("picked %s with only Single unlocked", bedKinds[g.termBeds].Beds)
	}
	if why := order(t, g, 0, "Twin"); why != "TWIN IS NOT UNLOCKED" || g.changing(0) {
		t.Errorf("a change to locked Twin: %q", why)
	}
	g.own(Mirror)
	g.earned[Money] += shop[twin].Earned[Money]
	if !g.onSale(twin) || g.onSale(double) {
		t.Error("after the Mirror and its earnings, want Twin on sale and Double not yet")
	}
	g.own(BedsTwin)
	g.earned[Money] += shop[double].Earned[Money]
	if !g.onSale(double) {
		t.Error("Double is not on sale after Twin and its earnings")
	}
	g.termBeds = 0
	if g.pickBeds(1); bedKinds[g.termBeds].Beds != "Twin" {
		t.Errorf("W/S picked %s, want Twin", bedKinds[g.termBeds].Beds)
	}
	if g.pickBeds(1); bedKinds[g.termBeds].Beds != "Single" {
		t.Errorf("W/S went on to %s past locked Double, want Single", bedKinds[g.termBeds].Beds)
	}
}

// T277: the "Change type" Service. An empty Room changes on the world's first frame after the order (the
// Terminal closing), its reel rolling reelFrames; the fee is the target layout's, Single too; capacity and
// Rate follow the beds; the Room's Mood and its dirt stay.
func TestChangeEmptyRoom(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.own(BedsTwin)
	g.wallet[Money] = 1000
	g.setDirtLevel(0, 3*spotUnit)
	mood, dirt, walls := g.roomMood(0), g.dirtLevel(0), len(g.walls)
	if i := row(t, rowChange); shop[i].Tab != tabHotel || !g.onSale(i) {
		t.Fatal("Change type is not a Service on sale")
	}
	if why := order(t, g, 0, "Single"); why != "ROOM 101 IS ALREADY SINGLE" {
		t.Errorf("a change to the type it has: %q", why)
	}
	if why := order(t, g, 0, "Twin"); why != "" {
		t.Fatal(why)
	}
	if g.wallet != [2]Num{1000 - bedKinds[1].Fee, 0} || !g.changing(0) || len(g.pending()) != 1 || g.hotel.Rooms[0].Type.Beds != "Single" {
		t.Fatalf("after the order: wallet %v, changing %v, Room %s; want the fee paid and the Room still Single while time stands", g.wallet, g.changing(0), g.hotel.Rooms[0].Type.Beds)
	}
	if why := order(t, g, 0, "Single"); why != "ROOM 101 IS ALREADY CHANGING" {
		t.Errorf("a second change on the Room: %q", why)
	}
	g.step(0, 0)
	r := g.hotel.Rooms[0]
	if r.Type.Beds != "Twin" || len(r.Stays) != 2 || r.Type.Rate != rateOf(t, g, "Twin") || !g.reeling(0) || len(g.pending()) != 0 {
		t.Fatalf("a frame on: %s, %d Stay Spots, reeling %v; want Twin, 2, rolling", r.Type.Beds, len(r.Stays), g.reeling(0))
	}
	if !r.Mirror.Empty() {
		t.Error("a changed Room has a Mirror before the in-Room Mirror is bought")
	}
	if g.roomMood(0) != mood || g.dirtLevel(0) != dirt || len(g.walls) != walls {
		t.Errorf("Mood %d → %d, dirt %d → %d: the change touched the Room's Mood", mood, g.roomMood(0), dirt, g.dirtLevel(0))
	}
	floor := g.floorCells(0)
	for _, s := range g.spots(0) {
		if !slices.Contains(floor, s.cell) {
			t.Errorf("a dirt spot at %v is off the new floor", s.cell)
		}
	}
	for range reelFrames - 1 {
		if g.step(0, 0); !g.changing(0) || g.freeRoom(1) == 0 {
			t.Fatal("the Room took bookings while its reel rolled")
		}
	}
	if g.step(0, 0); g.changing(0) || g.reeling(0) {
		t.Fatal("the reel never stopped")
	}
	for i := 0; ; i++ { // the next Party books the new type
		if p := g.arrive(0); len(p.Guests) == 2 {
			if p.Rate != rateOf(t, g, "Twin") {
				t.Errorf("a pair pays $%d a night, want the Twin's $%d", p.Rate, rateOf(t, g, "Twin"))
			}
			break
		}
		if g.parties = nil; i > 200 {
			t.Fatal("no pair ever books the Twin")
		}
	}
	g.parties = nil
	if why := order(t, g, 0, "Single"); why != "" || g.wallet[Money] != 1000-bedKinds[1].Fee-bedKinds[0].Fee {
		t.Errorf("back to Single: %q, Money %v; want its own fee paid", why, g.wallet[Money])
	}
}

// T277: a booked Room's change is scheduled. Neither realm books the Room meanwhile; it lands once the Room
// is empty and clear of the Concierge and a Priest. From the CHANGES list it is cancelled with a full refund.
func TestChangeBookedRoom(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.unlock(Mirror)
	g.own(BedsTwin)
	g.wallet[Money] = 100
	p := g.arrive(1)
	p.Routine = homebody
	g.settleIn(p)
	if why := order(t, g, 1, "Twin"); why != "" {
		t.Fatal(why)
	}
	g.ghostArrivals[1] = arrival{g.clock, g.clock}
	for range hourFrames {
		g.step(0, 0)
	}
	if g.hotel.Rooms[1].Type.Beds != "Single" || len(g.pending()) != 1 {
		t.Fatal("the change landed with the Party in the Room")
	}
	if !g.vacant(1, Ghost) {
		t.Error("a ghost booked a Room waiting to change")
	}
	// cancel: the fee comes back and the Room books again
	g.termTab, g.termPick = tabCart, len(g.cart)+1
	if g.rowCount() != len(g.cart)+2 {
		t.Fatalf("%d cart rows, want the checkout and one waiting change", g.rowCount())
	}
	before := g.wallet[Money] // the Party's rent came in meanwhile
	if msg := g.press(); g.wallet[Money] != before+bedKinds[1].Fee || g.changing(1) || msg != "CHANGE CANCELLED: $"+fmtNum(bedKinds[1].Fee)+" BACK" {
		t.Fatalf("cancelled: %q, Money %v, changing %v; want the fee back", msg, g.wallet[Money], g.changing(1))
	}
	if why := order(t, g, 1, "Twin"); why != "" {
		t.Fatal(why)
	}
	// the Party checks out; the Concierge, then a Priest, stands in the Room: the change still waits
	in := g.workSpot(1)
	g.player.X, g.player.Y = float64(in.X), float64(in.Y-2*world.TileSize)
	p.Checkout = g.clock
	for i := 0; !g.vacant(1, Human); i++ {
		if g.step(0, 0); i > 60*tps {
			t.Fatal("the Party never checked out")
		}
	}
	for range 2 * tps {
		g.step(0, 0)
	}
	if g.hotel.Rooms[1].Type.Beds != "Single" {
		t.Fatal("the change landed with the Concierge in the Room")
	}
	g.player.X, g.player.Y = float64(g.hotel.ConciergeStart.X), float64(g.hotel.ConciergeStart.Y)
	g.priests = []*priest{{Guest: entities.NewGuest(in, fatherK.name), until: 1 << 30}}
	if g.roomClear(1) {
		t.Error("the Room counts as clear with a Priest in it")
	}
	g.priests = nil
	if g.step(0, 0); g.hotel.Rooms[1].Type.Beds != "Twin" || !g.reeling(1) {
		t.Fatalf("empty and clear, the Room is %s, reeling %v; want Twin, rolling", g.hotel.Rooms[1].Type.Beds, g.reeling(1))
	}
	for i := 0; g.vacant(1, Human); i++ { // and its next Party books the Twin
		if g.step(0, 0); i > 2*dayFrames {
			t.Fatal("no Party came to the changed Room")
		}
	}
	for _, o := range g.parties {
		if o.Occupies(1, Human) && o.Rate != rateOf(t, g, "Twin") {
			t.Errorf("the next Party pays $%d a night, want the Twin's $%d", o.Rate, rateOf(t, g, "Twin"))
		}
	}
}

// T278, T301: the in-Room Mirror is bought once, with Ectoplasm, for its first Room, where it is hung at
// once, guests in or not, with no reel and no wait. From then on a change costs
// Ectoplasm too, and every Room that changes, or that a key opens, has its own Mirror: SPACE under it shifts.
// A boarded Room can't change.
func TestRoomMirror(t *testing.T) {
	g := closedRun(t)
	s := g.in.(*script)
	g.roomOwned[0] = true
	g.own(Mirror)
	g.own(BedsTwin)
	g.wallet = [2]Num{200, 100}
	m := row(t, RoomMirror)
	if it := shop[m]; it.Tab != tabServices || it.Cost != (Cost{Ecto: roomMirrorEcto}) || !g.onSale(m) {
		t.Fatalf("the Room mirror: tab %d, cost %v, on sale %v; want a Service for Ectoplasm only", it.Tab, it.Cost, g.onSale(m))
	}
	g.termBeds = 1
	if c := changeCost(g); c != (Cost{Money: bedKinds[1].Fee}) {
		t.Errorf("a change costs %v before the in-Room Mirror, want Money only", c)
	}
	if why := order(t, g, 1, "Twin"); why != "ROOM 102 IS BOARDED UP" {
		t.Errorf("a change on a boarded Room: %q", why)
	}
	if why := g.addToCart(m, 1); why != "ROOM 102 IS BOARDED UP" {
		t.Errorf("the Mirror for a boarded Room: %q", why)
	}
	if why := g.addToCart(m, 0); why != "" {
		t.Fatal(why)
	}
	if why := g.placeOrder(); why != "" || g.wallet != [2]Num{200, 100 - roomMirrorEcto} || !g.unlocked(RoomMirror) {
		t.Fatalf("the order: %q, wallet %v, unlocked %v", why, g.wallet, g.unlocked(RoomMirror))
	}
	if why := g.addToCart(m, 0); why != "ALREADY OWNED" || slices.Contains(g.forSale(), m) {
		t.Errorf("a second in-Room Mirror: %q", why)
	}
	r := g.hotel.Rooms[0]
	if r.Mirror.Empty() || r.Type.Beds != "Single" || g.changing(0) {
		t.Fatalf("the first Room: mirror %v, %s, changing %v; want its Mirror on at once, still Single, no reel", r.Mirror, r.Type.Beds, g.changing(0))
	}
	if c := changeCost(g); c != (Cost{Money: bedKinds[1].Fee, Ecto: changeEcto}) {
		t.Errorf("a change costs %v after the in-Room Mirror, want Money and Ectoplasm", c)
	}
	if why := order(t, g, 0, "Twin"); why != "" {
		t.Fatal(why)
	}
	for g.step(0, 0); g.changing(0); {
		g.step(0, 0)
	}
	if r = g.hotel.Rooms[0]; r.Mirror.Empty() || r.Type.Beds != "Twin" {
		t.Errorf("changed again: mirror %v, %s; want it kept, on a Twin", r.Mirror, r.Type.Beds)
	}
	if g.openRoom(1); g.hotel.Rooms[1].Mirror.Empty() {
		t.Error("a Room a key opened after the in-Room Mirror has none")
	}
	at := r.Mirror.Min.Add(r.Mirror.Max).Div(2)
	g.player.X, g.player.Y = float64(at.X), float64(at.Y)
	s.hold()
	g.Update()
	s.hold(ebiten.KeySpace)
	if g.Update(); g.shifting == nil || !g.shifting.toGhost {
		t.Error("SPACE under the Room's Mirror did not shift")
	}

	// with a Party in and a change waiting: the Mirror is hung now, the change still waits; cancelling it
	// refunds its fee only
	g = closedRun(t)
	g.roomOwned[0] = true
	g.own(Mirror)
	g.own(BedsTwin)
	p := g.arrive(0)
	g.settleIn(p)
	p.Rate = 0 // no rent in the wallet
	g.wallet = [2]Num{bedKinds[1].Fee, roomMirrorEcto}
	if why := order(t, g, 0, "Twin"); why != "" {
		t.Fatal(why)
	}
	if why := g.addToCart(m, 0); why != "" || g.placeOrder() != "" {
		t.Fatal(why)
	}
	if r = g.hotel.Rooms[0]; r.Mirror.Empty() || r.Type.Beds != "Single" || len(g.pending()) != 1 || g.wallet != [2]Num{} {
		t.Fatalf("a Party in: mirror %v, %s, %d changes waiting, wallet %v; want the Mirror on, the change to Twin waiting", r.Mirror, r.Type.Beds, len(g.pending()), g.wallet)
	}
	if _, in := g.inside(0); len(in) == 0 {
		t.Error("hanging the Mirror moved the Party out")
	}
	g.cancelChange(g.pending()[0])
	if g.wallet != [2]Num{bedKinds[1].Fee, 0} || !g.unlocked(RoomMirror) || g.hotel.Rooms[0].Mirror.Empty() {
		t.Errorf("cancelled: wallet %v, unlocked %v; want the fee back and the Mirror kept", g.wallet, g.unlocked(RoomMirror))
	}
}

// T297, T303: the opener plays K1's part of the opening through the Terminal: Twin beds, a Room changed to
// Twin, the in-Room Mirror for it. The pacing goal (2026-11-11): the in-Room Mirror is bought within 10 minutes
// of regular play (60 game hours) on most Runs, and on none past 90 h. It holds since the Mirror is hung at
// once and a Room is cleaned with its guests in: 8 seeds finish at 29..77 h. The Mirror's price is not the
// lever (90, 120 and 150 Ectoplasm give the same ends); the wait is for Twin beds and the first change.
func TestPacing(t *testing.T) {
	var at []float64
	for seed := uint64(1); seed <= 8; seed++ {
		r := RunSim(Config{Cols: 2, Rows: 1, Days: 5, Seed: seed, Name: "opener", Strategy: Strategies["opener"], React: 30, KeyGap: 6})
		for _, f := range r.Failures {
			t.Errorf("seed %d %s: %s", seed, f.Clock, f.Reason)
		}
		if r.Changes == 0 {
			t.Errorf("seed %d: the opener never ordered a change of type", seed)
		}
		if r.RoomMirror >= 0 {
			at = append(at, r.RoomMirror)
		}
	}
	sort.Float64s(at)
	t.Logf("the first Room with its own Mirror, game hours, on %d of 8 seeds: %.1f", len(at), at)
	if len(at) != 8 || at[3] < 45 || at[4] > 60 || at[len(at)-1] > 90 { // Z1: a floor too, both middle Runs in the band
		t.Errorf("the in-Room Mirror on %d of 8 seeds, at %.1f h; want all 8, the median at 45..60 h, none past 90", len(at), at)
	}
}
