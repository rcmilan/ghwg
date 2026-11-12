package game

import (
	"testing"

	"ghosthotel/entities"
)

// T265: the pay factor is 0 at Mood -100, 1 at 0 and 2 at +100, linear; the hourly Money and the checkout
// tip follow it.
func TestMoodPay(t *testing.T) {
	g, _, p := settled(t)
	for mood, want := range map[int]Num{moodMin: 0, 0: 1, moodMax: 2, 50: 1.5} {
		if g.mood[p.Room] = mood; g.moodPay(p.Room) != want {
			t.Errorf("Mood %d: pay factor %v, want %v", mood, g.moodPay(p.Room), want)
		}
	}
	g.mood[p.Room] = 0
	base := p.Payout(g.moodPay(p.Room))
	hour := func() Num {
		g.clock, p.owed = (g.clock/hourFrames+1)*hourFrames+1, 0 // off the hour: a frame's rent, owed
		g.charge(p)
		return p.owed
	}
	one := hour()
	if one <= 0 || base != p.Rate*p.Nights*tipPct/100 {
		t.Fatalf("at Mood 0: an hour pays %v, tip %d; want a plain Room's pay", one, base)
	}
	for mood, f := range map[int]Num{moodMin: 0, moodMax: 2} {
		g.mood[p.Room] = mood
		if got := hour(); got != one*f {
			t.Errorf("Mood %d: an hour pays %v, want %v", mood, got, one*f)
		}
		if got, want := p.Payout(g.moodPay(p.Room)), int(Num(p.Rate*p.Nights*tipPct)*f/100); got != want {
			t.Errorf("Mood %d: tip %d, want %d", mood, got, want)
		}
	}
	// on the hour the whole dollars owed are paid, as one "+$N" at the door: the counter and the floater agree
	g.mood[p.Room] = 0
	g.clock, p.owed, p.rent = (g.clock/hourFrames+1)*hourFrames, 2.75, 0
	money, floats := g.wallet[Money], len(g.floaters)
	g.charge(p)
	if g.wallet[Money] != money+2 || p.rent != 2 || p.owed >= 1 || len(g.floaters) != floats+1 || g.floaters[floats].text != "+$2" {
		t.Errorf("on the hour: wallet +%v, rent %v, owed %v, %d floaters; want $2 paid and floated, the cents carried", g.wallet[Money]-money, p.rent, p.owed, len(g.floaters)-floats)
	}
	g.clock++
	if g.charge(p); g.wallet[Money] != money+2 {
		t.Error("rent reached the wallet between the hours")
	}
	// T held: the Clock jumps over the hour's own frame, and the hour still pays
	g.fast, p.owed = true, 3
	g.clock = (g.clock/hourFrames+1)*hourFrames + 3
	if g.charge(p); g.wallet[Money] < money+5 {
		t.Errorf("fast Clock past the hour: wallet +%v, want the hour paid", g.wallet[Money]-money)
	}
	g.fast = false
}

// T266: spots show only below moodSpots, one per ~3.75 points (spotUnit dirt of 3/4 point each), up to the
// Closed Room at moodMin.
func TestMoodSpots(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	for mood, want := range map[int]int{moodMax: 0, moodSpots: 0, moodSpots - 1: 1, moodSpots - 4: 1, moodSpots - 8: 2, moodMin: 9} {
		g.mood[0], g.dirt, g.dirtN = mood, nil, nil
		if n := len(g.spots(0)); n != want {
			t.Errorf("Mood %d: %d spots, want %d", mood, n, want)
		}
	}
	if g.mood[0] = moodMin; !g.closed(0) {
		t.Fatal("a Room at moodMin must be Closed")
	}
}

// T267: g.haunts lowers Mood by the damage; a ghost's damage grows with its Phantasm; its Haunt (x) is the
// gap between haunts, so a stub ghost's Room loses the damage every gap and nothing between.
func TestHaunts(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.mood[0] = 0
	g.haunts(0, 7)
	if g.roomMood(0) != -7 || g.roomMood(1) != moodStart {
		t.Fatalf("a haunt of 7: Moods %v", g.mood)
	}
	ghost := entities.NewGuest(g.hotel.Spawn, guestCast[0])
	if ghost.Haunt != entities.DefaultHaunt || ghostHauntDamage(&ghost) != hauntBase {
		t.Fatalf("a fresh ghost: Haunt %d, damage %d", ghost.Haunt, ghostHauntDamage(&ghost))
	}
	ghost.Phantasm, ghost.Haunt = 3, 10
	damage := hauntBase + 3*hauntPerPhantasm
	if ghostHauntDamage(&ghost) != damage {
		t.Fatalf("damage %d, want %d", ghostHauntDamage(&ghost), damage)
	}
	g.mood[1], g.clock = 0, 0
	for g.clock < 3*10*minuteFrames+minuteFrames/2 {
		g.clock++
		if g.hauntTick(1, &ghost); g.clock == 10*minuteFrames-1 && g.roomMood(1) != 0 {
			t.Fatalf("haunted before the gap: Mood %d", g.roomMood(1))
		}
	}
	if g.roomMood(1) != -3*damage {
		t.Fatalf("3 gaps: Mood %d, want %d", g.roomMood(1), -3*damage)
	}
}
