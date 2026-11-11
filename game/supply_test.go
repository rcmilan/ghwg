package game

import (
	"image"
	"testing"

	"ghosthotel/world"
)

// supplyFront: the cell in front of supply k, where the Concierge stands to fetch it.
func supplyFront(g *Game, k int) image.Point {
	r := supplySpot(g.hotel, k)
	return image.Pt(r.Min.X+world.TileSize/2, r.Max.Y-world.TileSize/2)
}

// T195: a request picks one of the supplies, every one of them in time.
func TestAskWants(t *testing.T) {
	g, _, p := settled(t)
	seen := map[Unlock]bool{}
	g.clock = g.clock/dayFrames*dayFrames + 12*hourFrames // noon: awake
	for range 300 {
		p.asked, p.askEnded, p.want = 0, 0, ""
		for !p.asking() {
			g.updateAsks()
		}
		if supplyOf(p.want) < 0 {
			t.Fatalf("asked for %q, not a supply", p.want)
		}
		seen[p.want] = true
	}
	if len(seen) != len(supplies) {
		t.Fatalf("asked for %d kinds of supply, want all %d", len(seen), len(supplies))
	}
}

// T196: SPACE at a supply prop in real takes one from the stock into the hands; an empty one gives
// nothing; another one swaps, the held item going back; in ghost nothing.
func TestFetch(t *testing.T) {
	g, s := headless(t, 2, 1, 1)
	g.startRun()
	g.stock = map[Unlock]int{SupplyWater: 2}
	press(g, s, supplyFront(g, 1)) // Food: none in stock
	if g.hands != "" {
		t.Fatalf("an empty supply gave %q", g.hands)
	}
	g.ghost = true
	press(g, s, supplyFront(g, 0))
	if g.hands != "" {
		t.Fatal("fetched in ghost")
	}
	g.ghost = false
	press(g, s, supplyFront(g, 0))
	if g.hands != SupplyWater || g.stock[SupplyWater] != 1 {
		t.Fatalf("hands %q stock %d, want water and 1", g.hands, g.stock[SupplyWater])
	}
	g.stock[SupplyTowel] = 1
	press(g, s, supplyFront(g, 2))
	if g.hands != SupplyTowel || g.stock[SupplyTowel] != 0 || g.stock[SupplyWater] != 2 {
		t.Fatalf("swap: hands %q stock %v, want towel, the water back", g.hands, g.stock)
	}
}

// T197: the right item at the door pays at once, less with Room Ecto, and still gives Comfort, clears
// the ask, counts a serve tap and empties the hands. The wrong item, or no ask, does nothing.
func TestDeliver(t *testing.T) {
	g, s, p := settled(t)
	door := center(g.hotel.Rooms[p.Room].Spot())
	p.asked, p.want, g.hands = g.clock, SupplyFood, SupplyWater
	press(g, s, door)
	if !p.asking() || g.hands != SupplyWater || g.wallet[Money] != 0 {
		t.Fatal("the wrong item was taken")
	}
	g.hands = SupplyFood
	press(g, s, door)
	pay := supplies[supplyOf(SupplyFood)].Pay
	if p.asking() || g.hands != "" || g.wallet[Money] != pay || p.Guests[0].Comfort != comfortStep || g.taps[p.Room][TaskServe] != 1 {
		t.Fatalf("delivered: asking %v hands %q money %v Comfort %d taps %v, want false, empty, %v, %d, 1",
			p.asking(), g.hands, g.wallet[Money], p.Guests[0].Comfort, g.taps[p.Room][TaskServe], pay, comfortStep)
	}
	g.hands = SupplyFood
	press(g, s, door)
	if g.hands != SupplyFood || g.wallet[Money] != pay {
		t.Fatal("delivered with no ask")
	}
	g.wallet[Money], g.mood[p.Room], p.asked = 0, 0, g.clock
	g.deliver(p.Room, SupplyFood)
	if g.wallet[Money] != pay/2 {
		t.Fatalf("paid %v at Mood 0 (half the dirt), want %v", g.wallet[Money], pay/2)
	}
}
