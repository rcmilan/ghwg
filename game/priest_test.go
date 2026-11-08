package game

import (
	"testing"
)

// T103 (D147): a haunt raises its Room's Spook by the Fear it adds; at spookLimit the Room is Closed and
// takes no Walk-ins, while one under it does.
func TestRoomSpook(t *testing.T) {
	g, _, p := settled(t)
	g.haunt(p.Room)
	g.scare()
	if g.roomSpook[p.Room] != len(p.Guests)*fearPerMinute || g.roomSpook[1-p.Room] != 0 {
		t.Fatalf("one haunt-minute: Room Spook %v, want %d on Room %d only", g.roomSpook, len(p.Guests)*fearPerMinute, p.Room)
	}
	g.parties = nil
	g.roomSpook[0] = spookLimit
	if !g.closed(0) || g.closed(1) {
		t.Fatalf("closed: %v %v, want Room 0 only", g.closed(0), g.closed(1))
	}
	if q := g.walkIn(); q == nil || q.Room != 1 {
		t.Fatal("the Walk-in didn't take the open Room")
	}
	g.parties = nil
	g.roomSpook[1] = spookLimit
	if q := g.walkIn(); q != nil {
		t.Fatalf("a Walk-in took Closed Room %d", q.Room)
	}
}

// T104 (D151): a Strong Priest is paid at once, walks in, works and clears the Spook, then walks out. One
// Priest per Room at a time; no money, no Priest.
func TestPriest(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.roomSpook[0], g.money = spookLimit, priestStrong+priestWeak
	if msg := g.callPriest(0, true); msg != "" || g.money != priestWeak {
		t.Fatalf("call: %q, money %d; want no refusal and money %d", msg, g.money, priestWeak)
	}
	if msg := g.callPriest(0, false); msg == "" || g.money != priestWeak {
		t.Fatalf("a second Priest for Room 0 was called (money %d)", g.money)
	}
	for i := 0; len(g.priests) > 0; i++ {
		if i > 60*tps+priestMinutes*minuteFrames {
			t.Fatalf("the Priest never left: %+v", *g.priests[0])
		}
		if g.roomSpook[0] == 0 && g.priests[0].until == 0 {
			t.Fatal("the Spook cleared before the Priest worked")
		}
		g.step(0, 0)
	}
	if g.roomSpook[0] != 0 || g.closed(0) {
		t.Fatalf("after a Strong Priest: Room Spook %d, want 0", g.roomSpook[0])
	}
	g.roomSpook[1], g.money = 10, priestWeak-1
	if msg := g.callPriest(1, false); msg == "" || g.money != priestWeak-1 || len(g.priests) > 0 {
		t.Fatal("a Priest was called without the money")
	}
}

// T104: a Weak Priest halves the Room's Spook.
func TestWeakPriest(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.roomSpook[1], g.money = 151, priestWeak
	g.callPriest(1, false)
	for i := 0; len(g.priests) > 0 && i < 60*tps+priestMinutes*minuteFrames; i++ {
		g.step(0, 0)
	}
	if g.roomSpook[1] != 75 {
		t.Fatalf("after a Weak Priest: Room Spook %d, want 75", g.roomSpook[1])
	}
}

// T107: haunt-priest calls Priests through the Terminal by keys alone, and the sim's invariants hold.
func TestHauntPriestSim(t *testing.T) {
	r := RunSim(Config{Cols: 2, Rows: 1, Days: 7, Seed: 2, Strategy: Strategies["haunt-priest"], React: 30, KeyGap: 6})
	for _, f := range r.Failures {
		t.Errorf("%s: %s", f.Clock, f.Reason)
	}
	if r.Priests == 0 || r.PriestSpend != r.Priests*priestWeak {
		t.Fatalf("haunt-priest: %d Priests for $%d; want some Weak ones", r.Priests, r.PriestSpend)
	}
}

// T106 (D162): a Room shifts by Spook ÷ spookLimit; Closed, it pulses in ghost (0.6..1) and holds at 1 in real.
func TestRoomShift(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.roomSpook[0], g.roomSpook[1] = spookLimit/2, spookLimit
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
