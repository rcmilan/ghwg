package game

import (
	"fmt"
	"math"
	"strings"
)

// 08 W4 T374: the upgrades' rows in the Terminal. They are Items of the store (Item.Up = Upgrade+1), appended to
// shop in their own UPGRADES tab, hidden until they apply (upShows). Like everything else a level goes in the cart,
// one at a time, and the courier delivers it to the stock; it counts from the drop (T381 live test).

// upBlurb: the detail pane's line for each.
var upBlurb = [upgradeCount]string{
	LobbyDecor:    "Nicer lobby, nicer rates. Every Room.",
	Brochures:     "Fewer empty days between one Party and the next.",
	Soundproofing: "Walls that muffle the haunting. Moods hold.",
	SeanceTable:   "Ghosts show off for an audience. Ectoplasm per sighting.",
	TipJar:        "A jar by the door. Guests do read it.",
	CleaningKit:   "Better tools. Cleans end sooner.",
	BiggerShelves: "Each Pantry holds more of each Supply.",
	OfferingBox:   "A box for the Fathers' rituals. Ectoplasm each pays.",
}

var upIcon = [upgradeCount]string{
	LobbyDecor: "icon_deluxe", Brochures: "icon_door", Soundproofing: "icon_bed_single", SeanceTable: "icon_mirror",
	TipJar: "icon_key", CleaningKit: "icon_towel", BiggerShelves: "shelf_b", OfferingBox: fatherK.name,
}

// upRow: the shop index of each upgrade's row.
var upRow [upgradeCount]int

func init() {
	for u := range upgradeCount {
		d := upgrades[u]
		upRow[u] = len(shop)
		shop = append(shop, Item{ID: Unlock("up-" + strings.ReplaceAll(strings.ToLower(d.Name), " ", "-")), Tab: tabUpgrades, Name: d.Name, Icon: upIcon[u],
			Head: u == OfferingBox, Up: int(u) + 1, Price: func(g *Game) Cost { return Cost{Money: g.upPrice(u)} }})
	}
}

// upShows: the row is in the store: the Run has what the upgrade works on, or already owns a level of it.
func (g *Game) upShows(u Upgrade) bool {
	if g.up[u] > 0 {
		return true
	}
	switch u {
	case LobbyDecor, Brochures:
		return g.ownedCount() >= 2
	case TipJar:
		return g.ownedCount() >= 3
	case Soundproofing, SeanceTable:
		return g.unlocked(Mirror)
	case CleaningKit:
		return g.staffed() > 0
	case BiggerShelves:
		return g.hasPantry()
	}
	return g.unlocked(unlockK) || g.unlocked(unlockM) // OfferingBox
}

func (g *Game) upMaxed(u Upgrade) bool { c := upgrades[u].Cap; return c > 0 && g.up[u] >= c }

// levelUp: a level of u arrives with the courier: it counts from now, with a notice.
func (g *Game) levelUp(u Upgrade) {
	g.up[u]++
	g.notice(fmt.Sprintf("%s LV %d", strings.ToUpper(upgrades[u].Name), g.up[u]))
}

// upEffect: u's total effect at level n, as the row words it.
func upEffect(u Upgrade, n int) string {
	pct := func(x float64) int { return int(math.Round(x * 100)) }
	m := upMulAt(u, n)
	switch u {
	case LobbyDecor:
		return fmt.Sprintf("+%d%% RATE", pct(m-1))
	case Brochures:
		return fmt.Sprintf("-%d%% WAIT", pct(1-m))
	case Soundproofing:
		return fmt.Sprintf("-%d%% MOOD LOSS", pct(1-m))
	case SeanceTable:
		return fmt.Sprintf("+%d%% ECTO", pct(m-1))
	case TipJar:
		return fmt.Sprintf("+%d%% TIPS", pct(m-1))
	case CleaningKit:
		return fmt.Sprintf("-%d%% CLEAN TIME", pct(1-m))
	case BiggerShelves:
		return fmt.Sprintf("+%d STOCK", upStockAt(n))
	}
	return fmt.Sprintf("+%d%% RITUAL ECTO", pct(m-1)) // OfferingBox
}

// upLabel: the list row's name and, at the top level, the word that stands for the price.
func (g *Game) upLabel(u Upgrade) (name, word string) {
	name = fmt.Sprintf("%s LV %d", strings.ToUpper(upgrades[u].Name), g.up[u])
	if g.upMaxed(u) {
		word = "MAX"
	}
	return
}

// upStatus: the pane's line: what it gives now and what the next level adds up to.
func (g *Game) upStatus(u Upgrade) string {
	if g.upMaxed(u) {
		return "NOW " + upEffect(u, g.up[u])
	}
	return fmt.Sprintf("NOW %s   NEXT %s", upEffect(u, g.up[u]), upEffect(u, g.up[u]+1))
}

// UpView is an upgrade as a Strategy sees it (View.Ups).
type UpView struct {
	Shown bool // its row is in the store
	Max   bool
	Level int
	Price Num // of the next level
}

func (g *Game) upViews() (v [upgradeCount]UpView) {
	for u := range upgradeCount {
		v[u] = UpView{g.upShows(u), g.upMaxed(u), g.up[u], g.upPrice(u)}
	}
	return
}

// DevUpgrades: main's -term t,r,2 on the -shop start: every Room open, a housekeeper, the Pantry and a Father's call
// bought, a few levels of each upgrade, so every row shows.
func (g *Game) DevUpgrades() {
	for i := range g.roomOwned {
		g.roomOwned[i] = true
	}
	g.unlock(Mirror)
	g.unlock(unlockK)
	g.hire()
	g.boughtPantry()
	g.up = [upgradeCount]int{3, 2, 1, 0, 2, 1, 3, 12}
}
