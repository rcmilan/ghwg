package game

import (
	"image"
	"testing"

	"ghosthotel/entities"
)

// ghostIn books a ghost Party into the Room and puts it, staying, on its first Stay Spot.
func ghostIn(t *testing.T, g *Game, room, phantasm int) *Party {
	t.Helper()
	p := g.arriveGhost(room)
	if p == nil {
		t.Fatal("no ghost Party")
	}
	p.Guests = p.Guests[:1] // solo: a mate would stand elsewhere
	g.settleIn(p)
	p.Guests[0].Phantasm = phantasm
	return p
}

// T258: once the Mirror is unlocked each Room also books a ghost Party, apart from its human one; without
// it no ghost books. A ghost arrives from the Street, on the ghost layer, as one of the 4 characters, and
// pays no Money.
func TestGhostBookings(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	g.locked = map[Unlock]bool{Mirror: true}
	g.fast = true
	for g.clock < runStart+8*hourFrames {
		g.step(0, 0)
	}
	for _, p := range g.parties {
		if p.Realm == Ghost {
			t.Fatal("a ghost booked with the Mirror locked")
		}
	}
	g.unlock(Mirror)
	var ghost *Party
	for i := 0; i < 20*hourFrames && ghost == nil; i++ {
		g.step(0, 0)
		for _, p := range g.parties {
			if p.Realm == Ghost && ghost == nil {
				ghost = p
			}
		}
	}
	if ghost == nil {
		t.Fatal("no ghost Party booked with the Mirror unlocked")
	}
	if ghost.state != checkingIn || !ghost.Occupies(ghost.Room, Ghost) || g.vacant(ghost.Room, Ghost) {
		t.Errorf("a new ghost Party: state %d, its Room Occupied in the Ghost realm %v; want it walking in", ghost.state, !g.vacant(ghost.Room, Ghost))
	}
	if !g.hotelHas(ghost.Guests[0].Character) {
		t.Errorf("a ghost is %q, want one of the 4 characters %v", ghost.Guests[0].Character, shadowCast)
	}
	// independent of the human one: a Room holds both at once
	both := false
	for i := 0; i < 30*hourFrames && !both; i++ {
		g.step(0, 0)
		for r := range g.hotel.Rooms {
			both = both || !g.vacant(r, Human) && !g.vacant(r, Ghost)
		}
	}
	if !both {
		t.Error("no Room held a human and a ghost Party at once")
	}
	for i := 0; i < 6*dayFrames && !ghost.Paid; i++ {
		g.step(0, 0)
	}
	if !ghost.Paid || ghost.rent != 0 || ghost.tip != 0 {
		t.Errorf("a ghost Party checked out: paid %v, rent %v, tip %v; want it gone, paying nothing", ghost.Paid, ghost.rent, ghost.tip)
	}
}

func (g *Game) hotelHas(name string) bool {
	for _, c := range shadowCast {
		if c == name {
			return true
		}
	}
	return false
}

// T259: inRoom is a staying Party with everyone inside its Room, in either realm.
func TestInRoom(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	for _, realm := range [...]Realm{Human, Ghost} {
		var p *Party
		if realm == Human {
			p = g.walkIn()
		} else {
			p = g.arriveGhost(1)
		}
		if g.inRoom(p) {
			t.Errorf("%v: a Party walking in is in its Room", realm)
		}
		g.settleIn(p)
		if !g.inRoom(p) {
			t.Errorf("%v: a staying Party on its Stay Spots is not in its Room", realm)
		}
		p.Guests[0].X, p.Guests[0].Y = float64(g.hotel.Spawn.X), float64(g.hotel.Spawn.Y)
		if g.inRoom(p) {
			t.Errorf("%v: a Party with a Guest in the Street is in its Room", realm)
		}
		g.settleIn(p)
		for _, s := range [...]partyState{out, checkingOut, goingOut} {
			if p.state = s; g.inRoom(p) {
				t.Errorf("%v: a Party in state %d is in its Room", realm, s)
			}
		}
	}
}

// T260: a strong ghost in an area switches the area's Shadow People on, wherever it stands in it; a weak one
// doesn't. They appear while it's in and fade once it's out.
func TestAreaShadow(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	room := g.hotel.Rooms[0]
	p := ghostIn(t, g, 0, phantasmStrong)
	ghost := p.Guests[0]
	on := func(x, y float64) bool { return g.areaOn(g.areaRun(x, y)) }
	g.updateAreaShades()
	if !on(ghost.X, ghost.Y) {
		t.Fatal("a strong ghost in its Room: no Shadows on its wall")
	}
	for _, s := range room.Stays { // anywhere in the Room
		ghost.X, ghost.Y = float64(s.X), float64(s.Y)
		g.updateAreaShades()
		if !on(float64(room.Stays[0].X), float64(room.Stays[0].Y)) {
			t.Errorf("a strong ghost at %v: the Room's Shadows are off", s)
		}
	}
	hall := g.hotel.Lights[0] // out in a Corridor: that area lights, the Room's goes dark
	ghost.X, ghost.Y = float64(hall.X), float64(hall.Y)
	g.updateAreaShades()
	if !on(ghost.X, ghost.Y) {
		t.Error("a strong ghost in a Corridor: no Shadows there")
	}
	if on(float64(room.Stays[0].X), float64(room.Stays[0].Y)) {
		t.Error("the Room still has Shadows with the ghost in the Corridor")
	}
	ghost.Phantasm = phantasmStrong - 1 // weak
	g.updateAreaShades()
	if on(ghost.X, ghost.Y) {
		t.Error("a weak ghost switched Shadows on")
	}
	ghost.Phantasm = phantasmStrong
	g.updateAreaShades()
	if !on(ghost.X, ghost.Y) {
		t.Fatal("strong again: no Shadows")
	}
	p.state = out // gone from the map
	for range areaFade + 1 {
		g.updateAreaShades()
	}
	if len(g.areaShades) != 0 || on(ghost.X, ghost.Y) {
		t.Errorf("the Shadows stayed after the ghost left: %d left", len(g.areaShades))
	}
	if kindOf(phantasmStrong-1) != 0 || kindOf(phantasmStrong) == 0 || kindOf(phantasmMax) != len(ghostKinds)-1 {
		t.Errorf("kinds: Phantasm %d is %d, %d is %d, %d is %d", phantasmStrong-1, kindOf(phantasmStrong-1), phantasmStrong, kindOf(phantasmStrong), phantasmMax, kindOf(phantasmMax))
	}
}

// T261: a Room shuts to the Concierge while Guests of his own realm are inside: a ghost doesn't shut it in
// real, a human doesn't in ghost.
func TestDoorPerRealm(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	ghostIn(t, g, 0, 1)
	if g.ghost = false; g.occupied(0) {
		t.Error("a ghost inside shuts the Room to the Concierge in real")
	}
	if g.ghost = true; !g.occupied(0) {
		t.Error("a ghost inside doesn't shut the Room to the ghost Concierge")
	}
	h := g.walkIn()
	g.settleIn(h)
	if g.ghost = false; !g.occupied(h.Room) {
		t.Error("a human inside doesn't shut the Room in real")
	}
	g.setGhostRoom(h.Room)
	if g.ghost = true; g.occupied(h.Room) {
		t.Error("a human inside shuts the Room to the ghost Concierge")
	}
}

// setGhostRoom: no ghost in the Room (a test helper).
func (g *Game) setGhostRoom(room int) {
	for _, p := range g.parties {
		if p.Realm == Ghost && p.Room == room {
			p.state = out
		}
	}
}

// T260/M1: an in-Room ghost's haunt lowers the Room's Mood (only with the Mirror), and a ghost doesn't care
// about dirt.
func TestGhostHaunts(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	p := ghostIn(t, g, 0, 3)
	p.Guests[0].Haunt = 1
	g.clock = runStart + 5*hourFrames // a whole minute
	g.locked = map[Unlock]bool{Mirror: true}
	g.hauntGhosts()
	if g.roomMood(0) != moodMax {
		t.Errorf("a haunt with the Mirror locked: Mood %d", g.roomMood(0))
	}
	g.unlock(Mirror)
	g.hauntGhosts()
	if want := moodMax - ghostHauntDamage(p.Guests[0]); g.roomMood(0) != want {
		t.Errorf("a haunt: Mood %d, want %d", g.roomMood(0), want)
	}
	g.setDirtLevel(0, dirtMax-1) // filthy, not closed
	for range 3 * hourFrames {
		g.clock++
		g.updateGhost(p)
	}
	if p.state != staying || p.Walked || p.Fled {
		t.Errorf("a ghost in a dirty Room: state %d walked %v fled %v; want it staying", p.state, p.Walked, p.Fled)
	}
}

// T261: in ghost a ghost Party is drawn (hued) and in real it isn't; humans draw as silhouettes in ghost.
func TestGhostDrawn(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	p := ghostIn(t, g, 0, 1)
	in := func(guests []*entities.Guest, who *entities.Guest) bool {
		for _, o := range guests {
			if o == who {
				return true
			}
		}
		return false
	}
	g.ghost = false
	if in(g.visible(), p.Guests[0]) {
		t.Error("a ghost is drawn in real")
	}
	g.ghost = true
	if !in(g.visible(), p.Guests[0]) {
		t.Error("a ghost isn't drawn in ghost")
	}
	if kindHue(0) == kindHue(len(ghostKinds)-1) || len(ghostKinds) != 4 {
		t.Error("the four kinds need four hues")
	}
	_ = image.Point{}
}
