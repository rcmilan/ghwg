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

// D46: a Spin at Run start, then after every spinEvery-th Shift only.
func TestSpinCadence(t *testing.T) {
	g := mustNew(t)
	g.types = fakeTypes(g, map[string]int{"A": 10})
	g.startShift(false) // a new Run
	if !allNamed(g, "A") {
		t.Fatalf("Run start did not Spin: %v", roomTypeNames(g))
	}
	current := "A"
	for shift := 1; shift <= 6; shift++ {
		next := string(rune('A' + shift))
		g.types = fakeTypes(g, map[string]int{next: 10})
		g.endShift()
		if shift%spinEvery == 0 {
			current = next
		}
		if !allNamed(g, current) {
			t.Fatalf("after Shift %d: rooms %v, want all %s", shift, roomTypeNames(g), current)
		}
	}
	g.types = fakeTypes(g, map[string]int{"Z": 10})
	g.startShift(false) // back to a new Run: Spins at once, whatever the old Shift count
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
	g.spin(world.Draw(fakeTypes(g, map[string]int{"Changed": 99}), len(g.hotel.Rooms), g.rng)) // a mid-stay change must not reach the coins
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
	g.spin(next)
	if g.blocked(entities.FootBox(g.player.X, g.player.Y)) {
		t.Fatalf("Concierge left inside a solid at (%.0f,%.0f)", g.player.X, g.player.Y)
	}
	if d := image.Pt(int(g.player.X), int(g.player.Y)).Sub(r.Stay); d.X*d.X+d.Y*d.Y > 2*world.TileSize*world.TileSize {
		t.Errorf("Concierge moved %v from the Stay Spot, want the nearest open cell", d)
	}
}
