package game

import (
	"ghosthotel/world"
)

// The busy hotel (2026-11-14): a stress Run to watch everyone walk at once, and to count how often they get in
// each other's way. The hotel is grown to 3x3, every Room open with Twin beds, the Mirror owned so ghosts come
// too; Parties arrive minutes apart and are replaced minutes after they leave; three housekeepers work; the
// stock is refilled by a courier whenever a Supply runs low, at no cost. main's -busy and TestBusyHotel.
const (
	busyGap   = 5   // game minutes between arrivals, and from a Party leaving to the next
	busyStock = 3   // a Supply under this many brings a courier
	busyMood  = -40 // a Room's Mood down to this is cleaned by itself
)

// DevBusy restarts the Run as the busy hotel.
func (g *Game) DevBusy() {
	g.openAtStart, g.mirrorAtStart, g.bedsAtStart, g.introOn = true, true, "Twin", false
	g.startRun()
	g.busy = true
	for c, r := g.hotel.Grid(); c < maxCols || r < maxRows; c, r = g.hotel.Grid() {
		if _, err := g.grow(c < maxCols); err != nil {
			break
		}
	}
	twin, _ := world.TypeOf(g.types, "Twin")
	for i := range g.hotel.Rooms {
		g.openRoom(i)
		g.hotel.Land(i, twin, false)
		g.arrivals[i] = arrival{g.clock, g.clock + (i+1)*busyGap*minuteFrames}
		g.ghostArrivals[i] = arrival{g.clock, g.clock + (i+2)*busyGap*minuteFrames}
	}
	g.floorImg, g.stains = nil, nil
	g.walls, g.wallCell = wallRuns(g.hotel.Map)
	g.wallet[Money] = 100000
	for range keeperMax {
		g.hire()
	}
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}

// busyStep runs each frame of a busy Run: a courier restocks what runs low, and no Room rots.
func (g *Game) busyStep() {
	for i := range g.hotel.Rooms { // it is about walking: a Room the ghosts have rotted would empty the hotel
		if g.roomMood(i) <= busyMood {
			g.raiseMood(i, moodClean)
		}
	}
	if len(g.couriers) > 0 {
		return
	}
	var order []cartLine
	for i, it := range shop {
		if it.Tab == tabSupplies && g.stock[it.ID] < busyStock {
			order = append(order, cartLine{item: i, room: -1, n: 1})
		}
	}
	if len(order) > 0 {
		g.ship(order)
	}
}
