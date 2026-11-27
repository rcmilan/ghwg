package game

import (
	"image"
	"testing"

	"ghosthotel/world"
)

// Bug report (10): a Guest asks for an item, the Concierge fetches it, then the ask ends (it expires, or the Party
// leaves and it lapses); SPACE at that item's shelf in real still puts it back, its shelf empty, and the shelf glows.
func TestPutBackAfterAskEnds(t *testing.T) {
	for _, end := range []string{"expired", "left", "gone"} {
		t.Run(end, func(t *testing.T) {
			g, s, p := settled(t)
			k, front := supplyOf(SupplyFood), supplyFront(g, supplyOf(SupplyFood))
			g.stock = map[Unlock]int{SupplyFood: 1}
			p.asked, p.want = g.clock, SupplyFood
			press(g, s, front)
			if g.hands != SupplyFood || g.stock[SupplyFood] != 0 {
				t.Fatalf("fetch: hands %q stock %d", g.hands, g.stock[SupplyFood])
			}
			switch end {
			case "expired":
				g.clock = p.asked + askMinutes*minuteFrames
			case "left":
				p.state = leaving
			case "gone":
				g.parties = nil
			}
			g.updateAsks()
			if p.asking() && end != "gone" {
				t.Fatal("the ask is still up")
			}
			g.player.X, g.player.Y = float64(front.X), float64(front.Y)
			if act, r := g.actionable(); act != actSupply || r != supplyGlow(g, k) {
				t.Errorf("at the oven holding Food: act %v rect %v, want actSupply glowing on the oven", act, r)
			}
			g.state = StatePlay
			press(g, s, front)
			if g.hands != "" || g.stock[SupplyFood] != 1 {
				t.Errorf("put back: hands %q stock %d, want empty and 1", g.hands, g.stock[SupplyFood])
			}
		})
	}
}

// supplyGlow: the rect actionable lights for supply k: its prop, two cells tall.
func supplyGlow(g *Game, k int) (r image.Rectangle) {
	r = supplySpot(g.hotel, k)
	return image.Rect(r.Min.X, r.Min.Y-world.TileSize, r.Max.X, r.Max.Y-world.TileSize)
}
