package game

import (
	"image/color"
	"math"

	"ghosthotel/assets"

	"github.com/hajimehoshi/ebiten/v2"
)

// The opening intro (07 B1, 06's T224): a new Run opens on the whole hotel, both Rooms boarded up, in the ghost
// dimension; it zooms into the Lobby; three lines, one per SPACE, reinforce SHIFT; then a Mirror flip from
// ghost into real plays once and the Run begins. The world and the Clock wait for all of it. The sim, the
// tests and the dev prints never play it unless asked (introOn); main's -skipintro turns it off for play.
const (
	introHold       = 90 // frames on the whole hotel
	introZoomFrames = 60 // frames zooming into the Lobby
)

// introText: the three lines, a heading and a body each. DRAFT: placeholders until the user approves them.
var introText = [...]struct{ Head, Body string }{
	{"NIGHT SHIFT", "Somebody has to mind the desk.\nYou never did clock out."},
	{"GHOST SHIFT", "The living pay in dollars.\nThe dead pay in ectoplasm."},
	{"DIMENSION SHIFT", "Every Room has two sides.\nOpen one, then find the Mirror."},
}

type introPhase int

const (
	introWide  introPhase = iota // the whole hotel, still
	introZoom                    // zooming into the Lobby
	introLines                   // line n shows; SPACE for the next
	introFlip                    // the Mirror flip, ghost into real
)

// intro is the opening under way.
type intro struct {
	frame    int  // frames since it began
	line     int  // the line showing
	flipping bool // the flip has started
}

func (in *intro) phase() introPhase {
	switch {
	case in.flipping:
		return introFlip
	case in.frame < introHold:
		return introWide
	case in.frame < introHold+introZoomFrames:
		return introZoom
	}
	return introLines
}

// PlayIntro makes every Run start with the intro (main, for a person playing).
func (g *Game) PlayIntro() { g.introOn = true }

// StartIntro begins a Run at the intro's line (0: from the start; 1..3: that line showing; 4: the flip), for
// main's -intro prints.
func (g *Game) StartIntro(line int) {
	g.introOn = true
	g.startRun()
	if line > 0 {
		g.intro.frame = introHold + introZoomFrames
		g.intro.line = min(line, len(introText)) - 1
	}
	if line > len(introText) {
		g.intro.flipping = true
		g.startShift()
	}
}

// beginIntro, at the end of startRun: the Run opens in the ghost dimension, to flip out of it.
func (g *Game) beginIntro() {
	g.intro = nil
	if g.introOn {
		g.intro, g.ghost = &intro{}, true
	}
}

// updateIntro advances the intro one frame. SPACE skips the zoom to the lines, then shows the next line, then
// starts the flip; the flip ends it.
func (g *Game) updateIntro() {
	in := g.intro
	g.tick++ // the transition runs on it; the Clock and the world stay still
	g.updateShift()
	space := g.in.JustPressed(ebiten.KeySpace)
	switch in.phase() {
	case introWide, introZoom:
		if in.frame++; space {
			in.frame = introHold + introZoomFrames
		}
	case introLines:
		if space {
			if in.line++; in.line == len(introText) {
				in.line, in.flipping = len(introText)-1, true
				g.startShift()
			}
		}
	case introFlip:
		if g.shifting == nil {
			g.intro = nil
		}
	}
}

// drawIntro draws the intro's own frames; false leaves the frame to Draw (the flip is the Mirror's).
func (g *Game) drawIntro(screen *ebiten.Image) bool {
	in := g.intro
	if in == nil || g.state == StateTitle {
		return false
	}
	switch in.phase() {
	case introWide, introZoom:
		g.drawZoom(screen, in)
	case introLines:
		g.drawFrame(screen)
		t := introText[in.line]
		rect(screen, 60, 394, 840, 118, assets.Ink)
		rect(screen, 60, 394, 840, 4, assets.Accent)
		g.tintedAt(screen, t.Head, 84, 412, 3, assets.Accent, 1)
		g.tintedAt(screen, t.Body, 84, 456, 2, assets.Ghost, 1)
		g.tintedAt(screen, "SPACE", 836, 490, 1, assets.Floor, 1)
	default:
		return false
	}
	return true
}

// drawZoom draws the whole hotel at zoom into g.wide, then shows it through a view that eases from fitting the
// screen to the game's own: the picture centered, then the Lobby the Concierge stands in.
func (g *Game) drawZoom(screen *ebiten.Image, in *intro) {
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
	t := math.Min(1, math.Max(0, float64(in.frame-introHold)/introZoomFrames))
	e := t * t * (3 - 2*t)
	k := math.Pow(k0, 1-e) // the scale eases from k0 to 1
	ox, oy := g.cam.Origin(g.player.X, g.player.Y, mw, mh)
	cx, cy := float64(bw)/2*(1-e)+float64(ox+sw/2)*e, float64(bh)/2*(1-e)+float64(oy+sh/2)*e // the view's center
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-(cx - float64(sw)/2/k), -(cy - float64(sh)/2/k))
	op.GeoM.Scale(k, k)
	screen.Fill(color.Black)
	screen.DrawImage(g.wide, op)
}
