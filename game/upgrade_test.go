package game

import (
	"math"
	"reflect"
	"testing"
)

func TestUpMulAt(t *testing.T) {
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	for _, c := range []struct {
		u    Upgrade
		n    int
		want float64
	}{
		{LobbyDecor, 0, 1}, {LobbyDecor, 1, 1.04}, {LobbyDecor, 2, 1.076}, {LobbyDecor, 500, 1.4}, // boost: toward 1 + 10e
		{SeanceTable, 1, 1.1}, {SeanceTable, 2, 1.19}, {TipJar, 3, 1.271}, {OfferingBox, 1, 1.4}, {OfferingBox, 2, 1.76},
		{Brochures, 0, 1}, {Brochures, 1, 0.9}, {Brochures, 2, 0.81}, {Soundproofing, 3, 0.729}, // reducer: (1-e)^n
		{CleaningKit, 1, 0.85}, {CleaningKit, 2, 0.7225},
		{LobbyDecor, -1, 1}, // a bad level is no level
	} {
		if got := upMulAt(c.u, c.n); !near(got, c.want) {
			t.Errorf("upMulAt(%s, %d) = %v, want %v", upgrades[c.u].Name, c.n, got, c.want)
		}
	}
}

func TestUpStockAt(t *testing.T) {
	for n, want := range []int{0, 1, 2, 3, 3, 3} {
		if got := upStockAt(n); got != want {
			t.Errorf("upStockAt(%d) = %d, want %d", n, got, want)
		}
	}
	if upStockAt(-2) != 0 {
		t.Error("a negative level adds no stock")
	}
}

func TestUpPriceAt(t *testing.T) {
	// base x (n+1) x (1 + 0.15 x Rooms owned), the shape of keys and growth
	for _, c := range []struct {
		u        Upgrade
		n, rooms int
		want     Num
	}{
		{LobbyDecor, 0, 0, 150}, {LobbyDecor, 0, 2, 195}, {LobbyDecor, 1, 2, 390}, {Brochures, 2, 4, 576}, {BiggerShelves, 0, 1, 230},
	} {
		if got := upPriceAt(c.u, c.n, c.rooms); got != c.want {
			t.Errorf("upPriceAt(%s, %d, %d) = %v, want %v", upgrades[c.u].Name, c.n, c.rooms, got, c.want)
		}
	}
}

// With every level 0 each hook is neutral: the game plays as before the seam.
func TestUpgradesNeutralAtZero(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	for u := Upgrade(0); u < upgradeCount; u++ {
		if g.upMul(u) != 1 {
			t.Errorf("upMul(%s) = %v at level 0, want 1", upgrades[u].Name, g.upMul(u))
		}
	}
	if g.upStock() != 0 || g.pantryMax() != pantryStockMax || g.bookGapNow() != bookGap || g.cleanFrames(wipeFrames) != wipeFrames || g.ritualEcto(nil) != 0 {
		t.Error("a hook is not neutral at level 0")
	}
	if scaleInt(7, 1) != 7 {
		t.Error("scaleInt(n, 1) must be n")
	}
}

func TestUpgradeTableCaps(t *testing.T) {
	for u, d := range upgrades {
		if d.Name == "" || d.Base <= 0 || d.Effect <= 0 {
			t.Errorf("upgrade %d is not filled in: %+v", u, d)
		}
		if (d.Cap > 0) != (Upgrade(u) == BiggerShelves) {
			t.Errorf("%s: only Bigger shelves is capped (cap %d)", d.Name, d.Cap)
		}
	}
}

// The levels are saved with the Run, and a prestige clears them.
func TestUpgradesSaveAndReset(t *testing.T) {
	g1 := dressedRun(t, 1)
	g1.level = 1
	g1.boughtPantry()
	g1.up = [upgradeCount]int{1, 2, 3, 4, 5, 6, 3, 8}
	g1.pantryStock[0][SupplyFood] = pantryStockMax + 3 // only the Bigger shelves hold this
	memStore(t)
	g1.saveDawn()
	g2, _ := headless(t, 2, 1, 99)
	g2.startRun()
	g2.persist = true
	if !g2.resume(g2.loadSave()) {
		t.Fatal("resume failed")
	}
	if !reflect.DeepEqual(g2.up, g1.up) {
		t.Errorf("levels %v after the round trip, want %v", g2.up, g1.up)
	}
	if got := g2.pantryStock[0][SupplyFood]; got != pantryStockMax+3 {
		t.Errorf("pantry stock %d, want %d: the clamp must count Bigger shelves", got, pantryStockMax+3)
	}
	g2.resetRun()
	if g2.up != [upgradeCount]int{} {
		t.Errorf("levels %v after a prestige, want none", g2.up)
	}
}
