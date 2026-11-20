package game

import (
	"slices"
	"testing"
)

// 08 F0 (One of each Father): each Father is one person. While he is anywhere in the hotel, walking in, working
// or walking out, he can't be called to another Room; the other Father can. Once he's gone, he can again.
func TestOneOfEachFather(t *testing.T) {
	g, _ := headless(t, 3, 1, 1)
	g.startRun()
	g.unlock(Mirror)
	g.unlock(unlockK)
	g.unlock(unlockM)
	g.earned, g.wallet = [2]Num{10000, 10000}, [2]Num{10000, 10000}
	k, m := row(t, rowFatherK), row(t, rowFatherM)
	if why := g.buyNow(k, 0); why != "" {
		t.Fatalf("Father K to Room 0: %q", why)
	}
	if g.buyNow(k, 1) == "" || g.callPriest(2, false) == "" || len(g.priests) != 1 {
		t.Fatalf("Father K called again while in the hotel: %d Priests", len(g.priests))
	}
	if why := g.buyNow(m, 1); why != "" {
		t.Fatalf("Father M refused while only Father K is in: %q", why)
	}
	for i := 0; !g.priests[0].done; i++ {
		if i > 60*tps+priestMinutes*minuteFrames {
			t.Fatal("Father K never finished")
		}
		g.step(0, 0)
	}
	if g.buyNow(k, 2) == "" {
		t.Error("Father K called while walking out")
	}
	for i := 0; slices.ContainsFunc(g.priests, func(pr *priest) bool { return !pr.strong }); i++ {
		if i > 60*tps {
			t.Fatal("Father K never left")
		}
		g.step(0, 0)
	}
	if why := g.buyNow(k, 2); why != "" {
		t.Errorf("Father K gone, still refused: %q", why)
	}
}

// 08 F0 (Housekeeper's errand): a housekeeper carrying a Supply turns back the moment its ask is gone, before it
// reaches the door, and the Supply goes back to the stock.
func TestKeeperTurnsBack(t *testing.T) {
	g, _, p := settled(t)
	g.hire()
	k := g.keepers[0]
	g.stock = map[Unlock]int{SupplyWater: 1}
	p.asked, p.want = g.clock, SupplyWater
	until(t, g, "it takes the Water", func() bool { return k.job == serving })
	door := g.doorDrop(p.Room)
	p.asked = 0
	g.step(0, 0)
	if k.job != resting || g.stock[SupplyWater] != 1 {
		t.Fatalf("the ask gone: job %d, stock %d; want resting at once, the Water back", k.job, g.stock[SupplyWater])
	}
	if len(k.Path) > 0 && k.Path[len(k.Path)-1] == door {
		t.Error("it still walks to the Room's door")
	}
}

// 08 F0 (Ask window): an ask waits 45 s of real time at scale 1 before it expires.
func TestAskWindow(t *testing.T) {
	if askMinutes*minuteFrames != 45*tps {
		t.Fatalf("an ask waits %d frames, want %d (45 s)", askMinutes*minuteFrames, 45*tps)
	}
}

// 08 F0 (Sleeping guests): no MOVE while the Party sleeps; it comes back at wake-up. TALK is at the plate beside the
// door (T381 refine 2): a stain in the Room no longer hides it, SPACE inside cleans it.
func TestTalkRules(t *testing.T) {
	g, _ := matchRun(t, 2)
	p := settledHuman(t, g, 0, 2)
	ghostOf(g, 0, 9)
	standIn(g, 0)
	if g.talker() != p || !g.canMove(p) {
		t.Fatal("no TALK by day")
	}
	p.abed = true
	if g.canMove(p) {
		t.Error("MOVE while the Party sleeps")
	}
	p.abed = false
	g.addMood(0, -refuseMood) // a refusal's Mood: its stains
	if g.talker() != p {
		t.Fatal("a stain in the Room hides TALK at its door")
	}
}

// 08 F0 (Passing on): only a haunt with a human in the Room counts toward a ghost passing on; a lonely ghost lingers.
func TestLonelyGhostLingers(t *testing.T) {
	g, _ := matchRun(t, 2)
	ghost := ghostOf(g, 0, 1).Guests[0]
	g.haunt(0, ghost)
	if ghost.Haunted != 0 || spent(ghost) {
		t.Fatalf("a haunt with nobody in: %d haunts, spent %v; want it to count for nothing", ghost.Haunted, spent(ghost))
	}
	settledHuman(t, g, 0, 12)
	g.haunt(0, ghost)
	if !spent(ghost) {
		t.Fatalf("a haunt with a human in: %d haunts, want spent", ghost.Haunted)
	}
}

// 08 F0 (Nonsense lines): every line group has a nonsense line, picked more rarely than the others.
func TestNonsenseLines(t *testing.T) {
	groups := [][]string{helloLines, soloOut, soloBack, ghostHellos, ghostByes, ghostBacks, ghostCalm,
		talkCalm, talkUneasy, talkScared, talkNo, talkYes, keeperChats, keeperWelcome, keeperMoved,
		courierIn, courierOut, kOpen, kClose, mOpen, mClose, fatherKUnpaid}
	for _, tier := range calmLines {
		groups = append(groups, tier[0], tier[1])
	}
	groups = append(groups, sightLines[:]...)
	groups = append(groups, hallSights[:]...)
	for _, pairs := range [][][2]string{outLines, backLines} {
		var firsts []string
		for _, x := range pairs {
			firsts = append(firsts, x[0])
		}
		groups = append(groups, firsts)
	}
	for _, set := range groups {
		if !slices.ContainsFunc(set, func(l string) bool { return nonsense[l] }) {
			t.Errorf("no nonsense line in %q", set)
		}
	}
	g := mustNew(t)
	odd, n := 0, 20000
	for range n {
		if nonsense[g.pickLine(talkCalm)] {
			odd++
		}
	}
	if fair := n / len(talkCalm); odd == 0 || odd >= fair/2 {
		t.Errorf("the nonsense line came up %d times in %d, want some, under half of a fair share %d", odd, n, fair)
	}
}
