package game

import (
	"image"
	"testing"

	"ghosthotel/world"
)

var kitchenGrids = [][2]int{{2, 1}, {3, 3}}

// T293: the kitchen is the Lobby's part east of the Archway, walled off, with every Supply's prop on its back wall: Water at the
// fridge, Food at the oven, Towels at the shelf. The Concierge and the courier reach each from the Lobby.
func TestKitchen(t *testing.T) {
	want := map[Unlock]string{SupplyWater: "fridge", SupplyFood: "cabinet_mirror", SupplyTowel: "shelf_a"}
	for _, grid := range kitchenGrids {
		g, _ := headless(t, grid[0], grid[1], 1)
		g.startRun()
		h := g.hotel
		if !h.Kitchen.In(h.Lobby) || h.Kitchen.Max != h.Lobby.Max || h.Kitchen.Dy() != h.Lobby.Dy() || h.Kitchen.Min.X < h.Archway.X+world.TileSize {
			t.Fatalf("%v: the kitchen %v is not the Lobby's %v east part, short of the Archway at %v", grid, h.Kitchen, h.Lobby, h.Archway)
		}
		for y := h.Kitchen.Min.Y / world.TileSize; y < h.Kitchen.Max.Y/world.TileSize; y++ { // its west wall, but for the doorway
			x := h.Kitchen.Min.X/world.TileSize - 1
			if open := !h.Map.Solid(x, y); open != (h.Path(h.ConciergeStart, image.Pt(x*world.TileSize+8, y*world.TileSize+8), -1) != nil) || open && h.Map.Solid(x+1, y) {
				t.Errorf("%v: the kitchen's west wall cell (%d,%d): open %v, want a wall or a doorway one walks through", grid, x, y, open)
			}
		}
		// nobody walks round the wall's south end, behind the front face's top row
		if x, y := h.Kitchen.Min.X-3, h.Kitchen.Max.Y+8; !h.Map.BlockedBox(image.Rect(x-5, y-3, x+5, y+3)) {
			t.Errorf("%v: the kitchen's west wall is open at (%d,%d), where it meets the front face", grid, x, y)
		}
		for k, s := range supplies {
			spot := supplySpot(h, k)
			if s.Prop != want[s.ID] || !spot.In(h.Kitchen) {
				t.Errorf("%v: %s is fetched at %q in %v, want %q inside the kitchen %v", grid, s.ID, s.Prop, spot, want[s.ID], h.Kitchen)
			}
			for name, to := range map[string]image.Point{"the Concierge": supplyFront(g, k), "the courier": g.supplyDrop(k)} {
				if !to.In(spot) || h.Path(h.ConciergeStart, to, -1) == nil || h.Path(h.Spawn, to, -1) == nil {
					t.Errorf("%v: %s can't reach %s at %v", grid, name, s.ID, to)
				}
			}
		}
		for _, d := range kitchenDecor {
			if img := g.sprite(d.Prop); img == nil {
				t.Errorf("%v: no sprite %q", grid, d.Prop)
			}
		}
		taken := map[int]string{} // every prop on its own cells of the row
		for _, p := range append(kitchenDecor, supplies...) {
			for x := p.X; x < p.X+p.W; x++ {
				if taken[x] != "" || x >= world.KitchenW {
					t.Errorf("%v: %s on cell %d, which is %q's or outside the row", grid, p.Prop, x, taken[x])
				}
				taken[x] = p.Prop
			}
		}
	}
}

// T294: a Supply's icon and count show only with the Concierge within stockNear cells of the prop.
func TestStockCount(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	for k := range supplies {
		at := supplyFront(g, k)
		for d, want := range map[int]bool{0: true, stockNear: true, stockNear + 1: false} {
			g.player.X, g.player.Y = float64(at.X), float64(at.Y+d*world.TileSize)
			if g.stockShown(k) != want {
				t.Errorf("supply %d, %d cells away: count shown %v, want %v", k, d, !want, want)
			}
		}
	}
	g.player.X, g.player.Y = float64(g.hotel.ConciergeStart.X), float64(g.hotel.ConciergeStart.Y)
	for k := range supplies {
		if g.stockShown(k) {
			t.Errorf("supply %d's count shows from behind the counter", k)
		}
	}
}

// T311: the courier stops at each ordered Supply's prop in turn, and a prop is filled when he reaches it.
func TestCourierStops(t *testing.T) {
	g := storeRun(t)
	g.wallet[Money] = 10000
	for _, id := range []Unlock{SupplyTowel, SupplyWater, SupplyFood} {
		if why := g.addToCart(row(t, id), -1); why != "" {
			t.Fatal(why)
		}
	}
	if why := g.placeOrder(); why != "" {
		t.Fatal(why)
	}
	c := g.couriers[0]
	if len(c.drops) != len(supplies) {
		t.Fatalf("%d stops, want one per Supply ordered", len(c.drops))
	}
	clear(g.stock)
	filled := map[int]bool{}
	g.state = StatePlay
	for f := 0; len(g.couriers) > 0; f++ {
		if f > 3*60*tps {
			t.Fatal("the courier is still out after 3 minutes")
		}
		g.step(0, 0)
		for k, s := range supplies {
			if g.stock[s.ID] > 0 && !filled[k] { // just filled: he stands at this prop
				if at := image.Pt(int(c.X), int(c.Y)); !at.In(supplySpot(g.hotel, k)) {
					t.Fatalf("%s was filled with the courier at %v, not at its prop %v", s.ID, at, supplySpot(g.hotel, k))
				}
				filled[k] = true
			}
		}
	}
	if len(filled) != len(supplies) {
		t.Fatalf("%d Supplies filled, want %d", len(filled), len(supplies))
	}
}

// T312: the Lobby Mirror hangs on the Hall Wall 2 cells west of the Archway; its spot is in front of it,
// clear of the Terminal, the counter and the kitchen, and the Concierge reaches it.
func TestLobbyMirrorEast(t *testing.T) {
	for _, grid := range kitchenGrids {
		g, _ := headless(t, grid[0], grid[1], 1)
		g.startRun()
		h, m := g.hotel, g.hotel.Map
		const ts = world.TileSize
		c := h.Mirror.Div(ts)
		east := 0 // Hall Wall cells between the Archway and the Mirror
		for x := c.X + 1; m.Wall(x, c.Y-1); x++ {
			east++
		}
		if name := m.TS.Tiles[m.At(world.LayerDecor, c.X, c.Y-1)].Name; name != "prop_mirror_r1c0" || east != 1 {
			t.Errorf("%v: %q over the Mirror's cell, %d wall cells from the Archway; want the Mirror 2 cells west of it", grid, name, east)
		}
		if !h.Mirror.In(h.MirrorSpot) || h.MirrorSpot.Overlaps(h.Terminal) || h.MirrorSpot.Overlaps(h.Kitchen) || h.MirrorSpot.Overlaps(h.Stock) {
			t.Errorf("%v: the Mirror's spot %v: want it under the Mirror %v, clear of the Terminal %v and the kitchen", grid, h.MirrorSpot, h.Mirror, h.Terminal)
		}
		for x := h.MirrorSpot.Min.X / ts; x < h.MirrorSpot.Max.X/ts; x++ {
			if m.Solid(x, c.Y) {
				t.Errorf("%v: the Mirror's spot cell (%d,%d) is solid", grid, x, c.Y)
			}
		}
		if h.Path(h.ConciergeStart, h.Mirror, -1) == nil || h.Path(h.Spawn, h.Mirror, -1) == nil {
			t.Errorf("%v: the Mirror at %v can't be reached", grid, h.Mirror)
		}
	}
}
