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
	if i := row(t, rowChange); shop[i].Tab != tabRooms || !g.onSale(i) {
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
	before := g.wallet[Money] // the Party's rent came in meanwhile
	if g.cancelChange(g.changeFor(1)); g.wallet[Money] != before+bedKinds[1].Fee || g.changing(1) {
		t.Fatalf("cancelled: Money %v, changing %v; want the fee back", g.wallet[Money], g.changing(1))
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
	g.wallet = [2]Num{200, roomMirrorEcto + 10}
	m := row(t, RoomMirror)
	if it := shop[m]; it.Tab != tabRooms || it.Cost != (Cost{Ecto: roomMirrorEcto}) || !g.onSale(m) {
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
	why := g.placeOrder()
	paid := g.wallet == [2]Num{200, 10}
	if deliver(t, g); why != "" || !paid || !g.unlocked(RoomMirror) {
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
	if deliver(t, g); g.hotel.Rooms[0].Mirror.Empty() {
		t.Fatal("the courier did not hang the Mirror")
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

// The pacing goal (2026-11-14, 4c): a player meets most of the game within 10 minutes of real play. The goals
// Strategy plays the agreed order (Room 1, the Mirror, Room 2, then all that earns) and the middle Run of 8
// seeds must reach the first Growth about minute 4, the in-Room Mirror about 6, the first housekeeper about 7 and
// the first Deluxe upgrade (on sale after the Growth) about 9.5. Bands, not points. The
// longest stretch with nothing to buy and nothing to do is 30 s before the Growth (a 40 s haunt cycle leaves
// about that; a Room served past cozy takes one haunt without a stain, so the check allows 45), 60 s before the
// housekeeper, 90 s before the in-Room Mirror.
func TestPacing(t *testing.T) {
	var marks [4][]float64 // real minutes
	var waits [3]float64
	for seed := uint64(1); seed <= 8; seed++ {
		r := RunSim(Config{Cols: 2, Rows: 1, Days: 4, Seed: seed, Name: "goals", Strategy: Strategies["goals"], React: 30, KeyGap: 6})
		for _, f := range r.Failures {
			t.Errorf("seed %d %s: %s", seed, f.Clock, f.Reason)
		}
		if len(r.Grown) == 0 || r.HiredAt < 0 || r.RoomMirror < 0 || r.DeluxeAt < 0 {
			t.Errorf("seed %d: Growth %v, housekeeper %.1f h, in-Room Mirror %.1f h, Deluxe %.1f h; want all four within 4 days", seed, r.Grown, r.HiredAt, r.RoomMirror, r.DeluxeAt)
			continue
		}
		for k, h := range []float64{r.Grown[0], r.HiredAt, r.RoomMirror, r.DeluxeAt} {
			marks[k] = append(marks[k], h/6) // a game hour is 10 real seconds
		}
		for k, w := range r.Waits {
			waits[k] = max(waits[k], w)
		}
	}
	for k, band := range [][2]float64{{3, 5}, {5.5, 8.5}, {4.5, 8}, {8, 11}} {
		what := []string{"the first Growth", "the first housekeeper", "the in-Room Mirror", "the first Deluxe upgrade"}[k]
		if sort.Float64s(marks[k]); len(marks[k]) > 0 {
			mid := marks[k][len(marks[k])/2]
			t.Logf("%s, real minutes: %.1f", what, marks[k])
			if mid < band[0] || mid > band[1] {
				t.Errorf("%s: the middle Run at %.1f min, want %g..%g", what, mid, band[0], band[1])
			}
		}
	}
	t.Logf("longest waits, s: %.0f", waits)
	for k, most := range [3]float64{45, 60, 90} {
		if waits[k] > most {
			t.Errorf("stretch %d: %.0f s with nothing to buy and nothing to do, want %.0f at most", k, waits[k], most)
		}
	}
}

// 2026-11-14 (4b): the Deluxe tier. The unlock is bought with Ectoplasm on the HOTEL tab; until then no Room
// can be upgraded. Then "Deluxe" on a Room, for deluxeFee Ectoplasm, changes it to the Deluxe of the beds it
// has, at the Deluxe Rate; once Deluxe it can't be upgraded again, and a change of beds keeps it Deluxe.
func TestDeluxe(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.own(Mirror)
	g.own(BedsTwin)
	g.wallet = [2]Num{1000, deluxeEcto + deluxeFee}
	if g.onSale(row(t, Deluxe)) {
		t.Fatal("Deluxe is on sale before the hotel has grown")
	}
	g.grid0 = [2]int{1, 1} // as if the hotel had grown: it expands first (2026-11-14)
	if i := row(t, Deluxe); shop[i].Tab != tabHotel || shop[i].Cost != (Cost{Ecto: deluxeEcto}) || !g.onSale(i) {
		t.Fatal("Deluxe is not an Ectoplasm unlock on sale on the HOTEL tab")
	}
	up := row(t, rowDeluxe)
	if g.onSale(up) || g.addToCart(up, 0) == "" {
		t.Fatal("a Room can be upgraded before the Deluxe unlock")
	}
	g.own(Deluxe)
	g.wallet[Ecto] -= deluxeEcto
	if why := g.addToCart(up, 0); why != "" {
		t.Fatal(why)
	}
	if why := g.placeOrder(); why != "" {
		t.Fatal(why)
	}
	g.step(0, 0)
	want, _ := world.TypeIn(g.types, deluxeTier, "Single")
	if r := g.hotel.Rooms[0]; r.Type.Name != deluxeTier || r.Type.Beds != "Single" || r.Type.Rate != want.Rate || want.Rate <= rateOf(t, g, "Single") || g.wallet[Ecto] != 0 {
		t.Fatalf("upgraded: %s %s at %d, Ectoplasm %v; want Deluxe Single at %d, dearer than Standard, all the Ectoplasm spent", r.Type.Name, r.Type.Beds, r.Type.Rate, g.wallet[Ecto], want.Rate)
	}
	for g.changing(0) {
		g.step(0, 0)
	}
	g.wallet[Ecto] = deluxeFee
	if why := g.addToCart(up, 0); why != "ROOM 101 IS ALREADY DELUXE" {
		t.Fatalf("a second upgrade: %q", why)
	}
	if why := order(t, g, 0, "Twin"); why != "" {
		t.Fatal(why)
	}
	if g.step(0, 0); g.hotel.Rooms[0].Type.Name != deluxeTier || g.hotel.Rooms[0].Type.Beds != "Twin" {
		t.Fatalf("a Deluxe Room changed to Twin is %s %s, want Deluxe Twin", g.hotel.Rooms[0].Type.Name, g.hotel.Rooms[0].Type.Beds)
	}
}
