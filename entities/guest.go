package entities

import (
	"image"
	"math"

	"ghosthotel/world"
)

type Guest struct {
	X, Y      float64
	RoomIndex int           // the Room its Party is booked into, whose door opens for it; -1 before Booking
	Path      []image.Point // remaining waypoints, world px; set with SetPath
	Phasing   bool          // ignoring actor collisions after getting stuck (D22c)
	Character string        // LimeZu sheet name: Adam, Amelia or Alex (D25)
	Leader    *Guest        // the Party leader this guest trails (D90); nil for a leader
	Fear      int           // from haunts (D146); never decays
	Comfort   int           // from serves (D146, D149); never decays
	Anim      Anim

	best     float64 // shortest remaining path seen on this path
	stuck    int     // frames without beating best
	phaseEnd float64 // remaining path at which Phasing ends
}

func NewGuest(at image.Point, character string) Guest {
	return Guest{X: float64(at.X), Y: float64(at.Y), RoomIndex: -1, Character: character, Anim: NewAnim()}
}

// Mate: a and b are in one Party. Mates never bump, push or yield to each other: the Party walks as one.
func (g *Guest) Mate(o *Guest) bool { return g.Leader == o || o.Leader == g }

// waypointSlack (world px): a guest this close to a waypoint with more after it, or past it along the
// next leg and this close to that leg's line, has reached it. So a push past it never turns the guest
// back into whoever walks behind (D89), and a corner is never cut.
const waypointSlack = 4

// FollowPath walks toward the next waypoint; true once the path is used up.
func (g *Guest) FollowPath(speed float64) bool {
	if len(g.Path) == 0 {
		return true
	}
	if len(g.Path) > 1 {
		p, q := g.Path[0], g.Path[1]
		dx, dy := g.X-float64(p.X), g.Y-float64(p.Y)
		ux, uy := float64(q.X-p.X), float64(q.Y-p.Y)
		if n := math.Hypot(ux, uy); n > 0 {
			ux, uy = ux/n, uy/n
		}
		if math.Hypot(dx, dy) <= waypointSlack || dx*ux+dy*uy > 0 && math.Abs(dx*uy-dy*ux) <= waypointSlack {
			g.Path = g.Path[1:]
		}
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

// SetPath gives the guest a new route and restarts its progress tracking.
func (g *Guest) SetPath(p []image.Point) {
	g.Path, g.best, g.stuck, g.Phasing = p, math.Inf(1), 0, false
}

// Left is the remaining path length (waypoints are adjacent tile centers).
func (g *Guest) Left() float64 {
	if len(g.Path) == 0 {
		return 0
	}
	p := g.Path[0]
	return math.Hypot(float64(p.X)-g.X, float64(p.Y)-g.Y) + float64((len(g.Path)-1)*world.TileSize)
}

// TrackProgress runs once per frame, after movement and pushes. A walking guest whose remaining path
// hasn't shrunk for stuckFrames starts Phasing (ignores actor collisions) until it advances one tile.
func (g *Guest) TrackProgress(stuckFrames int) {
	if len(g.Path) == 0 {
		g.stuck, g.Phasing = 0, false
		return
	}
	left := g.Left()
	if left < g.best-0.1 {
		g.best, g.stuck = left, 0
	} else {
		g.stuck++
	}
	if g.Phasing && left <= g.phaseEnd {
		g.Phasing = false
	}
	if !g.Phasing && g.stuck >= stuckFrames {
		g.Phasing, g.phaseEnd, g.stuck = true, left-world.TileSize, 0
	}
}
