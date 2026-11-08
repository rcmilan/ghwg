package game

import (
	"image"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// settled: a fresh Run with one Homebody Party walked into its Room.
func settled(t *testing.T) (*Game, *script, *Party) {
	t.Helper()
	g, s := headless(t, 2, 1, 1)
	g.startRun()
	p := g.walkIn()
	p.Routine = homebody
	for i := 0; p.state != staying; i++ {
		if i > 60*tps {
			t.Fatal("the Party never reached its Room")
		}
		g.step(0, 0)
	}
	return g, s, p
}

func center(r image.Rectangle) image.Point { return r.Min.Add(r.Max).Div(2) }

// press puts the Concierge at pt and taps SPACE through Update.
func press(g *Game, s *script, pt image.Point) {
	g.player.X, g.player.Y = float64(pt.X), float64(pt.Y)
	s.hold()
	g.Update()
	s.hold(ebiten.KeySpace)
	g.Update()
}

// pairIn: a fresh Run on a cols×rows grid with a staying pair at the Stay Spots of Room room.
func pairIn(t *testing.T, cols, rows, room int) (*Game, *script, *Party) {
	t.Helper()
	g, s := headless(t, cols, rows, 1)
	g.startRun()
	r := g.hotel.Rooms[room]
	a, b := entities.NewGuest(r.Stays[0], "Adam"), entities.NewGuest(r.Stays[len(r.Stays)-1], "Alex")
	p := &Party{Guests: []*entities.Guest{&a, &b}, state: staying, Booking: Booking{Room: room, Nights: 1, Rate: 100, Checkout: 99 * dayFrames}}
	g.parties = []*Party{p}
	return g, s, p
}

// T188 (D181): a hex pays the wallet yield(room) Spook, counts one TaskHex tap and (re)starts the haunt:
// the whole Room shakes for hauntMinutes, each Guest inside +1 Fear every fearEvery game minutes
// (2026-11-08: the old haunt). Refused in real, when Hex is locked, and with no Guest inside.
func TestHex(t *testing.T) {
	g, _, p := pairIn(t, 2, 1, 0)
	at := center(g.hotel.Rooms[0].Spot())
	nothing := func(why string) {
		t.Helper()
		if g.hex(0, at) || g.wallet[Spook] != 0 || g.haunting(0) || g.taps[0][TaskHex] != 0 {
			t.Fatalf("%s: a hex landed (Spook %v, haunting %v, taps %v)", why, g.wallet[Spook], g.haunting(0), g.taps[0][TaskHex])
		}
	}
	nothing("in real")
	g.ghost = true
	g.locked = map[Unlock]bool{Hex: true}
	nothing("Hex locked")
	g.unlock(Hex)
	p.state = out
	nothing("its Party out")
	p.state = staying
	if g.hex(1, at) || g.taps[1][TaskHex] != 0 {
		t.Fatal("a Vacant Room was hexed")
	}

	if !g.hex(0, at) {
		t.Fatal("the hex was refused")
	}
	if p.Guests[0].Fear != 0 || g.wallet[Spook] != 1 || g.taps[0][TaskHex] != 1 {
		t.Fatalf("one hex: Fear %d, Spook %v, taps %v; want 0 (the haunt frightens, not the tap), 1, 1", p.Guests[0].Fear, g.wallet[Spook], g.taps[0][TaskHex])
	}
	if len(g.floaters) != 1 || g.floaters[0].kind != floatSpook || g.floaters[0].x != float64(at.X) {
		t.Fatalf("floaters %v, want one Spook \"+1\" at the tap", g.floaters)
	}
	o := g.hotel.Rooms[0].Origin
	jitter := 0
	for range 12 {
		jitter += abs(g.shake(prop{cx: o.X + 1, cy: o.Y + 1, l: world.LayerProps}))
		g.tick++
	}
	if jitter == 0 || g.shake(prop{cx: o.X + world.RoomW + 2, cy: o.Y + 1, l: world.LayerProps}) != 0 {
		t.Fatal("the haunted Room's furniture didn't shake, or furniture outside it did")
	}
	for range hauntMinutes * minuteFrames {
		g.clock++
		if g.clock%minuteFrames == 0 {
			g.scare()
		}
	}
	if f := p.Guests[0].Fear; g.haunting(0) || f < hauntFear-1 || f > hauntFear { // the parity of its first minute
		t.Fatalf("after a whole haunt: haunting %v, Fear %d; want over and about %d", g.haunting(0), f, hauntFear)
	}
}

// T189 (D181): in ghost, SPACE on a floor tile beside any of a Room's four walls, door included, hexes it
// there; two tiles away doesn't.
func TestHexAllSides(t *testing.T) {
	const ts = world.TileSize
	g, s, _ := pairIn(t, 3, 2, 4) // Room 4: Corridors on all four sides
	r := g.hotel.Rooms[4]
	o := r.Origin
	cell := func(x, y int) image.Point { return image.Pt(x*ts+ts/2, y*ts+ts/2) }
	g.ghost = true
	for side, at := range map[string]image.Point{
		"west":  cell(o.X-1, o.Y+5),
		"east":  cell(o.X+world.RoomW, o.Y+5),
		"north": cell(o.X+4, o.Y-1),
		"south": cell(o.X+2, o.Y+world.RoomH),
		"door":  cell(r.Door.X, r.Door.Y+2),
	} {
		if g.hotel.Map.Solid(at.X/ts, at.Y/ts) {
			t.Fatalf("%s: %v isn't floor", side, at.Div(ts))
		}
		taps := g.taps[4][TaskHex]
		press(g, s, at)
		if g.taps[4][TaskHex] != taps+1 {
			t.Errorf("SPACE beside the %s wall: no hex", side)
		}
		if f := g.floaters[len(g.floaters)-1]; f.x != float64(at.X) || f.y != float64(at.Y) {
			t.Errorf("%s: the \"+N\" floats from (%v,%v), want the tap's tile %v", side, f.x, f.y, at)
		}
	}
	taps := g.taps[4][TaskHex]
	press(g, s, cell(r.Door.X, r.Door.Y+3))
	press(g, s, cell(o.X+4, o.Y-2))
	if g.taps[4][TaskHex] != taps {
		t.Fatal("SPACE two tiles from the walls hexed the Room")
	}
}

// T190: yield(room) = 1 + Room Spook/hexScale, linear.
func TestYield(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	for spook, want := range map[int]Num{0: 1, 1: 1 + 1.0/hexScale, hexScale: 2, 3 * hexScale: 4} {
		if g.roomSpook[0] = spook; g.yield(0) != want {
			t.Errorf("yield at Room Spook %d: %v, want %v", spook, g.yield(0), want)
		}
	}
}

// T191: at rotLimit a Room rots: each Guest inside loses rotComfort a game minute, so the Party pays
// less, and its Spook floor shows full; at spookLimit it closes as before.
func TestRot(t *testing.T) {
	if rotLimit >= spookLimit {
		t.Fatalf("rotLimit %d must be under spookLimit %d", rotLimit, spookLimit)
	}
	g, _, p := pairIn(t, 2, 1, 0)
	pay := p.Payout()
	g.roomSpook[0] = rotLimit - 1
	g.scare()
	if p.Guests[0].Comfort != 0 || g.roomShift(0) >= 1 {
		t.Fatalf("under rotLimit: Comfort %d, floor %v; want 0 and under 1", p.Guests[0].Comfort, g.roomShift(0))
	}
	g.roomSpook[0] = rotLimit
	g.scare()
	g.scare()
	if p.Guests[0].Comfort != -2*rotComfort || p.Payout() >= pay || g.roomShift(0) != 1 || g.closed(0) {
		t.Fatalf("2 rotting minutes: Comfort %d, pay %d (was %d), floor %v, closed %v", p.Guests[0].Comfort, p.Payout(), pay, g.roomShift(0), g.closed(0))
	}
	for range 10 * comfortMax {
		g.scare()
	}
	if p.Guests[0].Comfort != -comfortMax || p.Payout() <= 0 {
		t.Fatalf("long rot: Comfort %d, pay %d; want floored at %d, still paying", p.Guests[0].Comfort, p.Payout(), -comfortMax)
	}
	if g.roomSpook[0] = spookLimit; !g.closed(0) {
		t.Fatal("not Closed at spookLimit")
	}
}

// T098 (D146): hexes raise Fear; at fearLimit the Party flees unpaid, and its Room is Vacant at once.
func TestFlee(t *testing.T) {
	g, _, p := settled(t)
	g.ghost = true
	room, at := p.Room, center(g.hotel.Rooms[p.Room].Spot())
	for i := 0; !p.Fled; i++ {
		if i > 2*fearLimit*minuteFrames {
			t.Fatalf("no flight: Fear %d", p.Guests[0].Fear)
		}
		if i%minuteFrames == 0 {
			g.hex(room, at)
		}
		g.step(0, 0)
	}
	if !g.vacant(room) || p.state != leaving {
		t.Fatalf("fled: vacant %v state %d, want Vacant and leaving", g.vacant(room), p.state)
	}
	for i := 0; len(g.parties) > 0 && g.parties[0] == p; i++ {
		if i > 60*tps {
			t.Fatal("the fled Party never left")
		}
		g.step(0, 0)
	}
	if g.wallet[Money] != 0 || p.Paid {
		t.Fatalf("a fled Party paid: money %v", g.wallet[Money])
	}
}

// T101: the haunter hexes by keys alone through the Pilot, and the sim's invariants hold: Spook only
// from a hex or a Shadow, no fled Party pays.
func TestHaunterSim(t *testing.T) {
	r := RunSim(Config{Cols: 2, Rows: 1, Days: 2, Seed: 2, Strategy: Strategies["haunter"], React: 30, KeyGap: 6})
	for _, f := range r.Failures {
		t.Errorf("%s: %s", f.Clock, f.Reason)
	}
	if r.Haunts == 0 || r.Spook == 0 || r.Fled == 0 {
		t.Fatalf("haunter: %d hexes, %d Spook, %d fled; want all > 0", r.Haunts, r.Spook, r.Fled)
	}
}

// T192: the hexer taps beside the walls of occupied Rooms by keys alone, and stops short of fearLimit.
func TestHexerSim(t *testing.T) {
	r := RunSim(Config{Cols: 2, Rows: 1, Days: 2, Seed: 2, Strategy: Strategies["hexer"], React: 30, KeyGap: 6})
	for _, f := range r.Failures {
		t.Errorf("%s: %s", f.Clock, f.Reason)
	}
	if r.Haunts == 0 || r.Spook == 0 || r.Fled != 0 {
		t.Fatalf("hexer: %d hexes, %d Spook, %d fled; want hexes and Spook, none fled", r.Haunts, r.Spook, r.Fled)
	}
}
