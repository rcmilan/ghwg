package game

import (
	"encoding/json"
	"image"
	"slices"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// itemsRun: a 2x1 Run with Rooms open, the Mirror bought, the given items owned.
func itemsRun(t *testing.T, ids ...itemID) *Game {
	t.Helper()
	g, _ := mirrorRun(t, 2, 1)
	g.roomOwned = nil
	for _, id := range ids {
		g.questRec.Items[id] = true
	}
	return g
}

func shopIndex(id Unlock) int {
	for i, it := range shop {
		if it.ID == id {
			return i
		}
	}
	return -1
}

func TestItemsOffUnowned(t *testing.T) {
	g := itemsRun(t)
	g.ghost = true
	g.items.Ward, g.items.Gerald = 1, 1
	if g.conciergeBoost() != 1 || g.warded(0) || g.stormJarOn() || g.revealCost() == 0 || g.sightingPay(0) != 1 {
		t.Error("an item works unowned")
	}
}

func TestShowDontTell(t *testing.T) {
	if g := itemsRun(t, itemBall); g.revealCost() != 0 {
		t.Error("REVEAL costs with Show, don't tell")
	}
}

func TestBroom(t *testing.T) {
	g := itemsRun(t, itemBroom)
	if g.conciergeBoost() != 1 {
		t.Error("the Broom works in real")
	}
	g.ghost = true
	if g.conciergeBoost() != broomBoost {
		t.Errorf("boost %v in ghost, want %v", g.conciergeBoost(), broomBoost)
	}
}

func TestGeraldAndStormPay(t *testing.T) {
	if itemPay(false, false) != 1 || itemPay(true, false) != geraldMul || itemPay(false, true) != stormMul || itemPay(true, true) != geraldMul*stormMul {
		t.Error("itemPay")
	}
	pay := func(g *Game, room int) Num {
		p := settledHuman(t, g, room, 5)
		before := g.wallet[Ecto]
		g.frighten(p, p.Guests[0], 6, 99)
		return g.wallet[Ecto] - before
	}
	base := pay(itemsRun(t), 0)
	g := itemsRun(t, itemGerald)
	g.items.Gerald = 1
	if got := pay(g, 0); got != base*geraldMul {
		t.Errorf("Gerald's Room paid %v, want %v", got, base*geraldMul)
	}
	if got := pay(g, 1); got != base {
		t.Errorf("another Room paid %v with Gerald elsewhere, want %v", got, base)
	}
}

func TestWard(t *testing.T) {
	g := itemsRun(t, itemWard)
	h0, h1 := settledHuman(t, g, 0, 5), settledHuman(t, g, 1, 5)
	g.items.Ward = 1
	if g.mayFlee(h0, h0.Guests[0]) || !g.mayFlee(h1, h1.Guests[0]) || !g.warded(0) || g.warded(1) {
		t.Error("the Ward must hold Room 0 only")
	}
}

func TestStormJarOncePerDay(t *testing.T) {
	g := itemsRun(t, itemStorm)
	g.earned = [2]Num{1000, 1000}
	i := shopIndex(rowStorm)
	if !g.stormJarOn() || !g.onSale(i) {
		t.Fatal("the jar is not on sale")
	}
	if why := g.addToCart(i, -1); why != "" {
		t.Fatal(why)
	}
	if why := g.placeOrder(); why != "" {
		t.Fatal(why)
	}
	if g.stormJarOn() || g.onSale(i) {
		t.Error("the jar sells twice a day")
	}
	g.clock += dayFrames
	if !g.stormJarOn() {
		t.Error("the jar is not back the next day")
	}
}

func TestCatCleansSpiritualOnly(t *testing.T) {
	g := itemsRun(t, itemCat)
	g.addDirtLevel(0, 10)
	g.ghostDirt(0, 10)
	count := func() (n, s int) {
		for _, sp := range g.spots(0) {
			if sp.spirit {
				s++
			} else {
				n++
			}
		}
		return
	}
	n0, s0 := count()
	if n0 == 0 || s0 == 0 {
		t.Fatalf("setup: %d normal %d spiritual spots", n0, s0)
	}
	for range 40 * tps * 10 {
		g.step(0, 0)
		if g.items.cat != nil && len(g.parties) == 0 {
			if _, s := count(); s == 0 {
				break
			}
		}
	}
	if n, s := count(); s != 0 || n != n0 {
		t.Errorf("after the Cat: %d normal (was %d), %d spiritual; want the normal ones left, none spiritual", n, n0, s)
	}
}

func TestCatNapsTwiceAShift(t *testing.T) {
	g := itemsRun(t, itemCat)
	naps, was := 0, false
	for range 4 * hourFrames {
		g.clock++
		g.step(0, 0)
		if c := g.items.cat; c != nil {
			if c.napping && !was {
				naps++
			}
			was = c.napping
		}
	}
	if naps != catNaps {
		t.Errorf("%d naps in 4 hours, want %d", naps, catNaps)
	}
}

func TestCatArrivesEachRun(t *testing.T) {
	g := itemsRun(t, itemCat)
	g.step(0, 0)
	if g.items.cat == nil {
		t.Fatal("no Cat")
	}
	g.clock += dayFrames // the old Run ran on
	g.step(0, 0)
	g.items.Gerald = 2
	g.startRun()
	g.step(0, 0)
	if g.items.cat == nil || g.items.Gerald != 0 {
		t.Errorf("new Run: cat %v, Gerald %d; want a fresh Cat and Gerald unplaced", g.items.cat, g.items.Gerald)
	}
	h := itemsRun(t)
	h.step(0, 0)
	if h.items.cat != nil {
		t.Error("a Cat unowned")
	}
}

func TestPlaceOnRing(t *testing.T) {
	g := itemsRun(t, itemGerald, itemWard)
	room := func(r int, id Unlock) bool {
		for _, i := range g.roomRows(r) {
			if shop[i].ID == id {
				return true
			}
		}
		return false
	}
	if !room(0, rowGerald) || !room(1, rowWard) {
		t.Fatal("no place options on the ring")
	}
	if why := g.buyNow(shopIndex(rowGerald), 0); why != "" {
		t.Fatal(why)
	}
	if g.items.Gerald != 1 || room(0, rowGerald) || !room(1, rowGerald) {
		t.Errorf("Gerald %d: must stand in Room 0, hidden there, offered in Room 1", g.items.Gerald)
	}
	if g.buyNow(shopIndex(rowGerald), 1); g.items.Gerald != 2 {
		t.Error("Gerald did not move")
	}
	h := itemsRun(t)
	for _, i := range h.roomRows(0) {
		if shop[i].ID == rowGerald || shop[i].ID == rowWard {
			t.Error("a place option with the item unowned")
		}
	}
}

func TestPlacementSurvivesSaveAndGrowth(t *testing.T) {
	g := itemsRun(t, itemGerald, itemWard)
	g.items.Gerald, g.items.Ward, g.items.Jar = 1, 2, 3
	r := g.checkpoint()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var back runState
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	g2 := itemsRun(t, itemGerald, itemWard)
	if err := g2.restore(&back); err != nil {
		t.Fatal(err)
	}
	if g2.items.Gerald != 1 || g2.items.Ward != 2 || g2.items.Jar != 3 {
		t.Errorf("restored %+v", g2.items)
	}
	if _, err := g2.grow(false); err != nil { // a floor: the Rooms move down a row, 2 columns on
		t.Fatal(err)
	}
	if g2.items.Gerald != 3 || g2.items.Ward != 4 || !g2.warded(3) {
		t.Errorf("after a floor: Gerald %d Ward %d, want 3 and 4", g2.items.Gerald, g2.items.Ward)
	}
}

// The Cat walks by the corridors and doors only: never on a wall cell, over a few days, spiritual dirt in two Rooms.
func TestCatNeverCrossesWalls(t *testing.T) {
	g := itemsRun(t, itemCat)
	for _, r := range []int{0, 1} {
		g.addDirtLevel(r, 10)
		g.ghostDirt(r, 10)
	}
	for i := range 3 * dayFrames {
		g.clock++
		g.step(0, 0)
		if c := g.items.cat; c != nil {
			if cell := g.hotel.Map.Cell(image.Pt(int(c.X), int(c.Y))); g.hotel.Map.Solid(cell.X, cell.Y) {
				t.Fatalf("frame %d: the Cat is on a wall cell %v at %.0f,%.0f", i, cell, c.X, c.Y)
			}
		}
	}
}

// The doors open for the Cat as for any actor (it once walked through shut ones).
func TestCatIsADoorActor(t *testing.T) {
	g := itemsRun(t, itemCat)
	g.updateCat()
	c := g.items.cat
	if !slices.ContainsFunc(g.doorActors(), func(a world.Actor) bool { return a.Box == entities.FootBox(c.X, c.Y) }) {
		t.Error("the Cat is not among the door's actors")
	}
}
