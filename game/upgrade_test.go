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
		{LobbyDecor, 0, 1}, {LobbyDecor, 1, 1.08}, {LobbyDecor, 2, 1.152}, {LobbyDecor, 500, 1.8}, // boost: toward 1 + 10e
		{SeanceTable, 1, 1.2}, {SeanceTable, 2, 1.38}, {TipJar, 3, 1.542}, {OfferingBox, 1, 1.8}, {OfferingBox, 2, 2.52},
		{Brochures, 0, 1}, {Brochures, 1, 0.8}, {Brochures, 2, 0.64}, {Soundproofing, 3, 0.512}, // reducer: (1-e)^n
		{CleaningKit, 1, 0.7}, {CleaningKit, 2, 0.49},
		{LobbyDecor, -1, 1}, // a bad level is no level
	} {
		if got := upMulAt(c.u, c.n); !near(got, c.want) {
			t.Errorf("upMulAt(%s, %d) = %v, want %v", upgrades[c.u].Name, c.n, got, c.want)
		}
	}
}

func TestUpPriceAt(t *testing.T) {
	// base x (n+1) x (1 + 0.15 x Rooms owned), the shape of keys and growth
	for _, c := range []struct {
		u        Upgrade
		n, rooms int
		want     Num
	}{
		{LobbyDecor, 0, 0, 150}, {LobbyDecor, 0, 2, 195}, {LobbyDecor, 1, 2, 390}, {Brochures, 2, 4, 576}, {CleaningKit, 0, 1, 184},
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
	if g.bookGapNow() != bookGap || g.cleanFrames(wipeFrames) != wipeFrames || g.ritualEcto(nil) != ritualEctoBase {
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
		if d.Cap != 0 {
			t.Errorf("%s: no upgrade is capped (cap %d)", d.Name, d.Cap)
		}
	}
}

// The levels are saved with the Run, and a prestige clears them.
func TestUpgradesSaveAndReset(t *testing.T) {
	g1 := dressedRun(t, 1)
	g1.up = [upgradeCount]int{1, 2, 3, 4, 5, 6, 8}
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
	g2.resetRun()
	if g2.up != [upgradeCount]int{} {
		t.Errorf("levels %v after a prestige, want none", g2.up)
	}
}

// A level paid for and still on the courier at dawn arrives on CONTINUE, like a Room's order (it goes to the stock:
// no Room).
func TestUpgradeInTransitSaved(t *testing.T) {
	g1 := dressedRun(t, 1)
	g1.up[LobbyDecor] = 1
	g1.ship([]cartLine{{item: upRow[LobbyDecor], room: -1, n: 1, each: Cost{Money: g1.upPrice(LobbyDecor)}}})
	memStore(t)
	g1.saveDawn()
	g2, _ := headless(t, 2, 1, 99)
	g2.startRun()
	g2.persist = true
	if !g2.resume(g2.loadSave()) {
		t.Fatal("resume failed")
	}
	if g2.up[LobbyDecor] != 2 {
		t.Errorf("Lobby decor at level %d after CONTINUE, want 2: the level on its way is lost", g2.up[LobbyDecor])
	}
}
