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
	for _, id := range []Unlock{SupplyWater, SupplyFood, SupplyTowel, Mirror, RoomLock, Consume, ShiftAnywhere, rowFatherK, rowFatherM} {
		if got := g.onSale(row(t, id)); got != slices.Contains(basic, id) {
			t.Errorf("%s on sale at the start: %v", id, got)
		}
	}
	for _, it := range shop { // T249, T250: no Hex, no bought Shadow
		if it.ID == "hex" || it.ID == "shadow" || it.Name == "Hex" || it.Name == "Shadow Person" {
			t.Errorf("the store still sells %q", it.Name)
		}
		if len(it.Blurb)*glyphW > paneW-24 {
			t.Errorf("%s: the blurb overflows the detail pane", it.ID)
		}
	}
}

// T184: an item opens once its Needs are owned and the hotel has earned its Earned since the last of
// them arrived, so buying never opens another item by itself.
func TestStoreOpens(t *testing.T) {
	g := storeRun(t)
	lock := row(t, RoomLock)
	g.earn(Ecto, 1000, noFloat) // earned before the Mirror: doesn't count
	g.wallet[Money] = 10
	if g.addToCart(row(t, Mirror), -1) != "" || g.placeOrder() != "" {
		t.Fatal("couldn't buy the Mirror")
	}
	if deliver(t, g); !g.unlocked(Mirror) {
		t.Fatal("the Mirror didn't arrive")
	}
	if g.onSale(lock) {
		t.Fatal("buying the Mirror opened the Room lock at once")
	}
	g.boughtAt[Mirror] = g.earned // ghosts now come in with the Mirror and may earn Ectoplasm while the courier leaves
	g.earn(Ecto, shop[lock].Earned[Ecto]-1, noFloat)
	if g.onSale(lock) {
		t.Fatal("Room lock open under its threshold")
	}
	g.earn(Ecto, 1, noFloat)
	if !g.onSale(lock) {
		t.Fatal("Room lock not open at its threshold")
	}
}

// T183: checkout pays the whole cart or nothing; Supplies go into stock by the case, unlocks are owned,
// Services start in their Room.
func TestCheckout(t *testing.T) {
	g := storeRun(t)
	g.unlock(Mirror)
	g.earned, g.wallet = [2]Num{1000, 1000}, [2]Num{1000, 1000}
	water, k := row(t, SupplyWater), row(t, rowFatherK)
	for _, add := range []struct{ i, room int }{{water, -1}, {water, -1}, {row(t, RoomLock), -1}, {k, 1}} {
		if why := g.addToCart(add.i, add.room); why != "" {
			t.Fatalf("add %s: %q", shop[add.i].ID, why)
		}
	}
	total := g.cartTotal()
	if want := (Cost{2*shop[water].Cost[Money] + shop[row(t, RoomLock)].Cost[Money] + priestWeak, shop[row(t, RoomLock)].Cost[Ecto]}); total != want {
		t.Fatalf("total %v, want %v", total, want)
	}
	g.wallet = [2]Num{total[Money] - 1, total[Ecto]}
	if g.placeOrder() == "" || len(g.cart) != 4-1 || g.unlocked(RoomLock) {
		t.Fatal("a short wallet still bought something")
	}
	g.wallet[Money]++
	if why := g.placeOrder(); why != "" || g.wallet != [2]Num{} || len(g.cart) != 0 {
		t.Fatalf("checkout: %q, $%v, cart %v", why, g.wallet[Money], g.cart)
	}
	if !g.priestOn(1) || g.priestOn(0) {
		t.Fatalf("Priest on 1 %v, on 0 %v; want on 1", g.priestOn(1), g.priestOn(0))
	}
	deliver(t, g) // E8: the courier brings the rest
	if g.stock[SupplyWater] != 2*shop[water].Pack || !g.unlocked(RoomLock) {
		t.Fatalf("delivered: stock %v, Room lock %v", g.stock, g.unlocked(RoomLock))
	}
}

// The cart refuses an owned unlock, a second copy of one, and a second Priest for a Room.
func TestCartRules(t *testing.T) {
	g := storeRun(t)
	g.unlock(Mirror)
	g.earned, g.wallet = [2]Num{1000, 1000}, [2]Num{1000, 1000}
	m, k, fm := row(t, Mirror), row(t, rowFatherK), row(t, rowFatherM)
	if g.addToCart(m, -1) == "" {
		t.Error("an owned unlock went in the cart")
	}
	if g.addToCart(row(t, RoomLock), -1) != "" || g.addToCart(row(t, RoomLock), -1) == "" {
		t.Error("an unlock went in the cart twice")
	}
	if g.addToCart(k, 0) != "" || g.addToCart(fm, 0) == "" || g.addToCart(fm, 1) != "" {
		t.Error("Priests: want one per Room")
	}
	g.cart, g.wallet = nil, [2]Num{15, 0}
	w := row(t, SupplyWater)
	if g.addToCart(w, -1) != "" || g.addToCart(w, -1) == "" {
		t.Error("the cart took more than the wallet holds")
	}
}

// T185: A/D picks the tab, W/S the row, SPACE adds (a Service asks its Room), the cart's last row checks
// out; in either dimension.
func TestStoreKeys(t *testing.T) {
	for _, ghost := range []bool{false, true} {
		g, s := headless(t, 2, 1, 1)
		g.startRun()
		g.unlock(Mirror) // Priests are sold once the Mirror is owned
		g.ghost = ghost
		g.earned = [2]Num{1000, 1000}
		g.wallet = [2]Num{priestWeak, 0}
		tap := func(keys ...ebiten.Key) {
			for _, k := range keys {
				s.hold()
				g.Update()
				s.hold(k)
				g.Update()
			}
		}
		g.openTerminal()
		pos := slices.Index(tabItems(tabServices), row(t, rowFatherK))
		tap(ebiten.KeyD, ebiten.KeyD) // Services
		for range pos {
			tap(ebiten.KeyS)
		}
		tap(ebiten.KeySpace, ebiten.KeyD, ebiten.KeySpace) // Father K, Room 1, in the cart
		if len(g.cart) != 1 || g.placing >= 0 {
			t.Fatalf("ghost %v: cart %v, placing %d; want Father K in the cart", ghost, g.cart, g.placing)
		}
		tap(ebiten.KeyD, ebiten.KeyS, ebiten.KeySpace) // the cart, its checkout row, place the order
		if g.state != StateTerminal || !g.priestOn(1) || g.priestOn(0) || g.wallet[Money] != 0 {
			t.Fatalf("ghost %v: open %v, Priest on %v %v, money %v; want Father K on Room 1 only", ghost, g.state == StateTerminal, g.priestOn(0), g.priestOn(1), g.wallet[Money])
		}
		tap(ebiten.KeyEscape)
		if g.state != StatePlay {
			t.Fatal("ESC didn't close the store")
		}
	}
}
