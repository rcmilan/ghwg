package game

import (
	"fmt"
	"image"
	"image/color"

	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// The Poltergeist (D150), Fear (D146) and Spook (D148). Placeholders, retuned at T112.
const (
	hauntMinutes  = 60  // game minutes a haunt lasts
	fearPerMinute = 1   // Fear each Guest inside a haunted Room gains per game minute
	fearLimit     = 100 // a Party with a member at this Fear flees, unpaid
	spookEvery    = 10  // game minutes between a Room's "+N" Spook floaters
)

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

// haunt sets the Poltergeist loose in a Room for hauntMinutes, restarting a running haunt.
func (g *Game) haunt(room int) {
	if len(g.haunted) != len(g.hotel.Rooms) {
		g.haunted = make([]int, len(g.hotel.Rooms))
	}
	g.haunted[room] = g.clock + hauntMinutes*minuteFrames
}

func (g *Game) haunting(room int) bool { return room < len(g.haunted) && g.clock < g.haunted[room] }

// scare runs each game minute: every Guest inside a haunted Room gains Fear, and the hotel as much Spook;
// in a Room with a Shadow (D169), 1 every shadowEvery minutes, never to fearLimit. A Party with a member
// at fearLimit flees.
func (g *Game) scare() {
	for _, p := range g.parties {
		if p.state == out || p.Fled || p.Paid {
			continue
		}
		r := g.hotel.Rooms[p.Room]
		for _, guest := range p.Guests {
			switch in := image.Pt(int(guest.X), int(guest.Y)).In(r.Shell()); {
			case in && g.haunting(p.Room):
				g.frighten(p, guest, fearPerMinute, fearLimit)
			case in && g.shaded(p.Room) && g.clock/minuteFrames%shadowEvery == 0:
				g.frighten(p, guest, 1, fearLimit-1)
			}
		}
		if p.spookShown > 0 && g.clock/minuteFrames%spookEvery == 0 {
			door := r.Door.Mul(world.TileSize)
			g.floaters = append(g.floaters, floater{float64(door.X + world.TileSize/2), float64(door.Y), fmt.Sprintf("+%d", p.spookShown), g.tick, floatSpook})
			p.spookShown = 0
		}
		for _, guest := range p.Guests {
			if guest.Fear >= fearLimit {
				g.flee(p)
				break
			}
		}
	}
}

// flee: the Party runs for the Exit, unpaid, after a hop in place (D164); its Room is Vacant at once.
func (g *Game) flee(p *Party) {
	p.Fled, p.state, p.fledAt, p.greet, p.talk = true, leaving, g.tick, noGreeting, nil
	for i := range p.Guests {
		p.talk = append(p.talk, line{i, "AAAH!", g.tick})
	}
	g.sendParty(p, g.hotel.Exit, g.hotel.Exit)
}

// shake: a haunted Room's furniture jitters ±1 world px on a 4-frame beat, each piece off its own phase
// (ghost only).
func (g *Game) shake(p prop) int {
	if !g.ghost || p.l != world.LayerProps {
		return 0
	}
	for i, r := range g.hotel.Rooms {
		o := r.Origin
		if g.haunting(i) && image.Pt(p.cx, p.cy).In(image.Rect(o.X, o.Y, o.X+world.RoomW, o.Y+world.RoomH)) {
			return 1 - (g.tick/4+p.cx+p.cy)%3
		}
	}
	return 0
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

// cyan prints v in the ghost palette's cyan: the Spook floaters.
func (g *Game) cyan(s *ebiten.Image, v string, x, y int) {
	g.inked(s, v, x, y, 0, 1, 1)
}

// inked prints v in the debug font at 1×, in color (r,gr,b).
func (g *Game) inked(s *ebiten.Image, v string, x, y int, r, gr, b float32) {
	printAt(s, v, x, y, 1, color.NRGBA{uint8(r * 255), uint8(gr * 255), uint8(b * 255), 255}, 1)
}
