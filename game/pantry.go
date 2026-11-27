package game

import (
	"fmt"
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"ghosthotel/world"
)

// The pantry (08 P1-U): a HOTEL upgrade, on sale from prestige level 1, that stands a shelf mid-corridor on each
// Corridor a New floor made (world.SetPantries). It has no stock of its own: it shows the kitchen's. While a Room asks
// for a Supply the kitchen has, SPACE at a pantry takes the one asked by the asking Room nearest to it; a housekeeper
// fetches from the nearer of the kitchen and the pantries. Buying Supplies stays in the kitchen. Prices are placeholders.
const (
	Pantry      Unlock = "pantry"
	pantryMoney        = 250
	pantryEcto         = 30
)

// levelUnlocks: what each prestige level unlocks besides the bigger grid, by level (08 grill 6). Later slices append.
var levelUnlocks = [...]Unlock{1: Pantry}

// unlockLevel: the prestige level that opens id, 0 if none does.
func unlockLevel(id Unlock) int {
	for lv, u := range levelUnlocks {
		if u == id && id != "" {
			return lv
		}
	}
	return 0
}

// levelHint: the Terminal's teaser for an item a level opens.
func levelHint(id Unlock) string { return fmt.Sprintf("Prestige level %d to unlock.", unlockLevel(id)) }

// hasPantry: bought this Run (a game without a lock table, as some tests', has none).
func (g *Game) hasPantry() bool { return g.locked != nil && g.unlocked(Pantry) }

// openPantries stands the shelves the hotel has Corridors for; a growth and the purchase call it.
func (g *Game) openPantries() { g.hotel.SetPantries(g.hasPantry()) }

// boughtPantry: the pantry arrives.
func (g *Game) boughtPantry() {
	g.own(Pantry)
	g.openPantries()
}

// pantryDrop: the center of pantry p's Spot, where whoever fetches from it stands.
func (g *Game) pantryDrop(p int) image.Point {
	s := g.hotel.Pantries[p].Spot
	return image.Pt((s.Min.X+s.Max.X)/2, (s.Min.Y+s.Max.Y)/2)
}

// atPantry: the pantry whose Spot the Concierge stands on, or -1.
func (g *Game) atPantry() int {
	for p, s := range g.hotel.Pantries {
		if g.feet().In(s.Spot) {
			return p
		}
	}
	return -1
}

// pantryPick: the Supply SPACE takes at pantry p, as a supply index: the one asked by the asking Room nearest to it
// (straight line, to its door) that the stock has and the hands don't already hold. -1: SPACE does nothing here.
func (g *Game) pantryPick(p int) int {
	from, best, pick := g.pantryDrop(p), 0, -1
	for i := range g.hotel.Rooms {
		d := g.doorDrop(i).Sub(from)
		for k, s := range supplies {
			if g.stock[s.ID] > 0 && g.hands != s.ID && g.wants(i, s.ID) && (pick < 0 || d.X*d.X+d.Y*d.Y < best) {
				best, pick = d.X*d.X+d.Y*d.Y, k
			}
		}
	}
	return pick
}

// pantrySpace: the Supply SPACE fetches at pantry p, as a supply index: in real its pick; failing that, the held
// item when no Room wants it, in either dimension (fetch puts it back in the stock). -1: SPACE does nothing here.
func (g *Game) pantrySpace(p int) int {
	if k := g.pantryPick(p); k >= 0 && !g.ghost {
		return k
	}
	for i := range g.hotel.Rooms {
		if g.wants(i, g.hands) {
			return -1
		}
	}
	return supplyOf(g.hands) // -1 with the hands empty
}

// pantryRect: the shelf's prop, two cells tall, for the glow.
func (g *Game) pantryRect(p int) image.Rectangle {
	c := g.hotel.Pantries[p].Cell
	return image.Rect(c.X*world.TileSize, (c.Y-1)*world.TileSize, (c.X+1)*world.TileSize, (c.Y+1)*world.TileSize)
}

// pantryStride: world px between the centers of two Supplies' marks over a shelf: the widest icon, "xN" and a margin.
const pantryStride = 24

// drawPantryStock: with the Concierge near a pantry, each Supply's icon and count (the kitchen's) side by side over
// its shelf. drawWorld calls it after the props.
func (g *Game) drawPantryStock(screen *ebiten.Image, ox, oy int) {
	for p := range g.hotel.Pantries {
		if !g.nearStock(g.pantryRect(p)) {
			continue
		}
		r := g.pantryRect(p)
		for k, s := range supplies {
			mid := (r.Min.X+r.Max.X)/2 + (2*k-len(supplies)+1)*pantryStride/2
			g.drawStockMark(screen, s.ID, g.stock[s.ID], mid, r.Min.Y, ox, oy)
		}
	}
}

// nearestStock: where a housekeeper k at its spot fetches item from: the nearer, straight line, of the kitchen and
// the pantries that have it. ok is false when none does.
func (g *Game) nearestStock(k *housekeeper, item Unlock) (at image.Point, ok bool) {
	best := 0
	try := func(to image.Point) {
		dx, dy := to.X-int(k.X), to.Y-int(k.Y)
		if d := dx*dx + dy*dy; !ok || d < best {
			at, best, ok = to, d, true
		}
	}
	if g.stock[item] == 0 {
		return
	}
	try(g.supplyDrop(supplyOf(item)))
	for p := range g.hotel.Pantries {
		try(g.pantryDrop(p))
	}
	return
}

// DevPantry is main's -pantry: level 1, the pantry bought, the hotel grown by a floor (use after -grow floor, or
// as alone), the kitchen stocked, a Room asking for Food, and the Concierge on its spot.
func (g *Game) DevPantry() {
	g.level = 1
	g.stock = map[Unlock]int{SupplyWater: 5, SupplyFood: 5, SupplyTowel: 5}
	g.boughtPantry()
	if len(g.hotel.Corridors) == 0 {
		g.grow(false)
	}
	if len(g.hotel.Pantries) == 0 {
		return
	}
	party := g.arrive(0) // Room 101 asks for Food, so the shelf's glow shows
	g.settleIn(party)
	party.asked, party.want = g.clock, SupplyFood
	at := g.pantryDrop(0)
	g.player.X, g.player.Y = float64(at.X), float64(at.Y)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}
