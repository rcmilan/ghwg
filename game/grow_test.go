package game

import (
	"image"
	"slices"
	"testing"

	"ghosthotel/entities"

	"github.com/hajimehoshi/ebiten/v2"
)

// T280 (07 H1): the hotel grows mid-Run, a column or a floor, in either order, and everything follows its
// place. A Room keeps its number, type, Mood, dirt and Party; everyone stands on walkable ground of the part
// they were in and walks on to where they were going; the new Rooms arrive boarded up, nobody booked.
func TestGrowRemap(t *testing.T) {
	for _, order := range [][]bool{{true, false}, {false, true}} {
		g, _ := headless(t, 2, 1, 1)
		g.startRun()
		g.DevDirt(10)
		home := g.arrive(1) // settled in Room 102
		g.settleIn(home)
		g.mood[1] = 37
		walking := g.arrive(0) // on its way in from the street
		away := g.arrive(1)    // and one out for the day, off the map
		away.state = out
		g.sendPriest(0, false)
		for range 4 * tps {
			g.step(0, 0)
		}
		for _, col := range order {
			type was struct {
				mood, dirt int
				beds       string
				owned      bool
			}
			before := map[string]was{}
			stains := map[string][]*ebiten.Image{} // each spot's stain picture, kept and moved with it (08 W3)
			g.stains = map[image.Point]*ebiten.Image{}
			for i, r := range g.hotel.Rooms {
				before[r.Number] = was{g.mood[i], len(g.spots(i)), r.Type.Beds, g.isOwned(i)}
				for _, s := range g.spots(i) {
					g.stains[s.cell] = ebiten.NewImage(1, 1)
					stains[r.Number] = append(stains[r.Number], g.stains[s.cell])
				}
			}
			rooms := map[*Party]string{}
			for _, p := range g.parties {
				rooms[p] = g.hotel.Rooms[p.Room].Number
			}
			inHall := map[*entities.Guest]bool{}
			for _, a := range g.actors() {
				inHall[a] = int(a.Y) >= g.hotel.HallY
			}
			playerInLobby := g.feet().In(g.hotel.Lobby)
			n := len(g.hotel.Rooms)
			if _, err := g.grow(col); err != nil {
				t.Fatal(err)
			}
			h := g.hotel
			cols, rows := h.Grid()
			if want := n + map[bool]int{true: rows, false: cols}[col]; len(h.Rooms) != want || len(g.mood) != want || len(g.roomOwned) != want {
				t.Fatalf("col %v: %d Rooms, %d Moods, %d owned flags; want %d", col, len(h.Rooms), len(g.mood), len(g.roomOwned), want)
			}
			for i, r := range h.Rooms {
				b, old := before[r.Number]
				if !old {
					if g.isOwned(i) || g.arrivals[i].at != 0 || g.ghostArrivals[i].at != 0 {
						t.Errorf("col %v: new Room %s: owned %v, booked %v; want it boarded up, nobody booked", col, r.Number, g.isOwned(i), g.arrivals[i])
					}
					continue
				}
				if g.isOwned(i) != b.owned || g.mood[i] != b.mood || len(g.spots(i)) != b.dirt || r.Type.Beds != b.beds {
					t.Errorf("col %v: Room %s: owned %v, Mood %d, %d dirt spots, %s; want it as it was: Mood %d, %d spots, %s",
						col, r.Number, g.isOwned(i), g.mood[i], len(g.spots(i)), r.Type.Beds, b.mood, b.dirt, b.beds)
				}
				for k, s := range g.spots(i) {
					if !h.Map.Center(s.cell.X, s.cell.Y).In(r.Shell()) {
						t.Errorf("col %v: Room %s: a dirt spot at cell %v is outside it", col, r.Number, s.cell)
					}
					if g.stains[s.cell] != stains[r.Number][k] {
						t.Errorf("col %v: Room %s: the dirt spot at cell %v lost its stain picture", col, r.Number, s.cell)
					}
				}
			}
			for p, number := range rooms {
				if h.Rooms[p.Room].Number != number || p.Guests[0].RoomIndex != p.Room { // on the map or out for the day
					t.Errorf("col %v: the Party of Room %s is now in Room %s, its Guest booked into %d", col, number, h.Rooms[p.Room].Number, p.Guests[0].RoomIndex)
				}
			}
			if at := image.Pt(int(home.Guests[0].X), int(home.Guests[0].Y)); !at.In(h.Rooms[home.Room].Shell()) {
				t.Errorf("col %v: the settled Guest stands at %v, outside its Room %v", col, at, h.Rooms[home.Room].Shell())
			}
			for _, a := range g.actors() {
				if h.Map.BlockedBox(entities.FootBox(a.X, a.Y)) || int(a.Y) >= h.HallY != inHall[a] {
					t.Errorf("col %v: %s stands at (%.0f,%.0f): in a wall, or out of the part it was in", col, a.Character, a.X, a.Y)
				}
			}
			if g.blocked(entities.FootBox(g.player.X, g.player.Y)) || g.feet().In(h.Lobby) != playerInLobby {
				t.Errorf("col %v: the Concierge stands at (%.0f,%.0f): in a wall, or out of the Lobby he was in", col, g.player.X, g.player.Y)
			}
		}
		for f := 0; walking.state != staying; f++ {
			if f > 60*tps {
				at := image.Pt(int(walking.Guests[0].X), int(walking.Guests[0].Y))
				t.Fatalf("order %v: the Party walking in never reached Room %s; it stands at %v", order, g.hotel.Rooms[walking.Room].Number, at)
			}
			g.step(0, 0)
		}
	}
}

// T281 (07 H1): "New column" and "New floor" are HOTEL rows, paid in Ectoplasm, each expansion twice the last.
// One at a time: the hotel grows as the Terminal closes, the world waiting while the cinematic plays, ESC
// or not. At 3×3 there is nothing left to buy, and a new Run starts small again.
func TestGrowBuy(t *testing.T) {
	g, s := headless(t, 2, 1, 1)
	g.startRun()
	g.own(Mirror)
	buy := func(id Unlock) string {
		g.openTerminal()
		g.termTab = tabHotel
		for k, i := range tabItems(tabHotel) {
			if shop[i].ID == id {
				g.termPick = k
			}
		}
		keys(g, s, ebiten.KeySpace)
		if len(g.cart) != 1 {
			why := g.refused
			keys(g, s, ebiten.KeyEscape)
			return why
		}
		g.termTab, g.termPick = tabCart, len(g.cart)
		keys(g, s, ebiten.KeySpace) // checkout: an order placed closes the store
		return ""
	}
	if why := buy(GrowCol); why != "CAN'T AFFORD" {
		t.Fatalf("New column with no Ectoplasm: %q", why)
	}
	want := [][2]int{{3, 1}, {3, 2}, {3, 3}}
	for k, id := range []Unlock{GrowCol, GrowFloor, GrowFloor} {
		price := growCost(g)[Ecto]
		if price != roomPrice(Num(int(growEcto)*(k+1)), g.ownedCount()) || g.priceOf(row(t, id)) != (Cost{Ecto: price}) { // a step dearer each growth
			t.Fatalf("expansion %d costs %v (the row says %v), want %d Ectoplasm before the Rooms owned", k+1, price, g.priceOf(row(t, id)), int(growEcto)*(k+1))
		}
		g.wallet[Ecto] = price
		if why := buy(id); why != "" || g.wallet[Ecto] != 0 || len(g.couriers) != 0 {
			t.Fatalf("%s: %q, %v Ectoplasm left, %d couriers; want it paid, no courier", id, why, g.wallet[Ecto], len(g.couriers))
		}
		clock := g.clock
		if keys(g, s, ebiten.KeyEscape); g.state != StateGrow {
			t.Fatalf("%s: state %d after the store closed and an ESC, want the cinematic", id, g.state)
		}
		s.hold()
		for f := 0; g.state == StateGrow; f++ {
			if f > 5*tps {
				t.Fatalf("%s: the cinematic never ends", id)
			}
			g.Update()
		}
		if c, r := g.hotel.Grid(); [2]int{c, r} != want[k] || g.clock > clock+1 || g.state != StatePlay {
			t.Fatalf("%s: the hotel is %dx%d, the Clock %d frames on, state %d; want %v, the world having waited", id, c, r, g.clock-clock, g.state, want[k])
		}
		finish(g) // 08 W5: the beat the Growth started is played out, or the next Growth waits (questGate)
	}
	g.wallet[Ecto] = 1e6
	if l := g.listed(tabHotel); slices.ContainsFunc(l, func(k int) bool { return shop[tabItems(tabHotel)[k]].Grow != 0 }) { // 08 F0-T: not listed
		t.Errorf("at 3x3 the store still lists a growth: %v", l)
	}
	if g.startRun(); len(g.hotel.Rooms) != 2 || len(g.mood) != 2 {
		t.Errorf("a new Run starts with %d Rooms, want the 2 it opened with", len(g.hotel.Rooms))
	}
}

// T283 (07 H1): the grower, playing a whole Run, grows the hotel and leaves no Failure behind.
func TestGrower(t *testing.T) {
	if testing.Short() {
		t.Skip("a 3-day Run")
	}
	res := RunSim(Config{Cols: 2, Rows: 1, Days: 3, Seed: 1, Name: "grower", Strategy: Strategies["grower"], React: 30, KeyGap: 6})
	if len(res.Failures) > 0 || res.Rooms <= 2 || len(res.Grown) == 0 {
		t.Errorf("the grower ended with %d Rooms, grew at %v, Failures %v; want a grown hotel and none", res.Rooms, res.Grown, res.Failures)
	}
}

// 08 W3: pulled back, a growth draws the hotel once; the warm-up's ghost frame must not leave its picture in the
// real dimension's slide (a print showed the arcade palette mid-growth).
func TestGrowPictureFollowsDimension(t *testing.T) {
	g, err := NewGrid(2, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	g.Start(false)
	g.DevGrow("col")
	for g.viewT < 1 {
		g.Update()
	}
	screen := ebiten.NewImage(screenW, screenH)
	g.ghost = true
	g.drawFrame(screen)
	g.ghost = false
	g.drawFrame(screen)
	if g.growing == nil || g.growing.drawn != "real" {
		t.Fatalf("the slide's picture is %q, want the real one", g.growing.drawn)
	}
}
