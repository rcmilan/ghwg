package game

import (
	"testing"

	"ghosthotel/entities"
)

// stepRun: a started Run with the Concierge standing in the hall, day.
func stepRun(t *testing.T) *Game {
	g := mustNew(t)
	g.startRun()
	return g
}

// footWalker: a moving walker at (x,y) on a foot-down frame.
func footWalker(x, y float64) walker {
	return walker{key: new(entities.Anim), x: x, y: y, moving: true, frame: 0}
}

func (g *Game) stepSelf() walker { return footWalker(g.player.X, g.player.Y) }

func TestStepGain(t *testing.T) {
	if got := stepGain(true, 0); got != 1 {
		t.Errorf("alone at night = %v, want 1", got)
	}
	if stepGain(false, 0) >= stepGain(true, 0) {
		t.Error("day should be quieter than night")
	}
	for _, night := range []bool{true, false} {
		for n := range 10 {
			if g := stepGain(night, n); g > 1 || g <= 0 || n > 0 && g >= stepGain(night, n-1) {
				t.Errorf("stepGain(%v,%d) = %v: want in (0,1] and falling", night, n, g)
			}
		}
	}
}

func TestStepsHeardOnlyInOwnArea(t *testing.T) {
	g := stepRun(t)
	p := g.stepSelf()
	near := footWalker(p.x+16, p.y)
	room := g.hotel.Rooms[0].Shell()
	far := footWalker(float64(room.Min.X+room.Dx()/2), float64(room.Min.Y+room.Dy()/2)) // another area, behind walls
	g.queueSteps([]walker{p, far})
	if got := g.stepsDue(1); len(got) != 1 || got[0].who != p.key {
		t.Fatalf("a walker in another area was heard: %+v", got)
	}
	g.queueSteps([]walker{g.stepSelf(), near})
	if got := g.stepsDue(2); len(got) != 2 {
		t.Fatalf("a walker in the same area: %d steps, want 2 (Concierge and walker)", len(got))
	}
}

func TestStepsNoneInGhost(t *testing.T) {
	g := stepRun(t)
	g.ghost = true
	g.snd.steps.pending = append(g.snd.steps.pending, footfall{gain: 1})
	g.updateSteps()
	if len(g.snd.steps.pending) != 0 || len(g.stepsDue(5)) != 0 {
		t.Error("steps survived the SHIFT to ghost")
	}
	// the real walkers list is empty of ghost guests: ghost-realm parties never step
	for _, w := range g.stepWalkers(nil)[1:] {
		for _, p := range g.parties {
			if p.Realm == Ghost {
				for _, gu := range p.Guests {
					if &gu.Anim == w.key {
						t.Error("a ghost guest steps")
					}
				}
			}
		}
	}
}

func TestStepsSnapToTheSlot(t *testing.T) {
	g := stepRun(t)
	g.stepsDue(3.05) // the beat is in slot 15 (5 a beat)
	g.queueSteps([]walker{g.stepSelf()})
	if g.stepsDue(3.15) != nil {
		t.Fatal("a step fired inside its slot")
	}
	if got := g.stepsDue(3.2); len(got) != 1 {
		t.Fatalf("crossing the next slot fired %d steps, want 1", len(got))
	}
	if g.stepsDue(3.3) != nil {
		t.Error("a step fired twice")
	}
}

func TestStepsCap(t *testing.T) {
	g := stepRun(t)
	p := g.stepSelf()
	ws := []walker{p}
	for i := 1; i <= 5; i++ { // the Concierge and 5 others, the nearest 3 and the Concierge should win
		ws = append(ws, footWalker(p.x+float64(i*3), p.y))
	}
	g.queueSteps(ws)
	got := g.stepsDue(1)
	if len(got) != stepsPerCap {
		t.Fatalf("%d steps due, want %d", len(got), stepsPerCap)
	}
	for i, f := range got {
		if f.who != ws[i].key {
			t.Errorf("step %d is not the %d-th nearest", i, i)
		}
	}
}

func TestFootDownOncePerStride(t *testing.T) {
	g := stepRun(t)
	a := g.stepSelf()
	n := 0
	for f := 0; f < 12; f++ { // two cycles, each frame held 3 passes
		for range 3 {
			a.frame = f % 6
			g.queueSteps([]walker{a})
		}
	}
	n = len(g.snd.steps.pending)
	if n != 4 {
		t.Errorf("%d steps in two cycles, want 4", n)
	}
}

// Standing still, the Concierge still hears someone walk by in his area (review: his area was only read when
// he moved, so a standing Concierge heard nobody).
func TestStepsHeardStandingStill(t *testing.T) {
	g := stepRun(t)
	you := g.stepSelf()
	you.moving = false
	g.queueSteps([]walker{you, footWalker(you.x+16, you.y)})
	if got := g.stepsDue(1); len(got) != 1 || got[0].gain != stepGain(g.nightNow(), 0) {
		t.Fatalf("a walker beside a standing Concierge: %+v, want one step, alone in the crowd", got)
	}
}

// Live test 1: a step queued on the last foot-down sounded after the walk keys were let go.
func TestStepsStopWithTheWalker(t *testing.T) {
	g := stepRun(t)
	you := g.stepSelf()
	g.queueSteps([]walker{you})
	you.moving = false
	g.queueSteps([]walker{you})
	if got := g.stepsDue(1); len(got) != 0 {
		t.Fatalf("%d steps after the stop, want none", len(got))
	}
}

// Live test 1: on the Street the steps go silent, the queue with them; Father P's cutscene too.
func TestStepsNoneOnTheStreet(t *testing.T) {
	g := stepRun(t)
	g.player.Y = float64(g.hotel.Spawn.Y)
	g.snd.steps.pending = append(g.snd.steps.pending, footfall{gain: 1})
	g.updateSteps()
	if len(g.snd.steps.pending) != 0 {
		t.Error("steps queued on the Street")
	}
	g.player.Y = 0 // and none in Father P's cutscene
	g.reel = &prestigeReel{}
	g.snd.steps.pending = append(g.snd.steps.pending, footfall{gain: 1})
	g.updateSteps()
	if len(g.snd.steps.pending) != 0 {
		t.Error("steps queued in Father P's cutscene")
	}
}

// The Terminal clicks with a footstep on a tab change.
func TestTerminalClicks(t *testing.T) {
	g, _, _ := settled(t)
	openTestMixer(t, g)
	g.uiStep()
	if g.snd.mix.Playing() != 1 {
		t.Error("no click")
	}
}
