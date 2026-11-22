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
	courierIn  = []string{"Delivery!", "Package for the hotel!", "Where does this go?", "Heavy one today.", "Fragile. Probably.", odd("I'm told this box is a duck.")}
	courierOut = []string{"All delivered!", "See you next order!", "Have a good one!", "Back to the van.", odd("If the box quacks, that's normal.")}
	// The Cat's ghost-realm lines (placeholders for the designer); in real it only meows.
	catRude = []string{"Stop following me.", "Pet me and lose the hand.", "I was here first. You're the guest.", "Nine lives, and I'm spending this one on you.", "Feed me, or I tell the ghosts where you sleep.", "You smell like the lobby carpet.", "Don't look at me. Not like that. Not at all.", "I've seen better management from a goldfish."}
	// A Priest's ritual lines (B1): the opening and the closing, each Father his own.
	kOpen         = []string{"Peace upon this house.", "Which room is troubled?", "I felt it from the street.", odd("I brought holy water. And crackers.")}
	kClose        = []string{"It is done.", "Rest easy now.", "Keep the candles lit.", odd("The spirit asked for a sandwich.")}
	mOpen         = []string{"Be gone, all that clings.", "This room will be heard.", "Step aside, shadows.", odd("I came to cleanse. Also for the soup.")}
	mClose        = []string{"It is finished. All of it.", "Go in peace. All of you.", "Nothing stays that I send away.", odd("The ghost left a tip. It was a button.")}
	fatherKUnpaid = []string{"I cannot work on credit.", "Payday passed me by. So do I.", "No pay, no prayer.", odd("My halo needs a retainer too.")}
)

// nonsense: the lines of nonsense humour (08 F0), one or so in every line group, marked with odd where it is
// written. pick keeps one oddKeep of the times it comes up, else picks again: they come up more rarely.
var nonsense = map[string]bool{}

const oddKeep = 0.25

func odd(l string) string {
	nonsense[l] = true
	return l
}

// pick: one of the set, from the lines' own stream; text is the line that tells a nonsense one.
func pick[T any](g *Game, set []T, text func(T) string) T {
	x := set[g.words.IntN(len(set))]
	if nonsense[text(x)] && g.words.Float64() >= oddKeep {
		x = set[g.words.IntN(len(set))]
	}
	return x
}

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
	if c := g.items.cat; c != nil {
		ss = append(ss, speaker{&c.Guest, &c.voice})
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
			v.said = line{0, g.pickLine(ls), g.tick}
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
