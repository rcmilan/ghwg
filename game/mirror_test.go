package game

import (
	"image"
	"math"
	"slices"
	"testing"

	"ghosthotel/world"
)

// mirrorRun: a Run on a cols x rows grid, every Room open, the Mirror owned and the Room mirror bought, nobody
// booked on their own; Room 0 hangs its mirror.
func mirrorRun(t *testing.T, cols, rows int) (*Game, *script) {
	t.Helper()
	g, s := headless(t, cols, rows, 1)
	g.startRun()
	g.own(Mirror)
	g.own(RoomMirror)
	g.arrivals, g.ghostArrivals, g.mirrorSlots = make([]arrival, len(g.hotel.Rooms)), make([]arrival, len(g.hotel.Rooms)), nil
	g.setRoom(0, g.hotel.Rooms[0].Type, true)
	return g, s
}

// 08 B1 (Room mirror, Two ghosts): a Room with a mirror takes a second ghost, which books on its own on the
// same schedule as a free slot, checks in through the mirror, and goes on its own; a Room without one never
// holds two.
func TestMirrorSecondGhost(t *testing.T) {
	g, _ := mirrorRun(t, 2, 1)
	first := ghostIn(t, g, 0, 5)
	other := ghostIn(t, g, 1, 5)
	g.fast = true
	var second *Party
	for i := 0; i < 8*hourFrames && second == nil; i++ {
		g.step(0, 0)
		for _, p := range g.parties {
			if p.Realm == Ghost && p.Room == 0 && p != first {
				second = p
			}
		}
	}
	if second == nil {
		t.Fatal("the mirror Room never booked a second ghost")
	}
	m := g.hotel.Rooms[0].Mirror
	if d := math.Hypot(second.Guests[0].X-float64(m.Min.X+m.Max.X)/2, second.Guests[0].Y-float64(m.Min.Y+m.Max.Y)/2); d > world.TileSize || !mirrorGhost(second) {
		t.Errorf("the second ghost came in %.0f px from the mirror (mirror ghost %v); want it through the mirror", d, mirrorGhost(second))
	}
	if g.ghostsIn(0) != 2 || g.ghostsIn(1) != 1 || !slices.Contains(g.parties, other) {
		t.Errorf("ghosts in Room 0 %d, in Room 1 %d; want 2 and 1", g.ghostsIn(0), g.ghostsIn(1))
	}
	for range 4 * hourFrames { // the Room without a mirror never takes a second
		g.step(0, 0)
		if g.ghostsIn(1) > 1 {
			t.Fatal("a Room without a mirror holds two ghosts")
		}
	}
	// each on its own: the first passes on, the second stays and the first's slot books again
	first.Guests[0].PassAt = g.clock
	for range passFrames + 1 {
		g.step(0, 0)
	}
	if slices.Contains(g.parties, first) {
		t.Fatal("the first ghost, passing on, is still a Party")
	}
	if g.ghostArrivals[0].at == 0 && g.mirrorSlot(0).at == 0 && g.ghostsIn(0) < 2 {
		t.Error("the freed slot didn't book its next ghost")
	}
}

// 08 B1 (Two ghosts): Matching reads the higher Phantasm of the Room's two ghosts.
func TestMirrorMatchingHigher(t *testing.T) {
	g, _ := mirrorRun(t, 2, 1)
	ghostOf(g, 0, 3)
	ghostOf(g, 0, 9)
	if g.ghostsIn(0) != 2 || g.roomPhantasm(0) != 9 {
		t.Fatalf("two ghosts of 3 and 9: %d in the Room, Phantasm %d; want 2 and 9", g.ghostsIn(0), g.roomPhantasm(0))
	}
	settledHuman(t, g, 0, 6)
	if g.pairing(0) != badPair {
		t.Errorf("a Nerve 6 human with a 9 beside a 3: pairing %d, want a bad pair", g.pairing(0))
	}
}

// 08 B1 (W2 gaps): sendOver sends every ghost of the Room over at once, both slots, each paying the Ectoplasm
// its remaining haunts would have earned; another Room's ghost stays.
func TestSendOver(t *testing.T) {
	g, _ := mirrorRun(t, 2, 1)
	a, b, c := ghostOf(g, 0, 4), ghostOf(g, 0, 10), ghostOf(g, 1, 6)
	a.Guests[0].Haunted = 1
	want := 3*hauntEcto(a.Guests[0]) + 10*hauntEcto(b.Guests[0])
	if hauntEcto(a.Guests[0]) <= 0 {
		t.Fatal("a haunt is worth no Ectoplasm")
	}
	g.sendOver(0)
	if g.wallet[Ecto] != want || g.earned[Ecto] != want {
		t.Errorf("sent over: Ectoplasm %v earned %v, want %v", g.wallet[Ecto], g.earned[Ecto], want)
	}
	if a.Guests[0].PassAt == 0 || b.Guests[0].PassAt == 0 || c.Guests[0].PassAt != 0 {
		t.Errorf("passing on: %d %d, the other Room's %d; want both here, none there", a.Guests[0].PassAt, b.Guests[0].PassAt, c.Guests[0].PassAt)
	}
	g.sendOver(0)
	if g.wallet[Ecto] != want {
		t.Errorf("sent over twice: Ectoplasm %v, want it paid once, %v", g.wallet[Ecto], want)
	}
}

// 08 B1 (Mirror retrofit, Ring layout, Mirrors and Father K): until the one-time Room mirror is owned the
// lower-right slot offers it; once owned, a Room without a mirror offers Add mirror there and one with a mirror
// offers nothing. Both cost Money and Ectoplasm. The courier brings it to the door and it is hung.
func TestAddMirror(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.own(Mirror)
	slot := func(room int) Unlock {
		if i := g.ring(room)[ringSlot[RoomMirror]]; i >= 0 {
			return shop[i].ID
		}
		return ""
	}
	if c := shop[row(t, RoomMirror)].Cost; slot(0) != RoomMirror || c[Money] == 0 || c[Ecto] == 0 {
		t.Fatalf("before the unlock: slot holds %q at %v; want the unlock, for Money and Ectoplasm", slot(0), c)
	}
	g.own(RoomMirror)
	g.setRoom(0, g.hotel.Rooms[0].Type, true)
	if slot(0) != "" || slot(1) != rowAddMirror {
		t.Fatalf("after: Room 0 (mirror) slot %q, Room 1 slot %q; want empty and Add mirror", slot(0), slot(1))
	}
	add := row(t, rowAddMirror)
	if c := shop[add].Cost; c[Money] == 0 || c[Ecto] == 0 {
		t.Errorf("Add mirror costs %v, want Money and Ectoplasm", c)
	}
	g.wallet = [2]Num{shop[add].Cost[Money] - 1, shop[add].Cost[Ecto]}
	if why := g.buyNow(add, 1); why != "CAN'T AFFORD" {
		t.Errorf("short of Money: %q", why)
	}
	g.wallet = shop[add].Cost
	if why := g.buyNow(add, 1); why != "" {
		t.Fatal(why)
	}
	if g.wallet != [2]Num{} || !g.hotel.Rooms[1].Mirror.Empty() || len(g.couriers) != 1 || slot(1) != "" {
		t.Fatalf("ordered: wallet %v, hung %v, couriers %d, slot %q; want paid, on its way, hidden", g.wallet, !g.hotel.Rooms[1].Mirror.Empty(), len(g.couriers), slot(1))
	}
	deliver(t, g)
	if g.hotel.Rooms[1].Mirror.Empty() || g.ghostSlots(1) != 2 || slot(1) != "" {
		t.Errorf("delivered: hung %v, slots %d, ring %q; want the mirror up, two ghost slots, no Add mirror", !g.hotel.Rooms[1].Mirror.Empty(), g.ghostSlots(1), slot(1))
	}
}

// growFloors: a 2x1 Run after n New floors.
func growFloors(t *testing.T, n int) *Game {
	t.Helper()
	g, _ := headless(t, 2, 1, 7)
	g.startRun()
	g.own(Mirror)
	for range n {
		g.growNext = growFloor
		g.beginGrow()
		g.growing, g.state, g.viewT = nil, StatePlay, 0
	}
	return g
}

// 08 B1 (Corridor mirrors, Ground corridor): a New floor's Corridor has a Mirror on a Room's front wall, covered,
// one a Corridor, none on the ground one; a New column keeps them; the same seed picks the same corners.
func TestCorridorMirrorsOnGrowth(t *testing.T) {
	g := growFloors(t, 2)
	if _, rows := g.hotel.Grid(); rows != 3 || len(g.hotel.Corridors) != 2 {
		t.Fatalf("%d rows, %d Corridor Mirrors; want 3 floors, 2 Mirrors (the ground Corridor has the Lobby's)", rows, len(g.hotel.Corridors))
	}
	for _, m := range g.hotel.Corridors {
		if m.Floor < 1 || m.Open || m.Room < 0 {
			t.Errorf("Mirror %+v: want a New floor's, covered", m)
		}
		if g.hotel.Rooms[m.Room].Origin.Y+world.RoomH != m.Spot.Min.Y/world.TileSize {
			t.Errorf("Mirror %+v: spot not in the Corridor under its Room", m)
		}
	}
	covered := 0
	for _, p := range g.sortedProps() {
		if g.covered(p) {
			covered++
		}
	}
	if covered != 4 { // two tiles each, nothing else
		t.Errorf("%d covered tiles, want 4", covered)
	}
	want := slices.Clone(g.hotel.Corridors)
	g.growNext = growCol
	g.beginGrow()
	if len(g.hotel.Corridors) != 2 || g.hotel.Corridors[0].Col != want[0].Col || g.hotel.Corridors[1].West != want[1].West {
		t.Errorf("after a New column: %+v, was %+v; want the same corners", g.hotel.Corridors, want)
	}
	h := growFloors(t, 2).hotel.Corridors
	for k := range h {
		if h[k].Col != want[k].Col || h[k].West != want[k].West {
			t.Errorf("same seed, Mirror %d at %d/%v then %d/%v", k, want[k].Col, want[k].West, h[k].Col, h[k].West)
		}
	}
	if g0, _ := headless(t, 2, 3, 1); len(g0.hotel.Corridors) != 0 {
		t.Error("a hotel that began 3 floors tall has Corridor Mirrors")
	}
}

// 08 B1 (Corridor mirrors, Glow): SPACE on the Corridor cell under a covered Mirror uncovers it for Ectoplasm, the
// price doubling; the glow shows only with the Ectoplasm and on that cell; uncovered, SPACE shifts there.
func TestCorridorMirrorUncover(t *testing.T) {
	g := growFloors(t, 2)
	s := g.in.(*script)
	m0, m1 := g.hotel.Corridors[0], g.hotel.Corridors[1]
	spot := center(m0.Spot)
	g.player.X, g.player.Y = float64(spot.X), float64(spot.Y)
	if act, r := g.actionable(); act != actNone || !r.Empty() {
		t.Fatalf("no Ectoplasm: actionable %d glow %v; want nothing", act, r)
	}
	if press(g, s, spot); g.hotel.Corridors[0].Open || g.shifting != nil {
		t.Fatal("uncovered with no Ectoplasm")
	}
	g.wallet[Ecto] = 3 * uncoverEcto
	if act, r := g.actionable(); act != actUncover || r != m0.Glass {
		t.Fatalf("with Ectoplasm: actionable %d glow %v, want uncover over %v", act, r, m0.Glass)
	}
	beside := spot.Add(image.Pt(world.TileSize, 0))
	if g.player.X, g.player.Y = float64(beside.X), float64(beside.Y); g.atCorridorMirror() >= 0 {
		t.Fatal("the next cell is on the Mirror's activation tile")
	}
	s.hold()
	g.Update()
	press(g, s, spot)
	if !g.hotel.Corridors[0].Open || g.wallet[Ecto] != 2*uncoverEcto {
		t.Fatalf("uncovered: open %v, Ectoplasm %v; want open and %d left", g.hotel.Corridors[0].Open, g.wallet[Ecto], 2*uncoverEcto)
	}
	if c := uncoverCost(g); c[Ecto] != 2*uncoverEcto {
		t.Errorf("the next costs %v, want double", c)
	}
	s.hold()
	g.Update()
	press(g, s, spot)
	if g.shifting == nil {
		t.Error("SPACE on an uncovered Corridor Mirror didn't shift")
	}
	if g.coveredCorridor(prop{cx: m0.Cell.X, cy: m0.Cell.Y}) || !g.coveredCorridor(prop{cx: m1.Cell.X, cy: m1.Cell.Y}) {
		t.Error("covered flags: the first still covered or the second uncovered")
	}
}

// 08 B1 (Corridor mirrors): the HOTEL row uncovers the next covered Mirror for the same price, at checkout, and
// is gone once none is left.
func TestCorridorMirrorRow(t *testing.T) {
	g := growFloors(t, 1)
	i := row(t, CorridorMirror)
	if !g.onSale(i) || g.priceOf(i) != (Cost{Ecto: uncoverEcto}) {
		t.Fatalf("on sale %v at %v; want on sale for %d Ectoplasm", g.onSale(i), g.priceOf(i), uncoverEcto)
	}
	g.wallet[Ecto] = 2 * uncoverEcto
	if why := g.addToCart(i, -1); why != "" {
		t.Fatal(why)
	}
	if why := g.placeOrder(); why != "" || !g.hotel.Corridors[0].Open || g.wallet[Ecto] != uncoverEcto {
		t.Fatalf("bought: %q, open %v, Ectoplasm %v; want it open and paid once", why, g.hotel.Corridors[0].Open, g.wallet[Ecto])
	}
	if g.onSale(i) || !g.gone(i) {
		t.Error("the row stays after the last Mirror is uncovered")
	}
}

// 08 B1 (W1 leftovers): dev starts combine. -match after -ghosts leaves Rooms 101 and 102 with the match's two
// pairs only (the later wins what both set), and never lowers a wallet another start filled.
func TestDevStartsCombine(t *testing.T) {
	g, _ := headless(t, 3, 1, 1)
	g.startRun()
	g.DevGhosts(9)
	g.wallet[Money] = 5000
	g.DevMatch(0)
	for room := range 2 {
		if g.ghostsIn(room) != 1 || g.humanIn(room) == nil {
			t.Errorf("Room %d: %d ghosts, human %v; want the match's one of each", room, g.ghostsIn(room), g.humanIn(room) != nil)
		}
	}
	if g.wallet[Money] != 5000 {
		t.Errorf("Money %v after -match, want the 5000 another start set kept", g.wallet[Money])
	}
}
