package game

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// keys taps each key in turn, a released frame between.
// view opens the hotel view as a player does (U2): SPACE where it has nothing else to do, here the Lobby's corner.
func view(g *Game, s *script) {
	at := freeSpot(g.hotel)
	g.player.X, g.player.Y = float64(at.X), float64(at.Y)
	keys(g, s, ebiten.KeySpace)
}

func keys(g *Game, s *script, ks ...ebiten.Key) {
	for _, k := range ks {
		s.hold()
		g.Update()
		s.hold(k)
		g.Update()
	}
}

// U1 (was T286): the hotel view follows the hotel: TAB opens it from play, the highlight walks every Room of
// a 2×1 and of a 3×3 with the same keys, and TAB or ESC closes it in one press from any depth. Every action on
// one Room is bought there, paid at once: SPACE on the Room, SPACE on the action; a change of type takes a
// third for the beds. The world waits meanwhile.
func TestHotelView(t *testing.T) {
	for _, grid := range [][2]int{{2, 1}, {3, 3}} {
		g, s := headless(t, grid[0], grid[1], 1)
		g.startRun()
		n := len(g.hotel.Rooms)
		if view(g, s); g.state != StateView || g.termDepth != 1 {
			t.Fatalf("%v: TAB: state %d depth %d, want the hotel view on its Rooms", grid, g.state, g.termDepth)
		}
		clock := g.clock
		seen := map[int]bool{g.termCursor: true}
		for range n {
			keys(g, s, ebiten.KeyD)
			seen[g.termCursor] = true
		}
		if len(seen) != n {
			t.Fatalf("%v: the highlight reached %d Rooms of %d", grid, len(seen), n)
		}
		if g.termCursor = 0; grid[1] > 1 {
			if keys(g, s, ebiten.KeyS); g.termCursor != grid[0] {
				t.Fatalf("%v: S from Room 0 went to %d, want the Room below, %d", grid, g.termCursor, grid[0])
			}
		}
		g.termCursor = 0
		if keys(g, s, ebiten.KeyW); g.termCursor != 0 || g.state != StateView {
			t.Fatalf("%v: W from the top row moved to %d or closed the view", grid, g.termCursor)
		}
		if g.clock != clock {
			t.Fatalf("%v: the Clock ran with the hotel view open", grid)
		}
		keys(g, s, ebiten.KeySpace) // a Room's actions
		if keys(g, s, ebiten.KeyA); g.termDepth != 1 {
			t.Fatalf("%v: A on a Room's actions: depth %d, want back on the Rooms", grid, g.termDepth)
		}
		keys(g, s, ebiten.KeySpace)
		if keys(g, s, ebiten.KeyEscape); g.state != StatePlay {
			t.Fatalf("%v: one ESC from a Room's actions left state %d, want play", grid, g.state)
		}
		view(g, s)
		keys(g, s, ebiten.KeySpace, ebiten.KeySpace) // a boarded Room's key can't be paid: the view stays
		if keys(g, s, ebiten.KeyEscape); g.state != StatePlay {
			t.Fatalf("%v: TAB from a Room's actions left state %d, want play", grid, g.state)
		}
	}
	if tabs != 3 || tabNames != [tabs]string{"SUPPLIES", "HOTEL", "CART"} {
		t.Fatalf("the store's tabs are %v, want no ROOMS tab", tabNames)
	}

	g := closedRun(t)
	s := g.in.(*script)
	g.own(Mirror)
	g.own(BedsTwin)
	g.earned = [2]Num{10000, 10000}
	g.wallet = [2]Num{keyPrice(0) + priestWeak + bedKinds[1].Fee, 0}
	g.state = StatePlay
	pos := func(id Unlock) (ks []ebiten.Key) {
		for _, i := range tabItems(tabRooms) {
			if shop[i].ID == id {
				return ks
			}
			ks = append(ks, ebiten.KeyS)
		}
		t.Fatalf("%s is not a Room's action", id)
		return nil
	}
	view(g, s)
	g.termCursor = 0
	keys(g, s, ebiten.KeyD, ebiten.KeySpace) // Room 102, its actions
	keys(g, s, pos(rowKey)...)
	keys(g, s, ebiten.KeySpace)
	if len(g.couriers) != 1 || len(g.cart) != 0 || g.wallet[Money] != priestWeak+bedKinds[1].Fee || g.state != StateView {
		t.Fatalf("the key: %d couriers, cart %v, Money %v; want it paid at once and on its way", len(g.couriers), g.cart, g.wallet[Money])
	}
	deliver(t, g)
	g.state = StatePlay
	view(g, s)
	keys(g, s, ebiten.KeySpace) // the highlight stayed on Room 102
	keys(g, s, pos(rowFatherK)...)
	keys(g, s, ebiten.KeySpace)
	if !g.priestOn(1) || g.wallet[Money] != bedKinds[1].Fee {
		t.Fatalf("Father K: on Room 102 %v, Money %v", g.priestOn(1), g.wallet[Money])
	}
	g.priests = nil
	p := g.arrive(1)
	g.settleIn(p)
	p.Rate = 0
	keys(g, s, ebiten.KeySpace)
	keys(g, s, pos(rowChange)...)
	keys(g, s, ebiten.KeySpace, ebiten.KeyS, ebiten.KeySpace) // the beds: Twin
	if c := g.changeFor(1); c == nil || c.to.Beds != "Twin" || g.wallet[Money] != 0 {
		t.Fatalf("the change: %+v, Money %v; want Room 102 waiting to turn Twin, paid", c, g.wallet[Money])
	}
	keys(g, s, ebiten.KeySpace) // the same row now cancels it
	keys(g, s, pos(rowChange)...)
	keys(g, s, ebiten.KeySpace)
	if g.changing(1) || g.wallet[Money] != bedKinds[1].Fee {
		t.Fatalf("cancelled: changing %v, Money %v; want the fee back", g.changing(1), g.wallet[Money])
	}
	for _, id := range []Unlock{rowKey, rowFatherK, rowFatherM, rowChange, RoomMirror} {
		if shop[row(t, id)].Tab != tabRooms {
			t.Errorf("%s is not a Room's action", id)
		}
	}
}

// U1: ESC never ends a Run by itself. It closes the Terminal in one press; with nothing open it pauses, the
// world waiting; ESC again or SPACE on Resume goes on, and only SPACE on Quit goes to the title.
func TestPause(t *testing.T) {
	g, s := headless(t, 2, 1, 1)
	g.startRun()
	g.openTerminal()
	if keys(g, s, ebiten.KeyEscape); g.state != StatePlay {
		t.Fatalf("ESC on the Terminal: state %d, want play", g.state)
	}
	clock := g.clock
	keys(g, s, ebiten.KeyEscape)
	keys(g, s, ebiten.KeyD, ebiten.KeyD)
	if g.state != StatePause || g.clock != clock+1 { // the frame before the ESC was one of play
		t.Fatalf("ESC in play: state %d, the Clock %d frames on; want the pause, the world waiting", g.state, g.clock-clock)
	}
	if keys(g, s, ebiten.KeyEscape); g.state != StatePlay {
		t.Fatalf("ESC on the pause: state %d, want play", g.state)
	}
	if keys(g, s, ebiten.KeyEscape, ebiten.KeySpace); g.state != StatePlay {
		t.Fatalf("SPACE on Resume: state %d, want play", g.state)
	}
	if keys(g, s, ebiten.KeyEscape, ebiten.KeyS, ebiten.KeySpace); g.state != StateTitle {
		t.Fatalf("SPACE on Quit: state %d, want the title", g.state)
	}
}

// T310: a bought in-Room Mirror is paid at once and carried by the courier: hung when he reaches the Room's
// door, its guests in or not; a waiting change of beds keeps waiting. One at a time.
func TestRoomMirrorCourier(t *testing.T) {
	g := closedRun(t)
	g.roomOwned[0], g.roomOwned[1] = true, true
	g.own(Mirror)
	g.own(BedsTwin)
	p := g.arrive(0)
	g.settleIn(p)
	p.Rate = 0
	g.wallet = [2]Num{bedKinds[1].Fee, 2 * roomMirrorEcto}
	if why := order(t, g, 0, "Twin"); why != "" {
		t.Fatal(why)
	}
	m := row(t, RoomMirror)
	if why := g.buyNow(m, 0); why != "" {
		t.Fatal(why)
	}
	if g.unlocked(RoomMirror) || !g.hotel.Rooms[0].Mirror.Empty() || len(g.couriers) != 1 || g.wallet[Ecto] != roomMirrorEcto {
		t.Fatalf("bought: unlocked %v, hung %v, %d couriers, Ectoplasm %v; want it paid and on its way", g.unlocked(RoomMirror), !g.hotel.Rooms[0].Mirror.Empty(), len(g.couriers), g.wallet[Ecto])
	}
	if why := g.buyNow(m, 1); why == "" {
		t.Fatal("a second in-Room Mirror was bought while the first is on its way")
	}
	deliver(t, g)
	if r := g.hotel.Rooms[0]; !g.unlocked(RoomMirror) || r.Mirror.Empty() || r.Type.Beds != "Single" || g.changeFor(0) == nil {
		t.Fatalf("delivered: unlocked %v, mirror %v, %s, waiting %v; want it hung, the change to Twin still waiting", g.unlocked(RoomMirror), r.Mirror, r.Type.Beds, g.changeFor(0))
	}
	if _, in := g.inside(0); len(in) == 0 {
		t.Error("hanging the Mirror moved the Party out")
	}
}
