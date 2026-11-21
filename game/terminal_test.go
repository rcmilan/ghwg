package game

import (
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// T334 (08 F0-T): a teaser says what unlocks it; once on sale it shows its blurb. Every store row that can
// be locked has a hint that fits the detail pane.
func TestTerminalHints(t *testing.T) {
	g := closedRun(t) // no Room open: only the key is on sale
	for _, i := range append(append(append(tabItems(tabSupplies), tabItems(tabHotel)...), tabItems(tabStaff)...), tabItems(tabUpgrades)...) {
		it := shop[i]
		if it.Up > 0 || g.questOff(i) { // an upgrade has no teaser: its row is hidden until it applies (upgrade_rows_test.go); a quest row neither (08 W5)
			continue
		}
		if g.onSale(i) {
			t.Errorf("%s on sale with no Room open", it.ID)
		}
		if it.Hint == "" || g.about(i) != it.Hint {
			t.Errorf("%s locked: about %q, want its hint %q", it.ID, g.about(i), it.Hint)
		}
		if len(it.Hint)*glyphW > paneW-24 {
			t.Errorf("%s: the hint overflows the detail pane", it.ID)
		}
	}
	g.openRoom(0)
	if m := row(t, Mirror); !g.onSale(m) || g.about(m) != shop[m].Blurb {
		t.Fatalf("the Mirror on sale %v, about %q; want its blurb", g.onSale(m), g.about(m))
	}
}

// T334: what can't be bought any more is not listed (an unlock bought, the staff full); W/S skip it, and a
// tab with nothing left lists nothing. The Strategies' Shop leaves it out too.
func TestTerminalHides(t *testing.T) {
	g, s := headless(t, 2, 1, 1)
	g.startRun()
	g.openRoom(0)
	g.own(Mirror)
	g.earned = [2]Num{1e6, 1e6}
	g.wallet = [2]Num{1e6, 1e6}
	g.openTerminal()
	g.termTab, g.termPick = tabHotel, 0
	g.openTerminal() // reopened on a hidden row: it moves to the first listed one
	if mirror := slices.Index(tabItems(tabHotel), row(t, Mirror)); slices.Contains(g.listed(tabHotel), mirror) || g.termPick == mirror {
		t.Fatalf("the bought Mirror is listed (%v) or picked (%d)", g.listed(tabHotel), g.termPick)
	}
	l := g.listed(tabHotel)
	keys(g, s, ebiten.KeyW)
	if g.termPick != l[len(l)-1] {
		t.Fatalf("W from the first listed row picked %d, want the last listed, %d", g.termPick, l[len(l)-1])
	}
	keys(g, s, ebiten.KeyS)
	if g.termPick != l[0] {
		t.Fatalf("S from the last listed row picked %d, want the first listed, %d", g.termPick, l[0])
	}
	for range keeperCap(g.ownedCount()) {
		g.hire()
	}
	g.own(unlockK) // the Fathers: both calls bought, K on his retainer
	g.own(unlockM)
	g.retainK()
	if h := slices.Index(tabItems(tabStaff), row(t, rowKeeper)); slices.Contains(g.listed(tabStaff), h) || slices.Contains(g.forSale(), row(t, rowKeeper)) {
		t.Fatalf("staff full: listed %v, for sale %v", g.listed(tabStaff), g.forSale())
	}
	if len(g.listed(tabStaff)) != 2 { // the first running shoes and Father P's teaser (the upgrades have their own tab)
		t.Fatalf("staff full: listed %v, want the next running shoes and Father P", g.listed(tabStaff))
	}
}

// T334: the list scrolls so the picked row stays in sight, centered when it can be.
func TestScrollTop(t *testing.T) {
	for _, c := range []struct{ pos, n, rows, want int }{
		{0, 5, 10, 0}, {9, 10, 10, 0}, {0, 14, 10, 0}, {7, 14, 10, 2}, {13, 14, 10, 4},
	} {
		if got := scrollTop(c.pos, c.n, c.rows); got != c.want {
			t.Errorf("scrollTop(%d, %d, %d) = %d, want %d", c.pos, c.n, c.rows, got, c.want)
		}
	}
}
