package game

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// Matching (07 X2): a human's Nerve against its Room's ghost's Phantasm. Humans are clueless about ghosts; Nerve
// is how much fright one takes before it flees. A ghost within it is a good pair: its scares pay Ectoplasm and the
// human stays and pays as usual. A ghost beyond it is a bad pair: Fear piles up and the Party flees unpaid.
// Placeholders, for the sim.
const (
	nerveMin  = 6 // a human Party's Nerve is rolled nerveMin..nerveMax
	nerveMax  = phantasmMax
	nerveFear = shadowSeen // FleeAt per point of Nerve: one sighting a haunt of a ghost within it never reaches it
	badRent   = 0.5        // of its hourly rent a human Party pays while in a bad pair (party.go's charge; live test 1)
)

// Matchmaking: the X2 upgrade (HOTEL, Ectoplasm, on sale once a housekeeper is hired): idle housekeepers fix
// bad pairs with the cheapest sure offer (keeper.go).
const (
	Matchmaking Unlock = "matchmaking"
	matchEcto          = 40 // placeholder
)

// nerve: the human's Nerve, read off its FleeAt.
func nerve(h *entities.Guest) int { return h.FleeAt / nerveFear }

// partyNerve: its least nervy member's.
func partyNerve(p *Party) (n int) {
	n = nerveMax
	for _, h := range p.Guests {
		n = min(n, nerve(h))
	}
	return n
}

// setNerve gives every member of the Party the Nerve n, and with it its FleeAt.
func setNerve(p *Party, n int) {
	for _, h := range p.Guests {
		h.FleeAt = n * nerveFear
	}
}

// terrify: a haunt is half of what a human in the Room takes when the ghost is beyond its Nerve, seen or not: a bad
// pair steps out on the second and flees on the third. A ghost within it only frightens, by its Shadow (shadow.go).
func (g *Game) terrify(room int, ghost *entities.Guest) {
	_, in := g.inside(room)
	for _, h := range in {
		if !fits(h, ghost) {
			h.Fear = min(h.FleeAt, h.Fear+h.FleeAt/2)
		}
	}
}

// fits: the ghost's Phantasm is within the human's Nerve.
func fits(human, ghost *entities.Guest) bool { return ghost.Phantasm <= nerve(human) }

// roomPhantasm: the strongest ghost booked into the Room, 0 with none.
func (g *Game) roomPhantasm(room int) (most int) {
	for _, p := range g.parties {
		if p.Occupies(room, Ghost) {
			for _, gh := range p.Guests {
				most = max(most, gh.Phantasm)
			}
		}
	}
	return most
}

// humanIn: the human Party booked into the Room, nil if it is Vacant.
func (g *Game) humanIn(room int) *Party {
	for _, p := range g.parties {
		if p.Occupies(room, Human) {
			return p
		}
	}
	return nil
}

// fitRoom: the Room a Party of this Nerve and size books (X2): of the free ones (freeRoom's), the strongest ghost
// within its Nerve, else the weakest beyond it; a tie goes to the Room whose own booking is due first. -1 if none.
func (g *Game) fitRoom(n, size int) int {
	due := func(r int) int {
		if a := g.arrivals[r].at; a > 0 {
			return a
		}
		return int(^uint(0) >> 1)
	}
	best := -1
	for i, r := range g.hotel.Rooms {
		if !g.isOwned(i) || !g.vacant(i, Human) || g.closed(i) || g.changing(i) || len(r.Stays) < size {
			continue
		}
		if best < 0 {
			best = i
			continue
		}
		a, b := g.roomPhantasm(i), g.roomPhantasm(best)
		switch fa, fb := a <= n, b <= n; {
		case fa != fb:
			if fa {
				best = i
			}
		case a != b:
			if a > b == fa {
				best = i
			}
		case due(i) < due(best):
			best = i
		}
	}
	return best
}

type pairing int

const (
	unpaired pairing = iota // no human Party, or no ghost, in the Room
	goodPair
	badPair
)

// pairing: how the Room's human Party and its ghost get on.
func (g *Game) pairing(room int) pairing {
	p, ph := g.humanIn(room), g.roomPhantasm(room)
	switch {
	case p == nil || ph == 0:
		return unpaired
	case ph > partyNerve(p):
		return badPair
	}
	return goodPair
}

// badRed: a bad pair's ink in the hotel view; a good pair's is the Ectoplasm ink.
var badRed = color.RGBA{230, 40, 50, 255}

// The pair in the hotel view (live test 2: diffuse, not a frame): the Room's walls' top band in its ink, and
// moteN motes of it rising in the Room, each moteRise world px over moteLoop real frames, fading in and out.
const (
	moteN    = 8
	moteRise = 24
	moteLoop = 180
)

type mote struct {
	at   image.Point // world px, its top-left
	size int         // art px
	a    float32
}

// motes: the Room's motes t real frames in, each in the shell at a spot drawn afresh each loop from a generator
// of its own, seeded by the Room, the mote and the loop: the sim's streams are never touched.
func motes(shell image.Rectangle, room int, t int64) (ms [moteN]mote) {
	for j := range ms {
		u := t + int64(j*moteLoop/moteN) // staggered
		r := rand.New(rand.NewPCG(uint64(room*moteN+j), uint64(u/moteLoop)))
		f := float64(u%moteLoop) / moteLoop
		size := 2 + r.IntN(2) // the whole hotel shows at about 1 screen px per art px: smaller is lost
		x := shell.Min.X + r.IntN(shell.Dx()-size+1)
		y := shell.Min.Y + moteRise + r.IntN(shell.Dy()-moteRise-size+1) - int(f*moteRise)
		ms[j] = mote{image.Pt(x, y), size, float32(math.Sin(math.Pi * f))}
	}
	return ms
}

// drawPair draws the Room's motes in its pair's ink c over the hotel view at scale k (screen px per world px),
// offset (tx,ty) (viewAt): motes only, the walls keep their colour (live test 4).
func (g *Game) drawPair(s *ebiten.Image, room int, c color.RGBA, k, tx, ty float64) {
	a := float32(g.viewT)
	for _, m := range motes(g.hotel.Rooms[room].Shell(), room, ebiten.Tick()) {
		w := max(1, int(math.Round(float64(m.size)*k)))
		rect(s, int(float64(m.at.X)*k+tx), int(float64(m.at.Y)*k+ty), w, w, color.NRGBA{c.R, c.G, c.B, uint8(200 * m.a * a)})
	}
}

// pairInk: the Room's pair's ink in the hotel view, false for none.
func (g *Game) pairInk(room int) (color.RGBA, bool) {
	switch g.pairing(room) {
	case badPair:
		return badRed, true
	case goodPair:
		return inkAt(Ecto, ebiten.Tick()), true
	}
	return color.RGBA{}, false
}

// movable: a human Party that can be sent to another Room: in it, on its way in or out for the day, or away.
func movable(p *Party) bool {
	return p.state == staying || p.state == checkingIn || p.state == goingOut || p.state == out
}

// relocate: the Party moves to Room to, swapping with the human Party there if any (X2). Each walks over by
// itself, its rent clock running on at the new Room's Rate; the Moods stay with the Rooms, and so do the next
// arrivals: the Room left takes over the booking of the one taken.
func (g *Game) relocate(p *Party, to int) {
	from, q := p.Room, g.humanIn(to)
	g.arrivals[from], g.arrivals[to] = g.arrivals[to], g.arrivals[from]
	g.rebook(p, to)
	if q != nil {
		g.rebook(q, from)
	}
}

// rebook: the Party's booking is Room to from now. Unless it is away it walks out of its old Room's door
// (moving), then in to the new one.
func (g *Game) rebook(p *Party, to int) {
	from := p.Room
	p.Room, p.Rate, p.abed, p.acts = to, g.hotel.Rooms[to].Type.Rate, false, nil
	if p.state == out {
		for _, h := range p.Guests {
			h.RoomIndex = to
		}
		return
	}
	spot := g.hotel.Rooms[from].Spot()
	at := spot.Min.Add(spot.Max).Div(2)
	p.state, p.greet = moving, noGreeting
	g.sendParty(p, at, at)
}

// DevMatch is main's -match: Room 101 holds a bad pair (a human of Nerve 2, a ghost of Phantasm 9), Room 102 a
// pair the other way round, the Concierge in Room 101. step 0 leaves it there (with -view 0 the hotel view's
// tints), 1 opens the talk, 2 has the swap on the house taken and both Parties under way.
func (g *Game) DevMatch(step int) {
	if step < 0 || len(g.hotel.Rooms) < 2 {
		return
	}
	g.unlock(Mirror)
	g.roomOwned = nil
	g.arrivals, g.ghostArrivals = make([]arrival, len(g.hotel.Rooms)), make([]arrival, len(g.hotel.Rooms))
	// Rooms 101 and 102 are this start's: Parties another start put there give way (the later start wins what
	// they both set), and the wallet is raised to the swap's price, never lowered under another start's.
	g.parties = slices.DeleteFunc(g.parties, func(p *Party) bool { return p.Room < 2 })
	g.wallet[Money] = max(g.wallet[Money], 600)
	for room, c := range [][2]int{{2, 9}, {9, 2}} {
		h := g.arrive(room)
		h.Guests, h.Routine, h.Nights = h.Guests[:1], homebody, 3
		h.schedule(g.clock)
		setNerve(h, c[0])
		gh := g.arriveGhost(room)
		gh.Guests, gh.Nights = gh.Guests[:1], 3
		gh.schedule(g.clock)
		gh.Guests[0].Phantasm = c[1]
		g.settleIn(h)
		g.settleIn(gh)
		h.Guests[0].X += 4 * world.TileSize // clear of its ghost, and of the Concierge at the door
		h.Guests[0].Y += world.TileSize
	}
	d := g.hotel.Rooms[0].Door
	at := g.hotel.Map.Center(d.X, d.Y-2)
	g.player.X, g.player.Y = float64(at.X), float64(at.Y)
	g.player.Anim.Facing = entities.FaceRight
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
	if step >= 1 {
		g.startTalk(g.humanIn(0))
	}
	if step >= 2 && g.parley != nil {
		g.parley.pick = onTheHouse
		g.makeOffer()
	}
}
