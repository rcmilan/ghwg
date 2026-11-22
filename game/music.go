package game

import (
	"math"

	"ghosthotel/sound"
	"ghosthotel/world"
)

// updateMusic (T406 MIX) drives the Sequencer each frame: where the music is (the clock, strictly; ×1 on the
// stopped screens), which voices are heard and how loud (the realm, the guest-speech rule), the Concierge's
// treatment, the finale's coda and Teo's alert. decisions.md "Music" holds the rules. musicPlan decides, with
// no audio needed; updateMusic only applies it.

// musicMode: how the music's position moves this frame.
type musicMode int

const (
	musicFollow musicMode = iota // the clock's beat, strictly (the world runs, at whatever scale)
	musicFree                    // a stopped screen: ×1, whatever the clock does
	musicFrozen                  // the pause: nothing advances
	musicSilent                  // the Terminal: nothing advances, nothing sounds (live test 3: it resumed oddly)
)

// musicPlan: the mode, each voice's gain (the Concierge steady at 1), the Concierge's treatment (ghost reverb)
// and the tape speed (1 but in Father P's race).
type musicPlan struct {
	mode  musicMode
	gain  [sound.Voices]float32
	ghost bool
	speed float64
}

// voiceTrim: each voice's level under its plan gain. The high ones sit lower (live test 3: Madame's celesta and
// Osvaldo's whistle were hard on the ears up close).
var voiceTrim = [sound.Voices]float32{sound.Concierge: 1, sound.Magenta: 1, sound.Magda: 1, sound.Celestine: .6, sound.Tim: 1, sound.Osvaldo: .6}

// RealTempo: the living realm plays the score this many times faster than the ghost (live test 3): cool jazz
// at 150 BPM in real (120 was still gloomy), a 60 BPM ballad in ghost.
const RealTempo = 2.5

// tempo: the score's beats per clock beat in the realm on screen.
func (g *Game) tempo() float64 {
	if g.ghost {
		return 1
	}
	return RealTempo
}

// tapeTop: the tape speed at the end of Father P's race (2 octaves up, 4 times the tempo).
const tapeTop = 4

// castVoice: a quest character's line, by cast Key. Teo and the Cat have none.
func castVoice(key string) (sound.Voice, bool) {
	switch key {
	case "magenta":
		return sound.Magenta, true
	case "magda":
		return sound.Magda, true
	case "celestine":
		return sound.Celestine, true
	case "tim":
		return sound.Tim, true
	case "osvaldo":
		return sound.Osvaldo, true
	}
	return 0, false
}

func (p *musicPlan) all() {
	for v := range p.gain {
		p.gain[v] = 1
	}
}

// musicPlan: mirrors Update's order of what is on screen.
func (g *Game) musicPlan() (p musicPlan) {
	p.ghost, p.speed = g.ghost, 1
	p.gain[sound.Concierge] = 1
	s := &g.scene
	switch {
	case g.state == StatePause:
		p.mode = musicFrozen
	case g.state == StateTerminal:
		p.mode, p.gain = musicSilent, [sound.Voices]float32{}
	case g.reel != nil: // Father P's cutscene: all gathered, all play; the race winds the tape up; the flash cuts it
		p.mode = musicFree
		p.all()
		u := min(max(float64(g.reel.t-reelRace)/(reelFlash-reelRace), 0), 1)
		p.speed = 1 + (tapeTop-1)*u*u
		if g.reel.t >= reelFlash {
			p.gain = [sound.Voices]float32{}
		}
	case s.on():
		p.mode = musicFree
		switch s.id {
		case "finale", "credits": // the circle is whole
			p.all()
		case "talk": // a quest close-up: the speaker
			if s.tg != nil {
				if v, ok := castVoice(s.tg.Who.Key); ok {
					p.gain[v] = 1
				}
			}
		case "O9":
			p.gain[sound.Tim] = 1
		}
	case g.intro != nil, g.state == StateGrow, g.state == StateTitle, g.state == StateBook:
		p.mode = musicFree // the world waits: the Concierge alone (the intro holds the title)
	default: // the world runs: the cast by the guest-speech rule
		for gu := range g.guestsIn(g.realm()) {
			v, ok := castVoice(gu.Who.Key)
			if !ok || !g.hears(gu) {
				continue
			}
			if g.viewT == 0 && (gu.X < g.cam.X || gu.X > g.cam.X+viewW || gu.Y < g.cam.Y || gu.Y > g.cam.Y+viewH) {
				continue // off screen is silent
			}
			d := math.Hypot(gu.X-g.player.X, gu.Y-g.player.Y) / (hearRange * world.TileSize)
			p.gain[v] = max(p.gain[v], float32(1-d))
		}
	}
	return p
}

func (g *Game) updateMusic() {
	s := &g.snd
	p := g.musicPlan()
	q := s.seq
	if q == nil {
		return
	}
	for v := range p.gain {
		k := voiceTrim[v]
		if g.scene.id == "finale" {
			k = 1 // the sforzando on the high ones at full strength
		}
		s.mix.SetGain(v, p.gain[v]*k)
	}
	q.SetGhost(p.ghost)
	s.mix.SetSpeed(p.speed)
	g.moveMusic(q, p)
}

// moveMusic moves the sequencer by the plan's mode.
func (g *Game) moveMusic(q *sound.Sequencer, p musicPlan) {
	if p.mode == musicFrozen {
		return
	}
	if p.mode == musicSilent { // the held notes let go; on leaving the music picks up the clock (it waited too)
		q.Seek(q.Beat())
		return
	}
	if g.scene.id == "finale" { // the chest (the score's coda), to the scene's frame: the lid at fLid, V7 at fDark, I7 at fEyes
		coda := g.snd.score.Coda
		if q.Beat() < coda {
			q.StartCoda()
		}
		q.Advance(coda + float64(g.scene.t)/sound.BeatFrames - q.Beat())
		return
	}
	if q.Done() { // the finale's music is over: the loop again (the credits keep all six playing)
		q.Seek(0)
	}
	if p.mode == musicFree {
		q.Advance(p.speed * g.tempo() / sound.BeatFrames)
		return
	}
	target := math.Mod(g.clockBeat()*g.tempo(), sound.LoopBeats) // a SHIFT cuts to the other realm's place
	if q.Beat() >= sound.LoopBeats {                             // after the coda
		q.Seek(target)
		return
	}
	d := target - q.Beat()
	if d < 0 {
		d += sound.LoopBeats // a wrap is a small forward step
	}
	if d > sound.BarBeats { // a stopped screen free-ran away from the clock
		q.Seek(target)
		return
	}
	q.Advance(d)
}
