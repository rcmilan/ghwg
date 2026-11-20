package game

import (
	"image"
	"iter"
	"math"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/world"
)

const (
	rampDays         = 3 // Day d lets in Guests for up to d/rampDays of the Rooms (at least 2): a bigger hotel fills gradually
	maxNights        = 3
	checkoutHour     = 10 // checkout is 10:00–12:00 on the final day (D88)
	checkoutJitter   = 120 * minuteFrames
	outHour          = 8 // a Tourist goes out at 08:00–10:00 (D88)
	outJitter        = 120 * minuteFrames
	backHour         = 19 // and comes back at 19:00–22:00
	backJitter       = 180 * minuteFrames
	hotelDayStart    = 6              // a Booking made before 06:00 belongs to the night before
	pairOdds         = 0.4            // a Walk-in is a pair 40% of the time, else solo (D77)
	trailGap         = world.TileSize // a follower stays about this far behind its leader (D90)
	floaterFrames    = 90
	floaterRisePerFr = 0.25 // world px
)

type partyState int

const (
	checkingIn  partyState = iota // walking from the Street to its Stay Spots, on arrival or back from an outing
	staying                       // in the Room until Checkout
	goingOut                      // a Tourist walking out of the Entrance for the day
	out                           // a Tourist away from the hotel: off the map
	checkingOut                   // walking out of the Room; pays as it steps out of it
	leaving                       // walking out of the Entrance for good, paid
	complaining                   // waiting in the hall for its dirty Room to be cleaned (dirt.go)
	cooling                       // out in the Corridor at its door to calm down, the first time Fear hits the limit (Step 2)
	moving                        // talked into another Room (X2, match.go): out of its old Room's door, then checkingIn to the new one
)

// Step 2 (Live test 5 grill, 2026-11-08): a Party pays by the hour while in its Room, nothing out of it,
// and tips at a finished Checkout; Fear at the limit makes it step out once, then flee. Placeholders.
const (
	rentHours   = 24 // game hours in the Room that pay one night's Rate
	coolMinutes = 30 // game minutes a Party cools down at its door
	coolFear    = 2  // Fear is divided by this when it steps out: a second limit comes sooner
)

type routine int

const (
	homebody routine = iota // stays in the Room
	tourist                 // out by day, every day but the checkout day
)

func (r routine) String() string { return [...]string{"HOMEBODY", "TOURIST"}[r] }

// Booking is a Party's stay, made by its app before it arrives (D139), with the Rate locked then (D53).
type Booking struct {
	Room     int
	Nights   int
	Routine  routine
	Rate     int
	Checkout int // clock frame the Party leaves its Room for good
	Paid     bool
	Realm    Realm // the layer it lives in (realm.go): Human today
	Fled     bool  // ran from a haunt (D146): never pays
	Walked   bool  // walked away from a dirty Room (dirt.go): never pays
	// The Party's own times of day, in frames (D88's jitter), and a Tourist's first day out.
	outAt, backAt, checkoutAt, firstOut int
}

// schedule fixes the stay's days once booked at clock c (D88): checkout on the final hotel day (before
// 06:00 counts as the night before), and a Tourist's first day out: the day after it arrives, so a Room pays
// from its first hour and the Mirror is within reach on Day 1 (2026-11-11; it was today if booked before 10:00).
func (b *Booking) schedule(c int) {
	b.Checkout = ((c-hotelDayStart*hourFrames)/dayFrames+b.Nights)*dayFrames + b.checkoutAt
	b.firstOut = c/dayFrames + 1
}

// Out: a Tourist is away at clock c (D88), between its out and back times on every day of its routine
// but the checkout day.
func (b Booking) Out(c int) bool {
	d, t := c/dayFrames, c%dayFrames
	return b.Routine == tourist && d >= b.firstOut && d < b.Checkout/dayFrames && t >= b.outAt && t < b.backAt
}

// Payout is the tip the Party banks at Checkout (Z1, 2026-11-13): one night's Rate, whatever the stay's
// length, times pay, the Room's Mood pay factor (moodPay).
func (p *Party) Payout(pay Num) int { return int(Num(p.Rate) * pay) }

// charge runs each frame of a stay (Z1, 2026-11-13): rent is paid up front, per Guest, on the Guest's own
// clock. The moment a Guest steps into its Room it pays its share of the hour (Rate/rentHours, split between
// the Party, times the Room's Mood pay factor as it stands, and badRent in a bad pair) as a "+$N" at the door, then again every game
// hour from that entry, in the Room or out of it. Whole dollars; a Guest's cents carry to its next hour.
// The loop pays every hour the Clock passed, so T held skips none.
func (g *Game) charge(p *Party) {
	if p.due == nil {
		p.due, p.cents = make([]int, len(p.Guests)), make([]Num, len(p.Guests))
	}
	r := g.hotel.Rooms[p.Room]
	for i, guest := range p.Guests {
		if p.due[i] == 0 {
			if !image.Pt(int(guest.X), int(guest.Y)).In(r.Shell()) {
				continue
			}
			p.due[i] = g.clock
		}
		for ; g.clock >= p.due[i]; p.due[i] += hourFrames {
			pay := g.moodPay(p.Room)
			if g.pairing(p.Room) == badPair { // a bad pair pays less (X2, live test 1)
				pay *= badRent
			}
			p.cents[i] += Num(p.Rate) * Num(g.upMul(LobbyDecor)) / rentHours / Num(len(p.Guests)) * pay
			if n := Num(int(p.cents[i])); n > 0 {
				p.cents[i] -= n
				p.rent += n
				g.earn(Money, n, r.Door.Mul(world.TileSize).Add(image.Pt(world.TileSize/2+(2*i-len(p.Guests)+1)*6, 0)))
			}
		}
	}
}

// stepOut: Fear hit the limit for the first time: each Guest walks off on its own to a random Corridor
// light, Fear cut by coolFear, and comes back after coolMinutes (Step 2; Live test 9, 2026-11-08: a pair
// only arrives and leaves together); the second time it flees.
func (g *Game) stepOut(p *Party) {
	p.state, p.cooledAt = cooling, g.clock
	lights := g.rng.Perm(len(g.hotel.Lights))
	for i, guest := range p.Guests {
		guest.Fear /= coolFear
		p.talk = append(p.talk, line{i, "I need air!", g.tick})
		g.send(guest, g.hotel.Lights[lights[i%len(lights)]])
	}
}

// Occupies: the Room is Occupied in the realm from the Party's arrival until it pays at its door (D91) or flees.
func (b Booking) Occupies(room int, realm Realm) bool {
	return b.Room == room && b.Realm == realm && !b.Paid && !b.Fled && !b.Walked
}

// Party is 1–2 Guests on one Booking (D76): Guests[0] leads, the other trails it (D90).
type Party struct {
	Guests     []*entities.Guest
	state      partyState
	goals      []image.Point // where each member is headed this leg
	peeled     bool          // this leg's leader has arrived and the follower heads for its own goal
	settled    bool          // it has reached its Stay Spots once: from then on its Room stays fogged (S6 review)
	abed       bool          // its Guests are in bed or on their way (D167)
	acts       []act         // what each Guest is doing in the Room by day (D168)
	ectoShown  int           // Ecto gained since its Room's last "+N" floater
	revealed   bool          // its quirk (and Nerve) is known: a REVEAL was paid (T375)
	asked      int           // the clock it asked for service, 0 when not asking (D163)
	want       Unlock        // the supply it asked for (supply.go)
	askEnded   int           // the clock its last request was served, expired or lapsed
	fledAt     int           // the tick it fled (D164)
	scares     int           // times Fear hit the limit: the first steps out, the second flees (Step 2)
	cooledAt   int           // the clock it stepped out
	rent       Num           // Money paid in the Room so far (Step 2)
	due        []int         // per Guest: the clock its next hour of rent is due, 0 until it first steps into the Room
	cents      []Num         // per Guest: the cents of its share not yet paid
	tip        Num           // the checkout tip it banked
	greet      greeting      // owed once the Concierge hears it (D165)
	welcomed   bool          // a housekeeper has welcomed it
	talk       []line        // its lines showing or about to (D165)
	snubbed    int           // the clock until which, having refused an offer, it won't be talked to (X2, talkmove.go)
	arrived    int           // the clock it walked in
	mess       int           // dirt its Guests leave a night (dirt.go)
	complainAt int           // the clock it started waiting in the hall (dirt.go)
	Booking
}

// floater rises and fades: a "+$X" over a door after a Checkout (D87), or a ghost-only "+N" of Ecto.
type floater struct {
	x, y float64 // world px
	text string
	at   int // tick
	kind floaterKind
}

type floaterKind int

const (
	floatMoney floaterKind = iota
	floatEcto
)

// guests: everyone on the map. A member walking out of the Entrance is gone once it reaches the Exit,
// so it never blocks the Street while its mate catches up.
func (g *Game) guests() []*entities.Guest {
	return slices.AppendSeq(slices.Collect(g.guestsIn(Human)), g.guestsIn(Ghost))
}

// guestsIn: the guests of one realm on the map, found as the loop asks: no slice (it ran per walker per
// frame). A loop that changes Paths or Party states while it runs wants slices.Collect.
func (g *Game) guestsIn(realm Realm) iter.Seq[*entities.Guest] {
	return func(yield func(*entities.Guest) bool) {
		for _, p := range g.parties {
			for _, guest := range p.Guests {
				if p.Realm == realm && p.shown(guest) {
					if !yield(guest) {
						return
					}
				}
			}
		}
	}
}

// vacant: no Party of the realm occupies the Room.
func (g *Game) vacant(room int, realm Realm) bool {
	for _, p := range g.parties {
		if p.Occupies(room, realm) {
			return false
		}
	}
	return true
}

// demand: the Parties the Day lets in (rampDays), so the first nights never fill
// the hotel.
func (g *Game) demand() int {
	n := len(g.hotel.Rooms)
	if g.busy {
		return n
	}
	return min(n, max(2, n*day(g.clock)/rampDays))
}

// maybeWalkIn lets in a Party each time a Room's booked arrival is due (dirt.go, 2026-11-08: bookings
// replace D77's random Walk-ins), unless the Day's demand is met or the Room isn't Vacant and open (D140).
// The Party takes the Room it fits best (X2, fitRoom); the Room left takes over that one's booking.
func (g *Game) maybeWalkIn() {
	booked := 0
	for i := range g.hotel.Rooms {
		if !g.vacant(i, Human) {
			booked++
		}
	}
	for i, a := range g.arrivals {
		if a.at > 0 && g.clock >= a.at && booked < g.demand() && g.isOwned(i) && g.vacant(i, Human) && !g.closed(i) && !g.changing(i) && g.spawnClear() {
			if p := g.arrive(-1); p != nil {
				g.arrivals[i], g.arrivals[p.Room] = g.arrivals[p.Room], arrival{}
			}
			booked++
		}
	}
}

// spawnClear: nobody stands within a tile of Spawn, so a Party appearing there can't land on another one
// (a follower still waiting for its leader to pull ahead).
func (g *Game) spawnClear() bool { return g.spawnClearIn(Human) }

// spawnClearIn: spawnClear among the realm's own, which a Party of the other never blocks.
func (g *Game) spawnClearIn(realm Realm) bool {
	for guest := range g.guestsIn(realm) {
		if math.Hypot(guest.X-float64(g.hotel.Spawn.X), guest.Y-float64(g.hotel.Spawn.Y)) < world.TileSize {
			return false
		}
	}
	return true
}

// freeRoom: the first Vacant Room, not Closed, with a Stay Spot for each of size Guests, or -1.
func (g *Game) freeRoom(size int) int {
	for i, r := range g.hotel.Rooms {
		if g.isOwned(i) && g.vacant(i, Human) && !g.closed(i) && !g.changing(i) && len(r.Stays) >= size {
			return i
		}
	}
	return -1
}

// walkIn puts a new Party on the Street, already booked (D139) into the Room it fits best (X2, fitRoom), and
// sends it straight to its Stay Spots. A pair with no 2-person Room free comes solo. nil if no Room is Vacant.
func (g *Game) walkIn() *Party { return g.arrive(-1) }

// arrive: walkIn into the given Room (tests and dev starts), or into the one it fits best (-1). A pair only
// fits a given Room with 2 Stay Spots. Each Party rolls its Nerve (X2).
func (g *Game) arrive(room int) *Party {
	given := room >= 0
	if !given && g.freeRoom(1) < 0 {
		return nil
	}
	n := nerveMin + g.rng.IntN(nerveMax-nerveMin+1)
	guest := entities.NewGuest(g.hotel.Spawn, g.dressed(guestCast[g.rng.IntN(len(guestCast))]))
	p := &Party{Guests: []*entities.Guest{&guest},
		Booking: Booking{Nights: 1 + g.rng.IntN(maxNights), Routine: routine(g.rng.IntN(2)),
			outAt:      outHour*hourFrames + g.rng.IntN(outJitter),
			backAt:     backHour*hourFrames + g.rng.IntN(backJitter),
			checkoutAt: checkoutHour*hourFrames + g.rng.IntN(checkoutJitter)}}
	if g.rng.Float64() < pairOdds {
		r := g.fitRoom(n, 2)
		if given {
			r = -1
			if len(g.hotel.Rooms[room].Stays) >= 2 {
				r = room
			}
		}
		if r >= 0 {
			mate := entities.NewGuest(g.hotel.Spawn, g.dressed(guestCast[g.rng.IntN(len(guestCast))]))
			mate.Leader = &guest
			p.Guests, room = append(p.Guests, &mate), r
		}
	}
	if room < 0 {
		room = g.fitRoom(n, 1)
	}
	setNerve(p, n)
	for _, guest := range p.Guests {
		g.rollHuman(guest)
	}
	g.walkIns++
	p.Room, p.Rate, p.arrived = room, g.hotel.Rooms[room].Type.Rate, g.clock
	for range p.Guests {
		p.mess += g.mess()
	}
	p.schedule(g.clock)
	for _, guest := range p.Guests {
		guest.RoomIndex = room
	}
	g.parties = append(g.parties, p)
	g.checkIn(p)
	return p
}

// checkIn sends the Party to its Stay Spots, one each: from the Street, on arrival or after an outing.
func (g *Game) checkIn(p *Party) {
	p.state, p.greet = checkingIn, greetHello
	if p.settled {
		p.greet = greetBack
	}
	g.sendParty(p, g.hotel.Rooms[p.Room].Stays...)
}

// sendParty starts a leg: the leader pathfinds to goals[0], and the follower trails it on the leader's own
// waypoints (D90), joining them at its own cell when the leader's route crosses it; once the leader
// arrives, the follower peels off to its own goal.
func (g *Game) sendParty(p *Party, goals ...image.Point) {
	p.goals, p.peeled = goals, false
	lead := p.Guests[0]
	g.send(lead, goals[0])
	for _, f := range p.Guests[1:] {
		at := image.Pt(int(f.X), int(f.Y))
		if i := slices.IndexFunc(lead.Path, func(w image.Point) bool { return g.hotel.Map.Cell(w) == g.hotel.Map.Cell(at) }); i >= 0 {
			f.SetPath(slices.Clone(lead.Path[i:]))
		} else {
			to := image.Pt(int(lead.X), int(lead.Y))
			if _, abed := g.bedAt(lead.X, lead.Y); abed && len(lead.Path) > 0 { // a bed is solid: join its side instead (D167)
				to = lead.Path[0]
			}
			f.SetPath(append(g.route(f, to), lead.Path...))
		}
	}
}

// walkParty advances the Party one frame; true once every member has arrived. The follower waits while it
// is within a tile of its leader (D90), who never waits for it: a pause outside the door read as a stall
// (Live test 6, 2026-11-08). Nobody else waits on a mate: that could close a loop with a guest that cut
// in between them.
func (g *Game) walkParty(p *Party, speed float64) bool {
	lead := p.Guests[0]
	dist := func(f *entities.Guest) float64 { return math.Hypot(f.X-lead.X, f.Y-lead.Y) }
	if len(lead.Path) > 0 {
		g.walk(lead, speed)
	}
	if len(lead.Path) == 0 && !p.peeled {
		p.peeled = true
		for i, f := range p.Guests[1:] {
			g.send(f, p.goals[min(i+1, len(p.goals)-1)])
		}
	}
	arrived := len(lead.Path) == 0
	for _, f := range p.Guests[1:] {
		if p.peeled || dist(f) >= trailGap {
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

// send gives a guest a path to `to`, around the Concierge where it stands now, the Guests waiting in
// the hall and any `avoid` cells; straight through if they wall it off.
func (g *Game) send(guest *entities.Guest, to image.Point, avoid ...image.Point) {
	if g.realmOf(guest) == g.realm() { // the Concierge of the other realm is no obstacle
		avoid = append(avoid, image.Pt(int(g.player.X), int(g.player.Y)))
	}
	for _, p := range g.parties { // and around Parties waiting in the hall (dirt.go): they stand there for hours
		for _, o := range p.Guests {
			if p.state == complaining && len(o.Path) == 0 && o != guest && !o.Mate(guest) {
				avoid = append(avoid, image.Pt(int(o.X), int(o.Y)))
			}
		}
	}
	path := g.route(guest, to, avoid...)
	if path == nil {
		path = g.route(guest, to)
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

// intrude: every intrudeEvery game minutes the Concierge spends in the Room with its Guests cost its Room intrudeMood
// (Live test 5, 2026-11-08): he may come in, lingering is rude. Only he lingers (08 B1: staff never cost Mood), and only in real. The first intrudeGrace minutes of a visit are free.
func (g *Game) intrude(p *Party) {
	if _, gs := g.inside(p.Room); g.ghost || !g.ticked || len(gs) == 0 || !g.feet().In(g.hotel.Rooms[p.Room].Shell()) {
		return
	}
	if l := &g.linger; l.room != p.Room || g.clock-l.last > 1 { // he just came in
		l.room, l.from = p.Room, g.clock
	}
	g.linger.last = g.clock
	if d := g.clock - g.linger.from - intrudeGrace*minuteFrames; d >= 0 && d%(intrudeEvery*minuteFrames) == 0 {
		g.addMood(p.Room, -intrudeMood)
	}
}

// updateParty runs one frame of a Party's stay: Street → Room (and out to the Street by day, a
// Tourist) → (Checkout at the door) → Street.
// false once it is gone.
func (g *Game) updateParty(p *Party) bool {
	if p.Realm == Ghost {
		return g.updateGhost(p)
	}
	exit := []image.Point{g.hotel.Exit, g.hotel.Exit}
	if !p.Paid && !p.Fled && !p.Walked && p.state != checkingOut {
		g.charge(p) // each Guest's hours run from its first step into the Room, in it or out
	}
	switch p.state {
	case checkingIn:
		if g.walkParty(p, arriveSpeed) && (p.settled || !g.settle(p)) {
			p.state, p.settled = staying, true
		}
	case staying:
		switch {
		case g.clock >= p.Checkout:
			p.state = checkingOut
			g.sendParty(p, exit...)
		case p.Out(g.clock): // a Tourist's day out (D88)
			p.state, p.greet = goingOut, greetOut
			g.sendParty(p, exit...)
		case g.parley != nil && g.parley.p == p: // spoken to (X2, talkmove.go): it stands and listens
		default:
			g.bedtime(p)
			g.intrude(p)
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
	case complaining:
		g.complain(p)
	case moving: // out of the old Room's door, its own until then; then in to the new one
		if g.walkParty(p, arriveSpeed) {
			for _, guest := range p.Guests {
				guest.RoomIndex = p.Room
			}
			g.checkIn(p)
			p.greet = noGreeting
		}
	case cooling: // each on its own, then back to its day or its bed (bedtime)
		for _, guest := range p.Guests {
			if len(guest.Path) > 0 {
				g.walk(guest, departSpeed)
			}
		}
		if g.clock-p.cooledAt >= coolMinutes*minuteFrames {
			p.state, p.abed = staying, false
			for i := range p.Guests {
				g.idle(p, i)
			}
		}
	case leaving:
		speed := departSpeed
		if p.Fled { // a hop in place, then a run (D164)
			if g.tick-p.fledAt <= hopFrames {
				return true
			}
			speed = fleeSpeed
		}
		if g.walkParty(p, speed) {
			return false
		}
	}
	return true
}

// checkout banks the Party's payout as it steps out of its door; the Room is Vacant at once (D87).
func (g *Game) checkout(p *Party, r world.Room) {
	p.Paid, p.state = true, leaving
	p.tip = Num(p.Payout(g.moodPay(p.Room))) * Num(g.upMul(LobbyDecor)*g.upMul(TipJar)*g.tipQuirkMul(p)) // the Room as the Party leaves it, mess aside; the Rate's decor, the Tip jar and a Big tipper
	g.leaveMess(p)
	g.book(p.Room, g.bookGapNow())
	door := r.Door.Mul(world.TileSize)
	g.earn(Money, p.tip, door.Add(image.Pt(world.TileSize/2, 0)))
}
