package game

import (
	"math"
	"slices"

	"ghosthotel/world"
)

// ritualDirt: the spiritual dirt a looped ritual puts back in the Room (dev, -ritual).
const ritualDirt = 12

// DevRitual is main's -ritual: Room 101 open and owned with spiritual dirt in it, the Concierge just inside its
// door, in the ghost dimension, and a Father walking in to do his ritual over and over: "k", "m" (for comparing),
// or "kpace" (Father K on his retainer with no dirt anywhere: he paces the corridors, the Room stays clean).
// Looped Fathers are on a retainer nobody pays, and each ritual's end puts the dirt back (priest.go's loop hook),
// so he goes straight on to the next one. The chant keeps its real 30 game minutes: 300 frames, 5 s.
func (g *Game) DevRitual(who string) {
	room := slices.IndexFunc(g.hotel.Rooms, func(r world.Room) bool { return r.Number == "101" })
	g.unlock(Mirror)
	g.openRoom(room)
	g.ghost = true
	if who == "kpace" {
		g.retainK()
	} else {
		g.sendPriest(room, who == "m")
		g.priests[len(g.priests)-1].loop = true
		g.ritualDirt(room)
	}
	pr := g.priests[len(g.priests)-1]
	pr.retained, pr.due = true, math.MaxInt
	at := g.workSpot(room)
	g.player.X, g.player.Y = float64(at.X), float64(at.Y)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}

// ritualDirt: the Room's dirt back to ritualDirt, all of it spiritual.
func (g *Game) ritualDirt(room int) {
	g.setDirtLevel(room, 0)
	g.spots(room)
	g.ghostDirt(room, ritualDirt)
}
