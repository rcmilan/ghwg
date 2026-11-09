package game

import (
	"fmt"
	"image"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// Supplies (D182, E5): what a Guest asks for, the prop it's fetched from, and what a delivery pays.
// Each prop stands where E8's pallet was, in the stock strip, in the store's order. Pays are placeholders.
type Supply struct {
	ID   Unlock
	Prop string // A1's sprite
	Pay  Num    // Money a delivery pays, times calm
}

var supplies = []Supply{
	{SupplyWater, "fridge", 5},
	{SupplyFood, "drawer_b", 10},
	{SupplyTowel, "shelf_a", 7},
}

func supplyOf(id Unlock) int {
	for k, s := range supplies {
		if s.ID == id {
			return k
		}
	}
	return -1
}

// supplySpot: supply k's 2-cell spot in the stock strip and the cell row in front of it, where the
// Concierge stands to fetch.
func supplySpot(h *world.Hotel, k int) image.Rectangle {
	x := h.Stock.Min.X + k*2*world.TileSize
	return image.Rect(x, h.Stock.Min.Y, x+2*world.TileSize, h.Stock.Max.Y+world.TileSize)
}

// atSupply: the supply the Concierge stands at, or -1.
func (g *Game) atSupply() int {
	for k := range supplies {
		if g.feet().In(supplySpot(g.hotel, k)) {
			return k
		}
	}
	return -1
}

// fetch puts one of supply k in the hands, from the stock; what was held goes back. An empty one gives nothing.
func (g *Game) fetch(k int) {
	id := supplies[k].ID
	if g.stock[id] == 0 {
		return
	}
	if g.hands != "" {
		g.stock[g.hands]++
	}
	g.stock[id]--
	g.hands = id
}

// deliver hands item to the Room's asking Party: it pays Pay × calm at once (calm falls with Room Spook)
// and gives serve's Comfort. False, and nothing taken, on the wrong item or with no ask.
func (g *Game) deliver(room int, item Unlock) bool {
	k := supplyOf(item)
	if k < 0 || !g.wants(room, item) || !g.serve(room) {
		return false
	}
	calm := Num(max(0, spookLimit-g.roomSpook[room])) / spookLimit
	door := g.hotel.Rooms[room].Door.Mul(world.TileSize)
	pay := supplies[k].Pay * calm
	g.earn(Money, pay, image.Pt(door.X+world.TileSize/2, door.Y))
	g.delivered += pay
	g.tapped(room, TaskServe)
	g.hands = ""
	return true
}

// wants: the Room's Party asks for item.
func (g *Game) wants(room int, item Unlock) bool {
	for _, p := range g.parties {
		if p.Occupies(room) && p.asking() && p.state == staying && p.want == item {
			return true
		}
	}
	return false
}

// supplyIcon: the store's icon for a supply, the same in the balloon, the hands and the pop-up.
func (g *Game) supplyIcon(id Unlock) *ebiten.Image {
	for _, it := range shop {
		if it.ID == id {
			img, _ := g.icon(it)
			return img
		}
	}
	return nil
}

// drawSupplies draws each supply prop on its spot, and, with the Concierge at one, its icon and count over it.
func (g *Game) drawSupplies(screen *ebiten.Image, ox, oy int) {
	at := g.atSupply()
	for k, s := range supplies {
		img, _ := g.icon(Item{Icon: s.Prop})
		if img == nil {
			continue
		}
		spot := supplySpot(g.hotel, k)
		w, h := img.Bounds().Dx(), img.Bounds().Dy()
		x, y := spot.Min.X+(2*world.TileSize-w)/2, g.hotel.Stock.Max.Y-h
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(zoom, zoom)
		op.GeoM.Translate(float64(x*zoom-ox), float64(y*zoom-oy))
		g.blit(screen, img, op, modeProp, 0, nil, 0)
		if k != at {
			continue
		}
		icon := g.supplyIcon(s.ID)
		label := fmt.Sprintf("x%d", g.stock[s.ID])
		ix, iy := (spot.Min.X+world.TileSize)*zoom-ox-(icon.Bounds().Dx()*zoom+6*len(label))/2, (y-2)*zoom-oy-icon.Bounds().Dy()*zoom
		op.GeoM.Reset()
		op.GeoM.Scale(zoom, zoom)
		op.GeoM.Translate(float64(ix), float64(iy))
		screen.DrawImage(icon, op)
		g.inked(screen, label, ix+icon.Bounds().Dx()*zoom+2, iy+icon.Bounds().Dy()*zoom-14, 1, 1, 1)
	}
}

// drawHands draws the held item over the Concierge's head; (x,y) are the feet, screen px, as drawCharacter's.
func (g *Game) drawHands(screen *ebiten.Image, x, y int) {
	icon := g.supplyIcon(g.hands)
	top := y + (entities.FootH/2-charH+8)*zoom // just over the hair: the sheet's head starts on row 9
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(float64(x-icon.Bounds().Dx()*zoom/2), float64(top-icon.Bounds().Dy()*zoom))
	screen.DrawImage(icon, op)
}
