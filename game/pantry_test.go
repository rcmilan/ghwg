package game

import (
	"image"
	"testing"

	"ghosthotel/world"
)

// pantryRun: a Run at prestige level 1 with the pantry bought and the hotel grown by n floors, an open Room's Party in.
func pantryRun(t *testing.T, floors int) *Game {
	t.Helper()
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.level = 1
	g.boughtPantry()
	for range floors {
		if _, err := g.grow(false); err != nil {
			t.Fatal(err)
		}
	}
	g.state = StatePlay
	return g
}

// 08 P1-U: a prestige level opens the pantry's row, with the level's hint until then.
func TestPantryLevelGate(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	i := row(t, Pantry)
	if g.onSale(i) || shop[i].Hint != "Prestige level 1 to unlock." || unlockLevel(Pantry) != 1 || unlockLevel(Mirror) != 0 {
		t.Errorf("level 0: on sale %v, hint %q, want not, the level's hint", g.onSale(i), shop[i].Hint)
	}
	if why := g.addToCart(i, -1); why != "COMING SOON" {
		t.Errorf("level 0 add to cart: %q", why)
	}
	g.level = 1
	g.wallet[Money], g.wallet[Ecto] = pantryMoney, pantryEcto
	g.state = StateTerminal
	if !g.onSale(i) || g.addToCart(i, -1) != "" || g.placeOrder() != "" {
		t.Fatal("level 1: the pantry is not for sale")
	}
	deliver(t, g)
	if !g.hasPantry() || g.gone(i) != true {
		t.Error("bought: want the pantry owned and its row gone")
	}
}

// A shelf on each Corridor a New floor made, apart from its Mirror, solid, with the Spot below it walkable; none
// without the pantry; the old ones stay, and keep their stock, as the hotel grows.
func TestPantryPlacement(t *testing.T) {
	g := pantryRun(t, 2)
	h := g.hotel
	if len(h.Pantries) != 2 || len(h.Corridors) != 2 {
		t.Fatalf("%d pantries, %d Corridor Mirrors, want 2 and 2", len(h.Pantries), len(h.Corridors))
	}
	g.pantryStock[0][SupplyFood] = 1
	for p, s := range h.Pantries {
		m := h.Corridors[p]
		if dx := s.Cell.X - m.Cell.X; dx > -3 && dx < 3 || s.Cell.Y != m.Spot.Min.Y/world.TileSize || !h.Map.Solid(s.Cell.X, s.Cell.Y) || h.Map.Solid(s.Cell.X, s.Cell.Y+1) {
			t.Errorf("pantry %d at %v, mirror at %v: want 3+ cells apart on its Corridor, solid, its Spot walkable", p, s.Cell, m.Cell)
		}
		if len(h.Path(h.Spawn, image.Pt(s.Spot.Min.X+8, s.Spot.Min.Y+8), -1)) == 0 {
			t.Errorf("pantry %d: no way to its Spot", p)
		}
	}
	if _, err := g.grow(true); err != nil {
		t.Fatal(err)
	}
	if _, err := g.grow(false); err != nil {
		t.Fatal(err)
	}
	if len(g.hotel.Pantries) != 3 || g.pantryStock[0][SupplyFood] != 1 || len(g.pantryStock) != 3 {
		t.Errorf("after growing: %d pantries, old stock %d, want 3 and 1", len(g.hotel.Pantries), g.pantryStock[0][SupplyFood])
	}
	g.locked[Pantry] = true // not bought: no shelf on a new floor
	if _, err := g.grow(false); err != nil {
		t.Fatal(err)
	}
	if len(g.hotel.Pantries) != 0 {
		t.Errorf("without the pantry: %d shelves", len(g.hotel.Pantries))
	}
}

// SPACE at a pantry takes a Supply as the kitchen shelf does (the one asked for first); an empty one does nothing.
func TestPantryConcierge(t *testing.T) {
	g := pantryRun(t, 1)
	g.stock = map[Unlock]int{}
	spot := center(g.hotel.Pantries[0].Spot)
	g.player.X, g.player.Y = float64(spot.X), float64(spot.Y)
	if act, _ := g.actionable(); act == actSupply {
		t.Error("an empty pantry offers a Supply")
	}
	g.pantryStock[0][SupplyWater], g.pantryStock[0][SupplyTowel] = 2, 1
	if act, r := g.actionable(); act != actSupply || r != g.pantryRect(0) {
		t.Fatalf("a stocked pantry: %v %v, want actSupply and its shelf", act, r)
	}
	g.space()
	if g.hands != SupplyWater || g.pantryStock[0][SupplyWater] != 1 || g.stock[SupplyWater] != 0 {
		t.Fatalf("took: hands %q, pantry Water %d, kitchen %d; want Water, 1, 0", g.hands, g.pantryStock[0][SupplyWater], g.stock[SupplyWater])
	}
	g.space() // holding something nobody asks for: SPACE puts it back
	if g.hands != "" || g.pantryStock[0][SupplyWater] != 2 {
		t.Errorf("put back: hands %q, pantry Water %d, want none, 2", g.hands, g.pantryStock[0][SupplyWater])
	}
	p := g.walkIn()
	p.asked, p.want = g.clock, SupplyTowel
	p.state = staying
	g.space() // the Towels are asked for: taken before the Water
	if g.hands != SupplyTowel {
		t.Errorf("a Room asks for Towels: took %q", g.hands)
	}
}

// A housekeeper fetches from the nearer of the kitchen and a pantry that both have the Supply.
func TestPantryNearestSource(t *testing.T) {
	g := pantryRun(t, 1)
	g.stock = map[Unlock]int{SupplyWater: 1}
	g.pantryStock[0][SupplyWater] = 1
	g.hire()
	k := g.keepers[0]
	for _, c := range []struct {
		name string
		at   image.Point
		want int
	}{{"by the pantry", g.pantryDrop(0).Add(image.Pt(0, 8)), 0}, {"in the kitchen", g.kitchenSpot(), -1}} {
		k.X, k.Y = float64(c.at.X), float64(c.at.Y)
		if src, _, ok := g.nearestStock(k, SupplyWater); !ok || src != c.want {
			t.Errorf("%s: source %d, want %d", c.name, src, c.want)
		}
	}
	g.stock[SupplyWater] = 0
	if src, _, ok := g.nearestStock(k, SupplyWater); !ok || src != 0 {
		t.Errorf("only the pantry has it: %d %v", src, ok)
	}
	g.pantryStock[0][SupplyWater] = 0
	if _, _, ok := g.nearestStock(k, SupplyWater); ok {
		t.Error("nobody has it: a source")
	}
}

// With nothing asked, an idle housekeeper carries Supplies from the kitchen to a low pantry, up to its max, and the
// Supplies are only moved.
func TestPantryRefill(t *testing.T) {
	g := pantryRun(t, 1)
	g.stock = map[Unlock]int{SupplyWater: 5, SupplyFood: 1}
	g.pantryStock[0][SupplyTowel] = 2
	g.hire()
	k := g.keepers[0]
	at := g.kitchenSpot()
	k.X, k.Y, k.Path = float64(at.X), float64(at.Y), nil
	g.wallet[Money] = 1000
	for f := 0; g.pantryStock[0][SupplyWater] != pantryStockMax || g.pantryStock[0][SupplyFood] != 1; f++ { // a few game hours of errands
		if f > 20000 {
			t.Fatalf("the pantry holds %v after %d frames, want 2 Water, 1 Food", g.pantryStock[0], f)
		}
		g.step(0, 0)
	}
	if g.stock[SupplyWater] != 5-pantryStockMax || g.stock[SupplyFood] != 0 {
		t.Errorf("kitchen left with Water %d, Food %d, want %d and 0", g.stock[SupplyWater], g.stock[SupplyFood], 5-pantryStockMax)
	}
	until(t, g, "it rests", func() bool { return k.job == resting })
}

// The pantry shows its stock like the kitchen: only with the Concierge within stockNear cells.
func TestPantryStockNear(t *testing.T) {
	g := dressedRun(t, 1)
	g.level = 1
	g.boughtPantry()
	r := g.pantryRect(0)
	g.player.X, g.player.Y = float64(r.Min.X+world.TileSize/2), float64(r.Max.Y+world.TileSize)
	if !g.nearStock(r) {
		t.Error("stock hidden next to the shelf")
	}
	g.player.X += float64((stockNear + 3) * world.TileSize)
	if g.nearStock(r) {
		t.Error("stock shown from afar")
	}
}
