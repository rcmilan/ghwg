package game

import (
	"math"
	"slices"
	"strings"
	"testing"
)

// listedUp: the upgrades the Terminal lists now.
func listedUp(g *Game) (us []Upgrade) {
	for u := range upgradeCount {
		if !g.gone(upRow[u]) {
			us = append(us, u)
		}
	}
	return
}

// A row shows only once what it works on is there, and stays once a level is bought.
func TestUpgradeRowsHiddenUntilTheyApply(t *testing.T) {
	g, _ := headless(t, 2, 2, 1)
	g.startRun()
	g.closeRooms()
	g.roomOwned[0] = true
	if got := listedUp(g); len(got) != 0 {
		t.Fatalf("one Room, no Mirror, no staff: rows %v, want none", got)
	}
	g.roomOwned[1] = true
	if got := listedUp(g); !slices.Equal(got, []Upgrade{LobbyDecor, Brochures}) {
		t.Errorf("2 Rooms: %v, want Lobby decor and Brochures", got)
	}
	g.roomOwned[2] = true
	if !slices.Contains(listedUp(g), TipJar) {
		t.Error("the Tip jar shows with the 3rd Room")
	}
	g.unlock(Mirror)
	if got := listedUp(g); !slices.Contains(got, Soundproofing) || !slices.Contains(got, SeanceTable) {
		t.Errorf("Mirror bought: %v, want Soundproofing and the Seance table", got)
	}
	for _, u := range []Upgrade{CleaningKit, BiggerShelves, OfferingBox} {
		if g.upShows(u) {
			t.Errorf("%s shows before its condition", upgrades[u].Name)
		}
	}
	g.hire()
	g.boughtPantry()
	g.unlock(unlockK)
	for _, u := range []Upgrade{CleaningKit, BiggerShelves, OfferingBox} {
		if !g.upShows(u) {
			t.Errorf("%s hidden after its condition", upgrades[u].Name)
		}
	}
}

// A level goes through the cart like everything else (T381 live test): SPACE puts it in, checkout pays, and the
// courier delivers it: the level counts only when he drops it at the stock.
func TestUpgradeOrderedAndDelivered(t *testing.T) {
	g, _ := headless(t, 2, 2, 1)
	g.startRun()
	g.closeRooms()
	g.roomOwned[0], g.roomOwned[1] = true, true
	price := g.upPrice(LobbyDecor)
	g.wallet = [2]Num{price - 1, 0}
	if why := g.addToCart(upRow[LobbyDecor], -1); why != "CAN'T AFFORD" {
		t.Fatalf("short of Money: %q", why)
	}
	g.wallet[Money] = price + 50
	g.openTerminal()
	g.termTab = tabUpgrades
	g.termPick = slices.Index(tabItems(tabUpgrades), upRow[LobbyDecor])
	if msg := g.press(); g.up[LobbyDecor] != 0 || g.wallet[Money] != price+50 || g.inCart(upRow[LobbyDecor]) != 1 {
		t.Fatalf("SPACE on the row: %q, level %d, wallet %v, in the cart %d", msg, g.up[LobbyDecor], g.wallet, g.inCart(upRow[LobbyDecor]))
	}
	if why := g.addToCart(upRow[LobbyDecor], -1); why == "" || g.inCart(upRow[LobbyDecor]) != 1 {
		t.Fatalf("a second level in the cart: %q", why)
	}
	if why := g.placeOrder(); why != "" || g.wallet[Money] != 50 || g.up[LobbyDecor] != 0 || len(g.couriers) != 1 {
		t.Fatalf("checkout: %q, wallet %v, level %d, %d couriers", why, g.wallet, g.up[LobbyDecor], len(g.couriers))
	}
	g.wallet[Money] = 1e6
	if why := g.addToCart(upRow[LobbyDecor], -1); why != "ON ITS WAY" {
		t.Fatalf("the next level while one is on its way: %q", why)
	}
	until(t, g, "the courier delivers the level", func() bool { return g.up[LobbyDecor] == 1 })
	if g.upPrice(LobbyDecor) != 2*price {
		t.Errorf("level 2 costs %v, want %v", g.upPrice(LobbyDecor), 2*price)
	}
	if !slices.ContainsFunc(g.note.items, func(n feedItem) bool { return strings.Contains(n.text, "LOBBY DECOR LV 1") }) {
		t.Error("no notice as the level arrives")
	}
	// Bigger shelves stops at its cap.
	g.up[BiggerShelves] = upgrades[BiggerShelves].Cap
	if why := g.addToCart(upRow[BiggerShelves], -1); why == "" {
		t.Error("a capped upgrade went in the cart")
	}
	if _, word := g.upLabel(BiggerShelves); word != "MAX" {
		t.Errorf("a capped row says %q, want MAX", word)
	}
}

// Every upgrade is in its own tab.
func TestUpgradesTab(t *testing.T) {
	for u := range upgradeCount {
		if shop[upRow[u]].Tab != tabUpgrades {
			t.Errorf("%s is in tab %s", upgrades[u].Name, tabNames[shop[upRow[u]].Tab])
		}
	}
}

func TestRitualEcto(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	if g.ritualEcto(nil) != ritualEctoBase {
		t.Errorf("a ritual pays %v, want %v", g.ritualEcto(nil), ritualEctoBase)
	}
	g.up[OfferingBox] = 2
	if want := ritualEctoBase * Num(2.52); math.Abs(float64(g.ritualEcto(nil)-want)) > 1e-9 {
		t.Errorf("Offering box 2: %v, want %v", g.ritualEcto(nil), want)
	}
}
