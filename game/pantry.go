package game

import (
	"fmt"
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"ghosthotel/world"
)

// The pantry (08 P1-U): a HOTEL upgrade, on sale from prestige level 1, that stands a shelf mid-corridor on each
// Corridor a New floor made (world.SetPantries). It holds every Supply, pantryStockMax of each. The Concierge takes
// from it as from the kitchen shelf, a housekeeper fetches from the nearer of the two, and an idle one refills it
// from the kitchen's stock. Buying Supplies stays in the kitchen. Prices are placeholders.
const (
	Pantry         Unlock = "pantry"
	pantryMoney           = 250
	pantryEcto            = 30
	pantryStockMax        = 2
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

// openPantries stands the shelves the hotel has Corridors for and gives each its stock; a growth and the purchase call it.
func (g *Game) openPantries() {
	g.hotel.SetPantries(g.hasPantry())
	for len(g.pantryStock) < len(g.hotel.Pantries) {
		g.pantryStock = append(g.pantryStock, map[Unlock]int{})
	}
}

// boughtPantry: the pantry arrives: it starts empty, whatever a Run before left.
func (g *Game) boughtPantry() {
	g.own(Pantry)
	g.pantryStock = nil
	g.openPantries()
}

// stockOf: the stock of src: -1 the kitchen's, else that pantry's. Nil for one the hotel does not have.
func (g *Game) stockOf(src int) map[Unlock]int {
	switch {
	case src < 0:
		return g.stock
	case src < len(g.pantryStock) && src < len(g.hotel.Pantries):
		return g.pantryStock[src]
	}
	return nil
}

// pantryDrop: the center of pantry p's Spot, where whoever fetches from it or fills it stands.
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

// pantryPick: the Supply SPACE swaps or takes at pantry p, as a supply index: the one a Room asks for, else what is in
// the hands (put back), else the first stocked. -1 if SPACE does nothing here.
func (g *Game) pantryPick(p int) int {
	st := g.stockOf(p)
	asked := func(id Unlock) bool {
		for i := range g.hotel.Rooms {
			if g.wants(i, id) {
				return true
			}
		}
		return false
	}
	for k, s := range supplies {
		if st[s.ID] > 0 && g.hands != s.ID && asked(s.ID) {
			return k
		}
	}
	if g.hands != "" {
		return supplyOf(g.hands)
	}
	for k, s := range supplies {
		if st[s.ID] > 0 {
			return k
		}
	}
	return -1
}

// pantryRect: the shelf's prop, two cells tall, for the glow.
func (g *Game) pantryRect(p int) image.Rectangle {
	c := g.hotel.Pantries[p].Cell
	return image.Rect(c.X*world.TileSize, (c.Y-1)*world.TileSize, (c.X+1)*world.TileSize, (c.Y+1)*world.TileSize)
}

// drawPantryStock: with the Concierge near a pantry, each Supply's icon and count over its shelf, stacked (as the
// kitchen shows its stock). drawWorld calls it after the props.
func (g *Game) drawPantryStock(screen *ebiten.Image, ox, oy int) {
	for p := range g.hotel.Pantries {
		if !g.nearStock(g.pantryRect(p)) {
			continue
		}
		r := g.pantryRect(p)
		for k, s := range supplies {
			g.drawStockMark(screen, s.ID, g.stockOf(p)[s.ID], (r.Min.X+r.Max.X)/2, r.Min.Y-k*(g.supplyIcon(s.ID).Bounds().Dy()+1), ox, oy)
		}
	}
}

// fetchPantry is the kitchen shelf's fetch (supply.go) on pantry p's stock.
func (g *Game) fetchPantry(p, k int) {
	kitchen := g.stock
	g.stock = g.stockOf(p)
	g.fetch(k)
	g.stock = kitchen
}

// nearestStock: where a housekeeper k at its spot fetches item from: the nearer, straight line, of the kitchen and
// the pantries that have it. ok is false when none does.
func (g *Game) nearestStock(k *housekeeper, item Unlock) (src int, at image.Point, ok bool) {
	best := 0
	try := func(s int, to image.Point) {
		dx, dy := to.X-int(k.X), to.Y-int(k.Y)
		if d := dx*dx + dy*dy; !ok || d < best {
			src, at, best, ok = s, to, d, true
		}
	}
	if g.stock[item] > 0 {
		try(-1, g.supplyDrop(supplyOf(item)))
	}
	for p := range g.hotel.Pantries {
		if g.stockOf(p)[item] > 0 {
			try(p, g.pantryDrop(p))
		}
	}
	return
}

// refillPantry sends idle k to carry one Supply from the kitchen to a pantry short of it, unless another already is.
func (g *Game) refillPantry(k *housekeeper) bool {
	for p := range g.hotel.Pantries {
		for _, s := range supplies {
			if g.stockOf(p)[s.ID] >= g.pantryMax() || g.stock[s.ID] == 0 || g.refilling(p, s.ID) {
				continue
			}
			k.job, k.src, k.item = refilling, p, s.ID
			g.sendKeeper(k, g.supplyDrop(supplyOf(s.ID)), -1)
			return true
		}
	}
	return false
}

// refilling: a housekeeper is on its way to fill pantry p with item.
func (g *Game) refilling(p int, item Unlock) bool {
	for _, o := range g.keepers {
		if (o.job == refilling || o.job == stocking) && o.src == p && o.item == item {
			return true
		}
	}
	return false
}

// DevPantry is main's -pantry: level 1, the pantry bought, the hotel grown by a floor (use after -grow floor, or
// as alone), the kitchen stocked, the first pantry holding one Food, and the Concierge on its spot.
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
	g.pantryStock[0][SupplyFood] = 1
	at := g.pantryDrop(0)
	g.player.X, g.player.Y = float64(at.X), float64(at.Y)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}
