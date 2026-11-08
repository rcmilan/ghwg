package game

import (
	"fmt"
	"image"

	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// The hex (D181), Fear (D146) and Spook (D148). Placeholders.
const (
	fearLimit     = 100                      // a Party with a member at this Fear flees, unpaid
	spookEvery    = 10                       // game minutes between a Room's "+N" Spook floaters (the Shadow's)
	hexScale      = 25                       // Room Spook per +1 of a hex's yield
	rotLimit      = 100                      // Room Spook from which a Room rots (under spookLimit)
	rotComfort    = 1                        // Comfort each Guest inside a rotting Room loses a game minute
	scaredMinutes = 30                       // game minutes a Party talks scared after a hex on its Room
	hauntMinutes  = 60                       // game minutes a haunt lasts; a hex restarts it (2026-11-08: back to the old haunt)
	fearEvery     = 2                        // game minutes per +1 Fear in a haunted Room: slower flees (2026-11-08)
	hauntFear     = hauntMinutes / fearEvery // the Fear one whole haunt gives
	fearWarn      = 0.75                     // of fearLimit: past it, a Guest trembles in ghost, a warning before it flees
)

// hexMark: a Room's last hex.
type hexMark struct {
	clock int // when, for the scared talk
	until int // the clock its haunt runs to: +1 Fear every fearEvery game minutes to each Guest inside
}

// beside: the Room whose outer walls world px at touches from outside, on any side (the door's
// included), or -1. Corridors are 2 cells wide, so no tile is beside two Rooms.
func beside(h *world.Hotel, at image.Point) int {
	for i, r := range h.Rooms {
		if s := r.Shell(); at.In(s.Inset(-world.TileSize)) && !at.In(s) {
			return i
		}
	}
	return -1
}

// yield: the Spook one hex earns, growing with the Room's Spook.
func (g *Game) yield(room int) Num { return 1 + Num(g.roomSpook[room])/hexScale }

func (g *Game) rotting(room int) bool {
	return room < len(g.roomSpook) && g.roomSpook[room] >= rotLimit
}

// hex (D181): one tap of the Poltergeist on the Room, from at (world px): yield(room) Spook floated from
// at, and the haunt (re)starts for hauntMinutes: the whole Room shakes and each Guest inside gains Fear
// (2026-11-08: the old haunt, tapped from any wall). Ghost only, Hex unlocked, Guests inside; false if
// refused.
func (g *Game) hex(room int, at image.Point) bool {
	_, gs := g.inside(room)
	if !g.ghost || !g.unlocked(Hex) || len(gs) == 0 {
		return false
	}
	g.earn(Spook, g.yield(room), at)
	g.tapped(room, TaskHex)
	if len(g.hexed) != len(g.hotel.Rooms) {
		g.hexed = make([]hexMark, len(g.hotel.Rooms))
	}
	g.hexed[room] = hexMark{g.clock, g.clock + hauntMinutes*minuteFrames}
	return true
}

func (g *Game) haunting(room int) bool { return room < len(g.hexed) && g.clock < g.hexed[room].until }

// scared: the Room was hexed in the last scaredMinutes (its Party talks scared, talk.go).
func (g *Game) scared(room int) bool {
	return room < len(g.hexed) && g.hexed[room].clock > 0 && g.clock-g.hexed[room].clock < scaredMinutes*minuteFrames
}

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

// hexable: the Room the Concierge stands beside with Guests inside, or -1 (the input layer's half of a
// hex, D177).
func (g *Game) hexable() int {
	room := beside(g.hotel, g.feet())
	if _, gs := g.inside(room); len(gs) == 0 {
		return -1
	}
	return room
}

// scare runs each game minute: every Guest inside a haunted Room gains 1 Fear, up to fearLimit, and the
// hotel as much Spook; inside a Room with a Shadow (D169) instead, 1 Fear every
// shadowEvery minutes, never to fearLimit, and the hotel as much Spook; inside a rotting Room it loses
// rotComfort, down to -comfortMax. A Party with a member at fearLimit flees.
func (g *Game) scare() {
	for _, p := range g.parties {
		if p.state == out || p.Fled || p.Paid {
			continue
		}
		r := g.hotel.Rooms[p.Room]
		for _, guest := range p.Guests {
			in := image.Pt(int(guest.X), int(guest.Y)).In(r.Shell())
			if in && g.haunting(p.Room) {
				if g.clock/minuteFrames%fearEvery == 0 {
					g.frighten(p, guest, 1, fearLimit)
				}
			} else if in && g.shaded(p.Room) && g.clock/minuteFrames%shadowEvery == 0 {
				g.frighten(p, guest, 1, fearLimit-1)
			}
			if in && g.rotting(p.Room) {
				guest.Comfort = max(-comfortMax, guest.Comfort-rotComfort) // below 0: the Party pays under its Rate
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

// cyan prints v in the ghost palette's cyan, over a black shadow: the Spook floaters.
func (g *Game) cyan(s *ebiten.Image, v string, x, y int) {
	g.inked(s, v, x+1, y+1, 0, 0, 0) // a black shadow, so it reads over cyan props
	g.inked(s, v, x, y, 0, 1, 1)
}

// inked prints v in the debug font at 1×, in color (r,gr,b).
func (g *Game) inked(s *ebiten.Image, v string, x, y int, r, gr, b float32) {
	if g.ink == nil {
		g.ink = ebiten.NewImage(256, 16)
	}
	g.ink.Clear()
	ebitenutil.DebugPrint(g.ink, v)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.Scale(r, gr, b, 1)
	s.DrawImage(g.ink, op)
}

// hexes: every hex tap this Run (the sim counts them).
func (g *Game) hexes() (n Num) {
	for _, t := range g.taps {
		n += t[TaskHex]
	}
	return n
}

// HexFrom is dev only (main's -shot -hex): the Concierge stands on map cell c and taps SPACE there in
// ghost, as the input layer would.
func (g *Game) HexFrom(c image.Point) {
	p := c.Mul(world.TileSize).Add(image.Pt(world.TileSize/2, world.TileSize/2))
	g.player.X, g.player.Y = float64(p.X), float64(p.Y)
	if room := g.hexable(); g.ghost && room >= 0 {
		g.hex(room, g.feet())
	}
}
