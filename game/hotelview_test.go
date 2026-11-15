package game

import (
	"slices"
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

// 08 F0 (Hotel view, ESC and HUD): SPACE where it has nothing else to do opens the hotel view on the whole hotel,
// no Room picked, and SPACE there does nothing. WASD picks a Room (the one the view remembers), then walks every
// Room of a 2×1 and of a 3×3 with the same keys; SPACE opens its ring. ESC steps back one level: the ring, the
// Room picked, the hotel, play.
func TestHotelView(t *testing.T) {
	for _, grid := range [][2]int{{2, 1}, {3, 3}} {
		g, s := headless(t, grid[0], grid[1], 1)
		g.startRun()
		g.own(Mirror) // the Fathers, so a Room has a ring
		g.earned = [2]Num{10000, 10000}
		n := len(g.hotel.Rooms)
		if view(g, s); g.state != StateView || g.termDepth != 1 {
			t.Fatalf("%v: SPACE: state %d depth %d, want the hotel view, no Room picked", grid, g.state, g.termDepth)
		}
		room := g.termCursor
		if keys(g, s, ebiten.KeySpace); g.termDepth != 1 {
			t.Fatalf("%v: SPACE with no Room picked: depth %d, want nothing done", grid, g.termDepth)
		}
		if keys(g, s, ebiten.KeyD); g.termDepth != 2 || g.termCursor != room {
			t.Fatalf("%v: D with no Room picked: depth %d on Room %d, want Room %d picked", grid, g.termDepth, g.termCursor, room)
		}
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
		if keys(g, s, ebiten.KeySpace); g.termDepth != 3 {
			t.Fatalf("%v: SPACE on a Room: depth %d, want its ring", grid, g.termDepth)
		}
		for _, depth := range []int{2, 1} {
			if keys(g, s, ebiten.KeyEscape); g.state != StateView || g.termDepth != depth {
				t.Fatalf("%v: ESC: state %d depth %d, want the hotel view at depth %d", grid, g.state, g.termDepth, depth)
			}
		}
		if keys(g, s, ebiten.KeyEscape); g.state != StatePlay {
			t.Fatalf("%v: ESC with no Room picked: state %d, want play", grid, g.state)
		}
	}
	if tabs != 4 || tabNames != [tabs]string{"SUPPLIES", "HOTEL", "STAFF", "CART"} {
		t.Fatalf("the store's tabs are %v, want no ROOMS tab", tabNames)
	}
}

// 08 F0 (Hotel view): the hotel view no longer pauses: the world runs on at viewSlow of its time scale.
func TestViewSlowMotion(t *testing.T) {
	g, s := headless(t, 2, 1, 1)
	g.startRun()
	view(g, s)
	clock := g.clock
	for range 40 {
		s.hold()
		g.Update()
	}
	if want := clock + int(40*viewSlow); g.clock != want {
		t.Fatalf("40 frames of the hotel view ran the Clock %d frames, want %d", g.clock-clock, want-clock)
	}
}

// Every action on one Room is bought on its ring, paid at once: SPACE on the Room, SPACE on the action; a change
// of type takes a third on its beds, and the same slot then calls it off, last round the ring.
func TestRingBuys(t *testing.T) {
	g := closedRun(t)
	s := g.in.(*script)
	g.own(Mirror)
	g.own(BedsTwin)
	g.earned = [2]Num{10000, 10000}
	g.wallet = [2]Num{keyPrice(0) + priestWeak + bedKinds[1].Fee, 0}
	g.state = StatePlay
	to := func(id Unlock) (ks []ebiten.Key) { // D from where the ring opens to the action
		r := g.ring(g.termCursor)
		for k := g.ringStart(g.termCursor); shop[r[k]].ID != id; k = turnRing(r, k, 1) {
			if ks = append(ks, ebiten.KeyD); len(ks) > ringSlots {
				t.Fatalf("%s is not on Room %d's ring", id, g.termCursor)
			}
		}
		return ks
	}
	view(g, s)
	g.termCursor = 0
	keys(g, s, ebiten.KeyD, ebiten.KeyD, ebiten.KeySpace) // Room 101 picked, then 102; its ring: the key alone
	keys(g, s, ebiten.KeySpace)
	if len(g.couriers) != 1 || len(g.cart) != 0 || g.wallet[Money] != priestWeak+bedKinds[1].Fee || g.state != StateView || g.termDepth != 2 {
		t.Fatalf("the key: %d couriers, cart %v, Money %v, depth %d; want it paid at once and on its way, the Room still picked", len(g.couriers), g.cart, g.wallet[Money], g.termDepth)
	}
	deliver(t, g)
	view(g, s)
	keys(g, s, ebiten.KeyD, ebiten.KeySpace) // Room 102 again, its ring
	keys(g, s, to(rowFatherK)...)
	keys(g, s, ebiten.KeySpace)
	if !g.priestOn(1) || g.wallet[Money] != bedKinds[1].Fee {
		t.Fatalf("Father K: on Room 102 %v, Money %v", g.priestOn(1), g.wallet[Money])
	}
	g.priests = nil
	p := g.arrive(1)
	g.settleIn(p)
	p.Rate = 0
	if keys(g, s, ebiten.KeySpace); g.ring(1)[g.termPick] != row(t, rowChange) {
		t.Fatalf("the ring opened on slot %d, want its top, Change type", g.termPick)
	}
	keys(g, s, ebiten.KeySpace, ebiten.KeySpace) // its beds: Twin, the only one Room 102 hasn't got
	if c := g.changeFor(1); c == nil || c.to.Beds != "Twin" || g.wallet[Money] != 0 {
		t.Fatalf("the change: %+v, Money %v; want Room 102 waiting to turn Twin, paid", c, g.wallet[Money])
	}
	keys(g, s, ebiten.KeySpace)
	keys(g, s, to(rowChange)...)
	keys(g, s, ebiten.KeySpace)
	if g.changing(1) || g.wallet[Money] != bedKinds[1].Fee {
		t.Fatalf("cancelled: changing %v, Money %v; want the fee back", g.changing(1), g.wallet[Money])
	}
	for _, id := range []Unlock{rowKey, rowFatherK, rowFatherM, rowChange, RoomMirror, rowDeluxe} {
		if shop[row(t, id)].Tab != tabRooms {
			t.Errorf("%s is not a Room's action", id)
		}
	}
}

// 2026-11-15 (Room menu, Ring layout, Unaffordable actions, One of each Father): a Room's actions sit round it in
// fixed slots, only those it can take now; the ring opens on its top, or the next clockwise; D/S turn it
// clockwise and A/W back, past the empty slots. One the wallet can't pay stays and SPACE on it shakes no. Change
// type opens the beds the Room hasn't got. A Father's call is gone while he is anywhere in the hotel; a boarded
// Room shows its key alone, at the top.
func TestRing(t *testing.T) {
	g, s := headless(t, 2, 1, 1)
	g.startRun()
	g.own(Mirror)
	g.earned = [2]Num{10000, 10000}
	ids := func(room int) (v []Unlock) {
		for _, i := range g.roomRows(room) {
			v = append(v, shop[i].ID)
		}
		return v
	}
	if got := ids(0); !slices.Equal(got, []Unlock{RoomMirror, rowFatherM, rowFatherK}) {
		t.Fatalf("one bed layout, Deluxe locked: the ring %v", got)
	}
	if k := g.ringStart(0); k != ringSlot[RoomMirror] {
		t.Fatalf("the top empty: the ring opens on slot %d, want the next action clockwise, the Room mirror", k)
	}
	g.own(BedsTwin)
	g.own(BedsDouble)
	g.own(Deluxe)
	if got := ids(0); !slices.Equal(got, []Unlock{rowChange, rowDeluxe, RoomMirror, rowFatherM, rowFatherK}) {
		t.Fatalf("the full ring %v, want Change type, Deluxe, Room mirror, Father M, Father K", got)
	}
	g.wallet = [2]Num{}
	view(g, s)
	g.termCursor = 0
	keys(g, s, ebiten.KeyD, ebiten.KeySpace)
	for _, k := range []struct {
		key  ebiten.Key
		want Unlock
	}{{ebiten.KeyD, rowDeluxe}, {ebiten.KeyS, RoomMirror}, {ebiten.KeyW, rowDeluxe}, {ebiten.KeyA, rowChange}, {ebiten.KeyA, rowFatherK}} {
		if keys(g, s, k.key); shop[g.ring(0)[g.termPick]].ID != k.want {
			t.Fatalf("turned to %s, want %s", shop[g.ring(0)[g.termPick]].ID, k.want)
		}
	}
	if keys(g, s, ebiten.KeySpace); g.priestOn(0) || g.termDepth != 3 || g.shake == 0 {
		t.Fatalf("SPACE on Father K with no Money: called %v, depth %d, shake %d; want no call, the ring kept, a shake", g.priestOn(0), g.termDepth, g.shake)
	}
	keys(g, s, ebiten.KeyD, ebiten.KeySpace)
	if g.termDepth != 4 || !slices.Equal(g.changeBeds(0), []int{1, 2}) || g.termBeds != 1 {
		t.Fatalf("Change type: depth %d, beds %v on %d; want Twin and Double, never Single, on Twin", g.termDepth, g.changeBeds(0), g.termBeds)
	}
	if keys(g, s, ebiten.KeyD); g.termBeds != 2 {
		t.Fatalf("D on the beds: %d, want Double", g.termBeds)
	}
	if keys(g, s, ebiten.KeyEscape); g.termDepth != 3 {
		t.Fatalf("ESC on the beds: depth %d, want the ring", g.termDepth)
	}
	g.sendPriest(1, false)
	if got := ids(0); slices.Contains(got, rowFatherK) || !slices.Contains(got, rowFatherM) {
		t.Fatalf("Father K at work in Room 102: Room 101's ring %v, want Father M and not Father K", got)
	}
	g.roomOwned = []bool{true, false}
	if got := ids(1); !slices.Equal(got, []Unlock{rowKey}) || g.ring(1)[0] != row(t, rowKey) {
		t.Fatalf("a boarded Room's ring %v, want its key alone, at the top", got)
	}
}

// 2026-11-14 (Stray cancel): SPACE where it has nothing to do never calls a waiting change off: from play it opens
// the hotel view, where SPACE with no Room picked does nothing; and a Room's ring opens past calling it off.
func TestStrayCancel(t *testing.T) {
	g, s := headless(t, 2, 1, 1)
	g.startRun()
	g.own(Mirror)
	g.own(BedsTwin)
	g.earned, g.wallet = [2]Num{10000, 10000}, [2]Num{1000, 0}
	g.settleIn(g.arrive(0))
	if why := order(t, g, 0, "Twin"); why != "" {
		t.Fatal(why)
	}
	view(g, s)
	if keys(g, s, ebiten.KeySpace, ebiten.KeySpace, ebiten.KeySpace); g.changeFor(0) == nil {
		t.Fatal("stray SPACEs called the change off")
	}
	g.termCursor = 0
	keys(g, s, ebiten.KeyD, ebiten.KeySpace)
	if r := g.ring(0); g.termDepth != 3 || r[0] != row(t, rowChange) || g.termPick == 0 {
		t.Fatalf("the ring opened on slot %d, depth %d; want it open past calling the change off", g.termPick, g.termDepth)
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

// 2026-11-15 (SOON rows, Mirror retrofit): Change type is not among a Room's actions while there is no other bed
// layout to change to; owning the in-Room Mirror no longer makes one (Change type lists bed types only).
func TestChangeRowHidden(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	has := func() bool { return slices.Contains(g.roomRows(0), row(t, rowChange)) }
	if has() {
		t.Fatal("only Single unlocked: Change type is offered")
	}
	if g.own(RoomMirror); has() {
		t.Fatal("the in-Room Mirror owned, only Single unlocked: Change type is offered")
	}
	if g.own(BedsTwin); !has() {
		t.Fatal("Twin beds unlocked: no Change type")
	}
}

// 2026-11-14: a Room not bought offers its key and nothing else; a bought one never offers a key.
func TestKeyRow(t *testing.T) {
	g := closedRun(t)
	if rows := g.roomRows(0); len(rows) != 1 || shop[rows[0]].ID != rowKey {
		t.Fatalf("a boarded Room's actions: %v, want the key alone", rows)
	}
	g.openRoom(0)
	g.own(BedsTwin) // something to do in it: Change type
	if slices.Contains(g.roomRows(0), row(t, rowKey)) || len(g.roomRows(0)) == 0 {
		t.Fatalf("a bought Room's actions: %v, want some, and no key", g.roomRows(0))
	}
}
