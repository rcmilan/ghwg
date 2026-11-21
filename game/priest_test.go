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
	g.mood[0], g.wallet[Ecto] = moodMin, 2*priestStrong
	if msg := g.callPriest(0); msg != "" || g.wallet[Ecto] != priestStrong {
		t.Fatalf("call: %q, Ecto %v; want no refusal and Ecto %d", msg, g.wallet[Ecto], priestStrong)
	}
	if msg := g.callPriest(0); msg == "" || g.wallet[Ecto] != priestStrong {
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
	g.wallet[Ecto] = priestStrong - 1
	if msg := g.callPriest(1); msg == "" || g.wallet[Ecto] != priestStrong-1 || len(g.priests) > 0 {
		t.Fatal("a Priest was called without the money")
	}
}

// T104: a Weak Priest halves the Room's dirt (Mood -100 → half the dirt left).
func TestWeakPriest(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.mood[1] = moodMin
	g.sendPriest(1, false)
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
		g.DevFather(true)
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
		if len(g.priests) != 1 || !g.priests[0].strong || g.priests[0].RoomIndex != a || g.afford(Cost{Ecto: priestStrong}) {
			t.Fatalf("%v: priests %v, Ecto %v; want Father M on Room 101 and M's call out of reach (red)", grid, g.priests, g.wallet[Ecto])
		}
		if act, _ := g.actionable(); act != actView || g.feet().Y < g.hotel.HallY {
			t.Fatalf("%v: SPACE does %v at %v; want the hotel view, in the hall", grid, act, g.feet())
		}
		has := func(id Unlock) bool { return slices.Contains(g.roomRows(b), row(t, id)) }
		if has(rowFatherM) {
			t.Fatalf("%v: Room 102's ring has Father M's call while he is in", grid)
		}
		g.priests = nil // M walked out
		if !has(rowFatherM) {
			t.Fatalf("%v: Room 102's ring has no Father M once he is gone", grid)
		}
	}
}

// 08 B1-P rituals: walk to the Room's middle, an opening line, a chant, a burst, a closing line, out. The effect
// lands with the burst, not before; both lines are always said.
func TestRitual(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.ghostDirt(0, 80)
	before := g.dirtLevel(0)
	g.sendPriest(0, false)
	pr := g.priests[0]
	var seen []phase
	var said []string
	for i := 0; len(g.priests) > 0; i++ {
		if i > 120*tps+priestMinutes*minuteFrames {
			t.Fatalf("the ritual never ended: phases %v", seen)
		}
		if len(seen) == 0 || seen[len(seen)-1] != pr.phase {
			seen = append(seen, pr.phase)
			said = append(said, pr.said.text)
		}
		if pr.phase != rClose && pr.phase != rLeave && g.dirtLevel(0) != before {
			t.Fatalf("the dirt changed in phase %d, before the burst landed", pr.phase)
		}
		if n := len(g.ritualLights()); (pr.phase == rChant || pr.phase == rBurst) != (n == 1) {
			t.Fatalf("phase %d has %d ritual lights", pr.phase, n)
		}
		if pr.phase == rChant && pr.lift() < 0 {
			t.Fatal("negative hop")
		}
		g.step(0, 0)
	}
	if want := []phase{rWalk, rOpen, rChant, rBurst, rClose, rLeave}; !slices.Equal(seen, want) {
		t.Fatalf("phases %v, want %v", seen, want)
	}
	if !slices.Contains(kOpen, said[1]) || !slices.Contains(kClose, said[4]) {
		t.Fatalf("lines %q, want K's opening then K's closing", said)
	}
	if g.dirtLevel(0) != before/2 {
		t.Fatalf("dirt %d, want half of %d", g.dirtLevel(0), before)
	}
}

// 08 B1-P: Father M's burst also sends every ghost in the Room over.
func TestFatherMSendsOver(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.unlock(Mirror)
	q := ghostOf(g, 0, 1)
	g.wallet[Ecto] = priestStrong
	if why := g.callPriest(0); why != "" {
		t.Fatal(why)
	}
	for i := 0; !g.priests[0].done; i++ {
		if i > 120*tps+priestMinutes*minuteFrames {
			t.Fatal("Father M never finished")
		}
		if q.Guests[0].PassAt != 0 {
			t.Fatal("the ghost passed on before the burst")
		}
		g.step(0, 0)
	}
	if q.Guests[0].PassAt == 0 {
		t.Fatal("Father M's burst didn't send the ghost over")
	}
}

// 08 B1-P retainer: Father K goes to the Room with the most spiritual dirt, one after another, paces the
// corridors with none, and heads for the next Room that gets some.
func TestRetainerRounds(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.wallet[Ecto] = 1000
	g.retainK()
	if !g.fatherIn(false) || g.noFather(0, false) == "" {
		t.Fatal("a retained Father K can be called")
	}
	until(t, g, "he paces with nothing to clean", func() bool { return g.priests[0].phase == rPace && g.priests[0].lap > 0 })
	g.ghostDirt(0, 30)
	g.ghostDirt(1, 80)
	pr := g.priests[0]
	until(t, g, "he heads for the worst Room", func() bool { return pr.phase == rWalk })
	if pr.RoomIndex != 1 {
		t.Fatalf("went to Room %d, want Room 1 with the most spiritual dirt", pr.RoomIndex)
	}
	for i := 0; g.spiritDirt(0) > 0 || g.spiritDirt(1) > 0; i++ {
		if i > 300*minuteFrames*5 {
			t.Fatalf("spiritual dirt left: %d %d", g.spiritDirt(0), g.spiritDirt(1))
		}
		g.step(0, 0)
	}
	until(t, g, "he paces again", func() bool { return pr.phase == rPace })
	if len(g.priests) != 1 || g.spiritDirt(0)+g.spiritDirt(1) != 0 {
		t.Fatal("he left or dirt remains")
	}
	g.ghostDirt(0, 40)
	until(t, g, "he heads for Room 0", func() bool { return pr.phase == rWalk && pr.RoomIndex == 0 })
}

// 08 B1-P retainer: the wage is Ecto a day; short on payday he leaves at once, saying so, and the STAFF row
// offers him again.
func TestRetainerPayday(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.unlock(unlockK)
	g.wallet[Ecto] = retainerEcto
	i := row(t, rowRetain)
	if why := g.addToCart(i, -1); why != "" {
		t.Fatal(why)
	}
	if why := g.addToCart(i, -1); why == "" {
		t.Fatal("two retainers in the cart")
	}
	if why := g.placeOrder(); why != "" || !g.fatherIn(false) || g.wallet[Ecto] != 0 {
		t.Fatalf("hired: %q, in %v, Ecto %v", why, g.fatherIn(false), g.wallet[Ecto])
	}
	if g.addToCart(i, -1) == "" || !g.gone(i) {
		t.Fatal("the retainer row offers a Father K who is already in")
	}
	pr := g.priests[0]
	for pr.phase != rLeave {
		if g.clock > 2*dayFrames {
			t.Fatal("never left")
		}
		g.step(0, 0)
	}
	if g.clock < dayFrames || !slices.Contains(fatherKUnpaid, pr.said.text) || pr.retained {
		t.Fatalf("left at %d saying %q", g.clock, pr.said.text)
	}
	until(t, g, "out the Exit", func() bool { return len(g.priests) == 0 })
	if g.gone(i) || !g.onSale(i) {
		t.Fatal("the row doesn't offer him again")
	}
	g.wallet[Ecto] = 3 * retainerEcto
	g.retainK()
	for g.clock < 3*dayFrames {
		g.step(0, 0)
	}
	if len(g.priests) != 1 || !g.priests[0].retained || g.wallet[Ecto] >= 3*retainerEcto {
		t.Fatal("a paid retainer didn't stay and pay by the day")
	}
}
