package entities

import (
	"image"
	"math"
)

type GuestState int

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
	Path       []image.Point // remaining waypoints, world px
	SpawnAt    float64
	PauseUntil float64
}

func NewGuest(roomIndex int, at image.Point, spawnAt float64) Guest {
	return Guest{X: float64(at.X), Y: float64(at.Y), State: GuestQueued, RoomIndex: roomIndex, SpawnAt: spawnAt}
}

// FollowPath walks toward the next waypoint; true once the path is used up.
func (g *Guest) FollowPath(speed float64) bool {
	if len(g.Path) == 0 {
		return true
	}
	if p := g.Path[0]; g.WalkTo(float64(p.X), float64(p.Y), speed) {
		g.Path = g.Path[1:]
	}
	return len(g.Path) == 0
}

// WalkTo moves straight toward (x,y); true once there.
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
