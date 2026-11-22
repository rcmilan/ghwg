package game

import (
	"math"
	"slices"
	"testing"

	"ghosthotel/sound"
	"ghosthotel/world"
)

// heard: the voices with any gain in the plan.
func heard(p musicPlan) (vs []sound.Voice) {
	for v, g := range p.gain {
		if g > 0 {
			vs = append(vs, sound.Voice(v))
		}
	}
	return vs
}

// atGuest puts the first guest of p outside its Room, the Concierge dx tiles right of it, the camera on him.
func atGuest(g *Game, p *Party, key string, dx int) {
	r := g.hotel.Rooms[p.Room]
	gu := p.Guests[0]
	gu.Who.Key = key
	gu.X, gu.Y = float64(center(r.Spot()).X), float64(center(r.Spot()).Y)
	g.player.X, g.player.Y = gu.X+float64(dx*world.TileSize), gu.Y
	snapCam(g)
}

func snapCam(g *Game) {
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}

// Real hears Magenta near, never through a wall, never Tim; ghost hears Tim and never Magda.
func TestMusicHearsByRealm(t *testing.T) {
	g, _, p := settled(t)
	atGuest(g, p, "magenta", 2)
	if got := heard(g.musicPlan()); !slices.Equal(got, []sound.Voice{sound.Concierge, sound.Magenta}) {
		t.Errorf("real, Magenta near: heard %v", got)
	}
	g.player.Y += 5 * world.TileSize // the corridor's wall between
	snapCam(g)
	if got := heard(g.musicPlan()); !slices.Equal(got, []sound.Voice{sound.Concierge}) {
		t.Errorf("real, behind a wall: heard %v", got)
	}
	atGuest(g, p, "tim", 2)
	p.Realm = Ghost // a ghost's Party, the Concierge in real
	if got := heard(g.musicPlan()); !slices.Equal(got, []sound.Voice{sound.Concierge}) {
		t.Errorf("real, a ghost's voice: heard %v", got)
	}
	g.ghost = true // the same Party, now heard
	if got := heard(g.musicPlan()); !slices.Equal(got, []sound.Voice{sound.Concierge, sound.Tim}) {
		t.Errorf("ghost, Tim near: heard %v", got)
	}
	atGuest(g, p, "magda", 2)
	p.Realm = Human
	if got := heard(g.musicPlan()); !slices.Equal(got, []sound.Voice{sound.Concierge}) {
		t.Errorf("ghost, Magda: heard %v", got)
	}
	if pl := g.musicPlan(); !pl.ghost || pl.mode != musicFollow {
		t.Errorf("ghost plan: %+v", pl)
	}
}

// The cast's gain falls from 1 to 0 with distance, and a voice off screen is silent.
func TestMusicGainFallsWithDistance(t *testing.T) {
	g, _, p := settled(t)
	var last float32 = 2
	for _, dx := range []int{0, 2, 4, 6} {
		atGuest(g, p, "magenta", dx)
		got := g.musicPlan().gain[sound.Magenta]
		if got <= 0 || got >= last {
			t.Errorf("%d tiles: gain %v after %v", dx, got, last)
		}
		last = got
	}
	atGuest(g, p, "magenta", 2)
	g.cam.X += 2 * viewW // the camera is elsewhere
	if got := g.musicPlan().gain[sound.Magenta]; got != 0 {
		t.Errorf("off screen: gain %v", got)
	}
}

// Stopped screens free-run with the Concierge alone; a quest close-up adds its speaker; the pause freezes.
func TestMusicStoppedScreens(t *testing.T) {
	g, _, p := settled(t)
	atGuest(g, p, "magenta", 1)
	for _, st := range []int{StateBook, StateGrow} {
		g.state = st
		if pl := g.musicPlan(); pl.mode != musicFree || !slices.Equal(heard(pl), []sound.Voice{sound.Concierge}) {
			t.Errorf("state %d: %+v", st, pl)
		}
	}
	g.state = StatePause
	if g.musicPlan().mode != musicFrozen {
		t.Error("the pause plays on")
	}
	g.state = StatePlay
	g.scene = sceneState{id: "talk", tg: p.Guests[0]}
	if pl := g.musicPlan(); pl.mode != musicFree || !slices.Equal(heard(pl), []sound.Voice{sound.Concierge, sound.Magenta}) {
		t.Errorf("a close-up with Magenta: %+v", pl)
	}
	p.Guests[0].Who.Key = "teo"
	if got := heard(g.musicPlan()); !slices.Equal(got, []sound.Voice{sound.Concierge}) {
		t.Errorf("Teo has no voice: %v", got)
	}
}

// The title is the Concierge in ghost treatment; the world shifting to real turns it.
func TestMusicTitleIsGhost(t *testing.T) {
	g, _, _ := settled(t)
	g.introOn = true
	g.beginIntro()
	pl := g.musicPlan()
	if pl.mode != musicFree || !pl.ghost || !slices.Equal(heard(pl), []sound.Voice{sound.Concierge}) {
		t.Errorf("title: %+v", pl)
	}
	g.ghost = false
	if g.musicPlan().ghost {
		t.Error("real once the world shifts")
	}
}

// The finale plays all six and the circle has all five even when nobody is in the hotel; the credits too.
func TestMusicFinaleWholeCircle(t *testing.T) {
	g, s, _ := sceneGame(t)
	g.parties = nil // Magda, Osvaldo and the rest have checked out
	g.playQuestScene("finale")
	for g.scene.t < fPoints+1 {
		stepPressing(g)
		_ = s
	}
	if n := len(g.scene.cast); n != 1+len(finaleKeys) {
		t.Fatalf("the circle has %d, want %d", n, 1+len(finaleKeys))
	}
	for i, c := range g.scene.cast[1:] {
		if c.g.Who.Key != finaleKeys[i] || c.ghost != finaleGhost(c.g.Who.Key) || c.g.Character == "" {
			t.Errorf("point %d: %q ghost %v", i, c.g.Who.Key, c.ghost)
		}
		if dx, dy := c.g.X-float64(c.slot.X), c.g.Y-float64(c.slot.Y); dx*dx+dy*dy > 1 {
			t.Errorf("%s is not on its point", c.g.Who.Key)
		}
	}
	if pl := g.musicPlan(); pl.mode != musicFree || len(heard(pl)) != int(sound.Voices) {
		t.Errorf("finale: %+v", pl)
	}
	g.scene = sceneState{id: "credits", from: "finale"}
	if pl := g.musicPlan(); len(heard(pl)) != int(sound.Voices) {
		t.Errorf("credits: %+v", pl)
	}
}

// openTestSound: the sound as OpenSound builds it, with no audio context.
func openTestSound(t *testing.T, g *Game) {
	t.Helper()
	s := &g.snd
	s.mix = sound.NewMixer()
	bank, err := sound.LoadBank(sound.Data)
	if err != nil {
		t.Fatal(err)
	}
	data, err := sound.Data.ReadFile("data/fugue.mid")
	if err != nil {
		t.Skip("no score yet")
	}
	sc, err := sound.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	s.bank, s.score, s.seq = bank, sc, sound.NewSequencer(sc, s.mix, bank)
}

// The music follows the clock strictly while the world runs, free-runs on a stopped screen, and finds the clock again.
func TestMusicFollowsClock(t *testing.T) {
	g, _, _ := settled(t)
	openTestSound(t, g)
	q := g.snd.seq
	g.clock, g.acc = 3*sound.BeatFrames+7, 0
	g.updateMusic()
	if want := g.clockBeat() * RealTempo; math.Abs(q.Beat()-want) > 1e-9 {
		t.Fatalf("followed to %v, want %v", q.Beat(), want)
	}
	g.state = StateBook // the clock waits; the music runs on
	b := q.Beat()
	for range 2 * sound.BeatFrames {
		g.updateMusic()
	}
	if got := q.Beat() - b; math.Abs(got-2*RealTempo) > .01 {
		t.Errorf("free-ran %v beats in two clock beats' frames, want %v in real", got, 2*RealTempo)
	}
	g.state = StatePlay
	g.updateMusic()
	if want := g.clockBeat() * RealTempo; math.Abs(q.Beat()-want) > 1e-9 {
		t.Errorf("back to the clock: %v, want %v", q.Beat(), want)
	}
	g.ghost = true // SHIFT: the ghost plays the same score at half the tempo
	g.updateMusic()
	if want := g.clockBeat(); math.Abs(q.Beat()-want) > 1e-9 {
		t.Errorf("in ghost: %v, want %v", q.Beat(), want)
	}
	g.state = StatePause
	b = q.Beat()
	g.updateMusic()
	if q.Beat() != b {
		t.Error("the pause moved the music")
	}
}

// Father P's race winds the tape up, all six playing; the flash cuts them.
func TestMusicReelTape(t *testing.T) {
	g, _, _ := settled(t)
	g.reel = &prestigeReel{t: reelGather}
	if pl := g.musicPlan(); pl.speed != 1 || len(heard(pl)) != int(sound.Voices) {
		t.Errorf("the gathering: %+v, want all six at speed 1", pl)
	}
	g.reel.t = reelFlash - 1
	if pl := g.musicPlan(); pl.speed < tapeTop-.1 {
		t.Errorf("the race's end at speed %v, want near %v", pl.speed, tapeTop)
	}
	g.reel.t = reelFlash
	if pl := g.musicPlan(); len(heard(pl)) != 0 {
		t.Errorf("the flash: %v still heard", heard(pl))
	}
}

// The finale's fanfare runs to the scene's frame: the V7 sounds at the dark.
func TestMusicFinaleFanfare(t *testing.T) {
	g, _, _ := settled(t)
	openTestSound(t, g)
	g.playQuestScene("finale")
	q, coda := g.snd.seq, g.snd.score.Coda
	for g.scene.t = -fBlack; g.scene.t <= fDark; g.scene.t++ {
		g.updateMusic()
	}
	if want := coda + float64(fDark)/sound.BeatFrames; math.Abs(q.Beat()-want) > 1e-9 {
		t.Errorf("at the dark the music is at %v, want %v", q.Beat(), want)
	}
}

// The high voices sit under the rest.
func TestVoiceTrim(t *testing.T) {
	for v, k := range voiceTrim {
		if high := sound.Voice(v) == sound.Celestine || sound.Voice(v) == sound.Osvaldo; high != (k < 1) || k <= 0 {
			t.Errorf("%s trim %.2f", sound.VoiceNames[v], k)
		}
	}
}

// The Terminal stops the music outright: silent, nothing held, nothing moving; on leaving it follows the clock.
func TestMusicTerminalSilent(t *testing.T) {
	g, _, _ := settled(t)
	openTestSound(t, g)
	q := g.snd.seq
	g.updateMusic()
	g.state = StateTerminal
	if p := g.musicPlan(); p.mode != musicSilent || len(heard(p)) != 0 {
		t.Fatalf("terminal plan %+v", p)
	}
	b := q.Beat()
	for range sound.BeatFrames {
		g.updateMusic()
	}
	if q.Beat() != b {
		t.Errorf("the music moved in the Terminal: %v -> %v", b, q.Beat())
	}
	g.state = StatePlay
	g.updateMusic()
	if want := math.Mod(g.clockBeat()*RealTempo, sound.LoopBeats); math.Abs(q.Beat()-want) > 1e-9 {
		t.Errorf("after the Terminal: %v, want %v", q.Beat(), want)
	}
}
