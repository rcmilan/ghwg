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
		t.Fatalf("no shop row %q", id)
	}
	return i
}

// T183 (D180): one table sells the Priests, the Shadow and every unlock; a buy pays its whole mixed Cost
// and runs, an unaffordable one changes nothing.
func TestShopBuy(t *testing.T) {
	for _, id := range []Unlock{Mirror, Terminal, Hex, SupplyWater, SupplyFood, SupplyTowel, Housekeep, RoomLock, Consume, ShiftAnywhere, rowFatherK, rowFatherM, rowShadow} {
		row(t, id)
	}
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.locked = map[Unlock]bool{Housekeep: true}
	hk := shop[row(t, Housekeep)]
	if hk.Cost[Money] == 0 || hk.Cost[Spook] == 0 {
		t.Fatalf("Housekeep costs %v; the test wants a mixed Cost", hk.Cost)
	}
	g.wallet = [2]Num{hk.Cost[Money], hk.Cost[Spook] - 1}
	if msg := g.buy(row(t, Housekeep), 0); msg == "" || g.unlocked(Housekeep) || g.wallet != [2]Num{hk.Cost[Money], hk.Cost[Spook] - 1} {
		t.Fatalf("an unaffordable unlock: %q, unlocked %v, wallet %v", msg, g.unlocked(Housekeep), g.wallet)
	}
	g.wallet[Spook]++
	if msg := g.buy(row(t, Housekeep), 0); msg != "" || !g.unlocked(Housekeep) || g.wallet != [2]Num{} {
		t.Fatalf("an affordable unlock: %q, unlocked %v, wallet %v; want it unlocked and the wallet empty", msg, g.unlocked(Housekeep), g.wallet)
	}

	g.wallet[Money] = priestWeak - 1
	if msg := g.buy(row(t, rowFatherK), 1); msg == "" || len(g.priests) > 0 || g.wallet[Money] != priestWeak-1 {
		t.Fatalf("Father K without the money: %q, %d Priests, $%v", msg, len(g.priests), g.wallet[Money])
	}
	g.wallet[Money]++
	if msg := g.buy(row(t, rowFatherK), 1); msg != "" || !g.priestOn(1) || g.wallet[Money] != 0 {
		t.Fatalf("Father K: %q, on Room 1 %v, $%v", msg, g.priestOn(1), g.wallet[Money])
	}
}

// T184 (D180): a row is listed once its Needs are unlocked and the wallet holds half of each part of its
// Cost; it stays listed after spending back below; a bought unlock leaves the list.
func TestShopReveal(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.locked = map[Unlock]bool{Housekeep: true, Mirror: true}
	i := row(t, Housekeep)
	c := shop[i].Cost
	listed := func(w [2]Num) bool {
		g.wallet = w
		g.reveal()
		return slices.Contains(g.listed(), i)
	}
	if !slices.Contains(shop[i].Needs, Mirror) {
		t.Fatalf("Housekeep needs %v; the test wants Mirror", shop[i].Needs)
	}
	if listed(c) {
		t.Fatal("listed with its Needs still locked")
	}
	g.unlock(Mirror)
	for _, w := range [][2]Num{{c[Money] * .49, c[Spook]}, {c[Money], c[Spook] * .49}} {
		if listed(w) {
			t.Fatalf("listed at %v, under half of %v", w, c)
		}
	}
	if !listed([2]Num{c[Money] / 2, c[Spook] / 2}) {
		t.Fatalf("not listed at half of %v", c)
	}
	if !listed([2]Num{}) {
		t.Fatal("unlisted after spending back below half")
	}
	g.wallet = c
	g.buy(i, 0)
	if g.reveal(); slices.Contains(g.listed(), i) {
		t.Fatal("a bought unlock is still listed")
	}
}

// T185: W/S picks a row, A/D the Room, SPACE buys, in either dimension.
func TestShopKeys(t *testing.T) {
	for _, ghost := range []bool{false, true} {
		g, s := headless(t, 2, 1, 1)
		g.startRun()
		g.ghost = ghost
		g.wallet = [2]Num{priestWeak, shadowPrice}
		tap := func(k ebiten.Key) {
			s.hold()
			g.Update()
			s.hold(k)
			g.Update()
		}
		g.openTerminal()
		if got, want := g.listed(), []int{row(t, rowFatherK), row(t, rowShadow)}; !slices.Equal(got, want) {
			t.Fatalf("listed %v, want Father K and the Shadow %v", got, want)
		}
		for _, k := range []ebiten.Key{ebiten.KeyD, ebiten.KeyS, ebiten.KeySpace} { // Room 1, down to the Shadow, buy
			tap(k)
		}
		if g.state != StateTerminal || !g.shaded(1) || g.shaded(0) || g.wallet[Spook] != 0 {
			t.Fatalf("ghost %v: open %v, shaded %v %v, Spook %v; want a Shadow in Room 1 only", ghost, g.state == StateTerminal, g.shaded(0), g.shaded(1), g.wallet[Spook])
		}
	}
}
