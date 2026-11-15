package game

import (
	"image"
	"math"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// Housekeepers (07 H1 step 3): staff hired at the store's STAFF tab, several at once. One brings a stocked
// Supply to a Room that asks and cleans a Room whose Party complains of the dirt; with nothing to do it roams
// the hotel. Each is paid a wage a game day after it was hired, and every day after. Short of it, it stops
// working (the wage is taken as soon as the Money is there); still unpaid a day later, it walks out.
// It tells the Concierge so in its own lines, as he comes near (2026-11-14: that is how the player learns
// something is wrong; there is no system text). Placeholders, step 4 tunes them.
const (
	rowKeeper    Unlock = "housekeeper"
	keeperName          = "Housekeeper" // Amelia as a maid: pale, black hair and dress, white apron
	keeperFee           = 100           // $ to hire one
	keeperWage          = 60            // $ each, a game day
	keeperEarned        = 500           // $ the hotel has taken before one answers the ad: about minute 6 of a Run, after its third Room (2026-11-14, 4c)
	keeperMax           = 3
	keeperSpeed         = conciergeSpeed
	keeperNear          = 3           // tiles: the Concierge this close hears what it has to say
	keeperChat          = 0.25        // of the times he comes near with nothing wrong, it says a line; a problem is always said
	keeperWarn          = 8           // game hours before a payday the wallet can't cover, it starts to worry
	keeperRest          = 20          // game minutes it stands at a spot before walking on, about
	keeperStroll        = arriveSpeed // its pace with no job on
)

var keeperLook = look{hair: 0x2b2630, skin: 0xffe6d6, top: 0x24202a, bottom: 0xf2f2f6}

var (
	keeperChats = []string{"All in order.", "Busy day!", "Mind the wet floor.", "Fresh towels smell nice.", "Room service never sleeps.", "Who keeps moving the lamp?",
		"Sheets, sheets, sheets.", "Dust never rests.", "Lovely guests today.", "I found a sock. Just one.", "The kettle's on.", odd("The mop and I are not speaking.")}
	keeperWelcome = []string{"Welcome!", "Welcome to the hotel!", "Enjoy your stay!", "So glad you came!", odd("Welcome! Mind the soup.")}
	keeperWorry   = []string{"Payday is close. Can we afford it?", "The till looks thin for payday."}
	keeperOwed    = []string{"No pay, no work.", "I wasn't paid today."}
	keeperQuit    = []string{"I quit. No pay, no stay."}
	keeperMoved   = []string{"Your new room is ready!", "This way, please!", "A better room, on us!", odd("Your new room has more corners!")}
)

func init() { ghostTints[keeperName] = []float32{.4, 1, 1} }

type job int

const (
	resting  job = iota // no job: roaming the hotel
	fetching            // to the wanted Supply's prop
	serving             // to the Room's door, the item in its hands
	cleaning            // to a stain in the Room
	quitting            // out the Exit
	matching            // to a bad pair's door, to talk its human Party into another Room (07 X2, Matchmaking)
)

type housekeeper struct {
	entities.Guest // RoomIndex is the Room it walks into or out of, whose door is open to its path; -1 otherwise
	job            job
	room           int    // the Room its job is for
	item           Unlock // the Supply it fetches, then carries
	due            int    // the clock of its next payday
	unpaid         bool   // its last wage is owed: it does nothing until it is paid
	rest           int    // the clock it next moves about the kitchen
	voice                 // it speaks as the Concierge comes near, and to welcome a Party walking in
}

// staffed: the housekeepers on the payroll: every one not walking out.
func (g *Game) staffed() int {
	n := 0
	for _, k := range g.keepers {
		if k.job != quitting {
			n++
		}
	}
	return n
}

// hire: a paid-for housekeeper walks in from the Entrance to the kitchen. Its first payday is a day away.
func (g *Game) hire() {
	k := &housekeeper{Guest: entities.NewGuest(g.hotel.Spawn, keeperName), due: g.clock + dayFrames}
	g.keepers = append(g.keepers, k)
	g.hired++
	g.sendKeeper(k, g.kitchenSpot(), -1)
}

// payStaff: pay, counted as staff cost (the sim's money check).
func (g *Game) payStaff(c Cost) bool {
	if !g.pay(c) {
		return false
	}
	g.staffPaid += c[Money]
	return true
}

// kitchenSpot: a cell of the kitchen a row clear of the props' fetch row, so one standing there is in nobody's
// way, and one no other housekeeper stands on or walks to.
func (g *Game) kitchenSpot() image.Point {
	const ts = world.TileSize
	s := g.hotel.Stock
	x0 := g.rng.IntN(world.KitchenW)
	for d := range world.KitchenW {
		at := image.Pt(s.Min.X+(x0+d)%world.KitchenW*ts+ts/2, s.Max.Y+ts+ts/2)
		if !slices.ContainsFunc(g.keepers, func(k *housekeeper) bool {
			to := image.Pt(int(k.X), int(k.Y))
			if len(k.Path) > 0 {
				to = k.Path[len(k.Path)-1]
			}
			return to.Div(ts) == at.Div(ts)
		}) {
			return at
		}
	}
	return image.Pt(s.Min.X+x0*ts+ts/2, s.Max.Y+ts+ts/2)
}

// sendKeeper sets it walking to `to`, through the door of room if it is going into or out of one.
func (g *Game) sendKeeper(k *housekeeper, to image.Point, room int) {
	k.RoomIndex = room
	g.reroute(&k.Guest, to)
}

// dropJob: whatever it was doing, it is resting now; a Supply in its hands goes back to the stock.
func (g *Game) dropJob(k *housekeeper) {
	if k.job == serving {
		g.stock[k.item]++
	}
	k.job, k.item = resting, ""
}

// roamSpot: somewhere in the hotel to walk to with nothing to do (2026-11-14: it roams the whole hotel): a
// cell of the hall, the kitchen or a Corridor it can reach, out of everyone's way: not in a Room, a doorway,
// the spot before a door, the Terminal's, a Mirror's or a Supply's. The kitchen if none turns up.
func (g *Game) roamSpot(k *housekeeper) image.Point {
	h, m := g.hotel, g.hotel.Map
	from := image.Pt(int(k.X), int(k.Y))
	for range 20 {
		c := image.Pt(g.rng.IntN(m.W), g.rng.IntN(m.H))
		at := m.Center(c.X, c.Y)
		if m.Solid(c.X, c.Y) || h.Choke(c) || at.Y >= h.HallY && !at.In(h.Lobby) || at.In(h.Terminal) || at.In(h.MirrorSpot) ||
			at.In(image.Rect(h.Stock.Min.X, h.Stock.Min.Y, h.Stock.Max.X, h.Stock.Max.Y+world.TileSize)) ||
			slices.ContainsFunc(h.Rooms, func(r world.Room) bool { return at.In(r.Shell()) || at.In(r.Spot()) }) {
			continue
		}
		if h.Path(from, at, g.roomAt(k.X, k.Y)) != nil {
			return at
		}
	}
	return g.kitchenSpot()
}

// goRest sends it off to roam, out of the Room it may stand in.
func (g *Game) goRest(k *housekeeper) {
	g.dropJob(k)
	k.rest = g.clock + (keeperRest/2+g.rng.IntN(keeperRest))*minuteFrames
	g.sendKeeper(k, g.roamSpot(k), g.roomAt(k.X, k.Y))
}

// nextJob gives a resting housekeeper the first job nobody else is on: a Room whose Party complains of its
// dirt, then a Room asking for a Supply the stock has. False if there is none.
func (g *Game) nextJob(k *housekeeper) bool {
	taken := func(room int, js ...job) bool {
		return slices.ContainsFunc(g.keepers, func(o *housekeeper) bool { return o.room == room && slices.Contains(js, o.job) })
	}
	for _, p := range g.parties {
		if p.Realm != Human {
			continue
		}
		switch {
		case p.state == complaining && !taken(p.Room, cleaning):
			if i := slices.IndexFunc(g.spots(p.Room), func(s spot) bool { return !s.spirit }); i >= 0 {
				c := g.spots(p.Room)[i].cell
				k.job, k.room = cleaning, p.Room
				g.sendKeeper(k, g.hotel.Map.Center(c.X, c.Y), p.Room)
				return true
			}
		case p.asking() && p.state == staying && g.stock[p.want] > 0 && !taken(p.Room, fetching, serving):
			k.job, k.room, k.item = fetching, p.Room, p.want
			g.sendKeeper(k, g.supplyDrop(supplyOf(p.want)), -1)
			return true
		}
	}
	for i := range g.hotel.Rooms { // Matchmaking (X2): a bad pair whose Party she can afford to move
		if p := g.humanIn(i); g.unlocked(Matchmaking) && p != nil && g.pairing(i) == badPair && movable(p) && !taken(i, matching) {
			if _, _, c, ok := g.sureOffer(p); ok && g.afford(Cost{Money: c}) {
				k.job, k.room = matching, i
				g.sendKeeper(k, g.doorDrop(i), -1)
				return true
			}
		}
	}
	return false
}

// payday runs each frame: an owed wage is taken as soon as the wallet covers it; on its payday it is paid, or
// owed from then, or, still owed from the last one, it walks out.
func (g *Game) payday(k *housekeeper) {
	wage := Cost{Money: keeperWage}
	if k.job == quitting {
		return
	}
	if k.unpaid && g.payStaff(wage) {
		k.unpaid, k.near = false, false
	}
	if g.clock < k.due {
		return
	}
	k.due += dayFrames
	switch {
	case k.unpaid:
		g.dropJob(k)
		k.job, k.near = quitting, false
		g.quits++
		g.sendKeeper(k, g.hotel.Exit, g.roomAt(k.X, k.Y))
	case !g.payStaff(wage):
		k.unpaid, k.near = true, false
		g.goRest(k)
	}
}

func (g *Game) updateKeepers() {
	kept := g.keepers[:0]
	for _, k := range g.keepers {
		speed := keeperSpeed
		if k.job == resting {
			speed = keeperStroll
		}
		arrived := g.walk(&k.Guest, speed*g.staffSpeed())
		if !g.headless {
			k.Anim.Update(k.X, k.Y, g.dt)
		}
		g.payday(k)
		g.keeperTalk(k)
		switch {
		case k.job == quitting:
			if arrived { // out the Exit
				continue
			}
		case k.unpaid:
		case k.job == resting:
			in := g.roomAt(k.X, k.Y) >= 0
			if !in && g.nextJob(k) {
				break
			}
			if arrived && (in || g.clock >= k.rest) {
				g.goRest(k)
			}
		case (k.job == fetching || k.job == serving) && !g.wants(k.room, k.item): // the ask is gone: turn back at once (08 F0)
			g.goRest(k)
		case !arrived:
		case k.job == fetching:
			if g.stock[k.item] == 0 {
				g.goRest(k)
				break
			}
			g.stock[k.item]--
			k.job = serving
			g.sendKeeper(k, g.doorDrop(k.room), -1)
		case k.job == serving:
			if g.handOver(k.room, k.item) {
				k.job = resting // handed over: nothing goes back to the stock
			}
			g.goRest(k)
		case k.job == cleaning:
			if g.normalDirt(k.room) > 0 {
				g.scrub(k.room, false)
			}
			g.goRest(k)
		case k.job == matching: // at the door: the cheapest sure offer, no talk staged
			if p := g.humanIn(k.room); p != nil && g.pairing(k.room) == badPair && movable(p) {
				if to, _, c, ok := g.sureOffer(p); ok && g.payOffer(c) {
					g.relocate(p, to)
					k.said = line{0, g.pickLine(keeperMoved), g.tick}
				}
			}
			g.goRest(k)
		}
		kept = append(kept, k)
	}
	clear(g.keepers[len(kept):])
	g.keepers = kept
}

// keeperTalk: as the Concierge comes near, it says what is wrong, if anything is; else, now and then, a line.
// And it welcomes a human Party walking in past it, each Party once (2026-11-14).
func (g *Game) keeperTalk(k *housekeeper) {
	g.comeNear(&k.Guest, &k.voice, func() []string { return g.keeperLines(k) })
	if k.unpaid || k.job == quitting || k.talking(g.tick) {
		return
	}
	for _, p := range g.parties {
		if lead := p.Guests[0]; p.Realm == Human && p.state == checkingIn && !p.welcomed && g.shown(lead) &&
			math.Hypot(lead.X-k.X, lead.Y-k.Y) <= keeperNear*world.TileSize {
			p.welcomed = true
			k.said = line{0, g.pickLine(keeperWelcome), g.tick}
			return
		}
	}
}

// keeperLines: what it may say now, nil for nothing.
func (g *Game) keeperLines(k *housekeeper) []string {
	switch {
	case k.job == quitting:
		return keeperQuit
	case k.unpaid:
		return keeperOwed
	case g.clock >= k.due-keeperWarn*hourFrames && g.wallet[Money] < Num(keeperWage*g.staffed()):
		return keeperWorry
	}
	return g.sometimes(keeperChats)
}

// keeperActors: the housekeepers, for the doors: they open any, as the rest of the staff.
func (g *Game) keeperActors() []world.Actor {
	var as []world.Actor
	for _, k := range g.keepers {
		as = append(as, world.Actor{Box: entities.FootBox(k.X, k.Y), Room: -1})
	}
	return as
}

// drawKeepers draws, over each housekeeper, the Supply it carries.
func (g *Game) drawKeepers(s *ebiten.Image, ox, oy int) {
	for _, k := range g.keepers {
		if k.job == serving {
			if icon := g.supplyIcon(k.item); icon != nil {
				g.drawHeld(s, icon, int(k.X*zoom)-ox, int(k.Y*zoom)-oy)
			}
		}
	}
}

// loadKeeper dresses Amelia as the housekeeper.
func (g *Game) loadKeeper() {
	b := g.bases["Amelia"]
	g.characters[keeperName] = dress([2]*image.RGBA{maid(b[0]), maid(b[1])}, palettes["Amelia"], keeperLook)
}

// maid: a copy of Amelia's sheet with, seen from the front, a white apron bib on the dress. Placeholder art, as the courier's box: the sheet's own pixels, turned white.
func maid(sheet *image.RGBA) *image.RGBA {
	out := image.NewRGBA(sheet.Bounds())
	copy(out.Pix, sheet.Pix)
	white, pal := opaque(0xf2f2f6), palettes["Amelia"]
	is := func(x, y int, r region) bool {
		p := sheet.RGBAAt(x, y)
		return p.A > 0 && pal[uint32(p.R)<<16|uint32(p.G)<<8|uint32(p.B)][min(1, y/bodyRow)] == r
	}
	for f := range sheet.Bounds().Dx() / charW {
		for x := f * charW; x < (f+1)*charW; x++ {
			for y := 24; y <= 26; y++ {
				if dx := x - f*charW; entities.Facing(f/entities.FramesPerFacing) == entities.FaceDown && dx >= 5 && dx <= 10 && is(x, y, top) {
					out.SetRGBA(x, y, white)
				}
			}
		}
	}
	return out
}

// DevStaff is main's -staff: the Run starts with n housekeepers in the kitchen, the stock full, money in the
// wallet (less than their wages: they are owed them already, and walk out in a day), and the Concierge beside them.
func (g *Game) DevStaff(n int, money float64) {
	g.stock = map[Unlock]int{SupplyWater: 5, SupplyFood: 5, SupplyTowel: 5}
	g.wallet[Money] = Num(money)
	for range n {
		g.hire()
		k := g.keepers[len(g.keepers)-1]
		at := g.kitchenSpot()
		k.X, k.Y, k.Path = float64(at.X), float64(at.Y), nil
		k.unpaid = money < float64(keeperWage*n) // short of their wages: they are owed them already
	}
	at := g.stockDrop()
	g.player.X, g.player.Y = float64(at.X), float64(at.Y+2*world.TileSize)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}
