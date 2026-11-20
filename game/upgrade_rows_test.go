package game

import (
	"math"
	"slices"
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

func TestBuyUpgrade(t *testing.T) {
	g, _ := headless(t, 2, 2, 1)
	g.startRun()
	g.closeRooms()
	g.roomOwned[0], g.roomOwned[1] = true, true
	price := g.upPrice(LobbyDecor)
	g.wallet = [2]Num{price - 1, 0}
	if why := g.buyUp(LobbyDecor); why != "CAN'T AFFORD" || g.up[LobbyDecor] != 0 || g.wallet[Money] != price-1 {
		t.Fatalf("short of Money: %q, level %d, wallet %v", why, g.up[LobbyDecor], g.wallet)
	}
	g.wallet[Money] = price + 50
	if why := g.buyUp(LobbyDecor); why == "" || g.up[LobbyDecor] != 1 || g.wallet[Money] != 50 {
		t.Fatalf("buy: %q, level %d, wallet %v", why, g.up[LobbyDecor], g.wallet)
	}
	if g.upPrice(LobbyDecor) != 2*price {
		t.Errorf("level 2 costs %v, want %v", g.upPrice(LobbyDecor), 2*price)
	}
	if len(g.cart) != 0 || len(g.couriers) != 0 {
		t.Error("an upgrade is not shipped")
	}
	// Bigger shelves stops at its cap.
	g.up[BiggerShelves] = upgrades[BiggerShelves].Cap
	g.wallet[Money] = 1e9
	if why := g.buyUp(BiggerShelves); why != "MAX LEVEL" || g.up[BiggerShelves] != upgrades[BiggerShelves].Cap {
		t.Errorf("past the cap: %q, level %d", why, g.up[BiggerShelves])
	}
	if _, word := g.upLabel(BiggerShelves); word != "MAX" {
		t.Errorf("a capped row says %q, want MAX", word)
	}
}

// SPACE on the row in the open Terminal buys it.
func TestUpgradeRowBoughtByTheTerminal(t *testing.T) {
	g, _ := headless(t, 2, 2, 1)
	g.startRun()
	g.closeRooms()
	g.roomOwned[0], g.roomOwned[1] = true, true
	g.wallet = [2]Num{1e6, 0}
	g.openTerminal()
	g.termTab = tabHotel
	g.termPick = slices.Index(tabItems(tabHotel), upRow[Brochures])
	if msg := g.press(); g.up[Brochures] != 1 {
		t.Errorf("SPACE on the Brochures row: %q, level %d", msg, g.up[Brochures])
	}
	if slices.Contains(g.forSale(), upRow[Brochures]) {
		t.Error("an upgrade is not a Strategy's cart item")
	}
}

func TestRitualEcto(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	if g.ritualEcto(nil) != ritualEctoBase {
		t.Errorf("a ritual pays %v, want %v", g.ritualEcto(nil), ritualEctoBase)
	}
	g.up[OfferingBox] = 2
	if want := ritualEctoBase * Num(1.76); math.Abs(float64(g.ritualEcto(nil)-want)) > 1e-9 {
		t.Errorf("Offering box 2: %v, want %v", g.ritualEcto(nil), want)
	}
}
