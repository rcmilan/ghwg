package game

import (
	"image"
	"math"
	"slices"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// T160 (D169), T211 (D185), T250: a Shadow is born on the wall nearest its box, by the Room's door, up to
// shadowCap per Room; a Strong Priest banishes them all, a Weak one leaves them. The store sells none.
func TestPlaceShadow(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.placeShadow(0)
	if !g.shaded(0) || g.shadowCap(0) != 1 {
		t.Fatalf("placed: shaded %v, cap %d; want a Shadow in Room 0 and a cap of 1 (placeholder)", g.shaded(0), g.shadowCap(0))
	}
	at := g.doorDrop(0)
	if sh := g.shades[0][0]; sh.run != g.runAt(float64(at.X), float64(at.Y)) || g.walls[sh.run].y > float64(at.Y) {
		t.Fatalf("born on run %d, want the one nearest the box at the door (%d)", sh.run, g.runAt(float64(at.X), float64(at.Y)))
	}
	g.placeShadow(1)
	g.shades[1] = append(g.shades[1], &shade{run: g.shades[1][0].run, essence: shadowEssence}) // past the cap, as R1 will allow
	g.wallet[Money] = priestWeak + priestStrong
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
	for _, i := range tabItems(tabServices) {
		if shop[i].Name == "Shadow Person" {
			t.Fatal("the SERVICES tab has a Shadow row")
		}
	}
}

// home puts a fresh Shadow at the west end of the Room's back wall, due now.
func home(g *Game, room int) *shade {
	run, xa, _ := g.backWall(room)
	sh := &shade{name: "Adam", next: g.clock, essence: shadowEssence, minute: g.clock / minuteFrames, run: run, x: xa, goal: xa, anim: entities.NewAnim()}
	g.shades[room] = shades{sh}
	return sh
}

// T161 (D169), T209 (D185): a Shadow runs home to its Room's back wall, to an end hidden behind a
// side wall. It patrols every 60 game minutes in a calm Room, every 20 in a Closed one, from that end to the
// other, and only with a Guest inside.
func TestShadowRuns(t *testing.T) {
	g, _, p := settled(t)
	room := p.Room
	for mood, want := range map[int]int{moodMax: 60, moodMin: 20} {
		if g.mood[room] = mood; g.shadowGap(room) != want*minuteFrames {
			t.Errorf("Room Mood %d: a run every %d frames, want %d game minutes", mood, g.shadowGap(room), want)
		}
	}
	g.mood[room] = moodMax
	g.placeShadow(room)
	sh := g.shades[room][0]
	run, xa, xb := g.backWall(room)
	const line = 6 // a side line's hitbox, px
	if o := g.hotel.Rooms[room].Origin; xa != float64(o.X*world.TileSize+line+charW/2) || xb != float64((o.X+world.RoomW)*world.TileSize-line-charW/2) {
		t.Fatalf("back wall ends %v, %v; want touching the side lines, not under them", xa, xb)
	}
	for i := 0; sh.run != run || sh.x != xa && sh.x != xb || sh.fade > 0; i++ {
		if i > 3000 || sh.patrol {
			t.Fatalf("never got home: run %d x %v, patrolling %v", sh.run, sh.x, sh.patrol)
		}
		g.step(0, 0)
	}
	sh.next = g.clock
	for range 2 {
		for !sh.patrol {
			g.step(0, 0)
		}
		from := sh.x
		for sh.patrol {
			g.step(0, 0)
		}
		if sh.run != run || min(from, sh.x) != xa || max(from, sh.x) != xb {
			t.Fatalf("a patrol went from x %.0f to %.0f on run %d, want across %.0f..%.0f on %d", from, sh.x, sh.run, xa, xb, run)
		}
	}
	for _, guest := range p.Guests { // out of the Room: the Shadow skips its turn
		guest.X, guest.Y = float64(g.hotel.Spawn.X), float64(g.hotel.Spawn.Y)
	}
	sh.next = g.clock
	g.updateShadows()
	g.updateShadows()
	if sh.patrol || sh.next != g.clock+g.shadowGap(room) {
		t.Fatalf("with nobody inside: patrolling %v, next %d; want no patrol, next at %d", sh.patrol, sh.next, g.clock+g.shadowGap(room))
	}
}

// T162 (D169): a patrol frightens a Guest facing it by shadowSeen, one facing away or asleep in bed by
// shadowUnseen; a seer hops and speaks; never fearLimit. Between patrols it scares nobody (2026-11-08: no
// hidden steady Fear).
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
		ecto, mood := g.wallet[Ecto], g.roomMood(room)
		sh := home(g, room)
		for g.updateShadows(); sh.patrol; g.updateShadows() {
		}
		gained := p.Guests[0].Fear - fear
		for _, guest := range p.Guests {
			if guest.Fear-fear != gained {
				t.Fatalf("mates facing alike gained %d and %d Fear", gained, guest.Fear-fear)
			}
		}
		if g.wallet[Ecto]-ecto != Num(gained*len(p.Guests)) || mood-g.roomMood(room) != len(p.Guests)*dirtMood(gained) {
			t.Fatalf("Ecto +%v, Room Mood -%d; want the Fear given, %d, and its weight in Mood", g.wallet[Ecto]-ecto, mood-g.roomMood(room), gained*len(p.Guests))
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
	for _, guest := range p.Guests {
		guest.Fear = 0
	}
	for range 2 * hourFrames / minuteFrames {
		g.clock += minuteFrames
		g.scare()
	}
	for _, guest := range p.Guests {
		if guest.Fear != 0 || p.Fled {
			t.Fatalf("2 game hours with a resting Shadow: Fear %d, fled %v; want 0", guest.Fear, p.Fled)
		}
	}
}

// T210 (D185): a Shadow spends 1 essence a patrol pass, and daylight drains more each game minute; it's
// drawn as opaque as its essence, and gone at 0.
func TestShadowEssence(t *testing.T) {
	g, _, p := settled(t)
	room := p.Room
	g.clock = 2 * hourFrames // night: no drain
	sh := home(g, room)
	for g.updateShadows(); sh.patrol; g.updateShadows() {
	}
	if sh.essence != shadowEssence-1 {
		t.Fatalf("after a pass at night: essence %v, want %v", sh.essence, shadowEssence-1)
	}
	drain := func(clock int) float64 {
		g.clock = clock
		sh := home(g, room)
		sh.next = math.MaxInt
		for range hourFrames {
			g.clock++
			g.updateShadows()
		}
		return shadowEssence - sh.essence
	}
	if day, night := drain(12*hourFrames), drain(0); math.Abs(day-60*shadowSunDrain) > 1e-9 || night != 0 {
		t.Fatalf("an idle game hour drained %v at noon and %v at midnight; want %v and 0", day, night, 60*shadowSunDrain)
	}
	if dawn := drain(6 * hourFrames); dawn <= 0 || dawn >= 60*shadowSunDrain {
		t.Fatalf("an hour from 06:00 drained %v, want between night's 0 and day's %v", dawn, 60*shadowSunDrain)
	}
	sh = home(g, room)
	if a := g.shadowAlpha(sh); a != 0 {
		t.Fatalf("alpha %v at its wall's end, want 0: it fades into the end", a)
	}
	_, xa, xb := g.backWall(room)
	sh.x = (xa + xb) / 2
	full := g.shadowAlpha(sh)
	sh.essence = 1
	if faint := g.shadowAlpha(sh); full != shadowOpacity || faint >= full/2 || faint < shadowOpacity/10 {
		t.Fatalf("alpha %v at full essence mid-wall, %v at 1; want shadowOpacity, and faint but showing", full, faint)
	}
	g.clock = 12 * hourFrames
	sh = home(g, room)
	sh.next, sh.essence = math.MaxInt, shadowSunDrain/2
	g.clock += minuteFrames
	g.updateShadows()
	if g.shaded(room) {
		t.Fatal("a Shadow drained to 0 is still there")
	}
}
