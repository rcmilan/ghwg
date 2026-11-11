package game

import (
	"image"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// Ghost guests (07 G1): once the Mirror is unlocked each Room also books a ghost Party, apart from its
// human one (the realms never bump nor queue, realm.go). A ghost pays no Money and ignores dirt; it earns
// only through encounters (V1). Its Phantasm sets its kind, the Mood a haunt costs (mood.go) and whether it
// is strong enough to switch an area's Shadow People on (shadow.go). All values are placeholders.
const (
	phantasmMax = 12  // a ghost's Phantasm is rolled 1..phantasmMax
	ghostHaunt  = 240 // x: game minutes between a ghost's haunts (the Mood pressure on a Room)
	ghostAlpha  = 0.55
)

// phantasmBands: the Phantasm each kind from the second starts at; the lowest kind is below the first edge.
var phantasmBands = [...]int{4, 7, 10}

// phantasmStrong: a ghost at or above this is strong: it switches an area's Shadow People on. A weak one
// (the lowest kind) doesn't.
const phantasmStrong = 4

type ghostKind struct {
	Name string
	Hue  [3]float32 // placeholder hues (A3, 2026-11-11): #6fd8e8, #a07ae8, #7ee89a, #e87aa8
}

var ghostKinds = [...]ghostKind{
	{"Wisp", [3]float32{111. / 255, 216. / 255, 232. / 255}},
	{"Specter", [3]float32{160. / 255, 122. / 255, 232. / 255}},
	{"Wraith", [3]float32{126. / 255, 232. / 255, 154. / 255}},
	{"Poltergeist", [3]float32{232. / 255, 122. / 255, 168. / 255}},
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
		if a.at > 0 && g.clock >= a.at && booked < g.demand() && g.vacant(i, Ghost) && !g.closed(i) && g.spawnClearIn(Ghost) {
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

// A human seen from the ghost dimension: a dim silhouette, so the ghosts read first (placeholders).
var silhouetteTint = []float32{.28, .28, .4}

const silhouetteAlpha = 0.6

// DevGhosts is main's -ghosts: a human Party and a strong ghost Party of the given Phantasm settle in Room 0
// for 3 nights, the Concierge standing in it (use with -shot -at 9300 for them asleep, or -ghost).
func (g *Game) DevGhosts(phantasm int) {
	h := g.walkIn()
	if h == nil || phantasm <= 0 {
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
