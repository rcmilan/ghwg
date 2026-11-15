package game

import (
	"image"
	"testing"

	"ghosthotel/world"
)

// 08 F0 (Glow, SPACE at a door, Put an item back): actionable names what SPACE does at the Concierge's spot and the
// prop it lights, and SPACE does exactly that: the Terminal from the cells touching the computer only, a shelf with
// something for it (taking, or putting back what you hold), a delivery anywhere on the doormat; nothing on the card
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
		{"the plate's cell, nothing held", plate, nil, actNone, false, func() bool { return g.state == StatePlay }},
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

// 08 F0 (ESC and HUD): the Terminal's "!" shows while something there never ordered this Run is on sale and
// affordable; once ordered it no longer counts, and a new Run counts it again.
func TestNewBuy(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	if g.newBuy() {
		t.Fatal(`"!" with an empty wallet`)
	}
	g.wallet[Money] = 1000
	if !g.newBuy() {
		t.Fatal(`no "!" with $1000 and nothing bought`)
	}
	for i, it := range shop {
		if it.Tab < tabCart && g.onSale(i) {
			g.cart = append(g.cart, cartLine{item: i, room: -1, n: 1, each: g.priceOf(i)})
		}
	}
	g.wallet[Money] = 1e6
	if g.placeOrder() != "" || g.newBuy() {
		t.Fatalf(`"!" after everything on sale was ordered`)
	}
	g.startRun()
	g.wallet[Money] = 1000
	if !g.newBuy() {
		t.Fatal(`no "!" in a new Run`)
	}
}
