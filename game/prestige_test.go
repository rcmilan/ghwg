package game

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 P1: the bonus multiplies every income once, in earn.
func TestBonusInEarn(t *testing.T) {
	g, _ := headless(t, 3, 3, 1)
	g.startRun()
	g.bonus = 2
	g.earn(Ecto, 10, noFloat)
	g.earn(Money, 5, noFloat)
	if g.wallet != [2]Num{10, 20} || g.runEcto != 10 || g.earned != [2]Num{10, 20} {
		t.Fatalf("wallet %v, runEcto %v, earned %v after Ecto 10 and Money 5 at x2; want [10 20], 10 (before the bonus), [10 20]", g.wallet, g.runEcto, g.earned)
	}
}

// Father P is on sale at the biggest grid with every key; SPACE opens a card, ESC closes it, a second SPACE pays.
func TestFatherPGate(t *testing.T) {
	g, s := headless(t, 3, 3, 1)
	g.startRun()
	p := row(t, rowFatherP)
	if !g.onSale(p) {
		t.Fatal("a full 3x3 hotel with every key: Father P is not on sale")
	}
	g.level = 1
	if g.onSale(p) {
		t.Fatal("level 1 wants a 4x4 hotel; Father P is on sale at 3x3")
	}
	g.level = 0
	g.roomOwned = make([]bool, 9)
	if g.onSale(p) {
		t.Fatal("Father P is on sale with no key bought")
	}
	g.roomOwned = nil
	g.ShopPrint(0, prestigeEcto-1)
	g.termTab = tabStaff
	g.termPick = indexIn(g, p)
	if keys(g, s, ebiten.KeySpace); g.prestigeAsk || g.refused != "CAN'T AFFORD" {
		t.Fatalf("card opened without the Ectoplasm (%q)", g.refused)
	}
	g.wallet[Ecto] = prestigeEcto
	if keys(g, s, ebiten.KeySpace); !g.prestigeAsk || len(g.cart) != 0 {
		t.Fatal("SPACE on Father P's row did not open the card")
	}
	if keys(g, s, ebiten.KeyEscape); g.prestigeAsk || g.state != StateTerminal {
		t.Fatalf("ESC: card open %v, state %d; want the card closed, the Terminal still open", g.prestigeAsk, g.state)
	}
	keys(g, s, ebiten.KeySpace)
	g.runEcto = 4000
	if keys(g, s, ebiten.KeySpace); g.level != 1 || g.bonus <= 1 || g.state != StatePlay {
		t.Fatalf("second SPACE: level %d, bonus %v, state %d; want a prestige", g.level, g.bonus, g.state)
	}
}

func indexIn(g *Game, i int) int {
	for k, j := range tabItems(g.termTab) {
		if j == i {
			return k
		}
	}
	return -1
}

// A prestige adds its gain to the bonus, never lowers it, and resets all but level and bonus.
func TestPrestigeReset(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.openAtStart = false
	g.startRun()
	g.grow(true)
	g.grow(false)
	g.grow(true)
	g.grow(false)
	g.own(unlockK)
	g.hire()
	g.runEcto, g.wallet = 1000, [2]Num{9000, 7}
	g.clock += 5000
	gain := g.prestigeGain() // 1000 + 9000/10 donated = 1900
	if want := 1.378; gain < want || gain > want+.001 {
		t.Fatalf("gain %v for 1900, want about %v", gain, want)
	}
	g.startPrestige()
	if g.level != 1 || g.bonus != 1+gain {
		t.Fatalf("level %d, bonus %v; want 1 and %v", g.level, g.bonus, 1+gain)
	}
	if c, r := g.hotel.Grid(); [2]int{c, r} != g.grid0 || g.clock != runStart || g.runEcto != 0 || g.wallet != [2]Num{startMoney, 0} ||
		len(g.keepers) != 0 || g.unlocked(Mirror) || g.ownedCount() != 0 || g.intro != nil || g.state != StatePlay {
		t.Fatalf("the Run did not start over: grid %v, clock %d, runEcto %v, wallet %v, %d keepers, %d Rooms owned", g.hotel.Map.W, g.clock, g.runEcto, g.wallet, len(g.keepers), g.ownedCount())
	}
	if p := image.Pt(int(g.player.X), int(g.player.Y)); p != g.hotel.ConciergeStart {
		t.Fatalf("the Concierge at %v, want the hall %v", p, g.hotel.ConciergeStart)
	}
	if g.unlocked(unlockK) {
		t.Fatal("Call Father K stayed owned")
	}
	before := g.bonus
	g.runEcto = 0
	g.startPrestige()
	if g.bonus < before || g.level != 2 {
		t.Fatalf("bonus %v after an empty Run, was %v: it must never drop", g.bonus, before)
	}
}

// Keys and growth steps cost base x (1 + 0.15 x Rooms owned); the housekeeper cap is a third of the Rooms, up.
func TestPriceCurveAndKeeperCap(t *testing.T) {
	for n, want := range map[int]Num{0: 100, 1: 115, 2: 130, 4: 160, 8: 220} {
		if got := keyPrice(n); got != want {
			t.Errorf("key with %d Rooms owned: %v, want %v", n, got, want)
		}
	}
	if got := growPrice(3, 1, 4); got != roomPrice(2*growEcto, 4) || got != 192 {
		t.Errorf("growth step 2 with 4 Rooms: %v, want 192", got)
	}
	for n, want := range map[int]int{0: 1, 1: 1, 3: 1, 4: 2, 6: 2, 7: 3, 9: 3, 10: 4} {
		if got := keeperCap(n); got != want {
			t.Errorf("housekeepers with %d Rooms: %d, want %d", n, got, want)
		}
	}
}
