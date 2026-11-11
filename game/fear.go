package game

import (
	"image"
	"image/color"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// Fear (D146) and the Ecto it earns (D148). Fear comes from the Shadow's sightings (shadow.go). Placeholders.
const (
	fearLimit  = entities.DefaultFleeAt // a Guest's default FleeAt: a Party with a member at it flees, unpaid
	ectoEvery  = 10                     // game minutes between a Room's "+N" Ecto floaters (the Shadow's)
	rotComfort = 1                      // Comfort each Guest inside a rotting Room loses a game minute
	fearWarn   = 0.75                   // of fearLimit: past it, a Guest trembles in ghost, a warning before it flees
)

func (g *Game) rotting(room int) bool { return g.roomMood(room) <= rotMood }

// atDoor: the Room whose door spot the Concierge stands on, or -1.
func (g *Game) atDoor() int {
	at := image.Pt(int(g.player.X), int(g.player.Y))
	for i, r := range g.hotel.Rooms {
		if at.In(r.Spot()) {
			return i
		}
	}
	return -1
}

func (g *Game) feet() image.Point { return image.Pt(int(g.player.X), int(g.player.Y)) }

// scare runs each game minute: a Guest inside a rotting Room loses rotComfort, down to -comfortMax. A
// Shadow scares only on its patrols (shadow.go). A Party with a member at fearLimit steps
// out the first time and flees the second (Step 2).
func (g *Game) scare() {
	for _, p := range g.parties {
		if p.state == out || p.Fled || p.Paid {
			continue
		}
		r := g.hotel.Rooms[p.Room]
		for _, guest := range p.Guests {
			in := image.Pt(int(guest.X), int(guest.Y)).In(r.Shell())
			if in && g.rotting(p.Room) {
				guest.Comfort = max(-comfortMax, guest.Comfort-rotComfort) // below 0: the Party pays under its Rate
			}
		}
		if p.ectoShown > 0 && g.clock/minuteFrames%ectoEvery == 0 {
			door := r.Door.Mul(world.TileSize)
			g.floaters = append(g.floaters, floater{float64(door.X + world.TileSize/2), float64(door.Y), "+" + fmtNum(Num(p.ectoShown)), g.tick, floatEcto})
			p.ectoShown = 0
		}
		for _, guest := range p.Guests {
			if guest.Fear >= guest.FleeAt {
				if p.scares++; p.scares == 1 { // out for air the first time, gone the second (Step 2)
					g.stepOut(p)
				} else {
					g.flee(p)
				}
				break
			}
		}
	}
}

// flee: the Party runs for the Exit, unpaid, after a hop in place (D164); its Room is Vacant at once.
func (g *Game) flee(p *Party) {
	p.Fled, p.state, p.fledAt, p.greet, p.talk = true, leaving, g.tick, noGreeting, nil
	g.leaveMess(p)
	g.book(p.Room, fleeGap)
	for i := range p.Guests { // "RUN!" only on the way out (Live test 9, 2026-11-08)
		p.talk = append(p.talk, line{i, []string{"AAAH!", "RUN!"}[i], g.tick})
	}
	g.sendParty(p, g.hotel.Exit, g.hotel.Exit)
}

// drawFear glows red around a Guest's body, as strong as its Fear (0..1 of fearLimit). (x,y) is its foot
// box center in screen px.
func (g *Game) drawFear(s *ebiten.Image, x, y int, fear float64) {
	r := g.glow.Bounds().Dx() / 2
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(float64(-r), float64(-r))
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(float64(x), float64(y-8*zoom))
	op.ColorScale.Scale(float32(fear), 0, 0, float32(fear))
	s.DrawImage(g.glow, op)
}

// inked prints v in the debug font at 1×, in color (r,gr,b).
func (g *Game) inked(s *ebiten.Image, v string, x, y int, r, gr, b float32) {
	printAt(s, v, x, y, 1, color.NRGBA{uint8(r * 255), uint8(gr * 255), uint8(b * 255), 255}, 1)
}
