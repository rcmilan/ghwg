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
	Fear      int           // from frights (D146); never decays
	FleeAt    int           // the Fear at which its Party steps out, then flees (D146)
	Phantasm  int           // how ghostly it is: sight and strength (07 stats; V1 reads it)
	Haunt     int           // x, haunt frequency: game minutes between its haunts (a ghost; G1 reads it)
	HauntAt   int           // the game minute its haunts count from: the one it settled in, so the Rooms' haunts do not all fall together (2026-11-14)
	Haunted   int           // a ghost: the haunts it has done. At its Phantasm it is spent and passes on (2026-11-14)
	PassAt    int           // the clock it began to pass on, 0 until then
	Anim      Anim

	best     float64 // shortest remaining path seen on this path
	stuck    int     // frames without beating best
	phaseEnd float64 // remaining path at which Phasing ends
	detours  int     // Detour calls since the last SetPath
}

// DefaultFleeAt is the Fear a fresh Guest flees at.
const DefaultFleeAt = 100

// DefaultHaunt is the gap between a fresh ghost's haunts, in game minutes (placeholder).
const DefaultHaunt = 60

func NewGuest(at image.Point, character string) Guest {
	return Guest{X: float64(at.X), Y: float64(at.Y), RoomIndex: -1, Character: character, FleeAt: DefaultFleeAt, Haunt: DefaultHaunt, Anim: NewAnim()}
}

// Mate: a and b are in one Party, or both trail one leader (a Room mirror's ghost and a pair's mate, 08 B1).
// Mates never bump, push or yield to each other: the Party walks as one.
func (g *Guest) Mate(o *Guest) bool {
	return g.Leader == o || o.Leader == g || g.Leader != nil && g.Leader == o.Leader
}

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
	g.Path, g.best, g.stuck, g.Phasing, g.detours = p, math.Inf(1), 0, false, 0
}

// Detour: SetPath for another way to the same goal, counted (Detours) until the next SetPath: a guest may
// take only so many on one errand, so no two can go round each other for ever.
func (g *Guest) Detour(p []image.Point) {
	n := g.detours
	g.SetPath(p)
	g.detours = n + 1
}

// Detours: the detours taken since the last SetPath.
func (g *Guest) Detours() int { return g.detours }

// Phase: the guest passes through whoever is in its way for one tile, from now.
func (g *Guest) Phase() {
	g.Phasing, g.phaseEnd, g.stuck, g.best = true, g.Left()-world.TileSize, 0, math.Inf(1)
}

// Stuck: frames the guest has walked without getting nearer its goal.
func (g *Guest) Stuck() int { return g.stuck }

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
