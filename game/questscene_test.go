package game

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func sceneGame(t *testing.T) (*Game, *script, *[]string) {
	t.Helper()
	g, s := headless(t, 2, 1, 1)
	g.startRun()
	var ends []string
	old := sceneDone
	sceneDone = func(_ *Game, id string) { ends = append(ends, id) }
	t.Cleanup(func() { sceneDone = old })
	return g, s, &ends
}

// run: Update until the scene is over, at most max frames; the frames it took.
func runScene(t *testing.T, g *Game, s *script, max int) int {
	t.Helper()
	for i := 0; ; i++ {
		if !g.scene.on() {
			return i
		}
		if i > max {
			t.Fatalf("scene %q never ended", g.scene.id)
		}
		s.hold()
		g.Update()
	}
}

// Each cutscene ends by itself and reports itself once.
func TestSceneEndsReportEvScene(t *testing.T) {
	for _, id := range []string{"O4", "O9"} {
		g, s, ends := sceneGame(t)
		g.playQuestScene(id)
		runScene(t, g, s, 2000)
		if len(*ends) != 1 || (*ends)[0] != id {
			t.Errorf("%s: ended %v, want [%s]", id, *ends, id)
		}
	}
}

// SPACE skips a cutscene only once its beat has been done before.
func TestSceneSkipOnReplayOnly(t *testing.T) {
	for _, seen := range []bool{false, true} {
		g, s, _ := sceneGame(t)
		if seen {
			g.questRec.Times[2] = 1
		}
		g.playQuestScene("O4")
		g.Update()
		s.hold(ebiten.KeySpace)
		g.Update()
		if got := !g.scene.on(); got != seen {
			t.Errorf("seen %v: SPACE skipped = %v", seen, got)
		}
	}
}

// The finale's first time: the MacGuffin is summoned and one Bonus step added, then the credits and CONTINUE, then it
// checks in. A second finale adds nothing.
func TestFinaleSummonsOnce(t *testing.T) {
	g, s, ends := sceneGame(t)
	g.grid0 = [2]int{1, 1} // grown
	bonus := g.bonus
	g.playQuestScene("finale")
	for i := 0; g.scene.id == "finale"; i++ {
		if i > 3*fLen {
			t.Fatal("the finale never ended")
		}
		s.hold()
		g.Update()
	}
	if g.scene.id != "credits" || !g.questRec.MacGuffin || g.bonus != bonus+macguffinBonus {
		t.Fatalf("after the finale: scene %q, MacGuffin %v, bonus %v -> %v", g.scene.id, g.questRec.MacGuffin, bonus, g.bonus)
	}
	if len(*ends) != 0 || g.macguffinParty() != nil {
		t.Fatal("the credits come before CONTINUE: no report, no guest yet")
	}
	g.grid0 = [2]int{} // the prestige's reset rebuilds a grown hotel; this one has no tileset to rebuild with
	g.Update()
	s.hold(ebiten.KeySpace) // the credits' page
	g.Update()
	for i := 0; g.scene.on() && i < 2*creditsHold; i++ {
		s.hold(ebiten.KeySpace) // CONTINUE only reads SPACE after its hold
		g.Update()
		s.hold()
		g.Update()
	}
	if g.scene.on() || len(*ends) != 1 || (*ends)[0] != "finale" || g.level != 1 || g.quest.Beat != 0 {
		t.Fatalf("CONTINUE: scene %v, reports %v, level %d, beat %d: want a prestige", g.scene.on(), *ends, g.level, g.quest.Beat)
	}
	if !g.questRec.MacGuffin || g.bonus < bonus+macguffinBonus {
		t.Fatalf("the prestige lost the MacGuffin record or its bonus: %v %v", g.questRec.MacGuffin, g.bonus)
	}
	g.playQuestScene("finale") // a replay: no credits, a prestige all the same, no second MacGuffin bonus
	want := g.bonus + g.prestigeGain()
	runScene(t, g, s, 3*fLen)
	if g.level != 2 || g.bonus != want {
		t.Fatalf("a second finale: level %d, bonus %v, want 2 and %v", g.level, g.bonus, want)
	}
}

// Father P's confirm card: nothing, then the first line once beat 3 has been done, then the second after the finale.
func TestFatherPQuestLine(t *testing.T) {
	g, _, _ := sceneGame(t)
	if g.fatherPLine() != "" {
		t.Fatal("a line before beat 3")
	}
	g.questRec.Times[2] = 1
	if g.fatherPLine() != fatherPQuest[0] {
		t.Fatalf("after beat 3: %q", g.fatherPLine())
	}
	g.questRec.MacGuffin = true
	if g.fatherPLine() != fatherPQuest[1] {
		t.Fatalf("after the finale: %q", g.fatherPLine())
	}
}
