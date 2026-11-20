package game

import (
	"image"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// Supplies (D182, E5): what a Guest asks for, the prop it's fetched from, and what a delivery pays.
// Each prop stands in the kitchen, against its back wall (F1): Water at the fridge, Food at the oven, Towels
// at the shelf. Pays are placeholders.
type Supply struct {
	ID   Unlock
	Prop string // A1's sprite
	Pay  Num    // Money a delivery pays, times calm
	X, W int    // its first cell in the kitchen's prop row, and the cells it takes
}

var supplies = []Supply{
	{SupplyWater, "fridge", 25, 5, 1},
	{SupplyFood, "cabinet_mirror", 50, 2, 1}, // the mirror cabinet, recolored as an oven (tools/sprites)
	{SupplyTowel, "shelf_a", 35, 1, 1},
	{SupplyVeggie, "cabinet", 50, 0, 1},   // the plain cabinet beside the oven (T378)
	{SupplyGourmet, "shelf_b", 100, 3, 1}, // pays double the Food
}

// baseSupplies: the supplies a request first picks among (g.rng.IntN keeps its range); the rest are meals the pick maps to.
const baseSupplies = 3

// mealOn: the Veggie meal is sold and asked once the hotel has grown, the Gourmet once a Room is Deluxe.
func (g *Game) mealOn(id Unlock) bool {
	switch id {
	case SupplyVeggie:
		return g.grown()
	case SupplyGourmet:
		for _, r := range g.hotel.Rooms {
			if r.Type.Name == deluxeTier {
				return true
			}
		}
		return false
	}
	return true
}

// meal maps a picked supply to what p really asks for: a Vegetarian's Food is the Veggie meal, a Deluxe Room's the
// Gourmet. No random draw, so the sim stays as it was while neither is on.
func (g *Game) meal(p *Party, id Unlock) Unlock {
	if id != SupplyFood {
		return id
	}
	for _, gu := range p.Guests {
		if gu.Who.Vegetarian && g.mealOn(SupplyVeggie) {
			return SupplyVeggie
		}
	}
	if g.hotel.Rooms[p.Room].Type.Name == deluxeTier && g.mealOn(SupplyGourmet) {
		return SupplyGourmet
	}
	return id
}

func supplyOf(id Unlock) int {
	for k, s := range supplies {
		if s.ID == id {
			return k
		}
	}
	return -1
}

// supplySpot: supply k's cells in the kitchen's prop row, solid, and the cell row in front of them, where
// the Concierge stands to fetch.
func supplySpot(h *world.Hotel, k int) image.Rectangle {
	x := h.Stock.Min.X + supplies[k].X*world.TileSize
	return image.Rect(x, h.Stock.Min.Y, x+supplies[k].W*world.TileSize, h.Stock.Max.Y+world.TileSize)
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

// fetch puts one of supply k in the hands, from the stock; what was held goes back. Holding k itself puts it back
// (08 F0), even on an empty shelf; any other empty one gives nothing.
func (g *Game) fetch(k int) {
	id := supplies[k].ID
	switch {
	case g.hands == id:
		g.stock[id]++
		g.hands = ""
	case g.stock[id] > 0:
		if g.hands != "" {
			g.stock[g.hands]++
		}
		g.stock[id]--
		g.hands = id
	}
}

// deliver hands item to the Room's asking Party: it pays Pay × calm at once (calm falls with the Room's dread)
// and serves (serve's Mood). False, and nothing taken, on the wrong item or with no ask.
func (g *Game) deliver(room int, item Unlock) bool {
	if !g.handOver(room, item) {
		return false
	}
	g.tapped(room, TaskServe)
	g.hands = ""
	return true
}

// handOver: deliver, by whoever carries the item: the Concierge or a housekeeper (keeper.go).
func (g *Game) handOver(room int, item Unlock) bool {
	k := supplyOf(item)
	if k < 0 || !g.wants(room, item) || !g.serve(room) {
		return false
	}
	calm := Num(1 - g.dread(room))
	door := g.hotel.Rooms[room].Door.Mul(world.TileSize)
	pay := supplies[k].Pay * calm
	g.earn(Money, pay, image.Pt(door.X+world.TileSize/2, door.Y))
	g.delivered += pay
	return true
}

// wants: the Room's Party asks for item.
func (g *Game) wants(room int, item Unlock) bool {
	for _, p := range g.parties {
		if p.Occupies(room, Human) && p.asking() && p.state == staying && p.want == item {
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

// drawHands draws the held item over the Concierge's head; (x,y) are the feet, screen px, as drawCharacter's.
func (g *Game) drawHands(screen *ebiten.Image, x, y int) {
	g.drawHeld(screen, g.supplyIcon(g.hands), x, y)
}

// drawHeld draws icon over the head of whoever's feet are at (x,y), screen px.
func (g *Game) drawHeld(screen, icon *ebiten.Image, x, y int) {
	top := y + (entities.FootH/2-charH+8)*zoom // just over the hair: the sheet's head starts on row 9
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(float64(x-icon.Bounds().Dx()*zoom/2), float64(top-icon.Bounds().Dy()*zoom))
	screen.DrawImage(icon, op)
}
