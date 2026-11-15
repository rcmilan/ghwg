package game

import (
	"image"
	"math"
	"testing"

	"ghosthotel/world"
)

// deliver runs the world until every courier has left.
func deliver(t *testing.T, g *Game) {
	t.Helper()
	g.state = StatePlay
	for f := 0; len(g.couriers) > 0; f++ {
		if f > 3*60*tps {
			t.Fatalf("a courier is still out after 3 minutes, at %v", image.Pt(int(g.couriers[0].X), int(g.couriers[0].Y)))
		}
		g.step(0, 0)
	}
}

// T238: checkout sends one courier from the Entrance. A Supply arrives when he reaches its prop, an unlock
// when he reaches the kitchen's drop, nothing before; a Priest starts at checkout. He waits while the
// Terminal is open (D73), then leaves through the Entrance.
func TestCourier(t *testing.T) {
	g := storeRun(t)
	g.unlock(Mirror)
	g.earned, g.wallet = [2]Num{1000, 1000}, [2]Num{1000, 1000}
	water, lock := row(t, SupplyWater), row(t, RoomLock)
	for _, add := range []struct{ i, room int }{{water, -1}, {lock, -1}, {row(t, rowFatherK), 0}} {
		if why := g.addToCart(add.i, add.room); why != "" {
			t.Fatalf("add %s: %q", shop[add.i].ID, why)
		}
	}
	if why := g.placeOrder(); why != "" {
		t.Fatal(why)
	}
	if !g.priestOn(0) {
		t.Fatal("the Priest didn't start at checkout")
	}
	if len(g.couriers) != 1 || g.stock[SupplyWater] != 0 || g.unlocked(RoomLock) {
		t.Fatalf("at checkout: %d couriers, stock %v, Room lock %v; want one courier and nothing arrived", len(g.couriers), g.stock, g.unlocked(RoomLock))
	}
	if g.addToCart(lock, -1) == "" {
		t.Fatal("the cart took again what the courier is bringing")
	}
	c := g.couriers[0]
	if at := image.Pt(int(c.X), int(c.Y)); at != g.hotel.Spawn {
		t.Fatalf("the courier starts at %v, want the Entrance's %v", at, g.hotel.Spawn)
	}
	for range 60 {
		g.Update()
	}
	if at := image.Pt(int(c.X), int(c.Y)); at != g.hotel.Spawn {
		t.Fatal("the courier walked while the Terminal was open")
	}
	near := func(p image.Point) bool { return math.Hypot(c.X-float64(p.X), c.Y-float64(p.Y)) <= world.TileSize }
	g.state = StatePlay
	stocked, locked := false, false
	for f := 0; len(g.couriers) > 0; f++ {
		if f > 3*60*tps {
			t.Fatal("the courier never left")
		}
		g.step(0, 0)
		if !stocked && g.stock[SupplyWater] > 0 {
			stocked = true
			if g.stock[SupplyWater] != shop[water].Pack || g.unlocked(RoomLock) || !near(g.supplyDrop(supplyOf(SupplyWater))) {
				t.Fatalf("stock %v, Room lock %v with the courier at %v; want the Water alone, at the fridge", g.stock, g.unlocked(RoomLock), image.Pt(int(c.X), int(c.Y)))
			}
		}
		if !locked && g.unlocked(RoomLock) {
			if locked = true; !stocked || !near(g.stockDrop()) {
				t.Fatalf("the Room lock arrived with the courier at %v, Water stocked %v; want it after the Water, at the kitchen's drop", image.Pt(int(c.X), int(c.Y)), stocked)
			}
		}
	}
	if !stocked || !locked {
		t.Fatal("the courier never brought the stock")
	}
	if !near(g.hotel.Exit) {
		t.Fatalf("the courier left at %v, not the Entrance", image.Pt(int(c.X), int(c.Y)))
	}
}

// An order of Priests only sends no courier.
func TestNoCourierForPriests(t *testing.T) {
	g := storeRun(t)
	g.unlock(Mirror)
	g.earned = [2]Num{1000, 1000}
	g.wallet = [2]Num{1000, 0}
	if g.addToCart(row(t, rowFatherM), 1) != "" || g.placeOrder() != "" {
		t.Fatal("couldn't buy Father M")
	}
	if !g.priestOn(1) || len(g.couriers) != 0 {
		t.Fatalf("Priest on 1 %v, %d couriers; want the Priest and no courier", g.priestOn(1), len(g.couriers))
	}
}
