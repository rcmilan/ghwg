package game

import (
	"image"
	"image/color"
	"iter"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// Courier and stock (D200): checkout sends a courier with the order. He fills each Supply at its prop in the
// kitchen (F1), drops the unlocks there too, then leaves. Placeholders.
const (
	courierSpeed = conciergeSpeed
	courierName  = "Courier"     // Alex in a brown uniform (T240)
	courierBox   = "Courier box" // the same, carrying a box
)

var courierLook = look{hair: 0x3a2a24, top: 0x7a5232, bottom: 0x5a3e2a, accent: 0xe0b040}

func init() {
	ghostTints[courierName], ghostTints[courierBox] = []float32{.4, 1, .4}, []float32{.4, 1, .4}
}

type courier struct {
	entities.Guest
	drops []drop // still to make, in order; then out the Exit
	voice
	out bool // the world has run since the order: he is at the Entrance. Until then (the store or the hotel view still open) he isn't drawn
}

// drop: where the courier hands over some of the order's lines.
type drop struct {
	at    image.Point
	lines []cartLine
	own   int // 1 + the Room whose door opens for the walk there (inside it, or out of it); 0 none
}

// carrying: some of the order is still in his hands (a stop with nothing to hand over is only a waypoint).
func (c *courier) carrying() bool {
	return slices.ContainsFunc(c.drops, func(d drop) bool { return len(d.lines) > 0 })
}

// intoRoom: the stops that take the courier inside Room room to the cell at, hand the lines over there and
// bring him back out to the Corridor in front of its door.
func (g *Game) intoRoom(room int, at image.Point, l cartLine) []drop {
	door := g.doorDrop(room)
	return []drop{{at: door}, {at: at, lines: []cartLine{l}, own: room + 1}, {at: door, own: room + 1}}
}

// pathTo: his walk to stop d from where he stands.
func (c *courier) pathTo(g *Game, d drop) {
	c.SetPath(g.hotel.Path(image.Pt(int(c.X), int(c.Y)), d.at, d.own-1))
}

// stockDrop: where the courier drops the unlocks, the cell below the middle of the kitchen's prop row.
func (g *Game) stockDrop() image.Point {
	s := g.hotel.Stock
	return image.Pt((s.Min.X+s.Max.X)/2, s.Max.Y+world.TileSize/2)
}

// doorDrop: the Corridor cell in front of the Room's door.
func (g *Game) doorDrop(room int) image.Point {
	return g.hotel.Rooms[room].Spot().Min.Add(image.Pt(world.TileSize/2, world.TileSize/2))
}

func (g *Game) updateCouriers() {
	kept := g.couriers[:0]
	for _, c := range g.couriers {
		c.out = true
		arrived := g.walk(&c.Guest, courierSpeed*g.staffSpeed())
		if !g.headless {
			c.Anim.Update(c.X, c.Y, g.dt)
		}
		g.comeNear(&c.Guest, &c.voice, func() []string {
			if c.carrying() {
				return g.sometimes(courierIn)
			}
			return g.sometimes(courierOut)
		})
		if !arrived {
			kept = append(kept, c)
			continue
		}
		if len(c.drops) == 0 { // out the Exit
			continue
		}
		for _, l := range c.drops[0].lines {
			g.receive(l)
		}
		c.drops = c.drops[1:]
		next := drop{at: g.hotel.Exit}
		if len(c.drops) > 0 {
			next = c.drops[0]
		}
		c.pathTo(g, next)
		kept = append(kept, c)
	}
	clear(g.couriers[len(kept):])
	g.couriers = kept
}

// coming: a courier is bringing item id, for the Room (-1: not a Service).
func (g *Game) coming(id Unlock, room int) bool {
	return slices.ContainsFunc(g.couriers, func(c *courier) bool {
		return slices.ContainsFunc(c.drops, func(d drop) bool {
			return slices.ContainsFunc(d.lines, func(l cartLine) bool { return shop[l.item].ID == id && l.room == room })
		})
	})
}

// paid: what the courier's order cost at checkout, while he still carries all of it (sim.go).
func (c *courier) paid() Cost {
	var t Cost
	for _, d := range c.drops {
		for _, l := range d.lines {
			t[Money] += l.cost()[Money]
			t[Ecto] += l.cost()[Ecto]
		}
	}
	return t
}

// walkers: the guests and the couriers of one realm, for bumping, yielding and pushing: a courier and a
// Party walking in met head-on and walked through each other (Live test 6, 2026-11-08).
func (g *Game) walkers(realm Realm) iter.Seq[*entities.Guest] {
	return func(yield func(*entities.Guest) bool) {
		for guest := range g.guestsIn(realm) {
			if !yield(guest) {
				return
			}
		}
		if realm == Human {
			for _, c := range g.couriers {
				if !yield(&c.Guest) {
					return
				}
			}
			for _, k := range g.keepers {
				if !yield(&k.Guest) {
					return
				}
			}
		}
	}
}

// courierActors: the couriers, for the doors: the Entrance opens for them, as for anyone.
func (g *Game) courierActors() []world.Actor {
	var as []world.Actor
	for _, c := range g.couriers {
		as = append(as, world.Actor{Box: entities.FootBox(c.X, c.Y), Room: -1})
	}
	return as
}

// loadCourier dresses Alex as the courier, with and without a box (T240).
func (g *Game) loadCourier() {
	b := g.bases["Alex"]
	g.characters[courierName] = dress(b, palettes["Alex"], courierLook)
	g.characters[courierBox] = dress([2]*image.RGBA{withBox(b[0]), withBox(b[1])}, palettes["Alex"], courierLook)
}

// withBox: a copy of a sheet with a cardboard box in the hands of every frame but the back views, where
// the body hides it. Placeholder art: rects in the sheet's browns (D199).
func withBox(sheet *image.RGBA) *image.RGBA {
	out := image.NewRGBA(sheet.Bounds())
	copy(out.Pix, sheet.Pix)
	ink, fill, shade, tape := opaque(0x3a3a50), opaque(0xb6937e), opaque(0x916662), opaque(0xd0be9c)
	x0 := map[entities.Facing]int{entities.FaceRight: 7, entities.FaceLeft: 1, entities.FaceDown: 4} // in front of the body
	for f := range sheet.Bounds().Dx() / charW {
		dx, ok := x0[entities.Facing(f/entities.FramesPerFacing)]
		if !ok {
			continue
		}
		x, y := f*charW+dx, 21
		for j := range 7 {
			for i := range 8 {
				c := fill
				switch {
				case i == 0 || i == 7 || j == 0 || j == 6:
					c = ink
				case j == 5:
					c = shade
				case i == 3 || i == 4:
					c = tape
				}
				out.SetRGBA(x+i, y+j, c)
			}
		}
	}
	return out
}

func opaque(c uint32) color.RGBA { return color.RGBA{uint8(c >> 16), uint8(c >> 8), uint8(c), 255} }
