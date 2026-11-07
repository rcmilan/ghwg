package game

import (
	"fmt"
	"image"
	"math"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// Walk-in tunables (D77–D79). Rates are Walk-ins per game hour, rolled once per game minute.
var walkInsPerHour = [24]float64{
	1, 1, // 00–02: late arrivals
	0, 0, 0, 0, // 02–06: none
	1, 1, 1, 1, 1, 1, 1, 1, // 06–14: a few
	2, 2, 2, 2, 2, 2, 2, 2, // 14–22: the peak
	1, 1,
}

const (
	rampDays         = 3                  // Day d lets in Guests for up to d/rampDays of the Rooms (at least 1): the hotel fills gradually
	patienceMin      = 120 * minuteFrames // a queued Party gives up after 2–3 game hours (D79)
	patienceJitter   = 60 * minuteFrames
	maxNights        = 3
	checkoutHour     = 10 // checkout is 10:00–12:00 on the final day (D88)
	checkoutJitter   = 120 * minuteFrames
	outHour          = 8 // a Tourist goes out at 08:00–10:00 (D88)
	outJitter        = 120 * minuteFrames
	backHour         = 19 // and comes back at 19:00–22:00
	backJitter       = 180 * minuteFrames
	routineStartHour = 10             // booked from 10:00 on, a Tourist's routine starts the next day
	hotelDayStart    = 6              // a Booking made before 06:00 belongs to the night before
	pairOdds         = 0.4            // a Walk-in is a pair 40% of the time, else solo (D77)
	trailGap         = world.TileSize // a follower stays about this far behind its leader (D90)
	floaterFrames    = 90
	floaterRisePerFr = 0.25 // world px
)

type partyState int

const (
	queuing     partyState = iota // walking to or standing in its Queue slot
	checkingIn                    // booked: walking to its Stay Spot, or back to it from an outing
	staying                       // in the Room until Checkout
	goingOut                      // a Tourist walking out of the Entrance for the day
	out                           // a Tourist away from the hotel: off the map
	checkingOut                   // walking out of the Room; pays as it steps out of it
	leaving                       // walking out of the Entrance for good, paid or not
)

type routine int

const (
	homebody routine = iota // stays in the Room
	tourist                 // out by day, every day but the checkout day
)

func (r routine) String() string { return [...]string{"HOMEBODY", "TOURIST"}[r] }

// Booking is a Party's stay (D99): instant and final (D98), with the Rate locked when it is made (D53).
type Booking struct {
	Room     int // -1 until booked
	Nights   int
	Routine  routine
	Rate     int
	Checkout int // clock frame the Party leaves its Room for good
	Paid     bool
	// The Party's own times of day, in frames (D88's jitter), and a Tourist's first day out.
	outAt, backAt, checkoutAt, firstOut int
}

// schedule fixes the stay's days once booked at clock c (D88): checkout on the final hotel day (before
// 06:00 counts as the night before), and a Tourist's first day out: today if booked before 10:00.
func (b *Booking) schedule(c int) {
	b.Checkout = ((c-hotelDayStart*hourFrames)/dayFrames+b.Nights)*dayFrames + b.checkoutAt
	b.firstOut = c / dayFrames
	if c%dayFrames >= routineStartHour*hourFrames {
		b.firstOut++
	}
}

// Out: a Tourist is away at clock c (D88), between its out and back times on every day of its routine
// but the checkout day.
func (b Booking) Out(c int) bool {
	d, t := c/dayFrames, c%dayFrames
	return b.Routine == tourist && d >= b.firstOut && d < b.Checkout/dayFrames && t >= b.outAt && t < b.backAt
}

// Payout is what the Party banks at Checkout (D86).
func (b Booking) Payout() int { return b.Rate * b.Nights }

// Occupies: the Room is Occupied from Booking until the Party pays at its door (D91).
func (b Booking) Occupies(room int) bool { return b.Room == room && !b.Paid }

// Party is 1–2 Guests on one Booking (D76): Guests[0] leads, the other trails it (D90).
type Party struct {
	Guests   []*entities.Guest
	Wish     string // a pair's bed wish, Twin or Double (D81); "" solo
	state    partyState
	slot     int           // its Queue lane, kept until it leaves the Queue
	patience int           // clock frame it gives up queueing (D79)
	goals    []image.Point // where each member is headed this leg
	peeled   bool          // this leg's leader has arrived and the follower heads for its own goal
	settled  bool          // it has reached its Stay Spots once: from then on its Room stays fogged (S6 review)
	Booking
}

// rateIn is the Room's per-night Rate for the Party: half for a pair in the 2-person layout it didn't
// ask for (D81). A pair never fits a Single.
func (p *Party) rateIn(r world.Room) int {
	if len(p.Guests) == 2 && r.Type.Beds != p.Wish {
		return r.Type.Rate / 2
	}
	return r.Type.Rate
}

// floater is a "+$X" rising over a door after a Checkout (D87).
type floater struct {
	x, y float64 // world px
	text string
	at   int // tick
}

// guests: everyone on the map. A member walking out of the Entrance is gone once it reaches the Exit,
// so it never blocks the Street while its mate catches up.
func (g *Game) guests() []*entities.Guest {
	var gs []*entities.Guest
	for _, p := range g.parties {
		for _, guest := range p.Guests {
			if p.state != out && !((p.state == goingOut || p.state == leaving) && len(guest.Path) == 0) {
				gs = append(gs, guest)
			}
		}
	}
	return gs
}

func (g *Game) vacant(room int) bool {
	for _, p := range g.parties {
		if p.Occupies(room) {
			return false
		}
	}
	return true
}

// demand: the Parties booked or queued that the Day lets in (rampDays), so the first nights never fill
// the hotel.
func (g *Game) demand() int {
	n := len(g.hotel.Rooms)
	return min(n, max(1, n*day(g.clock)/rampDays))
}

// maybeWalkIn rolls this game minute's Walk-in on the hourly curve (D77), unless every Queue lane is
// taken or the Day's demand is met: that also needs a Vacant Room.
func (g *Game) maybeWalkIn() {
	booked := 0
	for i := range g.hotel.Rooms {
		if !g.vacant(i) {
			booked++
		}
	}
	if len(g.queue) < len(g.hotel.Queue) && booked+len(g.queue) < g.demand() && g.spawnClear() && g.rng.Float64() < walkInsPerHour[hour(g.clock)]/60 {
		g.walkIn()
	}
}

// spawnClear: nobody stands within a tile of Spawn, so a Party appearing there can't land on another one
// (a follower still waiting for its leader to pull ahead).
func (g *Game) spawnClear() bool {
	return !slices.ContainsFunc(g.guests(), func(guest *entities.Guest) bool {
		return math.Hypot(guest.X-float64(g.hotel.Spawn.X), guest.Y-float64(g.hotel.Spawn.Y)) < world.TileSize
	})
}

// walkIn puts a new Party on the Street, joining the back of the Queue in the free lane nearest the
// Terminal. Lanes don't move up: the Party keeps its lane until booked (D78).
func (g *Game) walkIn() *Party {
	guest := entities.NewGuest(g.hotel.Spawn, guestCast[g.rng.IntN(len(guestCast))])
	p := &Party{Guests: []*entities.Guest{&guest}, patience: g.clock + patienceMin + g.rng.IntN(patienceJitter),
		Booking: Booking{Room: -1, Nights: 1 + g.rng.IntN(maxNights), Routine: routine(g.rng.IntN(2)),
			outAt:      outHour*hourFrames + g.rng.IntN(outJitter),
			backAt:     backHour*hourFrames + g.rng.IntN(backJitter),
			checkoutAt: checkoutHour*hourFrames + g.rng.IntN(checkoutJitter)}}
	if g.rng.Float64() < pairOdds {
		mate := entities.NewGuest(g.hotel.Spawn, guestCast[g.rng.IntN(len(guestCast))])
		mate.Leader = &guest
		p.Guests, p.Wish = append(p.Guests, &mate), [...]string{"Twin", "Double"}[g.rng.IntN(2)]
	}
	if g.walkIns == 0 {
		p.Nights = 1 // a Room frees up on Day 2, for that night's Spin (S1 review)
	}
	g.walkIns++
	for p.slot = 0; slices.ContainsFunc(g.queue, func(q *Party) bool { return q.slot == p.slot }); p.slot++ {
	}
	lane := g.hotel.Queue[p.slot]
	g.sendParty(p, lane, lane.Add(image.Pt(0, world.TileSize))) // the follower stands behind its leader (D78)
	g.parties = append(g.parties, p)
	g.queue = append(g.queue, p)
	return p
}

// atCounter: the front Party stands in its lane. A Walk-in still on its way can't be booked.
func (g *Game) atCounter() bool { return len(g.queue) > 0 && len(g.queue[0].Guests[0].Path) == 0 }

// fits: the front Party can be booked into the Room. "" if so, else why not.
func (g *Game) fits(room int) string {
	switch {
	case !g.atCounter():
		return "No guests at the counter."
	case !g.vacant(room):
		return "That room is booked."
	case len(g.queue[0].Guests) > len(g.hotel.Rooms[room].Stays):
		return "That room is too small."
	}
	return ""
}

// book books the front Party into the Room (D98) and sends it there. "" if booked, else why not.
func (g *Game) book(room int) string {
	if why := g.fits(room); why != "" {
		return why
	}
	p := g.queue[0]
	g.queue = g.queue[1:]
	p.Room, p.Rate = room, p.rateIn(g.hotel.Rooms[room])
	p.schedule(g.clock)
	for _, guest := range p.Guests {
		guest.RoomIndex = room
	}
	g.checkIn(p)
	return ""
}

// checkIn sends the Party to its Stay Spots, one each: from the Queue, or back in from the Street after
// an outing.
func (g *Game) checkIn(p *Party) {
	p.state = checkingIn
	g.sendParty(p, g.hotel.Rooms[p.Room].Stays...)
}

// sendParty starts a leg: the leader pathfinds to goals[0], and the follower trails it on the leader's own
// waypoints (D90), joining them at its own cell when the leader's route crosses it; once the leader
// arrives, the follower peels off to its own goal.
func (g *Game) sendParty(p *Party, goals ...image.Point) {
	p.goals, p.peeled = goals, false
	lead := p.Guests[0]
	g.send(p, lead, goals[0])
	for _, f := range p.Guests[1:] {
		at := image.Pt(int(f.X), int(f.Y))
		if i := slices.IndexFunc(lead.Path, func(w image.Point) bool { return w.Div(world.TileSize) == at.Div(world.TileSize) }); i >= 0 {
			f.SetPath(slices.Clone(lead.Path[i:]))
		} else {
			f.SetPath(append(g.hotel.Path(at, image.Pt(int(lead.X), int(lead.Y)), p.Room), lead.Path...))
		}
	}
}

// walkParty advances the Party one frame; true once every member has arrived. Going out, the leader
// waits outside the door while its follower is still in the Room (D90); the follower waits while it is
// within a tile of its leader, unless that leader is waiting for it. Nobody else waits on a mate: that could close a loop with a guest
// that cut in between them.
func (g *Game) walkParty(p *Party, speed float64) bool {
	lead := p.Guests[0]
	dist := func(f *entities.Guest) float64 { return math.Hypot(f.X-lead.X, f.Y-lead.Y) }
	in := func(guest *entities.Guest) bool {
		return p.Room >= 0 && image.Pt(int(guest.X), int(guest.Y)).In(g.hotel.Rooms[p.Room].Shell())
	}
	waits := !in(lead) && slices.ContainsFunc(p.Guests[1:], in)
	if len(lead.Path) > 0 && !waits {
		g.walk(lead, speed)
	}
	if len(lead.Path) == 0 && !p.peeled {
		p.peeled = true
		for i, f := range p.Guests[1:] {
			g.send(p, f, p.goals[min(i+1, len(p.goals)-1)])
		}
	}
	arrived := len(lead.Path) == 0
	for _, f := range p.Guests[1:] {
		if p.peeled || waits || dist(f) >= trailGap {
			g.walk(f, speed)
		}
		arrived = arrived && len(f.Path) == 0
	}
	return arrived
}

// outside: every member is out of the Room's shell.
func (p *Party) outside(r world.Room) bool {
	return !slices.ContainsFunc(p.Guests, func(guest *entities.Guest) bool {
		return image.Pt(int(guest.X), int(guest.Y)).In(r.Shell())
	})
}

// send gives a guest a path to `to`, around the guests standing in the Queue and the Concierge where it
// stands now (at the Terminal, while booking); straight through them if they wall it off.
func (g *Game) send(p *Party, guest *entities.Guest, to image.Point) {
	held := []image.Point{image.Pt(int(g.player.X), int(g.player.Y))}
	for _, q := range g.queue {
		if q != p {
			for _, o := range q.Guests {
				held = append(held, image.Pt(int(o.X), int(o.Y)))
			}
			held = append(held, g.hotel.Queue[q.slot])
			if len(q.Guests) > 1 {
				held = append(held, g.hotel.Queue[q.slot].Add(image.Pt(0, world.TileSize)))
			}
		}
	}
	from := image.Pt(int(guest.X), int(guest.Y))
	path := g.hotel.Path(from, to, p.Room, held...)
	if path == nil {
		path = g.hotel.Path(from, to, p.Room)
	}
	guest.SetPath(path)
}

func (g *Game) updateParties() {
	kept := g.parties[:0]
	for _, p := range g.parties {
		if g.updateParty(p) {
			kept = append(kept, p)
		}
	}
	clear(g.parties[len(kept):])
	g.parties = kept
}

// updateParty runs one frame of a Party's stay: Street → Queue → Room (and out to the Street by day, a
// Tourist) → (Checkout at the door) → Street.
// false once it is gone.
func (g *Game) updateParty(p *Party) bool {
	exit := []image.Point{g.hotel.Exit, g.hotel.Exit}
	switch p.state {
	case queuing:
		i := slices.Index(g.queue, p)
		if g.clock >= p.patience { // gives up and walks out (D79)
			g.queue = slices.Delete(g.queue, i, i+1)
			p.state = leaving
			g.sendParty(p, exit...)
			return true
		}
		g.walkParty(p, arriveSpeed)
	case checkingIn:
		if g.walkParty(p, arriveSpeed) {
			p.state, p.settled = staying, true
		}
	case staying:
		switch {
		case g.clock >= p.Checkout:
			p.state = checkingOut
			g.sendParty(p, exit...)
		case p.Out(g.clock): // a Tourist's day out (D88)
			p.state = goingOut
			g.sendParty(p, exit...)
		}
	case goingOut:
		if g.walkParty(p, departSpeed) {
			p.state = out
		}
	case out:
		if !p.Out(g.clock) && g.spawnClear() { // back from the Street; the Room stayed Occupied meanwhile (D91)
			for _, guest := range p.Guests {
				guest.X, guest.Y = float64(g.hotel.Spawn.X), float64(g.hotel.Spawn.Y)
			}
			g.checkIn(p)
		}
	case checkingOut:
		g.walkParty(p, departSpeed)
		if r := g.hotel.Rooms[p.Room]; p.outside(r) {
			g.checkout(p, r)
		}
	case leaving:
		if g.walkParty(p, departSpeed) {
			return false
		}
	}
	return true
}

// checkout banks the Party's payout as it steps out of its door; the Room is Vacant at once (D87).
func (g *Game) checkout(p *Party, r world.Room) {
	p.Paid, p.state = true, leaving
	g.money += p.Payout()
	door := r.Door.Mul(world.TileSize)
	g.floaters = append(g.floaters, floater{float64(door.X + world.TileSize/2), float64(door.Y), fmt.Sprintf("+$%d", p.Payout()), g.tick})
}
