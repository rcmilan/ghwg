package game

import (
	"image"
	"slices"
	"strings"

	"ghosthotel/assets"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// Ghost guests (07 G1): once the Mirror is unlocked each Room also books a ghost Party, apart from its
// human one (the realms never bump nor queue, realm.go). A ghost pays no Money and ignores dirt; it earns
// only through encounters (V1). Its Phantasm sets its kind, the Mood a haunt costs (mood.go) and whether it
// is strong enough to switch an area's Shadow People on (shadow.go). All values are placeholders.
const (
	phantasmMax = 12 // a ghost's Phantasm is rolled 1..phantasmMax
	// The haunt values (ghostHaunt, hauntBase, phantasmMax) are deliberately harsh placeholders: Mood falls
	// fast and Money with it, until X2's moving Parties between Rooms balances them. Do not tune before.
	ghostHaunt = 240 // x: game minutes between a ghost's haunts (the Mood pressure on a Room)
	ghostAlpha = 0.9
)

// phantasmBands: the Phantasm each kind from the second starts at; the lowest kind is below the first edge.
var phantasmBands = [...]int{4, 7, 10}

// phantasmStrong: a ghost at or above this is strong: its Shadow People follow it into the Corridors and the
// hall. Every ghost, weak too, has its own Room's on from the moment it comes in (2026-11-14).
const phantasmStrong = 4

type ghostKind struct {
	Name string
	Hue  [3]float32 // one flat arcade color per kind, the Pac-Man ghosts' (which kind gets which is a placeholder)
}

var ghostKinds = [...]ghostKind{
	{"Wisp", [3]float32{0, 1, 1}},                    // #00ffff
	{"Specter", [3]float32{1, 184. / 255, 1}},        // #ffb8ff
	{"Wraith", [3]float32{1, 184. / 255, 82. / 255}}, // #ffb852
	{"Poltergeist", [3]float32{1, 0, 0}},             // #ff0000
}

// kindOf: the index in ghostKinds of the band a Phantasm falls in.
func kindOf(phantasm int) (k int) {
	for _, edge := range phantasmBands {
		if phantasm >= edge {
			k++
		}
	}
	return k
}

func kindHue(k int) [3]float32 { return ghostKinds[k].Hue }

func strong(ghost *entities.Guest) bool { return ghost.Phantasm >= phantasmStrong }

// arriveGhost books a ghost Party into the Room and puts it on the Street, bound for its Stay Spots. It is
// solo, or a pair in a Room with two Stay Spots, one of the 4 characters each. A Room that already holds a
// ghost takes the next through its mirror (08 B1): solo, from the glass.
func (g *Game) arriveGhost(room int) *Party {
	if room < 0 || room >= len(g.hotel.Rooms) {
		return nil
	}
	r := g.hotel.Rooms[room]
	guest := g.newGhost()
	through := !g.vacant(room, Ghost) && g.hasMirror(room)
	p := &Party{Guests: []*entities.Guest{guest},
		Booking: Booking{Room: room, Realm: Ghost, Nights: 1 + g.chat.IntN(maxNights), Rate: r.Type.Rate,
			checkoutAt: checkoutHour*hourFrames + g.chat.IntN(checkoutJitter)}}
	if len(r.Stays) >= 2 && !through && g.chat.Float64() < pairOdds {
		mate := g.newGhost()
		mate.Leader = guest
		p.Guests = append(p.Guests, mate)
	}
	p.arrived = g.clock
	p.schedule(g.clock)
	for _, guest := range p.Guests {
		guest.RoomIndex = room
	}
	if g.grown() { // as rollNamedHuman (party.go)
		g.rollNamedGhost(p)
	}
	if through { // it never meets the first on the way (they are mates to the bumping), nor the Street
		for _, q := range g.parties {
			if q.Occupies(room, Ghost) {
				guest.Leader = q.Guests[0]
				break
			}
		}
		guest.X, guest.Y = float64(r.Mirror.Min.X+r.Mirror.Max.X)/2, float64(r.Mirror.Min.Y+r.Mirror.Max.Y)/2
	}
	g.parties = append(g.parties, p)
	g.checkIn(p)
	if through {
		g.sendParty(p, g.stayOf(p, 0))
	}
	return p
}

// hasMirror: the Room has its own mirror, so a second ghost slot.
func (g *Game) hasMirror(room int) bool { return !g.hotel.Rooms[room].Mirror.Empty() }

// mirrorGhost: the Party is the one that came through a Room mirror: a solo ghost trailing the Room's first.
func mirrorGhost(p *Party) bool {
	return p.Realm == Ghost && len(p.Guests) == 1 && p.Guests[0].Leader != nil
}

// stayOf: where Guest i of the Party stands: its Stay Spot; the mirror's ghost takes the Room's last, or
// beside the first when there is only one.
func (g *Game) stayOf(p *Party, i int) image.Point {
	r := g.hotel.Rooms[p.Room]
	if !mirrorGhost(p) {
		return r.Stays[i]
	}
	if at := r.Stays[len(r.Stays)-1]; len(r.Stays) > 1 {
		return at
	} else {
		return at.Add(image.Pt(world.TileSize, 0))
	}
}

// ghostSlots: how many ghost Parties the Room takes at once: one, two with a mirror.
func (g *Game) ghostSlots(room int) int {
	if g.hasMirror(room) {
		return 2
	}
	return 1
}

// ghostsIn: the ghost Parties booked into the Room.
func (g *Game) ghostsIn(room int) (n int) {
	for _, p := range g.parties {
		if p.Occupies(room, Ghost) {
			n++
		}
	}
	return n
}

// mirrorSlot: the Room's second ghost arrival, for a Room with a mirror (kept as long as the Rooms are).
func (g *Game) mirrorSlot(room int) *arrival {
	if len(g.mirrorSlots) != len(g.hotel.Rooms) {
		g.mirrorSlots = make([]arrival, len(g.hotel.Rooms))
	}
	return &g.mirrorSlots[room]
}

func (g *Game) newGhost() *entities.Guest {
	guest := entities.NewGuest(g.hotel.Spawn, shadowCast[g.chat.IntN(len(shadowCast))])
	guest.Phantasm, guest.Haunt = 1+g.chat.IntN(phantasmMax), ghostHaunt
	g.rollGhost(&guest)
	return &guest
}

// bookGhost: the Room's next ghost Party arrives gap game minutes from now: in its first arrival, or in the
// second (a mirror's) when the first is taken, so each slot books on its own.
func (g *Game) bookGhost(room, gap int) {
	slot := &g.ghostArrivals[room]
	if slot.at > 0 && g.hasMirror(room) {
		slot = g.mirrorSlot(room)
	}
	*slot = arrival{g.clock, g.clock + gap*minuteFrames}
}

// maybeGhostIn is maybeWalkIn for the ghost layer: nothing books until the Mirror is unlocked. Ghosts ignore
// dirt, but a Closed Room (Mood at the floor) takes no Party of either realm. A Room with a mirror also books
// a second ghost, on the same schedule as any free slot, apart from the first (08 B1).
func (g *Game) maybeGhostIn() {
	if !g.unlocked(Mirror) {
		return
	}
	booked := 0
	for i := range g.hotel.Rooms {
		if !g.vacant(i, Ghost) {
			booked++
		}
	}
	for i := range g.ghostArrivals {
		second := g.mirrorSlot(i)
		if !g.hasMirror(i) {
			*second = arrival{}
		} else if n := g.ghostsIn(i); g.isOwned(i) && n+b2i(g.ghostArrivals[i].at > 0)+b2i(second.at > 0) == 1 {
			g.bookGhost(i, g.bookGapNow()) // its second slot is free and unbooked
		}
		for _, a := range []*arrival{&g.ghostArrivals[i], second} {
			if a.at > 0 && g.clock >= a.at && g.isOwned(i) && g.ghostsIn(i) < g.ghostSlots(i) && (booked < g.demand() || !g.vacant(i, Ghost)) &&
				!g.closed(i) && !g.changing(i) && !g.stays(i) && (g.spawnClearIn(Ghost) || !g.vacant(i, Ghost)) {
				if g.vacant(i, Ghost) {
					booked++
				}
				*a = arrival{}
				g.arriveGhost(i)
			}
		}
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// inRoom: the Party is inside its Room, either realm: staying, with every Guest inside the Room's shell.
func (g *Game) inRoom(p *Party) bool {
	return p.state == staying && g.allInside(p)
}

// allInside: every member is within the Room's shell.
func (g *Game) allInside(p *Party) bool {
	shell := g.hotel.Rooms[p.Room].Shell()
	for _, guest := range p.Guests {
		if !image.Pt(int(guest.X), int(guest.Y)).In(shell) {
			return false
		}
	}
	return true
}

// hauntGhosts runs each game minute, before the arrivals: every ghost in its Room haunts when its x is due,
// lowering the Room's Mood (mood.go). Only with the Mirror.
func (g *Game) hauntGhosts() {
	if !g.unlocked(Mirror) {
		return
	}
	for _, p := range g.parties {
		if p.Realm == Ghost && g.inRoom(p) {
			for _, guest := range p.Guests {
				g.hauntTick(p.Room, guest)
			}
		}
	}
}

// updateGhost runs one frame of a ghost Party's stay: Street -> Room -> (Checkout) -> Street. No rent, no
// tip, no mess, no requests, no complaints.
func (g *Game) updateGhost(p *Party) bool {
	exit := []image.Point{g.hotel.Exit, g.hotel.Exit}
	switch p.state {
	case checkingIn:
		if g.walkParty(p, arriveSpeed) {
			p.state, p.settled = staying, true
			if g.unlocked(Mirror) { // it haunts as it settles in (2026-11-14, 4c): what a ghost does shows at once, not up to 4 h later
				for _, guest := range p.Guests {
					g.haunt(p.Room, guest)
					guest.HauntAt = g.clock / minuteFrames
				}
			}
		}
	case staying:
		if lead := p.Guests[0]; lead.PassAt > 0 { // passing on: it rises and is gone, and the Room books the next
			if g.clock-lead.PassAt >= passFrames {
				p.Paid = true
				g.logStay(p, b2o(p.banished, outBanished, outPassed))
				g.bookGhost(p.Room, g.bookGapNow())
				return false
			}
			break
		}
		if g.unlocked(Mirror) && g.mayPass(p) && !slices.ContainsFunc(p.Guests, func(guest *entities.Guest) bool { return !spent(guest) }) {
			for _, guest := range p.Guests { // a pair passes on together, once its stronger one is spent
				guest.PassAt, guest.Path = g.clock, nil
			}
			break
		}
		if g.clock >= p.Checkout && g.mayPass(p) { // a Homebody stays until sent over
			p.state = checkingOut
			g.sendParty(p, exit...)
			break
		}
		g.bedtime(p)
	case checkingOut:
		g.walkParty(p, departSpeed)
		if p.outside(g.hotel.Rooms[p.Room]) {
			p.Paid, p.state = true, leaving
			g.logStay(p, outPassed)
			g.bookGhost(p.Room, g.bookGapNow())
		}
	case leaving:
		return !g.walkParty(p, departSpeed)
	}
	return true
}

// Passing on (2026-11-14): a spent ghost crosses to the other side. Over passFrames it rises passRise world px
// off the floor, slowly then fast, paling to white as it thins to nothing. Placeholders.
const (
	passFrames = 150
	passRise   = 48.0
	ghostFaint = 0.45 // of ghostAlpha: a ghost one haunt from spent. It dims with each haunt, so how long it has left shows
)

// passing: how far through its passing the ghost is, 0 (not yet) .. 1 (gone).
func (g *Game) passing(ghost *entities.Guest) float64 {
	if ghost.PassAt == 0 {
		return 0
	}
	return clamp((float64(g.clock-ghost.PassAt)+g.acc)/passFrames, 0, 1)
}

// ghostLook: a ghost's color, opacity and lift off the floor: its kind's hue (a quest ghost's own) at ghostAlpha, dimmer with each
// haunt it has done, and the passing over that.
func (g *Game) ghostLook(ghost *entities.Guest) (tint []float32, alpha float32, lift float64) {
	h := kindHue(kindOf(ghost.Phantasm))
	if own, ok := questHues[ghost.Who.Key]; ok { // 08 W5 R2: a quest ghost's hue is its own
		h = own
	}
	left := 1 - float64(ghost.Haunted)/float64(max(1, ghost.Phantasm))
	t := g.passing(ghost)
	if t > 0 {
		left = 1 // it flares before it goes
	}
	for i := range h {
		h[i] += (1 - h[i]) * float32(t)
	}
	return h[:], float32(ghostAlpha * (ghostFaint + (1-ghostFaint)*clamp(left, 0, 1)) * (1 - t*t)), t * t * passRise
}

// visible: the guests drawn in the current dimension: in real the humans only (a ghost shows through its
// area's Shadows), in ghost both.
func (g *Game) visible() []*entities.Guest {
	if g.ghost {
		return g.guests()
	}
	return slices.Collect(g.guestsIn(Human))
}

// A human seen from the ghost dimension: a flat white silhouette with a small white glow (placeholders).
var silhouetteTint = []float32{1, 1, 1}

const (
	silhouetteAlpha     = 0.9
	silhouetteGlow      = 0.3 // a faint white glow, like Fear's red one
	silhouetteGlowScale = 0.6 // and smaller
)

// DevGhosts is main's -ghosts: a human Party and a strong ghost Party of the given Phantasm settle in Room 0
// for 3 nights, the Concierge standing in it (use with -shot -at 9300 for them asleep, or -ghost).
func (g *Game) DevGhosts(phantasm int) {
	if phantasm <= 0 {
		return
	}
	g.unlock(Mirror)
	g.roomOwned = nil // every Room open: a dev start (boarded since 07 B1, none took the walk-in)
	h := g.walkIn()
	if h == nil {
		return
	}
	h.Routine, h.Nights = homebody, 3
	h.schedule(g.clock)
	p := g.arriveGhost(h.Room)
	p.Nights = 3
	p.schedule(g.clock)
	p.Guests[0].Phantasm = phantasm
	g.settleIn(h)
	g.settleIn(p)
	h.Guests[0].X += 30 // the human stands clear of the ghosts, to be seen
	h.Guests[0].Y += 22
	for k := range ghostKinds { // and one more of each kind, standing in a row (layers: they never bump)
		q := g.arriveGhost(h.Room)
		q.Guests, q.Nights = q.Guests[:1], 3
		q.schedule(g.clock)
		g.settleIn(q)
		q.Guests[0].Phantasm = max(1, phantasmBands[max(0, k-1)]*min(k, 1))
		q.Guests[0].X += float64(k*20 - 10)
		q.Guests[0].Y += 10
	}
	r := g.hotel.Rooms[h.Room].Origin
	g.player.X, g.player.Y = float64((r.X+world.RoomW/2)*world.TileSize), float64((r.Y+world.RoomH-3)*world.TileSize)
	w, hh := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, hh)
}

// DevMirror is main's -mirror: the Room mirror is owned and Room 101 hangs its own, holding two ghosts (the second
// came in through it); Room 102 has none, so its ring offers Add mirror. The wallet can pay it; the Concierge
// stands in Room 101.
func (g *Game) DevMirror() {
	g.unlock(Mirror)
	g.roomOwned = nil
	g.own(RoomMirror)
	g.setRoom(0, g.hotel.Rooms[0].Type, true)
	g.wallet[Money], g.wallet[Ecto] = max(g.wallet[Money], 300), max(g.wallet[Ecto], 300)
	for n := g.ghostsIn(0); n < 2; n++ {
		p := g.arriveGhost(0)
		p.Guests, p.Nights = p.Guests[:1], 3
		p.schedule(g.clock)
		p.Guests[0].Phantasm = 5 + 4*n
		g.settleIn(p)
	}
	r := g.hotel.Rooms[0].Origin
	g.player.X, g.player.Y = float64((r.X+world.RoomW/2)*world.TileSize), float64((r.Y+world.RoomH-3)*world.TileSize)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}

// DevGhost is main's -ghost: the Run shows the ghost dimension.
func (g *Game) DevGhost(on bool) { g.ghost = on }

// settleIn puts the Party, staying, on its Stay Spots (dev prints and tests).
func (g *Game) settleIn(p *Party) {
	p.state, p.settled = staying, true
	for i, guest := range p.Guests {
		at := g.stayOf(p, i)
		guest.X, guest.Y, guest.Path = float64(at.X), float64(at.Y), nil
	}
}

// covered: the prop is a tile of the lobby's Mirror (the two cells above its standing spot) while the Mirror
// is still locked, or of a Corridor Mirror not yet uncovered: drawn covered. A Room's own mirror prop is a separate upgrade (K1) and is never covered.
func (g *Game) covered(p prop) bool {
	c := g.hotel.Map.Cell(g.hotel.Mirror)
	if !strings.HasPrefix(g.ts.Tiles[p.id].Name, "prop_mirror_") {
		return false
	}
	return !g.unlocked(Mirror) && p.cx == c.X && (p.cy == c.Y-2 || p.cy == c.Y-1) || g.coveredCorridor(p) // or a Corridor Mirror not yet uncovered
}

// coverHalf: the covered Mirror's upper (r0) or lower (r1) 16x16 tile.
func (g *Game) coverHalf(tile string) *ebiten.Image {
	if g.cover == nil {
		img, _, err := ebitenutil.NewImageFromFileSystem(assets.FS, "sprites/mirror_covered.png")
		if err != nil {
			panic(err)
		}
		g.cover = img
	}
	y := 0
	if strings.HasPrefix(tile, "prop_mirror_r1") {
		y = world.TileSize
	}
	if g.coverHalves[y/world.TileSize] == nil { // kept: blit bakes each half's ghost look once
		g.coverHalves[y/world.TileSize] = g.cover.SubImage(image.Rect(0, y, world.TileSize, y+world.TileSize)).(*ebiten.Image)
	}
	return g.coverHalves[y/world.TileSize]
}
