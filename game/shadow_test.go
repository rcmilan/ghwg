package game

import (
	"image"
	"slices"
	"testing"

	"ghosthotel/entities"

	"github.com/hajimehoshi/ebiten/v2"
)

// T160 (D169): a Shadow costs shadowPrice Spook at the Terminal's Room menu, one per Room; a Strong Priest
// banishes it, a Weak one leaves it.
func TestBuyShadow(t *testing.T) {
	g, s := headless(t, 2, 1, 1)
	g.startRun()
	g.spook = 2*shadowPrice - 1
	tap := func(k ebiten.Key) {
		s.hold()
		g.Update()
		s.hold(k)
		g.Update()
	}
	g.openTerminal()
	for _, k := range []ebiten.Key{ebiten.KeySpace, ebiten.KeyW, ebiten.KeySpace} { // Room 0's menu, up from Father K wraps to the Shadow
		tap(k)
	}
	if !g.shaded(0) || g.spook != shadowPrice-1 {
		t.Fatalf("bought at the Terminal: shaded %v, Spook %d; want a Shadow in Room 0 and %d Spook", g.shaded(0), g.spook, shadowPrice-1)
	}
	if msg := g.buyShadow(0); msg == "" || g.spook != shadowPrice-1 {
		t.Fatalf("a second Shadow for Room 0: %q, Spook %d", msg, g.spook)
	}
	if msg := g.buyShadow(1); msg == "" || g.shaded(1) || g.spook != shadowPrice-1 {
		t.Fatalf("a Shadow without the Spook: %q, Spook %d", msg, g.spook)
	}
	g.spook = shadowPrice
	g.buyShadow(1)
	g.money = priestWeak + priestStrong
	g.callPriest(0, false)
	g.callPriest(1, true)
	for i := 0; len(g.priests) > 0; i++ {
		if i > 60*tps+priestMinutes*minuteFrames {
			t.Fatal("the Priests never left")
		}
		g.step(0, 0)
	}
	if !g.shaded(0) || g.shaded(1) {
		t.Fatalf("after Father K in Room 0 and Father M in Room 1: shaded %v %v, want true false", g.shaded(0), g.shaded(1))
	}
}

// T161 (D169): a Shadow runs every 240 game minutes at 0 Room Spook, every 60 at the limit, from outside
// one side wall to outside the other, and only with a Guest inside.
func TestShadowRuns(t *testing.T) {
	g, _, p := settled(t)
	room := p.Room
	for spook, want := range map[int]int{0: 240, spookLimit: 60, 2 * spookLimit: 60} {
		if g.roomSpook[room] = spook; g.shadowGap(room) != want*minuteFrames {
			t.Errorf("Room Spook %d: a run every %d frames, want %d game minutes", spook, g.shadowGap(room), want)
		}
	}
	g.roomSpook[room] = 0
	g.shades[room] = &shade{name: "Adam", next: g.clock, anim: entities.NewAnim()}
	sh := g.shades[room]
	x0, x1, _ := wallRun(g.hotel.Rooms[room])
	lo, hi := x0-charW/2, x1+charW/2
	for range 2 {
		for sh.dir == 0 {
			g.step(0, 0)
		}
		if sh.x > lo+shadowSpeed && sh.x < hi-shadowSpeed {
			t.Fatalf("a run started at x %.0f, want from outside a side wall (%.0f or %.0f)", sh.x, lo, hi)
		}
		from := sh.x
		for sh.dir != 0 {
			g.step(0, 0)
		}
		if to := sh.x; min(from, to) > lo+shadowSpeed || max(from, to) < hi-shadowSpeed { // from is a step in
			t.Fatalf("a run went from x %.0f to %.0f, want across %.0f..%.0f", from, to, lo, hi)
		}
	}
	for _, guest := range p.Guests { // out of the Room: the Shadow skips its turn
		guest.X, guest.Y = float64(g.hotel.Spawn.X), float64(g.hotel.Spawn.Y)
	}
	sh.next = g.clock
	g.updateShadows()
	if sh.dir != 0 || sh.next != g.clock+g.shadowGap(room) {
		t.Fatalf("with nobody inside: running %v, next %d; want no run, next at %d", sh.dir != 0, sh.next, g.clock+g.shadowGap(room))
	}
}

// T162 (D169): a run frightens a Guest facing it by shadowSeen, one facing away or asleep in bed by
// shadowUnseen; a seer hops and speaks. 1 Fear every shadowEvery minutes; never fearLimit.
func TestShadowFear(t *testing.T) {
	g, _, p := settled(t)
	room, r := p.Room, g.hotel.Rooms[p.Room]
	run := func(clock int, face entities.Facing, at func(i int) image.Point, fear int) int {
		t.Helper()
		g.clock = clock
		for i, guest := range p.Guests {
			pt := at(i)
			guest.X, guest.Y, guest.Path, guest.Fear, guest.Anim.Facing = float64(pt.X), float64(pt.Y), nil, fear, face
		}
		p.talk = nil
		spook, roomSpook := g.spook, g.roomSpook[room]
		g.shades[room] = &shade{name: "Adam", next: g.clock, anim: entities.NewAnim()}
		for g.updateShadows(); g.shades[room].dir != 0; g.updateShadows() {
		}
		gained := p.Guests[0].Fear - fear
		for _, guest := range p.Guests {
			if guest.Fear-fear != gained {
				t.Fatalf("mates facing alike gained %d and %d Fear", gained, guest.Fear-fear)
			}
		}
		if g.spook-spook != gained*len(p.Guests) || g.roomSpook[room]-roomSpook != gained*len(p.Guests) {
			t.Fatalf("Spook +%d, Room Spook +%d; want the Fear given, %d", g.spook-spook, g.roomSpook[room]-roomSpook, gained*len(p.Guests))
		}
		return gained
	}
	stay := func(i int) image.Point { return r.Stays[i] }
	noon := 12 * hourFrames
	if got := run(noon, entities.FaceDown, stay, 0); got != shadowUnseen || len(p.talk) > 0 {
		t.Fatalf("facing away: +%d Fear, lines %v; want +%d and no line", got, p.talk, shadowUnseen)
	}
	if got := run(noon, entities.FaceUp, stay, 0); got != shadowSeen || len(p.talk) != len(p.Guests) {
		t.Fatalf("facing it: +%d Fear, lines %v; want +%d and a line each", got, p.talk, shadowSeen)
	}
	for i, l := range p.talk {
		if l.who != i || !slices.Contains(sightLines, l.text) {
			t.Fatalf("line %d: %+v, want a sighting line by Guest %d", i, l, i)
		}
	}
	g.tick += hopFrames / 2
	if h := g.lift(p.Guests[0]); h < startleHeight-1 || h > startleHeight {
		t.Fatalf("a seer's hop at its top: %.1f world px, want %d", h, startleHeight)
	}
	g.tick += hopFrames
	if h := g.lift(p.Guests[0]); h != 0 {
		t.Fatalf("a seer's hop is over, still %.1f px up", h)
	}
	if got := run(2*hourFrames, entities.FaceUp, func(i int) image.Point { return sleepSpot(r.Lanes[i]) }, 0); got != shadowUnseen {
		t.Fatalf("asleep in bed facing it: +%d Fear, want +%d", got, shadowUnseen)
	}
	if got := run(noon, entities.FaceUp, stay, fearLimit-5); got != 4 {
		t.Fatalf("at Fear %d: +%d, want 4 (up to fearLimit-1)", fearLimit-5, got)
	}
	g.clock = shadowEvery * minuteFrames * 40 // a steady-Fear minute
	g.scare()
	g.clock += minuteFrames
	g.scare()
	for _, guest := range p.Guests {
		if guest.Fear != fearLimit-1 || p.Fled {
			t.Fatalf("at fearLimit-1: Fear %d, fled %v; want it stays, unfled", guest.Fear, p.Fled)
		}
		guest.Fear = 0
	}
	g.clock = shadowEvery * minuteFrames * 40
	g.scare()
	g.clock += minuteFrames
	g.scare()
	for _, guest := range p.Guests {
		if guest.Fear != 1 {
			t.Fatalf("steady Fear over 2 minutes, one a %d-minute mark: %d, want 1", shadowEvery, guest.Fear)
		}
	}
}
