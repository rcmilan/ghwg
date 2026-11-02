package game

import (
	"image"
	"slices"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// fakeTypes: copies of the first loaded Room Type (same tiles) under new names and Rates.
func fakeTypes(g *Game, rates map[string]int) []world.RoomType {
	var ts []world.RoomType
	for name, rate := range rates {
		rt := g.types[0]
		rt.Name, rt.Rate = name, rate
		ts = append(ts, rt)
	}
	slices.SortFunc(ts, func(a, b world.RoomType) int { return a.Rate - b.Rate })
	return ts
}

func roomTypeNames(g *Game) []string {
	var ns []string
	for _, r := range g.hotel.Rooms {
		ns = append(ns, r.Type.Name)
	}
	return ns
}

func allNamed(g *Game, name string) bool {
	for _, n := range roomTypeNames(g) {
		if n != name {
			return false
		}
	}
	return true
}

// D46: a Spin at Run start, then each time the last guest leaves (the end of Shift 2).
func TestSpinCadence(t *testing.T) {
	g := mustNew(t)
	g.types = fakeTypes(g, map[string]int{"A": 10})
	g.startShift(false) // a new Run
	if !allNamed(g, "A") {
		t.Fatalf("Run start did not Spin: %v", roomTypeNames(g))
	}
	for cycle := 1; cycle <= 3; cycle++ {
		next := string(rune('A' + cycle))
		g.types = fakeTypes(g, map[string]int{next: 10})
		g.endShift()
		if !allNamed(g, next) {
			t.Fatalf("after Shift 2 of cycle %d: rooms %v, want all %s", cycle, roomTypeNames(g), next)
		}
	}
	g.types = fakeTypes(g, map[string]int{"Z": 10})
	g.startShift(false) // back to a new Run: Spins at once
	if !allNamed(g, "Z") {
		t.Fatalf("a new Run did not Spin: %v", roomTypeNames(g))
	}
}

// D53: a coin pays the Rate of the guest's Room when it was booked, even if the type changes later.
func TestCoinIsRateAtBooking(t *testing.T) {
	g := mustNew(t)
	g.types = fakeTypes(g, map[string]int{"Cheap": 10, "Dear": 30})
	g.startShift(false)
	booked := map[int]int{} // room index -> Rate at Booking
	for i, r := range g.hotel.Rooms {
		booked[i] = r.Type.Rate
	}
	for i := range g.confirmed {
		g.confirmed[i] = true
	}
	g.beginArrivals()
	g.spin(world.Draw(fakeTypes(g, map[string]int{"Changed": 99}), len(g.hotel.Rooms), g.rng), true) // a mid-stay change must not reach the coins
	for f := 0; g.state != StateBooking; f++ {
		if f == 20000 {
			t.Fatal("Shift never ended")
		}
		g.player.X, g.player.Y = float64(g.hotel.ConciergeStart.X), float64(g.hotel.ConciergeStart.Y) // out of coin reach
		g.step(0, 0)
	}
	if len(g.pickups) != len(g.hotel.Rooms) {
		t.Fatalf("%d coins, want %d", len(g.pickups), len(g.hotel.Rooms))
	}
	want, got := []int{}, []int{}
	for _, room := range g.assignments {
		want = append(want, booked[room])
	}
	for _, c := range g.pickups {
		got = append(got, c.Value)
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("coin values %v, want the booked Rates %v", got, want)
	}
	g.money = 0
	g.pickups = g.pickups[:1]
	g.player.X, g.player.Y = g.pickups[0].X, g.pickups[0].Y
	v := g.pickups[0].Value
	g.updateMoney()
	if g.money != v {
		t.Errorf("picking up a %d coin gave $%d", v, g.money)
	}
}

// D50: after a Spin, a Concierge standing where the new type is solid is moved to the nearest open spot.
func TestSpinUnsticksConcierge(t *testing.T) {
	g := mustNew(t)
	g.startShift(false)
	r := g.hotel.Rooms[0]
	g.player.X, g.player.Y = float64(r.Stay.X), float64(r.Stay.Y)
	walled := g.types[0]
	walled.Tiles[world.LayerWalls] = slices.Clone(walled.Tiles[world.LayerWalls])
	wall, _ := g.hotel.Map.TS.ID("wall_mid_bottom")
	stay := walled.Stay.Div(world.TileSize)
	walled.Tiles[world.LayerWalls][stay.Y*world.RoomW+stay.X] = wall
	next := make([]world.RoomType, len(g.hotel.Rooms))
	for i := range next {
		next[i] = g.types[0]
	}
	next[0] = walled
	g.spin(next, true)
	if g.blocked(entities.FootBox(g.player.X, g.player.Y)) {
		t.Fatalf("Concierge left inside a solid at (%.0f,%.0f)", g.player.X, g.player.Y)
	}
	if d := image.Pt(int(g.player.X), int(g.player.Y)).Sub(r.Stay); d.X*d.X+d.Y*d.Y > 2*world.TileSize*world.TileSize {
		t.Errorf("Concierge moved %v from the Stay Spot, want the nearest open cell", d)
	}
}

// D48-D50: an animated Spin changes the types at once; each Room's cells change at its own reel stop,
// one after another in Room order.
func TestReelLandsInOrder(t *testing.T) {
	g := mustNew(t)
	all := g.types
	if all[0].Tiles[world.LayerFloor][0] == all[1].Tiles[world.LayerFloor][0] {
		t.Fatal("the first two Room Types share a floor; the test can't tell them apart")
	}
	g.types = all[:1]
	g.startShift(false)
	g.spin(world.Draw(all[1:2], len(g.hotel.Rooms), g.rng), false)
	start := g.reelAt
	if !allNamed(g, all[1].Name) {
		t.Fatalf("types after an animated Spin: %v, want all %s at once (D49)", roomTypeNames(g), all[1].Name)
	}
	floor := func(i int) world.TileID {
		o := g.hotel.Rooms[i].Origin
		return g.hotel.Map.At(world.LayerFloor, o.X+1, o.Y+3)
	}
	last := start + (len(g.hotel.Rooms)-1)*reelStagger + reelFrames
	for g.tick <= last {
		for i := range g.hotel.Rooms {
			stop := start + i*reelStagger + reelFrames
			if landed := floor(i) == all[1].Tiles[world.LayerFloor][0]; landed != (g.tick >= stop) {
				t.Fatalf("tick %d: room %d landed=%v, want its reel to stop at tick %d", g.tick-start, i, landed, stop-start)
			}
		}
		g.step(0, 0)
	}
}

// D48: an animated Spin glides the camera out until every Room is framed, holds while the reels roll,
// then glides back; the Concierge waits throughout. Run-start Spins skip it.
func TestSpinCinematic(t *testing.T) {
	g := mustNew(t)
	g.startShift(false)
	if g.spinning() || g.spinView() != 0 {
		t.Fatal("the Run-start Spin played the cinematic")
	}
	g.spin(world.Draw(g.types, len(g.hotel.Rooms), g.rng), false)
	prev := 0.0
	for g.tick < g.reelAt { // out: rises, reels still
		if f := g.spinView(); f < prev || g.landed > 0 {
			t.Fatalf("tick %d: view %.2f after %.2f, landed %d; want a rising glide before any reel", g.tick, f, prev, g.landed)
		}
		prev = g.spinView()
		g.step(0, 0)
	}
	for g.tick < g.spinEnd()-spinCamFrames { // framed while the reels roll and hold
		if f := g.spinView(); f != 1 || !g.spinning() {
			t.Fatalf("tick %d: view %.2f, spinning %v; want the Rooms framed", g.tick, f, g.spinning())
		}
		g.step(0, 0)
	}
	if g.landed != len(g.hotel.Rooms) {
		t.Fatalf("camera leaving with %d of %d Rooms landed", g.landed, len(g.hotel.Rooms))
	}
	for g.spinning() {
		g.step(0, 0)
	}
	if f := g.spinView(); f != 0 {
		t.Fatalf("cinematic over with view %.2f, want back on the Concierge", f)
	}
	cx, cy, z := g.roomsView() // every Room fits on screen
	for _, r := range g.hotel.Rooms {
		for _, p := range []image.Point{r.Origin, r.Origin.Add(image.Pt(world.RoomW, world.RoomH))} {
			sx, sy := (float64(p.X*world.TileSize)-cx)*z+viewW*zoom/2, (float64(p.Y*world.TileSize)-cy)*z+viewH*zoom/2
			if sx < 0 || sy < 0 || sx > viewW*zoom || sy > viewH*zoom {
				t.Errorf("room %s corner %v lands off screen at (%.0f,%.0f) in the all-Rooms view", r.Number, p, sx, sy)
			}
		}
	}
}

// D61: Space skips the cinematic: every reel lands at once and the camera glides back from wherever it
// got to, without a jump, never slower than the full glide.
func TestSpinSkip(t *testing.T) {
	for _, at := range []struct {
		name  string
		after int // ticks into the cinematic before Space
	}{{"mid-glide", spinCamFrames / 2}, {"framed", spinCamFrames + reelFrames/2}} {
		t.Run(at.name, func(t *testing.T) {
			g := mustNew(t)
			g.startShift(false)
			g.spin(world.Draw(g.types, len(g.hotel.Rooms), g.rng), false)
			for range at.after {
				g.step(0, 0)
			}
			before := g.spinView()
			g.skipSpin()
			if g.landed != len(g.hotel.Rooms) {
				t.Fatalf("%d of %d Rooms landed after a skip", g.landed, len(g.hotel.Rooms))
			}
			prev, end := before, g.tick+spinCamFrames
			for g.spinning() {
				if f := g.spinView(); f > prev+1e-9 || prev-f > 0.1 {
					t.Fatalf("tick %d: view %.3f after %.3f; want a smooth glide back", g.tick, f, prev)
				}
				prev = g.spinView()
				g.step(0, 0)
				if g.tick > end {
					t.Fatalf("camera still out %d ticks after the skip", spinCamFrames)
				}
			}
			if g.spinView() != 0 {
				t.Fatalf("skip ended with view %.2f", g.spinView())
			}
		})
	}
}
