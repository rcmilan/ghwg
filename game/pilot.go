package game

import (
	"image"
	"math"
	"slices"

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
		if c == r.Door || c == r.Door.Add(image.Pt(0, 1)) || to.In(r.Shell()) || at.In(r.Shell()) { // into or out of a Room: its door is open
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
		case v.MirrorLocked && v.Ghost != ghost: // covered: nothing to flip with
			return true, false
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

// Fetch flips to real, walks in front of the item's supply prop and presses SPACE until the item is in the
// hands (E5). It fails when the prop is empty, or after 3 taps.
func (p *Pilot) Fetch(item Unlock) {
	p.Flip(false)
	r := supplySpot(p.hotel, supplyOf(item))
	p.GoTo(image.Pt(r.Min.X+world.TileSize/2, r.Max.Y-world.TileSize/2))
	taps := 0
	p.do(func(p *Pilot, v View) (bool, bool) {
		switch {
		case v.Hands == item:
			return true, true
		case taps >= 3 || v.Stock[item] == 0:
			return true, false
		case p.tap(ebiten.KeySpace):
			taps++
		}
		return false, false
	})
}

// Serve flips to real, walks to the Room's door spot and presses SPACE until its request is answered, with
// whatever the hands hold (Fetch first). It fails when the request is gone before then.
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

// Clean walks to the dirt spot at (world px) in the Room and presses SPACE until the Room's spots drop.
// It fails when they don't after 3 taps.
func (p *Pilot) Clean(room int, at image.Point) {
	p.GoTo(at)
	n, taps := -1, 0
	p.do(func(p *Pilot, v View) (bool, bool) {
		if n < 0 {
			n = len(v.Rooms[room].Dirt)
		}
		switch {
		case len(v.Rooms[room].Dirt) < n:
			return true, true
		case taps >= 3:
			return true, false
		case p.tap(ebiten.KeySpace):
			taps++
		}
		return false, false
	})
}

// CallPriest buys a Priest for the Room at the Terminal. It fails when the Terminal refuses (no money,
// or a Priest is already on the Room).
func (p *Pilot) CallPriest(room int, strong bool) {
	row := rowFatherK
	if strong {
		row = rowFatherM
	}
	p.Buy(row, room, func(v View) bool { return v.Rooms[room].Priest })
}

// Change orders a Room's change to a bed layout (K1): Buy, picking the layout with W/S once the Room's
// Change type is open. ok once the Room is changing or has changed.
func (p *Pilot) Change(room int, beds string) {
	p.buy(rowChange, room, beds, func(v View) bool { return v.Rooms[room].Changing || v.Rooms[room].Beds == beds })
}

// Buy opens the Terminal and orders one item through the store's keys: its tab, its row, SPACE, then the
// cart's checkout, and closes the Terminal. An action for a Room goes through the hotel view instead, from
// where he stands: TAB, the Room, the action (and its beds), paid at once. ok if got holds then. It gives up when the item isn't for sale or the
// store refuses it.
func (p *Pilot) Buy(id Unlock, room int, got func(View) bool) { p.buy(id, room, "", got) }

func (p *Pilot) buy(id Unlock, room int, beds string, got func(View) bool) {
	i := slices.IndexFunc(shop, func(it Item) bool { return it.ID == id })
	tab := shop[i].Tab
	pos := slices.Index(tabItems(tab), i)
	if shop[i].room() {
		pressed, leave, paid, opened, spins := false, false, false, false, 0
		p.do(func(p *Pilot, v View) (bool, bool) {
			t := v.Terminal
			if !t.View {
				if opened {
					return true, paid && got(v)
				}
				opened = p.tap(ebiten.KeyTab)
				return false, false
			}
			opened = true
			paid = paid || pressed && t.Depth == 1 // bought: the store is back on the grid
			leave = leave || paid || t.Message != "" || !slices.Contains(v.Shop, i) || spins > len(bedKinds)
			k, last := ebiten.KeySpace, false
			switch {
			case leave:
				k = ebiten.KeyEscape
			case t.Depth == 1 && t.Cursor != room:
				k = ebiten.KeyD
			case t.Depth == 2 && t.Pick != pos:
				k = ebiten.KeyS
			case t.Depth == 3 && t.Beds != beds:
				k = ebiten.KeyS
				spins++
			default:
				last = t.Depth == 3 || t.Depth == 2 && beds == ""
			}
			if !p.tap(k) && k == ebiten.KeyS && t.Depth == 3 {
				spins--
			}
			pressed = pressed || last && p.lastTap == p.frame
			return false, false
		})
		return
	}
	p.OpenTerminal()
	cart0, paid := -1, false
	p.do(func(p *Pilot, v View) (bool, bool) {
		t := v.Terminal
		if !t.Open {
			return true, paid && got(v)
		}
		if cart0 < 0 {
			cart0 = t.Cart
		}
		added := t.Cart > cart0
		var k ebiten.Key
		switch {
		case paid || !added && (t.Message != "" || !slices.Contains(v.Shop, i)):
			k = ebiten.KeyEscape
		case !added && t.Tab != tab, added && t.Tab != tabCart:
			k = ebiten.KeyD
		case !added && t.Pick != pos:
			k = ebiten.KeyS
		case added && t.Pick != t.Lines:
			k = ebiten.KeyS
		default:
			k = ebiten.KeySpace
		}
		if p.tap(k) && k == ebiten.KeySpace && added {
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
