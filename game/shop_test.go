package game

import (
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func row(t *testing.T, id Unlock) int {
	t.Helper()
	i := slices.IndexFunc(shop, func(it Item) bool { return it.ID == id })
	if i < 0 {
		t.Fatalf("no store item %q", id)
	}
	return i
}

// storeRun: a Run with every Hotel unlock still to buy, as -shop starts it.
func storeRun(t *testing.T) *Game {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.ShopPrint(0, 0)
	return g
}

// T183 (2026-11-08): every item is in the catalog; the basics (the Supplies and the Mirror) are on sale
// from the start; the rest is coming soon.
func TestStoreCatalog(t *testing.T) {
	g := storeRun(t)
	basic := []Unlock{SupplyWater, SupplyFood, SupplyTowel, Mirror}
	for _, id := range []Unlock{SupplyWater, SupplyFood, SupplyTowel, Mirror, Hex, Housekeep, RoomLock, Consume, ShiftAnywhere, rowFatherK, rowFatherM, rowShadow} {
		if got := g.onSale(row(t, id)); got != slices.Contains(basic, id) {
			t.Errorf("%s on sale at the start: %v", id, got)
		}
	}
}

// T184: an item opens once its Needs are owned and the hotel has earned its Earned since the last of
// them arrived, so buying never opens another item by itself.
func TestStoreOpens(t *testing.T) {
	g := storeRun(t)
	hex := row(t, Hex)
	g.earn(Spook, 1000, noFloat) // earned before the Mirror: doesn't count
	g.wallet[Money] = 10
	if g.addToCart(row(t, Mirror), -1) != "" || g.placeOrder() != "" {
		t.Fatal("couldn't buy the Mirror")
	}
	if deliver(t, g); !g.unlocked(Mirror) {
		t.Fatal("the Mirror didn't arrive")
	}
	if g.onSale(hex) {
		t.Fatal("buying the Mirror opened Hex at once")
	}
	g.earn(Spook, shop[hex].Earned[Spook]-1, noFloat)
	if g.onSale(hex) {
		t.Fatal("Hex open under its threshold")
	}
	g.earn(Spook, 1, noFloat)
	if !g.onSale(hex) {
		t.Fatal("Hex not open at its threshold")
	}
}

// T183: checkout pays the whole cart or nothing; Supplies go into stock by the case, unlocks are owned,
// Services start in their Room.
func TestCheckout(t *testing.T) {
	g := storeRun(t)
	g.unlock(Mirror)
	g.earned, g.wallet = [2]Num{1000, 1000}, [2]Num{1000, 1000}
	water, k := row(t, SupplyWater), row(t, rowFatherK)
	for _, add := range []struct{ i, room int }{{water, -1}, {water, -1}, {row(t, Hex), -1}, {k, 1}} {
		if why := g.addToCart(add.i, add.room); why != "" {
			t.Fatalf("add %s: %q", shop[add.i].ID, why)
		}
	}
	total := g.cartTotal()
	if want := (Cost{2*shop[water].Cost[Money] + shop[row(t, Hex)].Cost[Money] + priestWeak}); total != want {
		t.Fatalf("total %v, want %v", total, want)
	}
	g.wallet = [2]Num{total[Money] - 1, 0}
	if g.placeOrder() == "" || len(g.cart) != 4-1 || g.unlocked(Hex) {
		t.Fatal("a short wallet still bought something")
	}
	g.wallet[Money]++
	if why := g.placeOrder(); why != "" || g.wallet[Money] != 0 || len(g.cart) != 0 {
		t.Fatalf("checkout: %q, $%v, cart %v", why, g.wallet[Money], g.cart)
	}
	if !g.priestOn(1) || g.priestOn(0) {
		t.Fatalf("Priest on 1 %v, on 0 %v; want on 1", g.priestOn(1), g.priestOn(0))
	}
	deliver(t, g) // E8: the courier brings the rest
	if g.stock[SupplyWater] != 2*shop[water].Pack || !g.unlocked(Hex) {
		t.Fatalf("delivered: stock %v, Hex %v", g.stock, g.unlocked(Hex))
	}
}

// The cart refuses an owned unlock, a second copy of one, and a second Priest or Shadow for a Room.
func TestCartRules(t *testing.T) {
	g := storeRun(t)
	g.unlock(Mirror)
	g.earned, g.wallet = [2]Num{1000, 1000}, [2]Num{1000, 1000}
	m, k, fm, sh := row(t, Mirror), row(t, rowFatherK), row(t, rowFatherM), row(t, rowShadow)
	if g.addToCart(m, -1) == "" {
		t.Error("an owned unlock went in the cart")
	}
	if g.addToCart(row(t, Hex), -1) != "" || g.addToCart(row(t, Hex), -1) == "" {
		t.Error("an unlock went in the cart twice")
	}
	if g.addToCart(k, 0) != "" || g.addToCart(fm, 0) == "" || g.addToCart(fm, 1) != "" {
		t.Error("Priests: want one per Room")
	}
	g.placeShadow(1)
	if g.addToCart(sh, 1) == "" || g.addToCart(sh, 0) != "" {
		t.Error("Shadow: want refused where one is, taken where none is")
	}
	g.cart, g.wallet = nil, [2]Num{15, 0}
	w := row(t, SupplyWater)
	if g.addToCart(w, -1) != "" || g.addToCart(w, -1) == "" || g.addToCart(sh, 0) == "" {
		t.Error("the cart took more than the wallet holds")
	}
}

// T185: A/D picks the tab, W/S the row, SPACE adds (a Service asks its Room), the cart's last row checks
// out; in either dimension.
func TestStoreKeys(t *testing.T) {
	for _, ghost := range []bool{false, true} {
		g, s := headless(t, 2, 1, 1)
		g.startRun()
		g.ghost = ghost
		g.earned = [2]Num{1000, 1000}
		g.wallet = [2]Num{0, shadowPrice}
		tap := func(keys ...ebiten.Key) {
			for _, k := range keys {
				s.hold()
				g.Update()
				s.hold(k)
				g.Update()
			}
		}
		g.openTerminal()
		pos := slices.Index(tabItems(tabServices), row(t, rowShadow))
		tap(ebiten.KeyD, ebiten.KeyD) // Services
		for range pos {
			tap(ebiten.KeyS)
		}
		tap(ebiten.KeySpace, ebiten.KeyD, ebiten.KeySpace) // the Shadow, Room 1, in the cart
		if len(g.cart) != 1 || g.placing >= 0 {
			t.Fatalf("ghost %v: cart %v, placing %d; want the Shadow in the cart", ghost, g.cart, g.placing)
		}
		tap(ebiten.KeyD, ebiten.KeyS, ebiten.KeySpace) // the cart, its checkout row, place the order
		if g.state != StateTerminal || !g.coming(rowShadow, 1) || g.coming(rowShadow, 0) || g.wallet[Spook] != 0 {
			t.Fatalf("ghost %v: open %v, shaded %v %v, Spook %v; want a Shadow in Room 1 only", ghost, g.state == StateTerminal, g.shaded(0), g.shaded(1), g.wallet[Spook])
		}
		tap(ebiten.KeyEscape)
		if g.state != StatePlay {
			t.Fatal("ESC didn't close the store")
		}
	}
}
