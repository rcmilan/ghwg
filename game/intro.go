package game

import (
	"image/color"
	"math"
	"strings"

	"ghosthotel/assets"
	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// The opening (07 B1, 06's T224; reworked 2026-11-11). The title is the whole hotel in the ghost dimension,
// its Rooms boarded up, the Concierge nowhere, ghosts strolling the street. SPACE: three brief lines appear
// over it, one after the other on a timer. SPACE again: the Concierge walks in from the street as a Guest
// would, while the view closes in to the game's own and the world shifts to real; then the Run begins. The
// world and the Clock wait for all of it. The sim, the tests and the dev prints never play it (introOn).
const (
	introLineGap  = 2 * tps  // frames between one line and the next
	introFade     = tps / 3  // frames a line fades in over; all fade out over the same
	introStroll   = 0.25     // world px per frame: a street ghost's pace
	introTextSize = hudScale // the lines are as big as the wallet's counters
)

// introLines: the three lines. DRAFT: placeholders until the user approves them. SHIFT is drawn in the
// Ectoplasm ink.
var introLines = [...]string{
	"YOUR SHIFT STARTS NOW",
	"RENT ROOMS TO THE LIVING AND THE DEAD",
	"USE THE MIRROR TO SHIFT",
}

type introPhase int

const (
	introTitle introPhase = iota // the title: the whole hotel, waiting for SPACE
	introText                    // the lines appear on a timer; SPACE once they all show
	introEnter                   // the Concierge walks in; the view closes in; the world shifts to real
)

// intro is the opening under way.
type intro struct {
	phase  introPhase
	frame  int               // frames into the phase
	ghosts []*entities.Guest // strolling the street
	dirs   []float64         // each one's heading along it, ±1
	walk   entities.Guest    // the Concierge's walk in
	total  float64           // its length, world px
}

// PlayIntro makes the title and every Run start with the opening (main, for a person playing).
func (g *Game) PlayIntro() {
	g.introOn = true
	g.toTitle()
}

// StartIntro puts the opening at a phase (0: the title; 1: the three lines showing; 2: the walk in), for
// main's -intro prints.
func (g *Game) StartIntro(phase int) {
	g.PlayIntro()
	if phase >= 1 {
		g.state, g.intro.phase, g.intro.frame = StatePlay, introText, len(introLines)*introLineGap
	}
	if phase >= 2 {
		g.enter()
	}
}

// toTitle shows the title. With the opening on, the next Run is already built behind it: the title is its
// hotel.
func (g *Game) toTitle() {
	if g.introOn {
		g.startRun()
	}
	g.state = StateTitle
}

// beginIntro, at the end of startRun: the hotel waits in the ghost dimension, ghosts on its street.
func (g *Game) beginIntro() {
	g.intro = nil
	if !g.introOn {
		return
	}
	in := &intro{}
	mw, _ := g.mapSize()
	for k := range ghostKinds { // one of each kind, spread along the street's three rows
		at := g.hotel.Spawn
		at.X, at.Y = (2*k+1)*mw/(2*len(ghostKinds)), at.Y+(k%3-1)*world.TileSize
		gh := entities.NewGuest(at, shadowCast[g.chat.IntN(len(shadowCast))])
		gh.Phantasm = max(1, phantasmBands[max(0, k-1)]*min(k, 1))
		in.ghosts, in.dirs = append(in.ghosts, &gh), append(in.dirs, float64(1-2*(k%2)))
	}
	g.intro, g.ghost = in, true
}

// enter starts the walk in: the Concierge on the street, bound for his desk.
func (g *Game) enter() {
	in := g.intro
	in.phase, in.frame = introEnter, 0
	in.walk = entities.NewGuest(g.hotel.Spawn, "Bob")
	in.walk.SetPath(g.hotel.Path(g.hotel.Spawn, g.hotel.ConciergeStart, -1))
	in.total = max(1, in.walk.Left())
	g.player.X, g.player.Y = in.walk.X, in.walk.Y
}

// closeIn: how far the view has closed in, 0 the whole hotel .. 1 the game's own: the share of the walk done.
func (in *intro) closeIn() float64 {
	if in.phase != introEnter {
		return 0
	}
	t := 1 - in.walk.Left()/in.total
	return t * t * (3 - 2*t)
}

// updateIntro advances the opening one frame.
func (g *Game) updateIntro() {
	in := g.intro
	g.tick++ // the transition and the inks run on it; the Clock and the world stay still
	in.frame++
	g.updateShift()
	mw, _ := g.mapSize()
	for i, gh := range in.ghosts { // they turn back at the street's ends
		if gh.X += in.dirs[i] * introStroll; gh.X < world.TileSize || gh.X > float64(mw-world.TileSize) {
			in.dirs[i] = -in.dirs[i]
		}
		gh.Anim.Update(gh.X, gh.Y, 1)
	}
	space := g.in.JustPressed(ebiten.KeySpace)
	all := (len(introLines) - 1) * introLineGap // the frame the last line appears
	switch in.phase {
	case introTitle:
		if space {
			g.state, in.phase, in.frame = StatePlay, introText, 0
		}
	case introText:
		switch {
		case space && in.frame < all: // in a hurry: every line at once
			in.frame = all
		case space:
			g.enter()
		}
	case introEnter:
		w := &in.walk
		x, y, path := w.X, w.Y, w.Path
		done := w.FollowPath(conciergeSpeed)
		if g.hotel.DoorBlocks(entities.FootBox(w.X, w.Y)) { // he waits for the Entrance to slide open, like a Guest
			w.X, w.Y, w.Path, done = x, y, path, false
		}
		g.player.X, g.player.Y = w.X, w.Y
		g.player.Anim.Update(w.X, w.Y, 1)
		g.hotel.UpdateDoors(g.doorActors(), g.boarded, 1)
		if e := g.hotel.Doors[len(g.hotel.Doors)-1]; g.shifting == nil && g.ghost && entities.FootBox(w.X, w.Y).Overlaps(e.Rect) { // the shift starts as he steps into the Entrance
			g.startShift()
		}
		if done && g.shifting == nil {
			g.ghost, g.intro = false, nil
			g.player.Anim.Facing = entities.FaceDown // at his desk, facing the Terminal
			mw, mh := g.mapSize()
			g.cam.Snap(g.player.X, g.player.Y, mw, mh)
		}
	}
}

// drawIntro draws the opening's frame in the current dimension: the hotel, then the title or the lines.
func (g *Game) drawIntro(screen *ebiten.Image) {
	in := g.intro
	g.drawZoom(screen, in.closeIn())
	sw := screen.Bounds().Dx()
	center := func(v string, y int, scale float64, c color.Color, a float32) int {
		x := (sw - int(float64(len(v)*glyphW)*scale)) / 2
		printAt(screen, v, x, y, scale, c, a)
		return x
	}
	if in.phase == introTitle {
		rect(screen, 0, 12, sw, glyphH*(introTextSize+3)+16, color.RGBA{0, 0, 0, 170}) // at the top: the street stays clear
		center("G H O S T   H O T E L", 20, introTextSize, assets.Ghost, 1)
		center("[ SPACE ] open the hotel", 20+glyphH*introTextSize, 2, resInk(Money), 1)
		center("WASD move / SPACE use / ESC title or quit", 20+glyphH*(introTextSize+2), 1, color.White, 1)
		return
	}
	fade := float32(1) // the lines go as the Concierge walks in
	if in.phase == introEnter {
		fade = max(0, 1-float32(in.frame)/introFade)
	}
	if fade == 0 {
		return
	}
	lineH := glyphH*introTextSize + 8
	top := (screen.Bounds().Dy() - lineH*len(introLines)) / 2
	rect(screen, 0, top-12, sw, lineH*len(introLines)+16, color.RGBA{0, 0, 0, uint8(170 * fade)})
	for i, l := range introLines {
		a := fade
		if in.phase == introText {
			a = min(1, max(0, float32(in.frame-i*introLineGap)/introFade))
		}
		if a == 0 {
			continue
		}
		x, y := center(l, top+i*lineH, introTextSize, silver, a), top+i*lineH
		if k := strings.Index(l, "SHIFT"); k >= 0 {
			printAt(screen, "SHIFT", x+k*glyphW*introTextSize, y, introTextSize, resInk(Ecto), a)
		}
	}
	if in.phase == introText && in.frame >= (len(introLines)-1)*introLineGap+introFade {
		center("[ SPACE ]", top+len(introLines)*lineH+16, 2, resInk(Money), 1)
	}
}

// drawZoom draws the whole hotel at zoom into g.wide, then shows it through a view e of the way (0..1) from
// fitting the screen, centered, to the game's own on the Concierge.
func (g *Game) drawZoom(screen *ebiten.Image, e float64) {
	mw, mh := g.mapSize()
	bw, bh := mw*zoom, mh*zoom
	if g.wide == nil || g.wide.Bounds().Dx() != bw || g.wide.Bounds().Dy() != bh {
		g.wide = ebiten.NewImage(bw, bh)
	}
	g.wide.Fill(assets.Night)
	if g.ghost {
		g.wide.Fill(color.Black)
	}
	g.drawWorld(g.wide, 0, 0)
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	k0 := math.Min(float64(sw)/float64(bw), float64(sh)/float64(bh))
	k := math.Pow(k0, 1-e) // the scale eases from k0 to 1
	cx, cy := clampView(g.player.X, g.player.Y, mw, mh)
	cx, cy = float64(bw)/2*(1-e)+(cx*zoom+float64(sw)/2)*e, float64(bh)/2*(1-e)+(cy*zoom+float64(sh)/2)*e // the view's center
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-(cx - float64(sw)/2/k), -(cy - float64(sh)/2/k))
	op.GeoM.Scale(k, k)
	screen.Fill(color.Black)
	screen.DrawImage(g.wide, op)
}
