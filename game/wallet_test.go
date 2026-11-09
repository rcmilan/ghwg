package game

import (
	"image"
	"testing"
)

// T175: earn then pay; a mixed cost is paid whole or not at all.
func TestWallet(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.earn(Money, 10, image.Pt(40, 40))
	g.earn(Spook, 5, noFloat)
	if g.wallet != [2]Num{10, 5} {
		t.Fatalf("earned 10 and 5: wallet %v", g.wallet)
	}
	if len(g.floaters) != 1 || g.floaters[0].text != "+$10" || g.floaters[0].kind != floatMoney {
		t.Fatalf("floaters %+v; want one +$10 money floater", g.floaters)
	}
	if g.afford(Cost{Money: 10, Spook: 6}) || g.pay(Cost{Money: 10, Spook: 6}) || g.wallet != [2]Num{10, 5} {
		t.Fatalf("an unaffordable mixed cost was paid: wallet %v", g.wallet)
	}
	if !g.pay(Cost{Money: 4, Spook: 5}) || g.wallet != [2]Num{6, 0} {
		t.Fatalf("paid $4 + 5 Spook: wallet %v, want [6 0]", g.wallet)
	}
}

// T176: the zero value unlocks everything; a locked id opens once unlocked.
func TestUnlocks(t *testing.T) {
	g := &Game{}
	if !g.unlocked(Mirror) {
		t.Fatal("the zero Game has the Mirror locked")
	}
	g.locked = map[Unlock]bool{Mirror: true}
	if g.unlocked(Mirror) || !g.unlocked(Hex) {
		t.Fatal("only the Mirror should be locked")
	}
	g.unlock(Mirror)
	if !g.unlocked(Mirror) {
		t.Fatal("the Mirror stayed locked after unlock")
	}
}

// T177: each tap counts once, per Room and Task.
func TestTapped(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.tapped(1, TaskHex)
	g.tapped(1, TaskHex)
	g.tapped(0, TaskServe)
	if g.taps[1][TaskHex] != 2 || g.taps[0][TaskServe] != 1 || g.taps[0][TaskHex] != 0 {
		t.Fatalf("taps %v", g.taps)
	}
}
