package game

import (
	"image"
	"math/rand/v2"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// Idle kitchen life (08 W5, circle.md): now and then, as an idle Housekeeper is sent to roam, it goes to the fridge, the
// sink or the stove instead, stands facing it and, one visit in two, says its line; sometimes it says a line
// where it stands. Never a job's delay: the visit only changes where the idle walk ends, so goRest's timing and
// its draws on g.rng are the same with or without it. The rolls come from a hash of the clock and its index
// (lifeRoll), not from any of the sim's streams. Placeholder rates.
const (
	lifeVisitOdds = 6   // 1 in n idle walks goes to a prop
	lifeLineOdds  = 2   // 1 in n visits says a line
	lifeAnyOdds   = 8   // 1 in n idle walks says kitchenAny where it ends (the visits aside)
	lifeFrames    = 300 // a line stays this long (the Street's gossipFrames)
)

// kitchenLifeOff turns the feature off, for the test that proves the sim does not notice it.
var kitchenLifeOff bool

// lifeProps: the kitchen's props a visit goes to, with their lines: the fridge and the oven are Supplies', the sink is the
// counter (kitchenDecor's drawer).
var lifeProps = [3]struct {
	supply int // index in supplies, or -1 for the sink
	line   string
}{{0, kitchenFridge}, {-1, kitchenSink}, {1, kitchenStove}}

// lifeStand: the cell centre in front of lifeProps[i], where a Housekeeper stands.
func (g *Game) lifeStand(i int) image.Point {
	if s := lifeProps[i].supply; s >= 0 {
		return g.supplyDrop(s)
	}
	return image.Pt(g.hotel.Stock.Min.X+kitchenDecor[0].X*world.TileSize+world.TileSize/2, g.hotel.Stock.Max.Y+world.TileSize/2)
}

// lifeRoll: n sides from a hash of the clock, the Housekeeper's index and a salt: its own stream, off the sim's.
func (g *Game) lifeRoll(k *housekeeper, salt uint64, n int) int {
	return rand.New(rand.NewPCG(uint64(g.clock), uint64(slices.Index(g.keepers, k))<<8|salt)).IntN(n)
}

// lifeVisit runs as an idle Housekeeper has just been sent to roam: it may be sent to a prop instead, and a line
// is picked for where it ends.
func (g *Game) lifeVisit(k *housekeeper) {
	k.lifeLine, k.lifeProp = "", false
	if kitchenLifeOff || g.roomAt(k.X, k.Y) >= 0 { // in a Room it leaves through the door: not a visit's walk
		return
	}
	switch {
	case g.lifeRoll(k, 1, lifeVisitOdds) == 0:
		i := g.lifeRoll(k, 2, len(lifeProps))
		at := g.lifeStand(i)
		g.sendKeeper(k, at, -1)
		k.lifeAt, k.lifeProp = at, true
		if g.lifeRoll(k, 3, lifeLineOdds) == 0 {
			pool := []string{lifeProps[i].line}
			if g.questRec.MacGuffin {
				pool = append(pool, kitchenAfter)
			}
			k.lifeLine = pool[g.lifeRoll(k, 4, len(pool))]
		}
	case g.lifeRoll(k, 5, lifeAnyOdds) == 0:
		k.lifeAt, k.lifeLine = image.Pt(int(k.X), int(k.Y)), kitchenAny
		if n := len(k.Path); n > 0 {
			k.lifeAt = k.Path[n-1]
		}
	}
}

// lifeSpeak runs each frame for an idle Housekeeper: at a prop it faces it; where its line was meant to
// be said, it says it, unless another of the staff is speaking. The line shows in real only (hears).
func (g *Game) lifeSpeak(k *housekeeper) {
	if !k.lifeProp && k.lifeLine == "" {
		return
	}
	if kitchenLifeOff || image.Pt(int(k.X), int(k.Y)).Div(world.TileSize) != k.lifeAt.Div(world.TileSize) {
		return
	}
	if k.lifeProp {
		k.Anim.Facing = entities.FaceUp
	}
	if k.lifeLine == "" || k.unpaid {
		return
	}
	for _, s := range g.speakers() {
		if s.talking(g.tick) {
			return // it waits its turn while it stands there
		}
	}
	// said.at is set ahead, so that talking(), which ends a line sayFrames after it, keeps it lifeFrames
	k.said = line{0, k.lifeLine, g.tick + lifeFrames - sayFrames}
	k.lifeLine = ""
}

// DevKitchenLife: -kitchenlife, one Housekeeper at prop i (0 fridge, 1 sink, 2 stove) saying its line.
func (g *Game) DevKitchenLife(i int) {
	g.DevStaff(1, 500)
	k := g.keepers[0]
	at := g.lifeStand(i)
	k.X, k.Y, k.Path, k.lifeAt, k.lifeProp, k.rest = float64(at.X), float64(at.Y), nil, at, true, g.clock+hourFrames
	k.lifeLine = lifeProps[i].line
	g.lifeSpeak(k)
}
