package game

import (
	"image"
	"math"
	"slices"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// until steps the world until ok, or fails after a game hour and a half.
func until(t *testing.T, g *Game, what string, ok func() bool) {
	t.Helper()
	for f := 0; !ok(); f++ {
		if f > 90*minuteFrames {
			t.Fatalf("%s: not after 90 game minutes", what)
		}
		g.step(0, 0)
	}
}

// 07 H1 step 3: hired at the store, a housekeeper walks in; it brings a stocked Supply to the
// Room that asks, and the delivery pays as the Concierge's does.
func TestKeeperServes(t *testing.T) {
	g, _, p := settled(t)
	g.earned[Money], g.wallet[Money] = keeperEarned, keeperFee
	g.state = StateTerminal
	if why := g.addToCart(row(t, rowKeeper), -1); why != "" {
		t.Fatal(why)
	}
	if why := g.placeOrder(); why != "" {
		t.Fatal(why)
	}
	if len(g.keepers) != 1 || len(g.couriers) != 0 || g.wallet[Money] != 0 || g.staffPaid != keeperFee {
		t.Fatalf("hired: %d housekeepers, %d couriers, $%v left, $%v counted; want 1, 0, 0, %d", len(g.keepers), len(g.couriers), g.wallet[Money], g.staffPaid, keeperFee)
	}
	k := g.keepers[0]
	g.state = StatePlay
	until(t, g, "it reaches the kitchen", func() bool { return image.Pt(int(k.X), int(k.Y)).In(g.hotel.Kitchen) })

	g.stock = map[Unlock]int{SupplyWater: 1}
	p.asked, p.want = g.clock, SupplyWater
	until(t, g, "it takes the Water", func() bool { return k.job == serving })
	if g.stock[SupplyWater] != 0 {
		t.Fatalf("carrying the Water, the stock still has %d", g.stock[SupplyWater])
	}
	money := g.wallet[Money]
	until(t, g, "it serves the Room", func() bool { return !p.asking() })
	if g.wallet[Money] <= money || g.serves != 1 || g.stock[SupplyWater] != 0 {
		t.Fatalf("served: $%v from $%v, %d serves, stock %d; want paid, 1, 0", g.wallet[Money], money, g.serves, g.stock[SupplyWater])
	}

	// nothing in stock: it stays; an ask that lapses on the way puts the Supply back
	p.asked, p.want = g.clock, SupplyFood
	for range 60 {
		g.step(0, 0)
	}
	if k.job != resting {
		t.Fatalf("with no Food in stock its job is %d, want resting", k.job)
	}
	g.stock[SupplyFood] = 1
	until(t, g, "it takes the Food", func() bool { return k.job == serving })
	p.asked = 0
	until(t, g, "it gives up", func() bool { return k.job == resting })
	if g.stock[SupplyFood] != 1 {
		t.Fatalf("the Food it couldn't hand over: stock %d, want 1", g.stock[SupplyFood])
	}
}

// A Room whose Party complains of the dirt is cleaned by the housekeeper, and the Party goes back in.
func TestKeeperCleans(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	free := g.floorCells(0)
	var spots []spot
	for i := range dirtLimit/spotUnit + 1 {
		spots = append(spots, spot{free[i], false, spotUnit})
	}
	g.dirt, g.dirtN = [][]spot{spots, nil}, []int{len(spots), 0}
	g.setDirtLevel(0, len(spots)*spotUnit)
	g.wallet[Money] = keeperWage
	p := g.arrive(0)
	until(t, g, "the Party complains", func() bool { return p.state == complaining }) // no one on the payroll yet: it sweeps first
	g.hire()
	until(t, g, "the Room is cleaned", func() bool { return g.normalDirt(0) == 0 })
	until(t, g, "the Party is back in", func() bool { return p.state == staying })
	k := g.keepers[0]
	until(t, g, "it walks out of the Room", func() bool { return g.roomAt(k.X, k.Y) < 0 })
}

// With nothing to do it roams the whole hotel, never stopping in anyone's way.
func TestKeeperRoams(t *testing.T) {
	g, _ := headless(t, 3, 2, 1)
	g.openAtStart = false // no Guests: nothing to do
	g.startRun()
	g.wallet[Money] = 10 * keeperWage
	g.hire()
	k, h := g.keepers[0], g.hotel
	hall, corridor := false, false
	for range 2 * dayFrames {
		g.step(0, 0)
		at := image.Pt(int(k.X), int(k.Y))
		if len(k.Path) > 0 {
			continue
		}
		if g.roomAt(k.X, k.Y) >= 0 || at.In(h.Terminal) || at.In(h.MirrorSpot) || slices.ContainsFunc(h.Rooms, func(r world.Room) bool { return at.In(r.Spot()) }) {
			t.Fatalf("it stopped at %v, in the way", at)
		}
		hall, corridor = hall || at.In(h.Lobby), corridor || at.Y < h.HallY
	}
	if !hall || !corridor {
		t.Fatalf("in two days it stood in the hall %v, in a Corridor %v; want both", hall, corridor)
	}
}

// The wage: taken a day after the hire and every day after. Short of it, the housekeeper stops and says so;
// paid as soon as the Money is there, it works on; still owed at the next payday, it walks out. Near a payday
// the wallet can't cover, it worries aloud.
func TestKeeperWage(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.openAtStart = false // every Room boarded up: no Guest pays in meanwhile
	g.startRun()
	g.wallet[Money] = keeperWage
	g.hire()
	k := g.keepers[0]
	day := func() {
		for range dayFrames {
			g.step(0, 0)
		}
	}
	if got := g.keeperLines(k); slices.Equal(got, keeperWorry) || slices.Equal(got, keeperOwed) {
		t.Fatalf("paid for, a day ahead, it says %q", got)
	}
	day()
	if k.unpaid || g.wallet[Money] != 0 || g.staffPaid != keeperWage {
		t.Fatalf("after a day: unpaid %v, $%v left, $%v counted; want paid", k.unpaid, g.wallet[Money], g.staffPaid)
	}
	g.clock = k.due - hourFrames
	if got := g.keeperLines(k); !slices.Equal(got, keeperWorry) {
		t.Fatalf("an hour from a payday the wallet can't cover it says %q, want its worry", got)
	}
	g.clock = k.due - dayFrames + 1
	day()
	if !k.unpaid || k.job == quitting || !slices.Equal(g.keeperLines(k), keeperOwed) {
		t.Fatalf("a payday missed: unpaid %v, job %d, says %q; want owed, staying, saying so", k.unpaid, k.job, g.keeperLines(k))
	}
	g.wallet[Money] = keeperWage
	g.step(0, 0)
	if k.unpaid || g.wallet[Money] != 0 {
		t.Fatalf("the Money came: unpaid %v, $%v left; want the wage taken at once", k.unpaid, g.wallet[Money])
	}
	day() // this payday is missed,
	day() // and so the next: it leaves
	if g.quits != 1 || g.staffed() != 0 {
		t.Fatalf("two paydays missed: %d walked out, %d on the payroll; want 1, 0", g.quits, g.staffed())
	}
	until(t, g, "it is out the Exit", func() bool { return len(g.keepers) == 0 })
}

// It speaks as the Concierge comes near, not all the while he stands there; a problem is always said.
func TestKeeperSpeaks(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.DevStaff(1, 0)
	k := g.keepers[0]
	k.unpaid = true
	g.player.X, g.player.Y = k.X-16, k.Y
	g.step(0, 0)
	if !slices.Contains(keeperOwed, k.said.text) {
		t.Fatalf("owed its wage, the Concierge beside it, it says %q", k.said.text)
	}
	at := k.said.at
	for range 5 * sayFrames {
		g.step(0, 0)
	}
	if k.said.at != at {
		t.Fatal("it said it again while the Concierge stood there")
	}
}

// The store's limit, and the sim's row: the staffer's hotel keeps its housekeeper paid.
func TestStaffer(t *testing.T) {
	g := storeRun(t)
	g.earned[Money], g.wallet[Money] = 10000, 10000
	i := row(t, rowKeeper)
	for n := range keeperCap(g.ownedCount()) {
		if why := g.addToCart(i, -1); why != "" {
			t.Fatalf("hire %d: %q", n+1, why)
		}
	}
	if g.addToCart(i, -1) == "" {
		t.Fatalf("the cart took housekeeper %d", keeperCap(g.ownedCount())+1)
	}
	if testing.Short() {
		return
	}
	r := RunSim(Config{Cols: 2, Rows: 1, Days: 4, Seed: 1, Name: "staffer", Strategy: Strategies["staffer"], React: 30, KeyGap: 6})
	if len(r.Failures) > 0 {
		t.Fatal(r.Failures[0])
	}
	if r.Hired != 1 || r.Quit != 0 || r.Serves == 0 {
		t.Fatalf("the staffer hired %d, %d walked out, %d serves; want 1, 0 and some", r.Hired, r.Quit, r.Serves)
	}
}

// The staff never get stuck on each other (2026-11-14: a courier and a housekeeper met and stood there): a
// courier and a housekeeper walking each to where the other started, between many pairs of places, all arrive.
func TestStaffPass(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.openAtStart = false
	g.startRun()
	g.wallet[Money] = 10 * keeperWage
	g.hire()
	k := g.keepers[0]
	spots := []image.Point{g.hotel.Spawn, g.hotel.Archway, g.doorDrop(0), g.doorDrop(1), g.stockDrop(), g.hotel.Mirror, g.hotel.ConciergeStart}
	for range 12 {
		spots = append(spots, g.roamSpot(k))
	}
	for i, a := range spots {
		for _, b := range spots[i+1:] {
			if g.hotel.Map.Cell(a) == g.hotel.Map.Cell(b) {
				continue
			}
			k.X, k.Y, k.job, k.rest = float64(a.X), float64(a.Y), quitting, 0 // quitting: it only walks where it is sent
			g.sendKeeper(k, b, -1)
			c := &courier{Guest: entities.NewGuest(b, courierName), drops: []drop{{at: a}}}
			c.SetPath(g.hotel.Path(b, a, -1))
			g.couriers = []*courier{c}
			for f := 0; len(k.Path) > 0 || len(c.drops) > 0; f++ {
				if f > 40*tps {
					t.Fatalf("between %v and %v: after 40 s the housekeeper is at %v, the courier at %v", a, b, image.Pt(int(k.X), int(k.Y)), image.Pt(int(c.X), int(c.Y)))
				}
				g.hotel.UpdateDoors(g.doorActors(), g.boarded, 1)
				g.walk(&k.Guest, keeperSpeed)
				if g.walk(&c.Guest, courierSpeed) {
					c.drops = nil
				}
			}
		}
	}
}

// Two housekeepers swapping places through the kitchen, from every cell of it, at a stroll and at work, both
// arrive (2026-11-14: each went round the other's way, again and again, and neither ever passed).
func TestKeepersPass(t *testing.T) {
	for _, grid := range [][2]int{{2, 1}, {3, 3}} {
		g, _ := headless(t, grid[0], grid[1], 1)
		g.openAtStart = false
		g.startRun()
		g.wallet[Money] = 10 * keeperWage
		g.hire()
		g.hire()
		a, b := g.keepers[0], g.keepers[1]
		m, h := g.hotel.Map, g.hotel
		out := []image.Point{h.Spawn, h.Archway, g.doorDrop(0), h.ConciergeStart, h.Mirror}
		for range 8 {
			out = append(out, g.roamSpot(a))
		}
		for _, j := range []job{resting, quitting} { // resting strolls; quitting walks fast, and only where it is sent
			for c := range m.W * m.H {
				p := m.Center(c%m.W, c/m.W)
				if !p.In(h.Kitchen) || m.Solid(c%m.W, c/m.W) {
					continue
				}
				for _, q := range out {
					if h.Path(p, q, -1) == nil || h.Path(q, p, -1) == nil {
						continue
					}
					g.keepers = []*housekeeper{a, b}
					for _, k := range g.keepers {
						k.job, k.rest, k.due = j, math.MaxInt, math.MaxInt
					}
					a.X, a.Y, b.X, b.Y = float64(p.X), float64(p.Y), float64(q.X), float64(q.Y)
					g.sendKeeper(a, q, -1)
					g.sendKeeper(b, p, -1)
					for f := 0; (len(a.Path) > 0 || len(b.Path) > 0) && len(g.keepers) == 2; f++ {
						if f > 40*tps {
							t.Fatalf("grid %v, job %d, between %v and %v: after 40 s one is at %v, the other at %v", grid, j, p, q, image.Pt(int(a.X), int(a.Y)), image.Pt(int(b.X), int(b.Y)))
						}
						g.step(0, 0)
					}
				}
			}
		}
	}
}

// 08 B1 (Housekeepers): an idle housekeeper sweeps the Room with the most regular dirt; spiritual dirt stays
// the player's and the Priests'.
func TestKeeperSweeps(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.roomOwned, g.arrivals = nil, make([]arrival, len(g.hotel.Rooms)) // Rooms open, no Guests booked
	dirty := func(room, n int, spirit bool) []spot {
		free := g.floorCells(room)
		var ss []spot
		for i := range n {
			ss = append(ss, spot{free[i], spirit, spotUnit})
		}
		return ss
	}
	a, b := dirty(0, 1, false), dirty(1, 2, false)
	g.dirt, g.dirtN = [][]spot{a, b}, []int{1, 2}
	g.setDirtLevel(0, spotUnit)
	g.setDirtLevel(1, 2*spotUnit)
	g.wallet[Money] = 10 * keeperWage
	g.hire()
	until(t, g, "the dirtier Room is swept", func() bool { return g.normalDirt(1) == 0 })
	if g.normalDirt(0) == 0 {
		t.Error("the lighter Room was swept first")
	}
	until(t, g, "the other Room is swept", func() bool { return g.normalDirt(0) == 0 })
	g.dirt[0] = dirty(0, 1, true)
	g.setDirtLevel(0, spotUnit)
	for range 3 * minuteFrames * 60 {
		g.step(0, 0)
	}
	if n := len(g.spots(0)); n != 1 {
		t.Errorf("a housekeeper touched the spiritual dirt: %d spots left, want 1", n)
	}
}

// 08 B1 (Running shoes): +15% walking speed a tier, three tiers, for the staff; the row shows the next tier.
func TestRunningShoes(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.wallet = [2]Num{0, 1000}
	if g.staffSpeed() != 1 {
		t.Fatalf("no shoes: speed %v, want 1", g.staffSpeed())
	}
	row := func(tier int) int {
		return slices.IndexFunc(shop, func(it Item) bool { return it.Shoes == tier })
	}
	for tier := 1; tier <= 3; tier++ {
		for _, i := range tabItems(tabStaff) {
			if shop[i].Shoes != 0 && !g.gone(i) && shop[i].Shoes != tier {
				t.Fatalf("tier %d: the row of tier %d is listed", tier, shop[i].Shoes)
			}
		}
		i := row(tier)
		if g.gone(i) {
			t.Fatalf("tier %d row is not listed", tier)
		}
		g.hire() // someone to put shoes on
		g.earned[Ecto] = 1000
		if why := g.addToCart(i, -1); why != "" {
			t.Fatalf("tier %d: %s", tier, why)
		}
		if why := g.placeOrder(); why != "" {
			t.Fatal(why)
		}
		if want := 1 + 0.15*float64(tier); math.Abs(g.staffSpeed()-want) > 1e-9 {
			t.Errorf("tier %d: speed %v, want %v", tier, g.staffSpeed(), want)
		}
	}
	for _, i := range tabItems(tabStaff) {
		if shop[i].Shoes != 0 && !g.gone(i) {
			t.Errorf("all tiers bought, row %q still listed", shop[i].Name)
		}
	}
}

// keeperCleanTime: the frames a housekeeper stands at a complaining Room's stains before they are gone, with the
// Cleaning kit at level kit.
func keeperCleanTime(t *testing.T, kit int) int {
	t.Helper()
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.up[CleaningKit] = kit
	free := g.floorCells(0)
	var spots []spot
	for i := range dirtLimit/spotUnit + 1 {
		spots = append(spots, spot{free[i], false, spotUnit})
	}
	g.dirt, g.dirtN = [][]spot{spots, nil}, []int{len(spots), 0}
	g.setDirtLevel(0, len(spots)*spotUnit)
	g.wallet[Money] = keeperWage
	p := g.arrive(0)
	until(t, g, "the Party complains", func() bool { return p.state == complaining })
	g.hire()
	k := g.keepers[0]
	until(t, g, "it reaches the stains", func() bool { return k.job == cleaning && len(k.Path) == 0 && g.roomAt(k.X, k.Y) == 0 })
	n := 0
	for ; g.normalDirt(0) > 0; n++ {
		if n > 10*keeperClean {
			t.Fatal("the Room is never cleaned")
		}
		g.step(0, 0)
	}
	return n
}

// A housekeeper's clean takes keeperClean frames of work (2026-11-20, T380), which the Cleaning kit cuts by 15% a level.
func TestKeeperCleanTakesTime(t *testing.T) {
	base, kit := keeperCleanTime(t, 0), keeperCleanTime(t, 2)
	if base < keeperClean-2 || base > keeperClean+2 {
		t.Errorf("a clean took %d frames, want about %d", base, keeperClean)
	}
	if want := scaleInt(keeperClean, upMulAt(CleaningKit, 2)); kit < want-2 || kit > want+2 {
		t.Errorf("with the kit at level 2 a clean took %d frames, want about %d", kit, want)
	}
}
