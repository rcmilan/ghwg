package game

import (
	"slices"

	"ghosthotel/entities"

	"github.com/hajimehoshi/ebiten/v2"
)

// Guests talk (D165). Placeholders.
const (
	sayFrames    = 120      // a line shows this long
	answerFrames = 120      // a pair's follower answers this long after its leader
	calmOdds     = 1.0 / 90 // a line a game minute in the Room, unhaunted
	scaredOdds   = 1.0 / 20 // haunted
)

// greeting: what a Party will say once the Concierge first hears it.
type greeting int

const (
	noGreeting greeting = iota
	greetHello          // its first arrival
	greetOut            // a Tourist going out
	greetBack           // and coming back
)

// line: a Guest's line in a speech balloon, from tick at for sayFrames.
type line struct {
	who  int // the speaker's index in its Party
	text string
	at   int
}

var (
	outLines  = [][2]string{{"The park?", "Then the museum!"}, {"Museum first?", "Yes! Then the park!"}, {"Let's walk!", "To the park!"}}
	backLines = [][2]string{{"What a day!", "My feet hurt!"}, {"Loved the museum!", "The park was better!"}, {"I'm starving!", "Room service!"}}
	// In the Room, in thirds of a limit: [0] anyone says, [1] only a pair, said to the other.
	calmLines = [3][2][]string{ // between haunts, by Room Spook
		{{"Nice room!", "So comfy!", "Lovely view.", "Ahh, quiet."}, {"Isn't this nice?", "Best trip ever!"}},
		{{"It's cold in here.", "Smells odd...", "Hm. Drafty."}, {"Did you move my bag?", "Chilly, isn't it?"}},
		{{"I don't like this room.", "Did the lamp move?", "Can't relax here..."}, {"Let's not stay long.", "You feel watched too?"}},
	}
	scaredLines = [3][2][]string{ // haunted, by Fear or Room Spook, whichever is higher
		{{"What was that?", "Hello...?", "Who's there?"}, {"Did you hear that?", "Was that you?"}},
		{{"Something's here...", "I want to leave.", "I'm scared..."}, {"Hold my hand!", "You feel that too?"}},
		{{"HELP!", "I have to get out!"}, {"We have to go!", "RUN!"}},
	}
)

// clockGreeting: hello by the time of day.
func clockGreeting(c int) string {
	switch h := hour(c); {
	case h >= 5 && h < 12:
		return "Good morning!"
	case h >= 12 && h < 18:
		return "Good afternoon!"
	}
	return "Good evening!"
}

// updateTalk runs each frame: lines past sayFrames end, and a Party walking in or out says the greeting it
// owes the Concierge once its leader is first heard outside the Rooms (never from inside, even in ghost);
// then a pair chats between themselves, the leader's line and the follower's answer.
func (g *Game) updateTalk() {
	for _, p := range g.parties {
		p.talk = slices.DeleteFunc(p.talk, func(l line) bool { return g.tick-l.at >= sayFrames })
		if lead := p.Guests[0]; p.greet == noGreeting || p.state != checkingIn && p.state != goingOut || !g.shown(lead) || g.indoors(lead) || !g.hears(lead) {
			continue
		}
		switch hi := clockGreeting(g.clock); {
		case p.greet == greetHello:
			p.talk = append(p.talk, line{0, "Hello!", g.tick})
		case len(p.Guests) == 1:
			p.talk = append(p.talk, line{0, hi, g.tick})
		default:
			pairs := outLines
			if p.greet == greetBack {
				pairs = backLines
			}
			x := pairs[g.chat.IntN(len(pairs))]
			p.talk = append(p.talk, line{0, hi, g.tick}, line{0, x[0], g.tick + answerFrames}, line{1, x[1], g.tick + 2*answerFrames})
		}
		p.greet = noGreeting
	}
}

// updateRoomTalk runs each game minute: a Party in its Room, not asking nor talking, now and then says a
// line by its Room's Spook ("Zzz..." asleep), or while haunted a scared one by its speaker's Fear or the
// Room Spook, whichever is higher. A single never says a line meant for its mate.
func (g *Game) updateRoomTalk() {
	asleep := asleep(g.clock)
	for _, p := range g.parties {
		haunted := g.haunting(p.Room)
		odds := calmOdds
		if haunted {
			odds = scaredOdds
		}
		if p.state != staying || p.asking() || len(p.talk) > 0 || g.chat.Float64() >= odds {
			continue
		}
		who := g.chat.IntN(len(p.Guests))
		tier := min(2, 3*g.roomSpook[p.Room]/spookLimit)
		set := calmLines[tier]
		if haunted {
			set = scaredLines[max(tier, min(2, 3*p.Guests[who].Fear/fearLimit))]
		}
		lines := set[0]
		switch {
		case asleep && !haunted:
			lines = []string{"Zzz..."}
		case len(p.Guests) > 1:
			lines = slices.Concat(set[0], set[1])
		}
		p.talk = append(p.talk, line{who, lines[g.chat.IntN(len(lines))], g.tick})
	}
}

// shown: the guest is on the map.
func (g *Game) shown(guest *entities.Guest) bool { return slices.Contains(g.guests(), guest) }

// drawTalk draws each line that has started by its speaker's head, while the Concierge hears it. An asking
// Party shows only its "!!".
func (g *Game) drawTalk(s *ebiten.Image, ox, oy int) {
	for _, p := range g.parties {
		if p.asking() {
			continue
		}
		for _, l := range p.talk {
			if guest := p.Guests[l.who]; g.tick >= l.at && g.shown(guest) && g.hears(guest) {
				g.say(s, l.text, guest, ox, oy)
			}
		}
	}
}
