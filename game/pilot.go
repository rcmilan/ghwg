package game

import (
	"image"
	"math"

	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// Pilot turns a Strategy's intents into keys, one frame at a time, at a human pace (D103, D104). It
// reads the View and emits keys into a script; it never touches the world.
type Pilot struct {
	React  int // frames before starting each action: reading the screen
	KeyGap int // frames between taps; a tap lasts one frame

	hotel   *world.Hotel
	queue   []action
	frame   int
	readyAt int
	lastTap int
	keys    []ebiten.Key // this frame's
}

// action runs one frame; done once finished, ok if it got what it wanted. A failed action drops the queue.
type action func(p *Pilot, v View) (done, ok bool)

func newPilot(h *world.Hotel, react, keyGap int) *Pilot {
	return &Pilot{React: react, KeyGap: keyGap, hotel: h, lastTap: math.MinInt / 2}
}

func (p *Pilot) Idle() bool { return len(p.queue) == 0 }

func (p *Pilot) do(a action) {
	if p.Idle() {
		p.readyAt = p.frame + p.React
	}
	p.queue = append(p.queue, a)
}

// step: the keys to hold this frame. Nothing while the title is up.
func (p *Pilot) step(v View) []ebiten.Key {
	p.frame++
	p.keys = p.keys[:0]
	if p.Idle() || v.Title || p.frame < p.readyAt {
		return nil
	}
	if done, ok := p.queue[0](p, v); done {
		p.queue, p.readyAt = p.queue[1:], p.frame+p.React
		if !ok {
			p.queue = nil
		}
	}
	return p.keys
}

// tap presses k for this frame if KeyGap has passed since the last tap (2 at least: a key held two
// frames running is not pressed twice).
func (p *Pilot) tap(k ebiten.Key) bool {
	if p.frame-p.lastTap < max(p.KeyGap, 2) {
		return false
	}
	p.keys, p.lastTap = append(p.keys, k), p.frame
	return true
}

// GoTo walks the Concierge to world point `to` along a path around the guests in sight, holding WASD;
// it re-paths when it makes no progress for stuckFrames, and gives up after 3 paths.
func (p *Pilot) GoTo(to image.Point) { p.do(goTo(to)) }

func goTo(to image.Point) action {
	var path []image.Point
	best, stuck, tries := math.Inf(1), 0, 0
	return func(p *Pilot, v View) (bool, bool) {
		at := v.Concierge
		if path == nil {
			if tries++; tries > 3 {
				return true, false
			}
			if path = p.route(v, at, to); path == nil {
				return true, false
			}
			best, stuck = math.Inf(1), 0
		}
		for len(path) > 0 && abs(path[0].X-at.X) <= 1 && abs(path[0].Y-at.Y) <= 1 {
			path, best = path[1:], math.Inf(1)
		}
		if len(path) == 0 {
			return true, true
		}
		w := path[0]
		if d := math.Hypot(float64(w.X-at.X), float64(w.Y-at.Y)); d < best-0.5 {
			best, stuck = d, 0
		} else if stuck++; stuck > stuckFrames {
			path = nil
			return false, false
		}
		if w.X-at.X > 1 {
			p.keys = append(p.keys, ebiten.KeyD)
		} else if at.X-w.X > 1 {
			p.keys = append(p.keys, ebiten.KeyA)
		}
		if w.Y-at.Y > 1 {
			p.keys = append(p.keys, ebiten.KeyS)
		} else if at.Y-w.Y > 1 {
			p.keys = append(p.keys, ebiten.KeyW)
		}
		return false, false
	}
}

// route: a path from at to `to` around the guests in sight; through them if they wall it off. A Room's
// door is open to the path only when `to` is in that doorway.
func (p *Pilot) route(v View, at, to image.Point) []image.Point {
	own, c := -1, to.Div(world.TileSize)
	for i, r := range p.hotel.Rooms {
		if c == r.Door || c == r.Door.Add(image.Pt(0, 1)) {
			own = i
		}
	}
	var avoid []image.Point
	for _, cell := range v.Guests {
		if cell != c && cell != at.Div(world.TileSize) {
			avoid = append(avoid, cell.Mul(world.TileSize).Add(image.Pt(world.TileSize/2, world.TileSize/2)))
		}
	}
	if path := p.hotel.Path(at, to, own, avoid...); path != nil {
		return path
	}
	return p.hotel.Path(at, to, own)
}

// OpenTerminal walks to the computer and presses SPACE.
func (p *Pilot) OpenTerminal() {
	p.GoTo(p.hotel.ConciergeStart)
	taps := 0
	p.do(func(p *Pilot, v View) (bool, bool) {
		switch {
		case v.Terminal.Open:
			return true, true
		case taps >= 3 || !v.Concierge.In(p.hotel.Terminal):
			return true, false
		case p.tap(ebiten.KeySpace):
			taps++
		}
		return false, false
	})
}

// Flip walks to the Mirror and presses SPACE, unless the hotel is already in that dimension: then it
// does nothing at all.
func (p *Pilot) Flip(ghost bool) {
	walk, walked, taps := goTo(p.hotel.Mirror), false, 0
	p.do(func(p *Pilot, v View) (bool, bool) {
		switch {
		case v.Shifting: // the transition runs out first (D170)
		case v.Ghost == ghost:
			return true, true
		case !walked:
			done, ok := walk(p, v)
			if done && !ok {
				return true, false
			}
			walked = done
		case taps >= 3 || !v.Concierge.In(p.hotel.MirrorSpot):
			return true, false
		case p.tap(ebiten.KeySpace):
			taps++
		}
		return false, false
	})
}

// Haunt flips to ghost, walks to the Room's door spot and presses SPACE until the Room is haunted.
func (p *Pilot) Haunt(room int) {
	p.Flip(true)
	spot := p.hotel.Rooms[room].Spot()
	p.GoTo(spot.Max.Sub(image.Pt(world.TileSize/2, world.TileSize/2))) // the cell off the doorway
	taps := 0
	p.do(func(p *Pilot, v View) (bool, bool) {
		switch {
		case taps > 0 && v.Rooms[room].Haunted:
			return true, true
		case taps >= 3 || v.Rooms[room].Vacant || !v.Concierge.In(spot):
			return true, false
		case p.tap(ebiten.KeySpace):
			taps++
		}
		return false, false
	})
}

// Serve flips to real, walks to the Room's door spot and presses SPACE until its request is answered. It
// fails when the request is gone before then.
func (p *Pilot) Serve(room int) {
	p.Flip(false)
	spot := p.hotel.Rooms[room].Spot()
	p.GoTo(spot.Max.Sub(image.Pt(world.TileSize/2, world.TileSize/2)))
	taps := 0
	p.do(func(p *Pilot, v View) (bool, bool) {
		switch {
		case taps > 0 && !v.Rooms[room].Asking:
			return true, true
		case taps >= 3 || !v.Rooms[room].Asking || !v.Concierge.In(spot):
			return true, false
		case p.tap(ebiten.KeySpace):
			taps++
		}
		return false, false
	})
}

// CallPriest opens the Terminal, picks the Room and the Priest, pays, and closes it. It fails when the
// Terminal refuses (no money, or a Priest is already on the Room).
func (p *Pilot) CallPriest(room int, strong bool) {
	p.OpenTerminal()
	paid := false
	p.do(func(p *Pilot, v View) (bool, bool) {
		t := v.Terminal
		if !t.Open {
			return true, paid && v.Rooms[room].Priest
		}
		cols, _ := p.hotel.Grid()
		c, r, tc, tr := t.Cursor%cols, t.Cursor/cols, room%cols, room/cols
		var k ebiten.Key
		switch {
		case paid || t.Menu >= 0 && t.Menu != room:
			k = ebiten.KeyEscape
		case t.Menu == room && t.Strong != strong:
			k = ebiten.KeyS
		case t.Menu == room:
			k = ebiten.KeySpace
		case c < tc:
			k = ebiten.KeyD
		case c > tc:
			k = ebiten.KeyA
		case r < tr:
			k = ebiten.KeyS
		case r > tr:
			k = ebiten.KeyW
		default:
			k = ebiten.KeySpace
		}
		if p.tap(k) && k == ebiten.KeySpace && t.Menu == room {
			paid = true
		}
		return false, false
	})
}

// Close shuts the open Terminal with ESC.
func (p *Pilot) Close() {
	p.do(func(p *Pilot, v View) (bool, bool) {
		if !v.Terminal.Open {
			return true, true
		}
		p.tap(ebiten.KeyEscape)
		return false, false
	})
}

// Wait does nothing for n frames.
func (p *Pilot) Wait(n int) {
	p.do(func(*Pilot, View) (bool, bool) { n--; return n <= 0, true })
}

func abs(x int) int { return max(x, -x) }
