package game

import (
	"fmt"
	"image"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// autopilot plays a Game with a Strategy through the Pilot and a script, one frame per Update.
type autopilot struct {
	strat   Strategy
	pilot   *Pilot
	keys    *script
	started bool
	idle    bool // the Pilot was idle last frame
	minute  int  // the game minute Act last ran in
}

func (g *Game) autopilot(s Strategy, react, keyGap int) *autopilot {
	a := &autopilot{strat: s, pilot: newPilot(g.hotel, react, keyGap), keys: &script{}, minute: -1}
	g.in, g.auto = a.keys, a
	return a
}

// drive sets this frame's keys before Update reads them: SPACE on the title once, then the Pilot's.
func (a *autopilot) drive(g *Game) {
	if !a.started {
		a.started = true
		a.keys.hold(ebiten.KeySpace)
		return
	}
	if a.pilot.Idle() && a.idle && g.clock/minuteFrames == a.minute { // nothing to do: skip the View
		a.pilot.frame++
		a.keys.hold()
		return
	}
	v := g.view()
	if a.pilot.Idle() {
		a.minute = g.clock / minuteFrames
		if c, ok := a.strat.(interface{ Cheat(*Game) }); ok {
			c.Cheat(g)
		}
		a.strat.Act(v, a.pilot)
	}
	a.idle = a.pilot.Idle()
	a.keys.hold(a.pilot.step(v)...)
}

// Config is one headless Run.
type Config struct {
	Cols, Rows    int
	Days          int // 24 h each, from the Run start (Day 1, 08:00)
	Seed          uint64
	Name          string // the Strategy's name, for the report
	Strategy      Strategy
	React, KeyGap int  // the Pilot's pace (D104); 0, 0 is frame-perfect
	NoCheck       bool // skip the invariants (D108)
	Mirror        bool // start the Run with the Mirror unlocked, so ghosts come (a normal Run starts locked)
	Open          bool // start the Run with every Room owned (a normal Run starts with them boarded up, 07 B1)
}

type Failure struct{ Clock, Reason string }

// Result is what one Run measured (D107).
type Result struct {
	Seed        uint64
	Strategy    string
	Cheat       bool
	MoneyByDay  []int // banked in each 24 h from the Run start
	Sold        int   // Room-nights: Rooms Occupied at each Midnight
	Available   int   // Rooms at each Midnight
	Bookings    int
	Ghosts      int // ghost Parties booked (07 G1): they pay no Money
	Paid        int
	Pairs       int     // Bookings of two
	Outings     int     // Tourist days out
	Fled        int     // Parties that ran from a haunt
	Walked      int     // Parties that walked away from a dirty Room (dirt.go)
	Cleans      int     // dirt spots cleaned
	Ecto        int     // at the end of the Run
	Serves      int     // requests answered
	Expired     int     // requests left to expire
	Priests     int     // called and paid
	PriestSpend int     // $ paid for them
	ClosedHours int     // Room-hours spent Closed
	MirrorAt    float64 // game hours from the Run start until the Mirror unlocked; -1 if it never did (07 B1)
	Failures    []Failure
}

// sim is a Run in progress: the Game, its autopilot, and what it has seen so far.
type sim struct {
	cfg       Config
	g         *Game
	a         *autopilot
	res       Result
	booked    []*Party
	seen      map[*Party]partyState // each Party's state last frame
	paid      map[*Party]bool
	fled      map[*Party]bool
	phased    map[*entities.Guest]bool
	legs      map[*Party]leg
	called    map[*priest]bool
	shipped   map[*courier]bool
	rent      map[*Party]Num  // each Party's rent counted so far
	seenGhost map[*Party]bool // the ghost Parties counted
	closed    int             // Room-frames Closed
}

type leg struct {
	state partyState
	from  int // the tick this leg began
}

// RunSim plays one Run headless, at full speed, and measures it (D101).
func RunSim(cfg Config) Result {
	s, err := newSim(cfg)
	if err != nil {
		return Result{Seed: cfg.Seed, Strategy: cfg.Name, Failures: []Failure{{"", err.Error()}}}
	}
	for !s.done() && s.frame() {
	}
	return s.finish()
}

func newSim(cfg Config) (*sim, error) {
	g, err := newWorld(cfg.Cols, cfg.Rows, cfg.Seed)
	if err != nil {
		return nil, err
	}
	s := &sim{cfg: cfg, g: g, a: g.autopilot(cfg.Strategy, cfg.React, cfg.KeyGap),
		seen: map[*Party]partyState{}, paid: map[*Party]bool{}, fled: map[*Party]bool{},
		phased: map[*entities.Guest]bool{}, legs: map[*Party]leg{}, called: map[*priest]bool{}, shipped: map[*courier]bool{}, rent: map[*Party]Num{}, seenGhost: map[*Party]bool{},
		res: Result{Seed: cfg.Seed, Strategy: cfg.Name, MoneyByDay: make([]int, cfg.Days), MirrorAt: -1}}
	g.mirrorAtStart, g.openAtStart = cfg.Mirror, cfg.Open
	_, s.res.Cheat = cfg.Strategy.(interface{ Cheat(*Game) })
	return s, nil
}

func (s *sim) done() bool {
	return s.g.state != StateTitle && s.g.clock >= runStart+s.cfg.Days*dayFrames
}

func (s *sim) fail(format string, args ...any) {
	s.res.Failures = append(s.res.Failures, Failure{clockText(s.g.clock), fmt.Sprintf(format, args...)})
}

// frame runs one Update and counts it; false once an invariant failed (the Run stops there).
func (s *sim) frame() bool {
	g := s.g
	vacant := 0
	if g.clock%dayFrames == dayFrames-1 {
		for i := range g.hotel.Rooms {
			if g.vacant(i, Human) {
				vacant++
			}
		}
	}
	clock, money, ecto, title, scaring := g.clock, g.wallet[Money], g.wallet[Ecto], g.state == StateTitle, s.scaring()
	delivered, serves := g.delivered, g.serves
	moods, cleans := slices.Clone(g.mood), slices.Clone(g.taps)
	working := map[*priest]bool{}
	for _, pr := range g.priests {
		working[pr] = !pr.done
	}
	g.Update()
	if title {
		return true
	}
	if s.res.MirrorAt < 0 && g.unlocked(Mirror) {
		s.res.MirrorAt = float64(g.clock-runStart) / hourFrames
	}
	if clock/dayFrames != g.clock/dayFrames { // Midnight
		for i := range g.hotel.Rooms {
			if !g.vacant(i, Human) {
				s.res.Sold++
			}
		}
		s.res.Available += len(g.hotel.Rooms)
		if n := len(g.hotel.Rooms); !s.cfg.NoCheck && day(clock) == 1 && n-vacant > min(n, max(2, n/rampDays)) {
			s.fail("%d Rooms were booked by the first Midnight (rampDays)", n-vacant)
		}
	}
	cleaned := false
	for i := range g.taps {
		if g.taps[i][TaskHousekeep] > cleans[i][TaskHousekeep] {
			cleaned = true
			s.res.Cleans++
		}
	}
	spent := 0
	for _, pr := range g.priests {
		if !s.called[pr] {
			s.called[pr], spent = true, spent+pr.price()
			s.res.Priests++
		}
	}
	s.res.PriestSpend += spent
	for _, c := range g.couriers { // the rest of a checkout: what the courier carries (D200)
		if !s.shipped[c] {
			s.shipped[c], spent = true, spent+int(c.paid()[Money])
		}
	}
	for i := range g.hotel.Rooms {
		if g.closed(i) {
			s.closed++
		}
		finished := slices.ContainsFunc(g.priests, func(pr *priest) bool { return pr.RoomIndex == i && pr.done && working[pr] })
		swept := i < len(cleans) && g.taps[i][TaskHousekeep] > cleans[i][TaskHousekeep]
		if !s.cfg.NoCheck && i < len(moods) && g.mood[i] > moods[i] && !finished && !swept && g.serves == serves {
			s.fail("room %d's Mood rose %d with no Priest finishing, clean or serve there", i, g.mood[i]-moods[i])
		}
	}
	banked := g.delivered - delivered // paid at the door (E5)
	for _, p := range g.parties {
		if p.Realm == Ghost { // a ghost Party pays nothing, never flees nor complains: only counted
			if !s.seenGhost[p] {
				s.seenGhost[p] = true
				s.res.Ghosts++
			}
			continue
		}
		banked, s.rent[p] = banked+p.rent-s.rent[p], p.rent
		if !slices.Contains(s.booked, p) {
			s.booked = append(s.booked, p)
			s.res.Bookings++
			if !s.cfg.NoCheck && g.closed(p.Room) {
				s.fail("a Party booked Closed room %d (Mood %d)", p.Room, g.roomMood(p.Room))
			}
			if len(p.Guests) == 2 {
				s.res.Pairs++
			}
		}
		if p.Paid && !s.paid[p] {
			s.paid[p], banked = true, banked+p.tip
			s.res.Paid++
			if !s.cfg.NoCheck && day(g.clock) != day(p.Checkout) {
				s.fail("a %d-night Booking paid on %s, its checkout is %s", p.Nights, clockText(g.clock), clockText(p.Checkout))
			}
		}
		if (p.Fled || p.Walked) && !s.fled[p] {
			s.fled[p] = true
			if p.Fled {
				s.res.Fled++
			} else {
				s.res.Walked++
			}
		}
		if !s.cfg.NoCheck && p.Fled && p.Paid {
			s.fail("a fled Party (room %d) paid", p.Room)
		}
		if prev, ok := s.seen[p]; ok && p.state == out && prev != out {
			s.res.Outings++
		}
		s.seen[p] = p.state
	}
	if d := (g.clock - runStart) / dayFrames; d < len(s.res.MoneyByDay) {
		s.res.MoneyByDay[d] += int(g.wallet[Money] - money)
	}
	if !s.cfg.NoCheck {
		s.check(money, banked, spent)
		if g.wallet[Ecto] > ecto && !cleaned && !scaring && !s.scaring() { // before the frame: a Party scared then paid; after: one that fled
			s.fail("Ecto rose %s with no ghost clean nor a shadowed Room holding its Party", fmtNum(g.wallet[Ecto]-ecto))
		}
	}
	return len(s.res.Failures) == 0
}

// check asserts D108's per-frame invariants: money moves only by a Guest's rent or tip, a delivery, or a Priest's price, no
// guest starts phasing while the Concierge stands at the computer, no leg is walked over legBudget.
func (s *sim) check(money Num, banked Num, spent int) {
	const legBudget = 60 * tps // the longest walk, Street to the farthest Stay Spot on 3×3, is ~25 s
	g := s.g
	if d := g.wallet[Money] - money - banked + Num(spent); d > 1e-6 || d < -1e-6 {
		s.fail("money went up %s, but Guests paid %s and Priests cost %d", fmtNum(g.wallet[Money]-money), fmtNum(banked), spent)
	}
	home := image.Pt(int(g.player.X), int(g.player.Y)).Div(world.TileSize) == g.hotel.ConciergeStart.Div(world.TileSize)
	for _, guest := range g.guests() {
		if guest.Phasing && !s.phased[guest] && home {
			s.fail("a guest of room %d phased at %v with the Concierge at the Terminal",
				guest.RoomIndex, image.Pt(int(guest.X), int(guest.Y)))
		}
		s.phased[guest] = guest.Phasing
	}
	for _, p := range g.parties {
		l, ok := s.legs[p]
		if !ok || l.state != p.state {
			s.legs[p] = leg{p.state, g.tick}
			continue
		}
		walking := p.state != staying && slices.ContainsFunc(p.Guests, func(guest *entities.Guest) bool { return len(guest.Path) > 0 })
		if walking && g.tick-l.from > legBudget {
			s.fail("a Party (state %d, room %d) has walked one leg for %d frames: %s", p.state, p.Room, g.tick-l.from, where(p.Guests))
		}
	}
}

// scaring: some Room has a Shadow (D169) with its Party in the hotel, or a Corridor or the hall has one.
func (s *sim) scaring() bool {
	g := s.g
	if len(g.areaShades) > 0 { // a ghost's Shadow in a Corridor or the hall scares whoever walks by
		return true
	}
	return slices.ContainsFunc(g.parties, func(p *Party) bool {
		return !p.Paid && p.state != out && g.shaded(p.Room)
	})
}

// finish: a Booking past its checkout by 2 h that never paid nor fled fails the Run.
func (s *sim) finish() Result {
	s.res.Ecto, s.res.ClosedHours, s.res.Serves, s.res.Expired = int(s.g.wallet[Ecto]), s.closed/hourFrames, s.g.serves, s.g.expired
	if !s.cfg.NoCheck && len(s.res.Failures) == 0 {
		for _, p := range s.booked {
			if !p.Paid && !p.Fled && !p.Walked && s.g.clock > p.Checkout+2*hourFrames {
				s.fail("room %d's Booking never paid: checkout was %s", p.Room, clockText(p.Checkout))
			}
		}
	}
	return s.res
}

// where: the guests' positions, for a failure.
func where(gs []*entities.Guest) string {
	s := "guests at"
	for _, g := range gs {
		s += " " + image.Pt(int(g.X), int(g.Y)).String()
	}
	return s
}

// Play builds a windowed Game played by the named Strategy (watch mode, D112): any sim row, on screen.
func Play(name string, cols, rows int, seed uint64, react, keyGap int) (*Game, error) {
	s := Strategies[name]
	if s == nil {
		return nil, fmt.Errorf("unknown Strategy %q", name)
	}
	g, err := newWorld(cols, rows, seed)
	if err != nil {
		return nil, err
	}
	g.headless = false
	g.in = escape{g.autopilot(s, react, keyGap).keys}
	return g, g.loadImages()
}

// escape is the script with ESC from the real keyboard too, so a watched Run can still be left.
type escape struct{ *script }

func (e escape) JustPressed(k ebiten.Key) bool {
	return e.script.JustPressed(k) || k == ebiten.KeyEscape && keyboard{}.JustPressed(k)
}
