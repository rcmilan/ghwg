package game

import (
	"image"
	"math"
	"testing"
)

// pathLen: the length, world px, of the route from `from` to `to` (Path's waypoints); -1 when unreachable.
func pathLen(g *Game, from, to image.Point) float64 {
	p := g.hotel.Path(from, to, -1)
	if p == nil {
		return -1
	}
	n, at := 0.0, from
	for _, w := range p {
		n += math.Hypot(float64(w.X-at.X), float64(w.Y-at.Y))
		at = w
	}
	return n
}

// Bug 10 (unreachable delivery time): the ask window must cover a housekeeper's worst trip, from any cell of the
// hotel outside a Room to the stock she fetches from (the kitchen or the nearer pantry) and on to the asking Room's
// door, on every grid a Run reaches, with no running shoes: 45 s, askMinutes, is that window.
func TestAskCoversWorstTrip(t *testing.T) {
	window := float64(askMinutes * minuteFrames) // frames
	for _, c := range []struct {
		level  int
		pantry bool
	}{{0, false}, {1, false}, {1, true}, {2, false}, {2, true}} { // Level 3 on (6x6 with no pantry, 7x7 with one) is over: bug 10 report
		g, _ := headless(t, 2, 1, 1)
		g.startRun()
		g.level = c.level
		if c.pantry {
			g.boughtPantry()
		}
		n := maxGrid(c.level)
		for {
			cols, rows := g.hotel.Grid()
			if cols == n && rows == n {
				break
			}
			if _, err := g.grow(cols < n); err != nil {
				t.Fatal(err)
			}
		}
		g.stock = map[Unlock]int{SupplyWater: 1, SupplyFood: 1, SupplyTowel: 1}
		g.hire()
		k := g.keepers[0]
		m, worst, from, room, want := g.hotel.Map, 0.0, image.Point{}, -1, Unlock("")
		toDoor := map[image.Point][]float64{} // from a stock spot to each Room's door
		for y := range m.H {
			for x := range m.W {
				at := m.Center(x, y)
				if m.Solid(x, y) || g.roomAt(float64(at.X), float64(at.Y)) >= 0 {
					continue
				}
				k.X, k.Y = float64(at.X), float64(at.Y)
				for _, s := range supplies {
					src, _ := g.nearestStock(k, s.ID)
					a := pathLen(g, at, src)
					if a < 0 {
						continue
					}
					if toDoor[src] == nil {
						for r := range g.hotel.Rooms {
							toDoor[src] = append(toDoor[src], pathLen(g, src, g.doorDrop(r)))
						}
					}
					for r, b := range toDoor[src] {
						if b >= 0 && a+b > worst {
							worst, from, room, want = a+b, at, r, s.ID
						}
					}
				}
			}
		}
		frames := worst / keeperSpeed
		t.Logf("%dx%d pantry=%v: worst trip %.0f px from %v to Room %d = %.0f frames (%.1f s), %.0f%% of the %d-frame window",
			n, n, c.pantry, worst, from, room, frames, frames/60, 100*frames/window, int(window))
		if frames > window {
			t.Errorf("%dx%d pantry=%v: the worst trip takes %.0f frames, over the %.0f-frame ask window", n, n, c.pantry, frames, window)
		}
		// walked for real (doors, steering, the stroll's first frame), from that cell to that Room
		p := settledHuman(t, g, room, 12)
		k.X, k.Y, k.Path, k.job = float64(from.X), float64(from.Y), nil, resting
		p.asked, p.want = g.clock, want
		start, serves := g.clock, g.serves
		for p.asking() {
			g.step(0, 0)
		}
		t.Logf("  walked: %s %v after %d frames", map[bool]string{true: "served", false: "EXPIRED"}[g.serves > serves], want, g.clock-start)
		if g.serves == serves {
			t.Errorf("%dx%d pantry=%v: the housekeeper from %v did not reach Room %d in the ask window", n, n, c.pantry, from, room)
		}
	}
}

// Bug 10 (a delivery nobody waits for): a housekeeper carrying a Supply to a Room whose ask is gone turns back at
// once, hands nothing over and puts the Supply back in the stock: the ask expired, was served by the Concierge, or
// the Party left the Room (checked out, out for the day, fled, stepped out for air, talked into another Room).
func TestKeeperDropsDeadDelivery(t *testing.T) {
	for _, c := range []struct {
		name string
		end  func(g *Game, p *Party)
	}{
		{"expired", func(g *Game, p *Party) { p.asked = g.clock - askMinutes*minuteFrames + 1 }},
		{"served by the Concierge", func(g *Game, p *Party) {
			g.hands = SupplyWater
			g.stock[SupplyWater]--
			g.deliver(p.Room, SupplyWater)
		}},
		{"checked out", func(g *Game, p *Party) { p.Checkout = g.clock }},
		{"out for the day", func(g *Game, p *Party) {
			p.Routine, p.firstOut, p.outAt, p.backAt, p.Checkout = tourist, 0, 0, dayFrames, g.clock+2*dayFrames
		}},
		{"fled", func(g *Game, p *Party) { g.flee(p) }},
		{"stepped out", func(g *Game, p *Party) { g.stepOut(p) }},
		{"moved", func(g *Game, p *Party) { g.relocate(p, 1) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			g, _, p := settled(t)
			g.stock = map[Unlock]int{SupplyWater: 2}
			g.hire()
			k := g.keepers[0]
			at := g.kitchenSpot()
			k.X, k.Y, k.Path = float64(at.X), float64(at.Y), nil
			p.asked, p.want = g.clock, SupplyWater
			until(t, g, "it takes the Water", func() bool { return k.job == serving })
			serves, stock := g.serves, g.stock[SupplyWater]
			c.end(g, p)
			concierge := g.serves - serves
			for range 2 * minuteFrames {
				g.step(0, 0)
			}
			if k.job == serving || k.job == fetching || g.serves-serves != concierge { // free for the next job (a clean, here)
				t.Fatalf("job %d, %d serves by her; want off the delivery, 0", k.job, g.serves-serves-concierge)
			}
			if want := stock + 1 - concierge; g.stock[SupplyWater] != want {
				t.Fatalf("stock %d, want %d: the Water she carried goes back", g.stock[SupplyWater], want)
			}
		})
	}
}
