package entities

import (
	"math"
	"math/rand/v2"
)

// Facing indexes the 4 strips of a LimeZu sheet, in sheet order (D26).
type Facing int

const (
	FaceRight Facing = iota
	FaceUp
	FaceLeft
	FaceDown
)

const (
	FramesPerFacing = 6
	animFPS         = 8
	tps             = 60
	stillEps        = 0.05 // world px per frame below which an actor counts as standing

	idleCycle    = FramesPerFacing * tps / animFPS // frames for one pass of the idle strip
	idlePauseMin = 1 * tps                         // standing, the idle strip's first frame is held
	idlePauseMax = 3 * tps                         // for a random pause between passes
)

// Anim is an actor's facing and animation clock, driven by how its position changes each frame.
type Anim struct {
	Facing       Facing
	Moving       bool    // run sheet while moving, idle_anim while standing
	clock        float64 // frames into the current pass of the strip
	hold         float64 // idle: frames left to hold the first frame before the next pass
	lastX, lastY float64
	started      bool
}

func NewAnim() Anim { return Anim{Facing: FaceDown, hold: idlePause()} }

func idlePause() float64 { return float64(idlePauseMin + rand.IntN(idlePauseMax-idlePauseMin+1)) }

// Update takes the actor's position after this step's movement and pushes, dt frames of world time.
func (a *Anim) Update(x, y, dt float64) {
	dx, dy := x-a.lastX, y-a.lastY
	a.lastX, a.lastY = x, y
	moving := a.started && math.Max(math.Abs(dx), math.Abs(dy)) > stillEps*dt
	a.started = true
	if moving {
		a.Facing = facing(a.Facing, dx, dy)
	}
	switch {
	case moving != a.Moving:
		a.Moving, a.clock, a.hold = moving, 0, 0
		if !moving {
			a.hold = idlePause() // stand still a moment before fidgeting
		}
	case !moving && a.hold > 0:
		a.hold -= dt
	default:
		a.clock += dt
		if !moving && a.clock >= idleCycle {
			a.clock, a.hold = 0, idlePause()
		}
	}
}

// Toward: the facing along (dx,dy); down when it's zero.
func Toward(dx, dy float64) Facing { return facing(FaceDown, dx, dy) }

// facing: the dominant axis of (dx,dy). On an exact diagonal the current facing stays if it is one of
// the two candidates (no flip-flopping); otherwise horizontal wins.
func facing(cur Facing, dx, dy float64) Facing {
	h, v := FaceRight, FaceDown
	if dx < 0 {
		h = FaceLeft
	}
	if dy < 0 {
		v = FaceUp
	}
	switch ax, ay := math.Abs(dx), math.Abs(dy); {
	case math.Abs(ax-ay) < 1e-9:
		if cur == v {
			return v
		}
		return h
	case ax > ay:
		return h
	default:
		return v
	}
}

// Posed: an Anim held on facing f, on the run sheet if moving, `frames` into its strip: an actor shown in
// place (a dev line-up).
func Posed(f Facing, moving bool, frames float64) Anim {
	return Anim{Facing: f, Moving: moving, clock: frames}
}

// Frame is the index (0..23) into the 24-frame strip of the current sheet.
func (a Anim) Frame() int {
	return int(a.Facing)*FramesPerFacing + int(a.clock)*animFPS/tps%FramesPerFacing
}
