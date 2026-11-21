package game

import (
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 W5-STREET: passers-by in both Dimensions, with gossip and the bonus walk-in.
// Placeholders: a few at a time, an unhurried stroll, a line and a turn-in now and then.
const (
	maxPassers   = 3   // per Dimension, at once
	spawnOdds    = 240 // 1 in this a frame, per Dimension, while there is room
	passSpeed    = 0.6 // world px a frame
	nearHall     = 3   // tiles from the Entrance's line: near enough to say a line
	sayOdds      = 2   // 1 in this passers-by say one as they pass the Entrance
	gossipFrames = 300 // a line stays this long
	turnInOdds   = 8   // 1 in this living passers-by crossing the Entrance turns in as a walk-in
)

// passer is one of the Street's passers-by: a living one (real) or a ghost (ghost), never both.
type passer struct {
	entities.Guest
	ghost   bool
	dir     float64   // +1 east, -1 west
	dy      int       // its lane: px from the Entrance's line (Spawn.Y), so a Growth moves it with the Street
	passed  bool      // it reached the Entrance's stretch: its line is rolled
	rolled  bool      // it reached the Entrance: its turn-in is rolled
	tint    []float32 // a ghost's flat color, by kind
	say     string
	sayLeft int
}

// streetState: the passers-by, the weather and the storm. Visual and apart from the sim's streams.
type streetState struct {
	rng        *rand.Rand // its own stream: the sim's never move for it
	walkers    []*passer
	buf        []actor
	n          int // frames of street time
	force      int // dev: this weather every day (0 none)
	stormUntil int // the clock the Storm jar's storm ends at
	flash      float64
	nextFlash  int
	over       func(s *ebiten.Image, x, y int) // the overlay's draw, made once
	px         *ebiten.Image
	puffs      [3]*ebiten.Image        // fog's blobs, made once
	op         ebiten.DrawImageOptions // the weather's strokes
}

// gossipPool: the lines a living passer-by picks from at this questline stage: the stage's, and any.
func gossipPool(stage int, macguffin bool) []string {
	var lines []string
	switch {
	case macguffin:
		lines = gossipAfter
	case stage >= 4:
		lines = gossipLate
	case stage >= 1:
		lines = gossipEarly
	default:
		lines = gossipBefore
	}
	return append(slices.Clone(lines), gossipAny...)
}

// ghostGossip: a ghost's lines, after the finale also the one about the MacGuffin's Room (if it has one).
func (g *Game) ghostGossip() []string {
	lines := slices.Clone(gossipGhost)
	if g.questRec.MacGuffin {
		for _, p := range g.parties {
			for _, gu := range p.Guests {
				if gu.Who.Key == "macguffin" {
					return append(lines, strings.Replace(gossipGhostAfter, "{n}", g.hotel.Rooms[p.Room].Number, 1))
				}
			}
		}
	}
	return lines
}

// bookedRooms: the Rooms with a human Party in them.
func (g *Game) bookedRooms() (n int) {
	for i := range g.hotel.Rooms {
		if !g.vacant(i, Human) {
			n++
		}
	}
	return
}

// passerTurnsIn: a living passer-by turns in as a walk-in, into the Room it fits, if one is Vacant and the
// Day's demand allows it (maybeWalkIn's rules).
func (g *Game) passerTurnsIn() bool {
	return g.bookedRooms() < g.demand() && g.freeRoom(1) >= 0 && g.spawnClear() && g.arrive(-1) != nil
}

// spawnPasser puts a passer-by at one end of the Street, in one of its three lanes.
func (g *Game) spawnPasser(ghost bool) {
	s := &g.street
	mw, _ := g.mapSize()
	p := &passer{ghost: ghost, dir: float64(1 - 2*s.rng.IntN(2))}
	x := -world.TileSize / 2
	if p.dir < 0 {
		x = mw + world.TileSize/2
	}
	at := g.hotel.Spawn
	p.dy = (s.rng.IntN(3) - 1) * world.TileSize
	at.X, at.Y = x, at.Y+p.dy
	if ghost { // like the opening's: a Shadow Person's body, hued by a kind
		p.Guest = entities.NewGuest(at, shadowCast[s.rng.IntN(len(shadowCast))])
		p.Phantasm = 1 + s.rng.IntN(11)
		h := kindHue(kindOf(p.Phantasm))
		p.tint = h[:]
	} else { // the guests' sheets and random looks, from the street's own stream (dressed() draws g.chat)
		p.Guest = entities.NewGuest(at, guestCast[s.rng.IntN(len(guestCast))]+"#"+strconv.FormatUint(s.rng.Uint64(), 10))
	}
	s.walkers = append(s.walkers, p)
}

// onStreet: the Concierge is out on the Street (below the Entrance's door line), not in the hotel.
func (g *Game) onStreet() bool {
	return g.player.Y >= float64(g.hotel.Spawn.Y-3*world.TileSize/2)
}

// updateStreet runs each step of world time.
func (g *Game) updateStreet() {
	s := &g.street
	if g.intro != nil {
		return
	}
	if s.rng == nil {
		s.rng = rand.New(rand.NewPCG(5, 8))
	}
	s.n++
	g.updateWeather()
	var count [2]int
	for _, p := range s.walkers {
		if p.ghost {
			count[1]++
		} else {
			count[0]++
		}
	}
	for i := range count {
		if count[i] < maxPassers && s.rng.IntN(spawnOdds) == 0 {
			g.spawnPasser(i == 1)
		}
	}
	mw, _ := g.mapSize()
	sx := float64(g.hotel.Spawn.X)
	kept := s.walkers[:0]
	out := g.onStreet() // the Street is heard only from the Street
	talking := ""
	for _, p := range s.walkers {
		if out && p.sayLeft > 0 {
			talking = p.say
		} else {
			p.sayLeft = 0
		}
	}
	for _, p := range s.walkers {
		p.X += p.dir * passSpeed * g.dt
		p.Y = float64(g.hotel.Spawn.Y + p.dy) // read from the current hotel: a Growth moves the Street
		if !g.headless {
			p.Anim.Update(p.X, p.Y, g.dt)
		}
		if p.sayLeft > 0 {
			p.sayLeft--
		}
		if !p.passed && (p.X-sx)*p.dir > -nearHall*world.TileSize {
			p.passed = true                                       // reached the Entrance's stretch: roll its line
			if s.rng.IntN(sayOdds) == 0 && out && talking == "" { // one speaker at a time, only with the Concierge out
				pool := gossipPool(g.questStage(), g.questRec.MacGuffin)
				if p.ghost {
					pool = g.ghostGossip()
				}
				p.say, p.sayLeft = pool[s.rng.IntN(len(pool))], gossipFrames
				talking = p.say
			}
		}
		if !p.ghost && !p.rolled && (p.X-sx)*p.dir >= 0 { // at the Entrance: turn in, or walk on
			p.rolled = true
			if s.rng.IntN(turnInOdds) == 0 && g.passerTurnsIn() {
				continue
			}
		}
		if p.X > -world.TileSize && p.X < float64(mw+world.TileSize) {
			kept = append(kept, p)
		}
	}
	clear(s.walkers[len(kept):])
	s.walkers = kept
}

// streetActors: the passers-by of the Dimension shown, and an overlay drawn last: their balloons and the weather.
func (g *Game) streetActors() []actor {
	s := &g.street
	if g.intro != nil {
		return nil
	}
	s.buf = s.buf[:0]
	for _, p := range s.walkers {
		if p.ghost != g.ghost {
			continue
		}
		a := actor{x: p.X, y: p.Y, name: p.Character, anim: p.Anim}
		if p.ghost {
			a.tint, a.a, a.mode, a.up = p.tint, ghostAlpha, modeFlat, true
		}
		s.buf = append(s.buf, a)
	}
	if s.over == nil {
		s.over = g.drawStreetOver
	}
	_, mh := g.mapSize()
	return append(s.buf, actor{y: float64(mh + 1000), draw: s.over}) // below everything: over everything
}

// drawStreetOver: the weather over the Street and the windows, then the line each passer-by is saying. x,y are
// the overlay's foot box center in screen px, which gives the view's top left.
func (g *Game) drawStreetOver(dst *ebiten.Image, x, y int) {
	_, mh := g.mapSize()
	ox, oy := -x, screenPx(float64(mh+1000))-y
	if !g.ghost {
		g.drawWeather(dst, ox, oy)
	}
	for _, p := range g.street.walkers {
		if p.ghost == g.ghost && p.sayLeft > 0 && g.onStreet() {
			g.sayAt(dst, p.say, &p.Guest, ox, oy)
		}
	}
}

// DevStreet: the Concierge on the Street at the Entrance, passers-by of each Dimension about, two of them with
// a line showing (with -shot -at a few frames; -ghost for the ghost ones).
func (g *Game) DevStreet() {
	s := &g.street
	if s.rng == nil {
		s.rng = rand.New(rand.NewPCG(5, 8))
	}
	at := g.hotel.Spawn
	g.player.X, g.player.Y = float64(at.X), float64(at.Y+world.TileSize)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
	s.walkers = nil
	for i := range 6 {
		g.spawnPasser(i%2 == 1)
		p := s.walkers[i]
		p.dir, p.passed, p.rolled = float64(1-2*(i/2%2)), true, true
		p.dy = (i/2 - 1) * world.TileSize
		p.X, p.Y = float64(at.X+(i/2-1)*3*world.TileSize), float64(at.Y+p.dy)
		if i == 2 || i == 3 {
			pool := gossipPool(0, false)
			if p.ghost {
				pool = g.ghostGossip()
			}
			p.say, p.sayLeft = pool[i%len(pool)], 1<<30
		}
	}
}
