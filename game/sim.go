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
}

type Failure struct{ Clock, Reason string }

// Result is what one Run measured (D107).
type Result struct {
	Seed       uint64
	Strategy   string
	Cheat      bool
	MoneyByDay []int // banked in each 24 h from the Run start
	Sold       int   // Room-nights: Rooms Occupied at each Midnight
	Available  int   // Rooms at each Midnight
	Lost       int   // Walk-ins that gave up queueing (Patience)
	Refused    int   // picks the Terminal refused
	Bookings   int
	Paid       int
	Pairs      int // Bookings of two
	Outings    int // Tourist days out
	Failures   []Failure
}

// sim is a Run in progress: the Game, its autopilot, and what it has seen so far.
type sim struct {
	cfg    Config
	g      *Game
	a      *autopilot
	res    Result
	booked []*Party
	seen   map[*Party]partyState // each Party's state last frame
	paid   map[*Party]bool
	lost   map[*Party]bool
	phased map[*entities.Guest]bool
	legs   map[*Party]leg
}

type leg struct {
	state      partyState
	slot, from int // from: the tick this leg began
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
		seen: map[*Party]partyState{}, paid: map[*Party]bool{}, lost: map[*Party]bool{},
		phased: map[*entities.Guest]bool{}, legs: map[*Party]leg{},
		res: Result{Seed: cfg.Seed, Strategy: cfg.Name, MoneyByDay: make([]int, cfg.Days)}}
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
	var eligible []int // the Rooms the coming Midnight may roll
	vacant := 0
	if g.clock%dayFrames == dayFrames-1 {
		for i := range g.hotel.Rooms {
			if g.flips(i) {
				eligible = append(eligible, i)
			}
			if g.vacant(i) {
				vacant++
			}
		}
	}
	clock, money, title := g.clock, g.money, g.state == StateTitle
	g.Update()
	if title {
		return true
	}
	if clock/dayFrames != g.clock/dayFrames { // Midnight
		for i := range g.hotel.Rooms {
			if !g.vacant(i) {
				s.res.Sold++
			}
		}
		s.res.Available += len(g.hotel.Rooms)
		if !s.cfg.NoCheck {
			if !slices.Equal(g.spinRooms, eligible) && (g.spinning() || len(eligible) > 0) {
				s.fail("Midnight rolled Rooms %v, want the eligible %v", g.spinRooms, eligible)
			}
			if day(clock) == 1 && vacant == 0 {
				s.fail("every Room was booked by the first Midnight (rampDays)")
			}
		}
	}
	banked := 0
	for _, p := range g.parties {
		if p.Room >= 0 && !slices.Contains(s.booked, p) {
			s.booked = append(s.booked, p)
			s.res.Bookings++
			if len(p.Guests) == 2 {
				s.res.Pairs++
			}
		}
		if p.Room < 0 && p.state == leaving && !s.lost[p] {
			s.lost[p] = true
			s.res.Lost++
		}
		if p.Paid && !s.paid[p] {
			s.paid[p], banked = true, banked+p.Payout()
			s.res.Paid++
			if !s.cfg.NoCheck && day(g.clock) != day(p.Checkout) {
				s.fail("a %d-night Booking paid on %s, its checkout is %s", p.Nights, clockText(g.clock), clockText(p.Checkout))
			}
		}
		if prev, ok := s.seen[p]; ok && p.state == out && prev != out {
			s.res.Outings++
		}
		s.seen[p] = p.state
	}
	if d := (g.clock - runStart) / dayFrames; d < len(s.res.MoneyByDay) {
		s.res.MoneyByDay[d] += g.money - money
	}
	s.res.Refused = s.a.pilot.Refused
	if !s.cfg.NoCheck {
		s.check(money, banked)
	}
	return len(s.res.Failures) == 0
}

// check asserts D108's per-frame invariants: money moves only on a Checkout, no guest starts phasing
// while the Concierge stands at the computer, no leg is walked over legBudget.
func (s *sim) check(money, banked int) {
	const legBudget = 60 * tps // the longest walk, Street to the farthest Stay Spot on 3×3, is ~25 s
	g := s.g
	if g.money-money != banked {
		s.fail("money went up %d, but Checkouts paid %d", g.money-money, banked)
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
		if !ok || l.state != p.state || l.slot != p.slot {
			s.legs[p] = leg{p.state, p.slot, g.tick}
			continue
		}
		walking := p.state != staying && slices.ContainsFunc(p.Guests, func(guest *entities.Guest) bool { return len(guest.Path) > 0 })
		if walking && g.tick-l.from > legBudget {
			s.fail("a Party (state %d, room %d) has walked one leg for %d frames: %s", p.state, p.Room, g.tick-l.from, where(p.Guests))
		}
	}
}

// finish: a Booking past its checkout by 2 h that never paid fails the Run.
func (s *sim) finish() Result {
	if !s.cfg.NoCheck && len(s.res.Failures) == 0 {
		for _, p := range s.booked {
			if !p.Paid && s.g.clock > p.Checkout+2*hourFrames {
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
