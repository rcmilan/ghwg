package entities

import "math"

type GuestState int

const GuestRadius = 13.0

const (
	GuestQueued GuestState = iota
	GuestArriving
	GuestStaying
	GuestLeavingRoom
	GuestPaying
	GuestDeparting
)

type Guest struct {
	X, Y       float64
	State      GuestState
	RoomIndex  int
	Waypoint   int
	SpawnAt    float64
	PauseUntil float64
	Paid       bool
}

func NewGuest(roomIndex int, x, y, spawnAt float64) Guest {
	return Guest{X: x, Y: y, State: GuestQueued, RoomIndex: roomIndex, SpawnAt: spawnAt}
}

// WalkTo returns true after the destination is reached, with no pathfinding dependency
// needed for this deliberately open first lobby layout.
func (g *Guest) WalkTo(x, y, speed float64) bool {
	dx, dy := x-g.X, y-g.Y
	d := math.Hypot(dx, dy)
	if d <= speed {
		g.X, g.Y = x, y
		return true
	}
	g.X += dx / d * speed
	g.Y += dy / d * speed
	return false
}
