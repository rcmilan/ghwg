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
// without the pantry; the old ones stay as the hotel grows.
func TestPantryPlacement(t *testing.T) {
	g := pantryRun(t, 2)
	h := g.hotel
	if len(h.Pantries) != 2 || len(h.Corridors) != 2 {
		t.Fatalf("%d pantries, %d Corridor Mirrors, want 2 and 2", len(h.Pantries), len(h.Corridors))
	}
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
	if len(g.hotel.Pantries) != 3 {
		t.Errorf("after growing: %d pantries, want 3", len(g.hotel.Pantries))
	}
	g.locked[Pantry] = true // not bought: no shelf on a new floor
	if _, err := g.grow(false); err != nil {
		t.Fatal(err)
	}
	if len(g.hotel.Pantries) != 0 {
		t.Errorf("without the pantry: %d shelves", len(g.hotel.Pantries))
	}
}

// asks puts a Party in Room i asking for item.
func asks(g *Game, i int, item Unlock) {
	p := g.arrive(i)
	g.settleIn(p)
	p.asked, p.want, p.state = g.clock, item, staying
}

// SPACE at a pantry does nothing until a Room asks for a Supply the kitchen has, then takes the one asked by the
// asking Room nearest to it, from the shared stock; what is in the hands goes back to it.
func TestPantryConcierge(t *testing.T) {
	g := pantryRun(t, 1)
	g.stock = map[Unlock]int{SupplyWater: 2, SupplyTowel: 1}
	spot := center(g.hotel.Pantries[0].Spot)
	g.player.X, g.player.Y = float64(spot.X), float64(spot.Y)
	if act, _ := g.actionable(); act == actSupply {
		t.Error("a stocked pantry offers a Supply with nobody asking")
	}
	g.space()
	if g.hands != "" {
		t.Errorf("SPACE with nobody asking took %q", g.hands)
	}
	near, far := 0, 1
	d := func(i int) int {
		v := g.doorDrop(i).Sub(g.pantryDrop(0))
		return v.X*v.X + v.Y*v.Y
	}
	if d(far) < d(near) {
		near, far = far, near
	}
	asks(g, near, SupplyTowel)
	asks(g, far, SupplyWater)
	if act, r := g.actionable(); act != actSupply || r != g.pantryRect(0) {
		t.Fatalf("an asked-for Supply in stock: %v %v, want actSupply and its shelf", act, r)
	}
	g.space()
	if g.hands != SupplyTowel || g.stock[SupplyTowel] != 0 {
		t.Fatalf("took %q, kitchen Towels %d; want the nearest Room's Towels, 0 left", g.hands, g.stock[SupplyTowel])
	}
	g.space() // Towels in hand: the other Room's Water is next, the Towels going back
	if g.hands != SupplyWater || g.stock[SupplyTowel] != 1 || g.stock[SupplyWater] != 1 {
		t.Errorf("swap: hands %q, Towels %d, Water %d; want Water, 1, 1", g.hands, g.stock[SupplyTowel], g.stock[SupplyWater])
	}
	g.stock[SupplyWater], g.stock[SupplyTowel] = 0, 0
	if act, _ := g.actionable(); act == actSupply {
		t.Error("nothing asked is stocked, the pantry still offers")
	}
}

// A housekeeper fetches from the nearer of the kitchen and a pantry; both show the one stock.
func TestPantryNearestSource(t *testing.T) {
	g := pantryRun(t, 1)
	g.stock = map[Unlock]int{SupplyWater: 1}
	g.hire()
	k := g.keepers[0]
	for _, c := range []struct {
		name string
		from image.Point
		want image.Point
	}{{"by the pantry", g.pantryDrop(0).Add(image.Pt(0, 8)), g.pantryDrop(0)}, {"in the kitchen", g.kitchenSpot(), g.supplyDrop(supplyOf(SupplyWater))}} {
		k.X, k.Y = float64(c.from.X), float64(c.from.Y)
		if at, ok := g.nearestStock(k, SupplyWater); !ok || at != c.want {
			t.Errorf("%s: at %v, want %v", c.name, at, c.want)
		}
	}
	g.stock[SupplyWater] = 0
	if _, ok := g.nearestStock(k, SupplyWater); ok {
		t.Error("nobody has it: a source")
	}
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

// 10: holding an item no Room wants, SPACE at a pantry puts it back in the kitchen's stock (the shelf glows), in
// either dimension; an item a Room still wants stays in the hands.
func TestPantryPutBack(t *testing.T) {
	for _, ghost := range []bool{false, true} {
		g := pantryRun(t, 1)
		g.stock, g.hands, g.ghost = map[Unlock]int{SupplyTowel: 0}, SupplyTowel, ghost
		spot := center(g.hotel.Pantries[0].Spot)
		g.player.X, g.player.Y = float64(spot.X), float64(spot.Y)
		asks(g, 0, SupplyTowel)
		if act, _ := g.actionable(); act == actSupply {
			t.Errorf("ghost=%v: a Room wants the Towels in hand, the pantry still offers to take them", ghost)
		}
		g.parties = nil // the ask ended
		if act, r := g.actionable(); act != actSupply || r != g.pantryRect(0) {
			t.Fatalf("ghost=%v: holding Towels nobody wants: %v %v, want actSupply and its shelf", ghost, act, r)
		}
		g.space()
		if g.hands != "" || g.stock[SupplyTowel] != 1 {
			t.Errorf("ghost=%v: put back: hands %q, Towels %d; want empty, 1", ghost, g.hands, g.stock[SupplyTowel])
		}
		if act, _ := g.actionable(); act == actSupply {
			t.Errorf("ghost=%v: empty hands, nobody asking, the pantry still offers", ghost)
		}
		asks(g, 0, SupplyTowel) // a Room asks for what the stock has: the pantry hands it out in real only
		if act, _ := g.actionable(); (act == actSupply) == ghost {
			t.Errorf("ghost=%v: an asked-for Supply in stock: %v", ghost, act)
		}
	}
}
