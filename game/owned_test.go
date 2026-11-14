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
		if !g.boarded(i) {
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
	if shop[k].Tab != tabRooms || !g.onSale(k) {
		t.Fatal("the Room key is not on sale in ROOMS from the start")
	}
	if g.addToCart(k, 1) != "" {
		t.Fatal("the one key's worth does not buy the first key")
	}
	if why := g.addToCart(k, 1); why == "" { // one key a Room
		t.Fatal("a second key for the same Room went in the cart")
	}
	if why := g.addToCart(k, 0); why != "CAN'T AFFORD" { // a second Room's key is priced after the first
		t.Fatalf("a second key with Money for one: %q", why)
	}
	if why := g.placeOrder(); why != "" {
		t.Fatal(why)
	}
	if g.wallet[Money] != 0 || g.keysOut() != 1 || g.priceOf(k)[Money] != keyPrice(1) {
		t.Fatalf("Money %v after the order, %d keys on their way, the next at %v; want 0, 1, %v", g.wallet[Money], g.keysOut(), g.priceOf(k), keyPrice(1))
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
	if why := g.addToCart(k, 0); why != "CAN'T AFFORD" {
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

// the opening's state machine, without rendering: the title is the hotel in ghost with ghosts on the street;
// SPACE brings the three lines on a timer; SPACE again walks the Concierge in from the street while the world
// shifts to real; then the Run begins with the Clock untouched.
func TestIntro(t *testing.T) {
	g, s := headless(t, 2, 1, 1)
	g.openAtStart = false
	g.PlayIntro()
	frame := func(space bool) {
		if space {
			s.hold(ebiten.KeySpace)
		} else {
			s.hold()
		}
		g.Update()
	}
	in := g.intro
	if g.state != StateTitle || in == nil || in.phase != introTitle || !g.ghost || len(in.ghosts) == 0 || g.ownedCount() != 0 {
		t.Fatalf("state %d, intro %+v, ghost %v; want the title on the boarded hotel in ghost, ghosts on the street", g.state, in, g.ghost)
	}
	clock, x := g.clock, in.ghosts[0].X
	frame(false)
	if in.ghosts[0].X == x || g.state != StateTitle {
		t.Fatal("the street ghosts stand still on the title, or the title left without SPACE")
	}
	frame(true)
	frame(false)
	if in.phase != introText {
		t.Fatalf("phase %d after SPACE on the title, want the lines", in.phase)
	}
	for in.frame < (len(introLines)-1)*introLineGap { // the lines come on their own
		frame(false)
	}
	if in.phase != introText {
		t.Fatal("the lines moved on without SPACE")
	}
	frame(true)
	if in.phase != introEnter || image.Pt(int(g.player.X), int(g.player.Y)) != g.hotel.Spawn || len(in.walk.Path) == 0 {
		t.Fatalf("phase %d, the Concierge at %v; want him on the street, bound for his desk", in.phase, image.Pt(int(g.player.X), int(g.player.Y)))
	}
	shifted := false
	for f := 0; g.intro != nil; f++ {
		if f > 60*tps {
			t.Fatal("the Concierge never reached his desk")
		}
		frame(false)
		shifted = shifted || g.shifting != nil && !g.shifting.toGhost
	}
	if at := image.Pt(int(g.player.X), int(g.player.Y)); !shifted || g.ghost || at != g.hotel.ConciergeStart || g.clock != clock {
		t.Fatalf("shifted %v, ghost %v, at %v, clock %d (was %d); want real, at the desk, the Clock untouched", shifted, g.ghost, at, g.clock, clock)
	}
	frame(false)
	if g.clock == clock {
		t.Error("the world is still waiting after the opening")
	}
	keys(g, s, ebiten.KeyEscape, ebiten.KeyS, ebiten.KeySpace) // the pause's Quit: back to the title, a new hotel waiting behind it
	if g.state != StateTitle || g.intro == nil || g.intro.phase != introTitle {
		t.Error("ESC did not bring the title's opening back")
	}
}

// SPACE shows every line at once; a Run without introOn (the sim, the prints) has no opening at all.
func TestIntroSkips(t *testing.T) {
	g, s := headless(t, 2, 1, 1)
	g.PlayIntro()
	for _, space := range []bool{true, false, true} {
		s.hold()
		if space {
			s.hold(ebiten.KeySpace)
		}
		g.Update()
	}
	if g.intro.phase != introText || g.intro.frame < (len(introLines)-1)*introLineGap {
		t.Errorf("phase %d frame %d after SPACE in the lines, want them all showing", g.intro.phase, g.intro.frame)
	}
	g.introOn = false
	g.startRun()
	if g.intro != nil || g.ghost {
		t.Error("a Run with the intro off still has one")
	}
}

// 2026-11-11: no wrong first buy. With no Room open only the key is on sale; the Mirror comes after, from rent.
func TestNoStrandingBuy(t *testing.T) {
	g := closedRun(t)
	for i, it := range shop {
		if want := it.ID == rowKey; g.onSale(i) != want {
			t.Errorf("%s on sale %v with no Room open, want %v", it.Name, g.onSale(i), want)
		}
	}
	g.roomOwned[0] = true
	if !g.onSale(row(t, Mirror)) {
		t.Error("the Mirror is not on sale with a Room open")
	}
}

// 2026-11-11: the Mirror is within reach on Day 1 (16 h from 08:00), every Run: the first Room pays from its
// first hour (a Tourist stays in the day it arrives).
func TestMirrorOnDayOne(t *testing.T) {
	for seed := uint64(1); seed <= 12; seed++ {
		r := RunSim(Config{Cols: 2, Rows: 1, Days: 1, Seed: seed, Name: "opener", Strategy: Strategies["opener"], React: 30, KeyGap: 6})
		if r.MirrorAt < 0 || r.MirrorAt > 12 {
			t.Errorf("seed %d: mirror at %.1f h, want it by 20:00 on Day 1", seed, r.MirrorAt)
		}
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
		r := RunSim(Config{Cols: 2, Rows: 1, Days: 5, Seed: 2, Name: name, Strategy: Strategies[name], React: 30, KeyGap: 6})
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
