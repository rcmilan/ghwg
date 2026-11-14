package game

import (
	"image"
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

// phantasmStrong: a ghost at or above this is strong: it switches an area's Shadow People on. A weak one
// (the lowest kind) doesn't.
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
// solo, or a pair in a Room with two Stay Spots, one of the 4 characters each.
func (g *Game) arriveGhost(room int) *Party {
	if room < 0 || room >= len(g.hotel.Rooms) {
		return nil
	}
	r := g.hotel.Rooms[room]
	guest := g.newGhost()
	p := &Party{Guests: []*entities.Guest{guest},
		Booking: Booking{Room: room, Realm: Ghost, Nights: 1 + g.chat.IntN(maxNights), Rate: r.Type.Rate,
			checkoutAt: checkoutHour*hourFrames + g.chat.IntN(checkoutJitter)}}
	if len(r.Stays) >= 2 && g.chat.Float64() < pairOdds {
		mate := g.newGhost()
		mate.Leader = guest
		p.Guests = append(p.Guests, mate)
	}
	p.arrived = g.clock
	p.schedule(g.clock)
	for _, guest := range p.Guests {
		guest.RoomIndex = room
	}
	g.parties = append(g.parties, p)
	g.checkIn(p)
	return p
}

func (g *Game) newGhost() *entities.Guest {
	guest := entities.NewGuest(g.hotel.Spawn, shadowCast[g.chat.IntN(len(shadowCast))])
	guest.Phantasm, guest.Haunt = 1+g.chat.IntN(phantasmMax), ghostHaunt
	return &guest
}

// bookGhost: the Room's next ghost Party arrives gap game minutes from now.
func (g *Game) bookGhost(room, gap int) {
	g.ghostArrivals[room] = arrival{g.clock, g.clock + gap*minuteFrames}
}

// maybeGhostIn is maybeWalkIn for the ghost layer: nothing books until the Mirror is unlocked. Ghosts ignore
// dirt, but a Closed Room (Mood at the floor) takes no Party of either realm.
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
	for i, a := range g.ghostArrivals {
		if a.at > 0 && g.clock >= a.at && booked < g.demand() && g.isOwned(i) && g.vacant(i, Ghost) && !g.closed(i) && !g.changing(i) && g.spawnClearIn(Ghost) {
			g.ghostArrivals[i] = arrival{}
			g.arriveGhost(i)
			booked++
		}
	}
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
		}
	case staying:
		if g.clock >= p.Checkout {
			p.state = checkingOut
			g.sendParty(p, exit...)
			break
		}
		g.bedtime(p)
	case checkingOut:
		g.walkParty(p, departSpeed)
		if p.outside(g.hotel.Rooms[p.Room]) {
			p.Paid, p.state = true, leaving
			g.bookGhost(p.Room, bookGap)
		}
	case leaving:
		return !g.walkParty(p, departSpeed)
	}
	return true
}

// visible: the guests drawn in the current dimension: in real the humans only (a ghost shows through its
// area's Shadows), in ghost both.
func (g *Game) visible() []*entities.Guest {
	if g.ghost {
		return g.guests()
	}
	return g.guestsIn(Human)
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

// settleIn puts the Party, staying, on its Stay Spots (dev prints and tests).
func (g *Game) settleIn(p *Party) {
	p.state, p.settled = staying, true
	for i, guest := range p.Guests {
		at := g.hotel.Rooms[p.Room].Stays[i]
		guest.X, guest.Y, guest.Path = float64(at.X), float64(at.Y), nil
	}
}

// covered: the prop is a tile of the lobby's Mirror (the two cells above its standing spot) while the Mirror
// is still locked: drawn covered. A Room's own mirror prop is a separate upgrade (K1) and is never covered.
func (g *Game) covered(p prop) bool {
	c := g.hotel.Map.Cell(g.hotel.Mirror)
	return !g.unlocked(Mirror) && p.cx == c.X && (p.cy == c.Y-2 || p.cy == c.Y-1) && strings.HasPrefix(g.ts.Tiles[p.id].Name, "prop_mirror_")
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
	return g.cover.SubImage(image.Rect(0, y, world.TileSize, y+world.TileSize)).(*ebiten.Image)
}
