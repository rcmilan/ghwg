package game

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// T077 (04 D119): the Terminal books a Room first, then any Party at the counter it fits; with one
// fitting Party it goes straight to OK; a Room nobody fits is refused.
func TestTerminalRoomThenParty(t *testing.T) {
	g, ps := autoHotel(t, []string{"Double", "Single"}, 2, 1, 1)
	s := g.in.(*script)
	press := func(k ebiten.Key) {
		s.hold(k)
		g.Update()
		s.hold()
		g.Update()
	}
	g.openTerminal()
	g.terminalCursor = 0 // 301, a Double: all three fit
	press(ebiten.KeySpace)
	if !g.terminalList || g.terminalParty != ps[0] || g.terminalPick != 0 {
		t.Fatalf("SPACE on 301: list %v, party %v, pick %d; want the list focused on the front pair", g.terminalList, g.terminalParty, g.terminalPick)
	}
	press(ebiten.KeyS)
	press(ebiten.KeySpace)
	if !g.terminalOK || g.terminalParty != ps[1] {
		t.Fatalf("S, SPACE in the list: OK %v, party %v; want OK with the second Party", g.terminalOK, g.terminalParty)
	}
	press(ebiten.KeySpace)
	if g.state != StatePlay || ps[1].Room != 0 || ps[0].Room >= 0 || ps[1].Auto {
		t.Fatalf("OK: state %d, second Party in %d, front in %d; want the second booked by hand, the front still queued", g.state, ps[1].Room, ps[0].Room)
	}

	g.openTerminal()
	g.terminalCursor = 0
	press(ebiten.KeySpace)
	if g.terminalPick >= 0 || g.message != "That room is booked." {
		t.Fatalf("SPACE on a booked Room: pick %d, message %q; want it refused", g.terminalPick, g.message)
	}
	g.terminalCursor = 1 // 302, a Single: only the solo fits
	press(ebiten.KeySpace)
	if !g.terminalOK || g.terminalParty != ps[2] {
		t.Fatalf("SPACE on a Single: OK %v, party %v; want straight to OK with the solo", g.terminalOK, g.terminalParty)
	}
	press(ebiten.KeySpace)
	if ps[2].Room != 1 || ps[0].Room >= 0 {
		t.Fatalf("solo in %d, pair in %d; want the solo in 302 and the pair queued", ps[2].Room, ps[0].Room)
	}
}
