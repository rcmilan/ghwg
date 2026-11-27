package game

import (
	"image"
	"math"
	"math/rand/v2"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/sound"
)

// updateSteps (T403 STEPS) plays the footsteps: real only, on the walk cycle's foot-down snapped to the next slot,
// quieter the more walkers share the Concierge's area, never through walls, at most 4 at once. decisions.md
// "Effects" holds the rules. The decisions (stepGain, queueSteps, stepsDue) need no audio, so the tests run them
// headless; this only feeds them and plays what is due.
func (g *Game) updateSteps() {
	st := &g.snd.steps
	// Ghosts float: SHIFT cuts the steps, and the queue with them; the Street drowns them (live test 1); Father P's
	// cutscene moves everyone itself while its race spins the clock (a step every frame).
	if g.ghost || g.onStreet() || g.reel != nil {
		st.pending, st.last, st.now = st.pending[:0], nil, -1
		return
	}
	if now := g.now(); now != st.now { // the world only steps while the clock moves; stopped screens add nothing
		st.now = now
		st.walk = g.stepWalkers(st.walk[:0])
		g.queueSteps(st.walk)
	}
	for _, f := range g.stepsDue(g.clockBeat()) {
		if bank := g.snd.bank; bank != nil && len(bank.Steps) > 0 {
			// Package-level rand: the sim's seeded streams stay untouched, sound or not.
			rate := 0.94 + 0.12*rand.Float32()
			gain := float32(f.gain) * (0.85 + 0.15*rand.Float32())
			g.snd.mix.Play(bank.Steps[rand.IntN(len(bank.Steps))], sound.SFX, rate, gain)
		}
	}
}

// uiStep: a footstep as the Terminal's click, on a tab change and a confirmation (live test 3).
func (g *Game) uiStep() {
	if bank := g.snd.bank; g.snd.mix != nil && bank != nil && len(bank.Steps) > 0 {
		g.snd.mix.Play(bank.Steps[rand.IntN(len(bank.Steps))], sound.SFX, 0.94+0.12*rand.Float32(), uiStepGain)
	}
}

// Knobs (placeholders until a live listen).
const (
	uiStepGain   = 0.6 // the Terminal's click
	crowdK       = 0.8 // each other walker in the area divides a step's gain by another 0.8 of itself (live test 1: 0.35 was loud)
	dayGain      = 0.6 // by day the hotel is louder around the Concierge, so steps sit lower
	stepsPerCap  = 4   // most steps sounding in one slot
	slotsPerBeat = 5   // the steps' grid (real only): 5 a second at ×1, the eighths of the real music's 150 BPM
)

// footDown: the run strip's frames where a foot lands (two per 6-frame cycle). Frames 0 and 3 are the full
// stride with the body low; 1 and 4 are the lifted ones, 2 and 5 the passing ones.
func footDown(frame int) bool { return frame == 0 || frame == 3 }

// stepGain: 1.0 alone at night, lower by day and with each other walker in the area.
func stepGain(night bool, others int) float64 {
	dn := dayGain
	if night {
		dn = 1
	}
	return dn / (1 + crowdK*float64(max(others, 0)))
}

// walker: one character as the steps see it. frame is the strip frame, 0..5.
type walker struct {
	key    *entities.Anim
	x, y   float64
	moving bool
	frame  int
	area   int
}

// footfall: a step waiting for its slot, or due.
type footfall struct {
	who  *entities.Anim
	gain float64
	dist float64 // from the Concierge, for the nearest-first cap
}

type stepState struct {
	last    map[*entities.Anim]int // each walker's strip frame last frame, -1 when standing
	pending []footfall
	walk    []walker
	slot    int     // the slot the beat was in
	now     float64 // g.now() at the last pass
	due     []footfall
}

// stepWalkers: the Concierge first, then every real-realm actor (ghost guests float).
func (g *Game) stepWalkers(buf []walker) []walker {
	add := func(a *entities.Anim, x, y float64) {
		buf = append(buf, walker{key: a, x: x, y: y, moving: a.Moving, frame: a.Frame() % entities.FramesPerFacing})
	}
	add(&g.player.Anim, g.player.X, g.player.Y)
	for _, a := range g.actors() { // ponytail: actors() allocates its slice; only on frames the world moved
		if g.realmOf(a) == Human {
			add(&a.Anim, a.X, a.Y)
		}
	}
	return buf
}

// queueSteps: ws[0] is the Concierge. A walker whose strip just reached a foot-down frame queues a step if the
// Concierge hears it: same area (areaRun: a Room, a corridor, the hall, ... named by the wall above), no wall
// between. The crowd counts every walker in the area, heard or not.
func (g *Game) queueSteps(ws []walker) {
	st := &g.snd.steps
	if st.last == nil || len(st.last) > 64 { // actors come and go; start over rather than track them
		st.last = make(map[*entities.Anim]int)
	}
	night, walking := g.nightNow(), 0
	for i := range ws {
		if i == 0 || ws[i].moving { // the Concierge's area counts standing still too: he hears others walk by
			ws[i].area = g.areaRun(ws[i].x, ws[i].y)
		}
	}
	for _, w := range ws {
		if w.moving && w.area == ws[0].area {
			walking++
		}
	}
	you := image.Pt(int(ws[0].x), int(ws[0].y))
	for i, w := range ws {
		prev, seen := st.last[w.key]
		if !seen {
			prev = -1
		}
		if !w.moving {
			if seen && prev >= 0 { // just stopped: a step still waiting for its slot would land after the stop (live test 1)
				st.pending = slices.DeleteFunc(st.pending, func(f footfall) bool { return f.who == w.key })
			}
			if seen {
				st.last[w.key] = -1
			}
			continue
		}
		st.last[w.key] = w.frame
		if !footDown(w.frame) || prev == w.frame || w.area != ws[0].area {
			continue
		}
		if i > 0 && !g.sight(image.Pt(int(w.x), int(w.y)), you) {
			continue
		}
		st.pending = append(st.pending, footfall{w.key, stepGain(night, walking-1), math.Hypot(w.x-ws[0].x, w.y-ws[0].y)})
	}
}

// stepsDue: when the beat enters a new slot, the pending steps that sound now: the nearest stepsPerCap, the
// rest dropped. The slice is reused; read it before the next call.
func (g *Game) stepsDue(beat float64) []footfall {
	st := &g.snd.steps
	slot := int(math.Floor(beat * slotsPerBeat))
	if slot == st.slot {
		return nil
	}
	st.slot = slot
	slices.SortFunc(st.pending, func(a, b footfall) int {
		if a.dist < b.dist {
			return -1
		} else if a.dist > b.dist {
			return 1
		}
		return 0
	})
	st.due = append(st.due[:0], st.pending[:min(len(st.pending), stepsPerCap)]...)
	st.pending = st.pending[:0]
	return st.due
}
