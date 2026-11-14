package game

import (
	"math"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// The staff speak (2026-11-14): a housekeeper, a courier or a Priest says a line as the Concierge comes near,
// not all the while he stands there, and one of them at a time.
type voice struct {
	near bool // the Concierge was near last frame
	said line
}

func (v *voice) talking(tick int) bool { return v.said.text != "" && tick-v.said.at < sayFrames }

var (
	courierIn  = []string{"Delivery!", "Package for the hotel!", "Where does this go?", "Heavy one today.", "Fragile. Probably."}
	courierOut = []string{"All delivered!", "See you next order!", "Have a good one!", "Back to the van."}
	priestIn   = []string{"Peace upon this house.", "Which room is troubled?", "I felt it from the street."}
	priestOut  = []string{"It is done.", "Rest easy now.", "Keep the candles lit."}
)

type speaker struct {
	*entities.Guest
	*voice
}

// speakers: the staff on the map.
func (g *Game) speakers() []speaker {
	var ss []speaker
	for _, k := range g.keepers {
		ss = append(ss, speaker{&k.Guest, &k.voice})
	}
	for _, c := range g.couriers {
		if c.out {
			ss = append(ss, speaker{&c.Guest, &c.voice})
		}
	}
	for _, pr := range g.priests {
		ss = append(ss, speaker{&pr.Guest, &pr.voice})
	}
	return ss
}

// sometimes: the lines, keeperChat of the times; else nothing.
func (g *Game) sometimes(lines []string) []string {
	if g.words.Float64() < keeperChat {
		return lines
	}
	return nil
}

// comeNear runs each frame for a member of staff: as the Concierge comes near and hears it, it says one of
// lines(), if any; while another of the staff is speaking it waits for that line to end.
func (g *Game) comeNear(guest *entities.Guest, v *voice, lines func() []string) {
	near := math.Hypot(guest.X-g.player.X, guest.Y-g.player.Y) <= keeperNear*world.TileSize && g.hears(guest)
	if near && !v.near {
		for _, s := range g.speakers() {
			if s.talking(g.tick) {
				return
			}
		}
		if ls := lines(); len(ls) > 0 {
			v.said = line{0, ls[g.words.IntN(len(ls))], g.tick}
		}
	}
	v.near = near
}

// drawVoices draws the line each of the staff is saying, while the Concierge hears it.
func (g *Game) drawVoices(s *ebiten.Image, ox, oy int) {
	for _, sp := range g.speakers() {
		if sp.talking(g.tick) && g.hears(sp.Guest) {
			g.say(s, sp.said.text, sp.Guest, ox, oy)
		}
	}
}
