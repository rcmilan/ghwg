package game

import (
	"slices"
	"testing"
)

// T103 (D147), T248: a fright lowers its Room's Mood by the Fear it gives; at Mood -100 the Room is Closed and
// takes no Walk-ins, while one above it does.
func TestRoomMood(t *testing.T) {
	g, _, p := settled(t)
	g.frighten(p, p.Guests[0], 1, fearLimit)
	if g.roomMood(p.Room) != moodStart-dirtMood(1) || g.roomMood(1-p.Room) != moodStart {
		t.Fatalf("a fright: Moods %v, want %d in Room %d only", g.mood, moodStart-dirtMood(1), p.Room)
	}
	g.parties = nil
	g.mood[0] = moodMin
	if !g.closed(0) || g.closed(1) {
		t.Fatalf("closed: %v %v, want Room 0 only", g.closed(0), g.closed(1))
	}
	if q := g.walkIn(); q == nil || q.Room != 1 {
		t.Fatal("the Walk-in didn't take the open Room")
	}
	g.parties = nil
	g.mood[1] = moodMin
	if q := g.walkIn(); q != nil {
		t.Fatalf("a Walk-in took Closed Room %d", q.Room)
	}
}

// T104 (D151): a Strong Priest is paid at once, walks in, works and clears the dirt, then walks out. One
// Priest per Room at a time; no money, no Priest.
func TestPriest(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.mood[0], g.wallet[Money] = moodMin, priestStrong+priestWeak
	if msg := g.callPriest(0, true); msg != "" || g.wallet[Money] != priestWeak {
		t.Fatalf("call: %q, money %v; want no refusal and money %d", msg, g.wallet[Money], priestWeak)
	}
	if msg := g.callPriest(0, false); msg == "" || g.wallet[Money] != priestWeak {
		t.Fatalf("a second Priest for Room 0 was called (money %v)", g.wallet[Money])
	}
	for i := 0; len(g.priests) > 0; i++ {
		if i > 60*tps+priestMinutes*minuteFrames {
			t.Fatalf("the Priest never left: %+v", *g.priests[0])
		}
		if g.roomMood(0) != moodMin && g.priests[0].until == 0 {
			t.Fatal("the dirt cleared before the Priest worked")
		}
		g.step(0, 0)
	}
	if g.roomMood(0) != moodClean || g.closed(0) {
		t.Fatalf("after a Strong Priest: Room Mood %d, want %d", g.roomMood(0), moodClean)
	}
	g.setDirtLevel(1, 10)
	g.wallet[Money] = priestWeak - 1
	if msg := g.callPriest(1, false); msg == "" || g.wallet[Money] != priestWeak-1 || len(g.priests) > 0 {
		t.Fatal("a Priest was called without the money")
	}
}

// T104: a Weak Priest halves the Room's dirt (Mood -100 → half the dirt left).
func TestWeakPriest(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.mood[1], g.wallet[Money] = moodMin, priestWeak
	g.callPriest(1, false)
	for i := 0; len(g.priests) > 0 && i < 60*tps+priestMinutes*minuteFrames; i++ {
		g.step(0, 0)
	}
	if want := moodOf(dirtMax / 2); g.roomMood(1) != want {
		t.Fatalf("after a Weak Priest: Room Mood %d, want %d", g.roomMood(1), want)
	}
}

// T106 (D162): a Room shifts by dirt ÷ dirtMax; Closed, it pulses in ghost (0.6..1) and holds at 1 in real.
func TestRoomShift(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.mood[0], g.mood[1] = 0, moodMin
	if g.roomShift(0) != 0.5 || g.roomShift(1) != 1 || g.roomShift(-1) != 0 {
		t.Fatalf("real: shifts %v %v, want 0.5 1", g.roomShift(0), g.roomShift(1))
	}
	g.ghost = true
	lo, hi := float32(1), float32(0)
	for range 2 * tps {
		g.tick++
		lo, hi = min(lo, g.roomShift(1)), max(hi, g.roomShift(1))
	}
	if lo > 0.65 || hi < 0.99 || g.roomShift(0) != 0.5 {
		t.Fatalf("ghost: Closed Room pulses %.2f..%.2f, want 0.6..1", lo, hi)
	}
}

// 08 F0: main's -father: Rooms 101 and 102 open with guests in, Money for both calls, Father K walking to Room
// 101, the Concierge in the hall where SPACE opens the hotel view. Room 102's ring offers M, not K, until K is gone.
func TestDevFather(t *testing.T) {
	for _, grid := range [][2]int{{2, 1}, {3, 3}} {
		g, _ := headless(t, grid[0], grid[1], 1)
		g.openAtStart = false
		g.startRun()
		g.DevFather(false)
		at := map[string]int{}
		for i, r := range g.hotel.Rooms {
			at[r.Number] = i
		}
		a, b := at["101"], at["102"]
		in := func(room int) bool {
			return slices.ContainsFunc(g.parties, func(p *Party) bool { return p.Room == room && p.settled })
		}
		if !g.isOwned(a) || !g.isOwned(b) || !in(a) || !in(b) {
			t.Fatalf("%v: Rooms 101 and 102 owned %v %v, guests in %v %v; want both open with guests in", grid, g.isOwned(a), g.isOwned(b), in(a), in(b))
		}
		if len(g.priests) != 1 || g.priests[0].strong || g.priests[0].RoomIndex != a || g.wallet[Money] < priestStrong {
			t.Fatalf("%v: priests %v, money %v; want Father K on Room 101 and $%d left", grid, g.priests, g.wallet[Money], priestStrong)
		}
		if act, _ := g.actionable(); act != actView || g.feet().Y < g.hotel.HallY {
			t.Fatalf("%v: SPACE does %v at %v; want the hotel view, in the hall", grid, act, g.feet())
		}
		has := func(id Unlock) bool { return slices.Contains(g.roomRows(b), row(t, id)) }
		if has(rowFatherK) || !has(rowFatherM) {
			t.Fatalf("%v: Room 102's ring: K %v, M %v; want M only", grid, has(rowFatherK), has(rowFatherM))
		}
		g.priests = nil // K walked out
		if !has(rowFatherK) {
			t.Fatalf("%v: Room 102's ring has no Father K once he is gone", grid)
		}
	}
}
