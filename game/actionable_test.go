package game

import (
	"image"
	"testing"

	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 F0 (Glow, SPACE at a door, Put an item back): actionable names what SPACE does at the Concierge's spot and the
// prop it lights, and SPACE does exactly that: the Terminal from the cells touching the computer only, a shelf with
// something for it (taking, or putting back what you hold), a delivery anywhere on the doormat; the guest card on the
// plate's cell, at an empty shelf or the covered Mirror; the hotel view elsewhere, the doormat included.
func TestActionableAgrees(t *testing.T) {
	const ts = world.TileSize
	g, s, p := settled(t)
	h, mat := g.hotel, g.hotel.Rooms[p.Room].Mat()
	plate := center(g.hotel.Rooms[p.Room].Plate())
	for _, c := range []struct {
		name  string
		at    image.Point
		setup func()
		want  spaceAct
		glow  bool
		did   func() bool
	}{
		{"the Terminal", h.ConciergeStart, nil, actTerminal, true, func() bool { return g.state == StateTerminal }},
		{"a cell beside the Terminal", h.ConciergeStart.Add(image.Pt(ts, 0)), nil, actView, false, func() bool { return g.state == StateView }},
		{"the covered Mirror", h.Mirror, func() { g.locked[Mirror] = true }, actNone, false, func() bool { return g.state == StatePlay && g.shifting == nil }},
		{"an empty shelf, hands empty", supplyFront(g, 1), nil, actNone, false, func() bool { return g.state == StatePlay && g.hands == "" }},
		{"a shelf", supplyFront(g, 0), func() { g.stock = map[Unlock]int{SupplyWater: 1} }, actSupply, true,
			func() bool { return g.hands == SupplyWater && g.stock[SupplyWater] == 0 }},
		{"its empty shelf, holding it", supplyFront(g, 0), nil, actSupply, true, func() bool { return g.hands == "" && g.stock[SupplyWater] == 1 }},
		{"the doormat, nothing asked", center(mat), nil, actView, false, func() bool { return g.state == StateView }},
		{"the plate's cell, nothing held", plate, nil, actCard, true, func() bool { return g.state == StateTalk && g.parley.card != nil }},
		{"the doormat, the wrong item", center(mat), func() { p.asked, p.want, g.hands = g.clock, SupplyFood, SupplyWater }, actView, false,
			func() bool { return g.state == StateView && g.hands == SupplyWater }},
		{"the plate's cell, the asked item", plate, func() { g.hands = SupplyFood }, actDeliver, true, func() bool { return !p.asking() && g.hands == "" }},
	} {
		g.state, g.locked[Mirror] = StatePlay, false
		if c.setup != nil {
			c.setup()
		}
		g.player.X, g.player.Y = float64(c.at.X), float64(c.at.Y)
		act, r := g.actionable()
		if act != c.want || r.Empty() == c.glow {
			t.Errorf("%s: actionable %d, glow %v; want %d, glow %v", c.name, act, !r.Empty(), c.want, c.glow)
		}
		press(g, s, c.at)
		if !c.did() {
			t.Errorf("%s: SPACE didn't do what actionable says (state %d, hands %q, stock %v)", c.name, g.state, g.hands, g.stock)
		}
	}
}

// 08 F0 (ESC and HUD, live test 2026-11-15): the Terminal's "!" shows while something there is on sale and
// affordable that the open store hasn't shown yet this Run. A tab shown sees what it lists; an item that becomes
// affordable later brings the "!" back until its tab is shown; a new Run sees nothing yet.
func TestNewBuy(t *testing.T) {
	g, s := headless(t, 2, 1, 1)
	g.startRun()
	if g.newBuy() {
		t.Fatal(`"!" with an empty wallet`)
	}
	g.wallet[Money] = 60 // Water and the Mirror, not Food or Towels
	if !g.newBuy() {
		t.Fatal(`no "!" with $60 and nothing shown`)
	}
	g.termTab = tabHotel
	g.openTerminal()
	g.state = StatePlay
	if !g.newBuy() {
		t.Fatal(`no "!" with Water on sale and SUPPLIES never shown`)
	}
	g.termTab = tabSupplies
	g.openTerminal()
	g.state = StatePlay
	if g.newBuy() {
		t.Fatal(`"!" after both tabs with something affordable were shown`)
	}
	g.wallet[Money] = 1000
	if !g.newBuy() {
		t.Fatal(`no "!" with Food and Towels newly affordable`)
	}
	g.termTab = tabStaff
	g.openTerminal()
	keys(g, s, ebiten.KeyD, ebiten.KeyD) // CART, then SUPPLIES
	g.state = StatePlay
	if g.newBuy() {
		t.Fatal(`"!" after D showed SUPPLIES again`)
	}
	g.startRun()
	g.wallet[Money] = 1000
	if !g.newBuy() {
		t.Fatal(`no "!" in a new Run`)
	}
}
