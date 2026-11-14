package game

import (
	"image"
	"math"
	"slices"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
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
	if len(g.areaShades) != 0 || len(g.shades[0]) != 0 || on(ghost.X, ghost.Y) {
		t.Errorf("the Shadows stayed after the ghost left: %d left", len(g.areaShades)+len(g.shades[0]))
	}
	if kindOf(phantasmStrong-1) != 0 || kindOf(phantasmStrong) == 0 || kindOf(phantasmMax) != len(ghostKinds)-1 {
		t.Errorf("kinds: Phantasm %d is %d, %d is %d, %d is %d", phantasmStrong-1, kindOf(phantasmStrong-1), phantasmStrong, kindOf(phantasmStrong), phantasmMax, kindOf(phantasmMax))
	}
}

// T261 kept the Concierge out of a Room holding Guests of his own realm; K1 dropped the door rule (T298):
// only a boarded Room is shut, whoever is in it, in either realm.
func TestDoorPerRealm(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	ghostIn(t, g, 0, 1)
	g.settleIn(g.walkIn())
	for _, ghost := range []bool{false, true} {
		if g.ghost = ghost; g.boarded(0) {
			t.Errorf("ghost %v: a Room with Guests in is shut to the Concierge", ghost)
		}
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
	if g.roomMood(0) != moodStart {
		t.Errorf("a haunt with the Mirror locked: Mood %d", g.roomMood(0))
	}
	g.unlock(Mirror)
	g.hauntGhosts()
	if want := moodStart - ghostHauntDamage(p.Guests[0]); g.roomMood(0) != want {
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

// A normal Run starts with the Mirror locked: humans only, no haunts. Bought at the Terminal ($10) and
// delivered by the courier at the stock, it opens the ghost layer: then ghosts book and haunt.
func TestMirrorStartLocked(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	if g.unlocked(Mirror) || !g.unlocked(SupplyWater) || g.unlocked(BedsTwin) || g.unlocked(BedsDouble) || g.unlocked(RoomMirror) || len(g.locked) != 4 {
		t.Fatalf("a new Run's locks %v, want the Mirror and K1's beds and in-Room Mirror", g.locked)
	}
	mirror := row(t, Mirror)
	if !g.onSale(mirror) || g.owned(mirror) || !slices.Contains(g.forSale(), mirror) {
		t.Fatal("the Mirror isn't on the Terminal's shelf")
	}
	g.fast = true
	for g.clock < runStart+2*dayFrames {
		g.step(0, 0)
	}
	for _, p := range g.parties {
		if p.Realm == Ghost {
			t.Fatal("a ghost booked before the Mirror was bought")
		}
	}
	g.wallet[Money] = mirrorPrice
	if why := g.addToCart(mirror, -1); why != "" {
		t.Fatal(why)
	}
	if why := g.placeOrder(); why != "" {
		t.Fatal(why)
	}
	if g.unlocked(Mirror) || len(g.couriers) != 1 {
		t.Fatalf("bought: unlocked %v, %d couriers; want the courier on his way, the Mirror not yet", g.unlocked(Mirror), len(g.couriers))
	}
	for f := 0; !g.unlocked(Mirror); f++ {
		if f > 3*60*tps {
			t.Fatal("the courier never delivered the Mirror")
		}
		g.step(0, 0)
	}
	for i := 0; i < 2*dayFrames; i++ {
		g.step(0, 0)
		for _, p := range g.parties {
			if p.Realm == Ghost {
				return
			}
		}
	}
	t.Fatal("no ghost booked after the Mirror arrived")
}

// While the Mirror is locked it is drawn covered and SPACE at it shifts nothing; bought, the courier
// carries it to the Mirror's own spot and it unlocks on his drop there, not at the stock.
func TestMirrorCoveredAndDelivered(t *testing.T) {
	g, s := headless(t, 2, 1, 1)
	g.startRun()
	mc := g.hotel.Mirror.Div(world.TileSize)
	lobby := func(dy int, tile string) prop {
		id, _ := g.ts.ID(tile)
		return prop{cx: mc.X, cy: mc.Y - dy, l: world.LayerDecor, id: id}
	}
	room := g.hotel.Rooms[0].Origin
	inRoom, _ := g.ts.ID("prop_mirror_r0c0")
	if !g.covered(lobby(2, "prop_mirror_r0c0")) || !g.covered(lobby(1, "prop_mirror_r1c0")) ||
		g.covered(prop{cx: room.X + 2, cy: room.Y + 3, l: world.LayerProps, id: inRoom}) {
		t.Fatal("a locked lobby Mirror isn't drawn covered, or a Room's mirror prop is")
	}
	g.sortedProps()
	n := 0
	for _, p := range g.sortedProps() {
		if g.covered(p) {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("%d of the map's props draw covered, want the lobby Mirror's 2 tiles", n)
	}
	g.player.X, g.player.Y = float64(g.hotel.Mirror.X), float64(g.hotel.Mirror.Y)
	s.hold(ebiten.KeySpace)
	g.Update()
	if g.shifting != nil || g.message == "" {
		t.Fatalf("SPACE at the covered Mirror: shifting %v, message %q; want a refusal", g.shifting != nil, g.message)
	}
	s.hold()
	g.wallet[Money] = mirrorPrice
	if why := g.addToCart(row(t, Mirror), -1); why != "" {
		t.Fatal(why)
	}
	if why := g.placeOrder(); why != "" {
		t.Fatal(why)
	}
	c := g.couriers[0]
	if len(c.drops) != 1 || c.drops[0].at != g.hotel.Mirror {
		t.Fatalf("the Mirror's drop: %+v, want the Mirror's spot %v", c.drops, g.hotel.Mirror)
	}
	g.player.X, g.player.Y = float64(g.hotel.ConciergeStart.X), float64(g.hotel.ConciergeStart.Y)
	for f := 0; g.unlocked(Mirror) == false; f++ {
		if f > 3*60*tps {
			t.Fatal("the Mirror never arrived")
		}
		g.step(0, 0)
	}
	if math.Hypot(c.X-float64(g.hotel.Mirror.X), c.Y-float64(g.hotel.Mirror.Y)) > world.TileSize || g.covered(lobby(2, "prop_mirror_r0c0")) {
		t.Errorf("unlocked with the courier at (%v,%v); want him at the Mirror and it uncovered", c.X, c.Y)
	}
	g.player.X, g.player.Y = float64(g.hotel.Mirror.X), float64(g.hotel.Mirror.Y)
	s.hold(ebiten.KeySpace)
	g.Update()
	if g.shifting == nil {
		t.Error("SPACE at the installed Mirror doesn't shift")
	}
}

// Ghost guests are flat arcade colors, one per kind.
func TestGhostPalette(t *testing.T) {
	want := [4][3]float32{{0, 1, 1}, {1, 184. / 255, 1}, {1, 184. / 255, 82. / 255}, {1, 0, 0}}
	for k, w := range want {
		if kindHue(k) != w {
			t.Errorf("kind %d is %v, want %v", k, kindHue(k), w)
		}
	}
}

// A strong ghost's Shadow is the old one: it dashes along the Room's back wall while a human is in, and a
// human who sees it (or is in the Room for its pass) is frightened. The ghost's own realm isn't.
func TestAreaShadowMovesAndScares(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	g.locked = nil
	h := g.walkIn()
	h.Routine, h.Nights = homebody, 3
	h.schedule(g.clock)
	g.settleIn(h)
	p := ghostIn(t, g, h.Room, phantasmStrong)
	p.Nights = 3
	p.schedule(g.clock)
	ghost, human := p.Guests[0], h.Guests[0]
	lo, hi, fear := math.Inf(1), math.Inf(-1), 0
	g.fast = true
	for g.clock < runStart+2*dayFrames {
		g.step(0, 0)
		if list := g.shades[h.Room]; len(list) > 0 {
			lo, hi = min(lo, list[0].x), max(hi, list[0].x)
		}
		fear = max(fear, human.Fear)
		if human.Fear >= human.FleeAt {
			break
		}
	}
	if hi-lo < 2*world.TileSize {
		t.Errorf("the area Shadow only moved %.0f px (%.0f..%.0f); want it dashing along the wall", hi-lo, lo, hi)
	}
	if fear == 0 {
		t.Error("a human in the Room with the Shadow was never frightened")
	}
	if ghost.Fear != 0 {
		t.Errorf("the ghost, in its own realm, got Fear %d", ghost.Fear)
	}
}

// A Room's props in a fogged Room are under its fog, whatever they are, and the covered Mirror is the
// lobby's alone.
func TestRoomPropsFogged(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	o := g.hotel.Rooms[0].Origin
	g.fog = make([]float64, len(g.hotel.Rooms))
	g.fog[0] = 1
	id, _ := g.ts.ID("prop_mirror_r0c0")
	room := prop{cx: o.X + 2, cy: o.Y + 4, l: world.LayerProps, id: id}
	if g.propFog(room) == 0 || g.covered(room) {
		t.Errorf("a mirror prop in a fogged Room: fog %v covered %v; want it fogged and drawn as itself", g.propFog(room), g.covered(room))
	}
}

// T307 (Z1): a strong ghost's Shadow dashes as often as its ghost haunts, and in real shows only while it dashes.
func TestShadowDashOnly(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	g.locked = nil
	h := g.walkIn()
	h.Routine, h.Nights = homebody, 3
	h.schedule(g.clock)
	g.settleIn(h)
	p := ghostIn(t, g, h.Room, phantasmStrong)
	p.Guests[0].Haunt = 30
	g.updateShadows()
	sh := g.shades[h.Room][0]
	if got := g.dashGap(h.Room, sh); got != 30*minuteFrames {
		t.Errorf("the dash gap is %d frames, want its ghost's 30 game minutes", got)
	}
	p.Guests[0].Haunt = 10
	if got := g.dashGap(h.Room, sh); got != 10*minuteFrames {
		t.Errorf("a more relentless ghost: gap %d frames, want 10 game minutes", got)
	}
	sh.next = g.clock
	dashes, was := 0, false
	for range 35 * minuteFrames {
		g.clock++
		g.updateShadows()
		if sh.lvl = 1; !sh.patrol && g.shadowAlpha(sh) != 0 {
			t.Fatal("the Shadow shows in real while it isn't dashing")
		}
		if sh.patrol && !was {
			dashes++
		}
		was = sh.patrol
	}
	if dashes < 3 {
		t.Errorf("%d dashes in 35 game minutes at a 10-minute haunt, want 3", dashes)
	}
}

// Z1 live test: a Guest's sighting line comes from its ghost's kind, the stronger the worse; and in real the
// Concierge hears a Guest in the Room he stands in (he may walk in since K1), not one behind a Room's walls.
func TestSightLinesByStrength(t *testing.T) {
	g, _, h := settled(t)
	g.locked = nil
	p := ghostIn(t, g, h.Room, phantasmMax)
	human := h.Guests[0]
	run, xa, _ := g.backWall(h.Room)
	if got := g.areaPhantasm(run); got != phantasmMax {
		t.Fatalf("the Room's strongest ghost: Phantasm %d, want %d", got, phantasmMax)
	}
	for _, ph := range []int{phantasmStrong, phantasmMax} {
		p.Guests[0].Phantasm = ph
		human.Path, human.Anim.Facing, h.talk = nil, entities.FaceUp, nil
		sh := &shade{run: run, x: xa, seen: map[*entities.Guest]int{}}
		g.sighting(h, h.Guests, sh)
		if len(h.talk) != 1 || !slices.Contains(sightLines[kindOf(ph)], h.talk[0].text) {
			t.Errorf("Phantasm %d: lines %v, want one of its kind's %v", ph, h.talk, sightLines[kindOf(ph)])
		}
	}
	g.ghost = false
	g.player.X, g.player.Y = human.X+world.TileSize, human.Y
	if !g.hears(human) {
		t.Error("in real, the Concierge in the Room doesn't hear its Guest")
	}
	g.player.X, g.player.Y = float64(g.hotel.Spawn.X), float64(g.hotel.Spawn.Y)
	if g.hears(human) {
		t.Error("in real, a Guest in its Room is heard from outside it")
	}
}
