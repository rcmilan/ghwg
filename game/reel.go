package game

import (
	"image"
	"image/color"
	"math"

	"ghosthotel/assets"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// The reel (D48, restored for one Room by 07 K1): a changing Room's whole footprint rolls like a slot
// machine's reel, the old Room sliding out the bottom while the new one comes in from the top, in the world,
// in either dimension. The Room has already changed underneath (updateChanges); the reel is its picture.

// reelPics: a Room before and after its change, each as its floor, its maze tiles (walls) and the rest, so
// the ghost dimension's palette draws each as it does a still Room.
type reelPics [2][3]*ebiten.Image

func (p *reelPics) free() {
	for _, side := range p {
		for _, img := range side {
			if img != nil {
				img.Deallocate()
			}
		}
	}
	*p = reelPics{}
}

// roomPics paints the Room as the map holds it now, without the hotel's outer side line: it stays still.
func (g *Game) roomPics(room int) (p [3]*ebiten.Image) {
	o, m := g.hotel.Rooms[room].Origin, g.hotel.Map
	at := func(wall bool) func(l, x, y int) world.TileID {
		return func(l, x, y int) world.TileID {
			id := m.At(l, o.X+x, o.Y+y)
			if l == world.LayerDecor && g.hotel.OuterLine(o.X+x) || l > world.LayerFloor && maze(m.TS.Tiles[id].Name) != wall {
				return 0
			}
			return id
		}
	}
	p[0] = g.paint(world.RoomW, world.RoomH, world.LayerFloor, world.LayerFloor+1, at(false))
	p[1] = g.paint(world.RoomW, world.RoomH, world.LayerFloor+1, world.NumLayers, at(true))
	p[2] = g.paint(world.RoomW, world.RoomH, world.LayerFloor+1, world.NumLayers, at(false))
	return p
}

// reeling: the Room's reel is rolling.
func (g *Game) reeling(room int) bool {
	for _, c := range g.changes {
		if c.room == room && c.reel > 0 {
			return true
		}
	}
	return false
}

// reelingAt: map cell (x,y) is rolling with its Room's reel; the hotel's outer side line (decor) is not.
func (g *Game) reelingAt(l, x, y int) bool {
	for _, c := range g.changes {
		o := g.hotel.Rooms[c.room].Origin
		if c.reel > 0 && image.Pt(x, y).In(image.Rect(o.X, o.Y, o.X+world.RoomW, o.Y+world.RoomH)) {
			return l != world.LayerDecor || !g.hotel.OuterLine(x)
		}
	}
	return false
}

// drawReels draws each rolling Room, clipped to its footprint, eased out to its stop. Actors and the hotel's
// outer side line still draw over it.
func (g *Game) drawReels(screen *ebiten.Image, ox, oy int) {
	const cell, rh = world.TileSize * zoom, world.RoomH * world.TileSize * zoom
	for _, c := range g.changes {
		if c.reel == 0 || c.pics[0][0] == nil {
			continue
		}
		o := g.hotel.Rooms[c.room].Origin
		px := image.Rect(o.X*cell, o.Y*cell, (o.X+world.RoomW)*cell, (o.Y+world.RoomH)*cell).Sub(image.Pt(ox, oy)).Intersect(screen.Bounds())
		if px.Empty() {
			continue
		}
		clip := screen.SubImage(px).(*ebiten.Image)
		if clip.Fill(assets.Night); g.ghost {
			clip.Fill(color.Black)
		}
		t := min(float64(g.tick-c.reel)/reelFrames, 1)
		dy := math.Round((1 - math.Pow(1-t, 3)) * rh)
		for k, side := range c.pics {
			for l, img := range side {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(zoom, zoom)
				op.GeoM.Translate(float64(o.X*cell-ox), float64(o.Y*cell-oy)+dy-float64(k*rh))
				mode, shade := modeProp, float32(0)
				switch l {
				case 0:
					mode, shade = modeMaze, floorBlue
				case 1:
					mode, shade = modeMaze, 1
				}
				g.blit(clip, img, op, mode, shade, nil, g.roomShift(c.room))
			}
		}
	}
}
