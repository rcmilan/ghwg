package game

import (
	"image"
	"image/color"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// Courier and stock (D200): checkout sends a courier with the order. He drops Supplies and unlocks at the
// stock, left of the entrance hall, and a Shadow at its Room's door, then leaves. Placeholders.
const (
	courierSpeed = conciergeSpeed
	courierName  = "Courier"     // Alex in a brown uniform (T240)
	courierBox   = "Courier box" // the same, carrying a box
	stockShown   = 10            // icons a stock spot stacks at most: a pyramid of 4, 3, 2, 1
)

var courierLook = look{hair: 0x3a2a24, top: 0x7a5232, bottom: 0x5a3e2a, accent: 0xe0b040}

func init() {
	ghostTints[courierName], ghostTints[courierBox] = []float32{.4, 1, .4}, []float32{.4, 1, .4}
}

type courier struct {
	entities.Guest
	drops []drop // still to make, in order; then out the Exit
}

// drop: where the courier hands over some of the order's lines.
type drop struct {
	at    image.Point
	lines []cartLine
}

// stockDrop: where the courier stands to fill the stock, the cell below the middle of the area.
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
		arrived := g.walk(&c.Guest, courierSpeed)
		if !g.headless {
			c.Anim.Update(c.X, c.Y)
		}
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
		to := g.hotel.Exit
		if len(c.drops) > 0 {
			to = c.drops[0].at
		}
		c.SetPath(g.hotel.Path(image.Pt(int(c.X), int(c.Y)), to, -1))
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
			t[Spook] += l.cost()[Spook]
		}
	}
	return t
}

// walkers: the guests and the couriers, for bumping, yielding and pushing: a courier and a Party walking
// in met head-on and walked through each other (Live test 6, 2026-11-08).
func (g *Game) walkers() []*entities.Guest {
	gs := g.guests()
	for _, c := range g.couriers {
		gs = append(gs, &c.Guest)
	}
	return gs
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

// drawStock draws the stock area (T239): a pallet per Supply, its stock stacked on it as icons.
func (g *Game) drawStock(screen *ebiten.Image, ox, oy int) {
	if g.pallet == nil {
		g.pallet = ebiten.NewImage(2*world.TileSize-2, 4)
		g.pallet.Fill(opaque(0x3a3a50))
		g.pallet.SubImage(image.Rect(1, 0, 2*world.TileSize-3, 2)).(*ebiten.Image).Fill(opaque(0x9c786b))
		g.pallet.SubImage(image.Rect(1, 2, 2*world.TileSize-3, 3)).(*ebiten.Image).Fill(opaque(0x6b5052))
	}
	at := func(img *ebiten.Image, x, y int) {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(zoom, zoom)
		op.GeoM.Translate(float64(x*zoom-ox), float64(y*zoom-oy))
		g.blit(screen, img, op, modeProp, 0, nil, 0)
	}
	for k, i := range tabItems(tabSupplies) {
		x, bottom := g.hotel.Stock.Min.X+k*2*world.TileSize, g.hotel.Stock.Max.Y
		at(g.pallet, x+1, bottom-5)
		img, _ := g.icon(shop[i])
		if img == nil {
			continue
		}
		w, h := img.Bounds().Dx(), img.Bounds().Dy()
		stepX, stepY := (2*world.TileSize-2-w)/3, h/2+1
		n := min(g.stock[shop[i].ID], stockShown)
		for row, left := 0, 4; n > 0; row, left = row+1, left-1 { // the bottom row first, 4 wide, then 3, 2, 1
			for col := range min(n, left) {
				at(img, x+1+row*stepX/2+col*stepX, bottom-4-h-row*stepY)
			}
			n -= left
		}
	}
}
