package game

import (
	"image"
	"testing"

	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// closedRun: a Run as a player starts it: the Rooms boarded up, Money for one key, the Mirror locked.
func closedRun(t *testing.T) *Game {
	t.Helper()
	g, _ := headless(t, 2, 1, 1)
	g.openAtStart = false
	g.startRun()
	return g
}

// T270: a Run starts with every Room unowned and Money for exactly one key; an unowned Room books nobody in
// either realm, its door stays shut to the Concierge, and it is drawn boarded.
func TestRoomsStartClosed(t *testing.T) {
	g := closedRun(t)
	if g.ownedCount() != 0 || g.wallet[Money] != keyPrice(0) || g.wallet[Money] != startMoney {
		t.Fatalf("owned %d, Money %v; want none owned and Money for exactly the first key", g.ownedCount(), g.wallet[Money])
	}
	g.unlock(Mirror) // ghosts would book too, if they could
	for i := range g.hotel.Rooms {
		if g.occupied(i) != true {
			t.Errorf("Room %d is not shut", i)
		}
	}
	spot := g.hotel.Rooms[0].Spot()
	g.player.X, g.player.Y = float64(spot.Min.X+world.TileSize/2), float64(spot.Min.Y+world.TileSize/2)
	for range 3 * dayFrames / 10 { // 3 game days, a tenth of the frames: the clock fast-forwarded
		g.clock += 9
		g.step(0, 0)
		if g.hotel.Doors[0].Open() > 0 {
			t.Fatal("the boarded Room's door opened to the Concierge at its door")
		}
	}
	if len(g.parties) != 0 || g.walkIns != 0 {
		t.Fatalf("%d Parties in a hotel of closed Rooms", len(g.parties))
	}
	for i := range g.hotel.Rooms {
		if g.freeRoom(1) != -1 || !g.vacant(i, Human) || !g.vacant(i, Ghost) {
			t.Fatalf("Room %d took a booking", i)
		}
	}
	d := g.hotel.Doors[0].Rect
	if !g.conciergeBlocked(d) {
		t.Error("the Concierge can walk into a boarded Room")
	}
	if g.sprite("door_boarded") == nil {
		t.Error("the boarded door sprite does not load")
	}
}

// T271: the Room key is a Service: it picks the Room, the courier walks it to that Room's door, and on the
// drop the Room is owned and starts booking. Its price rises with each key owned.
func TestRoomKey(t *testing.T) {
	g := closedRun(t)
	k := row(t, rowKey)
	if shop[k].Tab != tabServices || !g.onSale(k) {
		t.Fatal("the Room key is not on sale in SERVICES from the start")
	}
	if g.addToCart(k, 1) != "" {
		t.Fatal("the one key's worth does not buy the first key")
	}
	g.wallet[Money] += 100 // a fat wallet still gets one key at a time
	if why := g.addToCart(k, 0); why != "A KEY IS ALREADY ON ITS WAY" {
		t.Fatalf("a second key in the cart: %q", why)
	}
	if why := g.placeOrder(); why != "" {
		t.Fatal(why)
	}
	if g.wallet[Money] != 100 || !g.keyPending() {
		t.Fatalf("Money %v after the order, key on its way %v", g.wallet[Money], g.keyPending())
	}
	if g.isOwned(1) {
		t.Fatal("the Room is owned before the courier drops the key")
	}
	g.state = StatePlay
	door := g.doorDrop(1)
	atDoor := false
	for f := 0; len(g.couriers) > 0 && f < 3*60*tps; f++ {
		g.step(0, 0)
		if len(g.couriers) > 0 && image.Pt(int(g.couriers[0].X), int(g.couriers[0].Y)).Div(world.TileSize) == door.Div(world.TileSize) {
			atDoor = true
		}
	}
	if !atDoor {
		t.Fatal("the courier never stood at the Room's door")
	}
	if !g.isOwned(1) || g.isOwned(0) || g.ownedCount() != 1 {
		t.Fatalf("owned: %v; want only Room 1", g.roomOwned)
	}
	for f := 0; len(g.parties) == 0 && f < 2*dayFrames; f++ {
		g.step(0, 0)
	}
	if len(g.parties) == 0 || g.parties[0].Room != 1 {
		t.Fatalf("parties %v; want a Party in the opened Room 1", g.parties)
	}
	// the next key costs more, and the owned Room takes no second one
	if keyPrice(1) <= keyPrice(0) || g.priceOf(k)[Money] != keyPrice(1) {
		t.Errorf("price after one key %v; want it above the first, %v", g.priceOf(k), keyPrice(0))
	}
	g.wallet[Money] = keyPrice(1) - 1
	if why := g.addToCart(k, 0); why != "NOT ENOUGH IN THE WALLET" {
		t.Errorf("a key one dollar short: %q", why)
	}
	g.wallet[Money] = keyPrice(1)
	if why := g.addToCart(k, 1); why != "ROOM "+g.hotel.Rooms[1].Number+" IS ALREADY OPEN" {
		t.Errorf("a key for an owned Room: %q", why)
	}
	if g.addToCart(k, 0) != "" || g.cartTotal()[Money] != keyPrice(1) {
		t.Errorf("second key in the cart for %v, want %v", g.cartTotal(), keyPrice(1))
	}
}

// the intro's state machine, without rendering: the whole hotel, the zoom, three lines one per SPACE, the flip
// out of ghost into real, then the Run begins with the world and the Clock untouched.
func TestIntro(t *testing.T) {
	g, s := headless(t, 2, 1, 1)
	g.introOn = true
	s.hold(ebiten.KeySpace)
	g.Update() // the title
	if g.intro == nil || !g.ghost || g.intro.phase() != introWide {
		t.Fatalf("intro %+v, ghost %v after the title; want the whole hotel in ghost", g.intro, g.ghost)
	}
	clock := g.clock
	frame := func(space bool) {
		if space {
			s.hold(ebiten.KeySpace)
		} else {
			s.hold()
		}
		g.Update()
	}
	frame(false)
	for g.intro.phase() == introWide {
		frame(false)
	}
	if g.intro.frame != introHold || g.intro.phase() != introZoom {
		t.Fatalf("the whole hotel held %d frames", g.intro.frame)
	}
	for g.intro.phase() == introZoom {
		frame(false)
	}
	for line := range introText {
		if g.intro.phase() != introLines || g.intro.line != line {
			t.Fatalf("phase %d line %d; want line %d showing", g.intro.phase(), g.intro.line, line)
		}
		frame(false)
		frame(false)
		if g.intro.line != line {
			t.Fatalf("line %d moved on without SPACE", line)
		}
		frame(true)
	}
	if g.intro.phase() != introFlip || g.shifting == nil || g.shifting.toGhost {
		t.Fatalf("after the third line: %+v, shifting %v; want a flip to real", g.intro, g.shifting)
	}
	for f := 0; g.intro != nil; f++ {
		if f > 2*flipFrames {
			t.Fatal("the flip never ended")
		}
		frame(false)
	}
	if g.ghost || g.shifting != nil || g.clock != clock {
		t.Fatalf("ghost %v, clock %d (was %d); want real, the Clock untouched", g.ghost, g.clock, clock)
	}
	frame(false)
	if g.clock == clock {
		t.Error("the world is still waiting after the intro")
	}
}

// SPACE skips the zoom to the lines; a Run without introOn (the sim, the prints) has no intro at all.
func TestIntroSkips(t *testing.T) {
	g, s := headless(t, 2, 1, 1)
	g.introOn = true
	s.hold(ebiten.KeySpace)
	g.Update()
	s.hold()
	g.Update()
	s.hold(ebiten.KeySpace)
	g.Update()
	if g.intro.phase() != introLines {
		t.Errorf("phase %d after SPACE in the zoom, want the lines", g.intro.phase())
	}
	g.introOn = false
	g.startRun()
	if g.intro != nil || g.ghost {
		t.Error("a Run with the intro off still has one")
	}
}

// T273: the opener buys the first key, then the Mirror, early (the pacing goal), and a Run of any Strategy
// that opens Rooms books guests from closed Rooms.
func TestOpenerReachesMirror(t *testing.T) {
	r := RunSim(Config{Cols: 2, Rows: 1, Days: 3, Seed: 1, Name: "opener", Strategy: Strategies["opener"], React: 30, KeyGap: 6})
	t.Logf("%+v", r)
	for _, f := range r.Failures {
		t.Errorf("%s: %s", f.Clock, f.Reason)
	}
	if r.MirrorAt < 0 || r.MirrorAt > 2*24 || r.Bookings == 0 {
		t.Errorf("mirror at %.1f h, %d Bookings; want the Mirror within 2 game days and guests in the opened Room", r.MirrorAt, r.Bookings)
	}
}

func TestStrategiesOpenRooms(t *testing.T) {
	for _, name := range []string{"server", "smart", "keeper"} {
		r := RunSim(Config{Cols: 2, Rows: 1, Days: 3, Seed: 2, Name: name, Strategy: Strategies[name], React: 30, KeyGap: 6})
		for _, f := range r.Failures {
			t.Errorf("%s %s: %s", name, f.Clock, f.Reason)
		}
		if r.Bookings == 0 || r.Paid == 0 {
			t.Errorf("%s: %d Bookings, %d paid from closed Rooms; want it to buy a key first", name, r.Bookings, r.Paid)
		}
	}
	if r := RunSim(Config{Cols: 2, Rows: 1, Days: 2, Seed: 2, Name: "never", Strategy: Strategies["never"], React: 30, KeyGap: 6}); r.Bookings != 0 || r.MirrorAt != -1 {
		t.Errorf("never: %d Bookings, mirror at %v; want none, it buys nothing", r.Bookings, r.MirrorAt)
	}
}
