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
	calmOdds     = 1.0 / 90 // a line a game minute in the Room
)

// greeting: what a Party will say once the Concierge first hears it.
type greeting int

const (
	noGreeting greeting = iota
	greetHello          // its first arrival
	greetOut            // a Tourist going out
	greetBack           // and coming back
	greetClean          // back from the hall once its Room is clean (Live test 4, 2026-11-08)
)

// line: a Guest's line in a speech balloon, from tick at for sayFrames.
type line struct {
	who  int // the speaker's index in its Party
	text string
	at   int
}

var (
	helloLines = []string{"Hello!", "Hi there!", "Good to be here!", "What a lovely place!", "Long trip!"}
	outLines   = [][2]string{{"The park?", "Then the museum!"}, {"Museum first?", "Yes! Then the park!"}, {"Let's walk!", "To the park!"},
		{"Coffee first?", "Always coffee!"}, {"Got the map?", "Got the camera!"}, {"Shopping today?", "And ice cream!"}}
	backLines = [][2]string{{"What a day!", "My feet hurt!"}, {"Loved the museum!", "The park was better!"}, {"I'm starving!", "Room service!"},
		{"So many stairs!", "Worth the view!"}, {"I bought too much.", "You always do!"}, {"Bed. Now.", "Dinner first!"}}
	soloOut  = []string{"Off to see the town!", "A walk will do me good.", "Back by dinner!"}
	soloBack = []string{"What a day!", "My feet hurt!", "Good to be back."}
	// In the Room, in thirds of a limit: [0] anyone says, [1] only a pair, said to the other.
	calmLines = [3][2][]string{ // by Room dirt
		{{"Nice room!", "So comfy!", "Lovely view.", "Ahh, quiet.", "Soft pillows!", "Great price.", "This bed is perfect.", "I could live here.", "Five stars."},
			{"Isn't this nice?", "Best trip ever!", "Let's stay longer!", "Shall we order in?", "Pass the pillow?"}},
		{{"It's cold in here.", "Smells odd...", "Hm. Drafty.", "Was that a creak?", "My ears are ringing.", "The tap is dripping.", "Is the window open?"},
			{"Did you move my bag?", "Chilly, isn't it?", "Did you hum just now?", "Did you hear that?", "Was that you?"}},
		{{"I don't like this room.", "Did the lamp move?", "Can't relax here...", "The mirror blinked.", "Stop whispering.", "Something's under the bed.", "The walls are breathing."},
			{"Let's not stay long.", "You feel watched too?", "Who knocked?", "Hold my hand.", "Leave the light on."}},
	}
)

// A ghost's lines (2026-11-14): heard only in the ghost dimension. Walking in, going out, coming back, and in
// its Room.
var (
	ghostHellos = []string{"Boo.", "Oooooh...", "Is it haunted? Good.", "I'll take the cold room.", "No mirrors, please.", "Checking in. Forever.", "Room with a boo, please.", "Don't mind the chill."}
	ghostByes   = []string{"Off to haunt the town.", "Back by midnight.", "Don't wait up. Ever."}
	ghostBacks  = []string{"Home, sweet tomb.", "Miss me?", "The town screamed nicely."}
	ghostCalm   = []string{"Lovely draft.", "So nicely gloomy.", "I miss my bones.", "Who left the light on?", "The living are so loud.", "Creak... creak...", "I never sleep.",
		"Rattle, rattle.", "This wallpaper died too.", "Who's haunting who?", "Cold spots, just right."}
)

// roomOnly: the lines that make sense only in a Room (2026-11-14: "GET ME OUT!" was heard in the hall). One is
// never shown by a Guest outside the Rooms: said inside, it ends at the door.
var roomOnly = map[string]bool{"Zzz...": true}

func init() {
	for _, tier := range calmLines {
		for _, set := range tier {
			for _, l := range set {
				roomOnly[l] = true
			}
		}
	}
	for _, set := range append(sightLines[:], ghostCalm) {
		for _, l := range set {
			roomOnly[l] = true
		}
	}
}

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
		p.talk = nil // a greeting replaces what it was saying
		switch hi := clockGreeting(g.clock); {
		case p.Realm == Ghost: // a ghost has its own lines, never the time of day
			set := ghostHellos
			switch {
			case p.state == goingOut:
				set = ghostByes
			case p.greet != greetHello:
				set = ghostBacks
			}
			p.talk = append(p.talk, line{0, set[g.chat.IntN(len(set))], g.tick})
		case p.greet == greetHello:
			p.talk = append(p.talk, line{0, helloLines[g.chat.IntN(len(helloLines))], g.tick})
		case p.greet == greetClean:
			p.talk = append(p.talk, line{0, "Clean at last!", g.tick})
			if len(p.Guests) > 1 {
				p.talk = append(p.talk, line{1, "Thank you!", g.tick + answerFrames})
			}
		case len(p.Guests) == 1:
			set := soloOut
			if p.greet == greetBack {
				set = soloBack
			}
			p.talk = append(p.talk, line{0, hi, g.tick}, line{0, set[g.chat.IntN(len(set))], g.tick + answerFrames})
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
// line by its Room's dirt ("Zzz..." asleep). A single never says a line meant for its mate. A ghost says one
// of its own, day or night: it never sleeps.
func (g *Game) updateRoomTalk() {
	asleep := asleep(g.clock)
	for _, p := range g.parties {
		if p.state != staying || p.asking() || len(p.talk) > 0 || g.chat.Float64() >= calmOdds {
			continue
		}
		who := g.chat.IntN(len(p.Guests))
		if p.Realm == Ghost {
			p.talk = append(p.talk, line{who, ghostCalm[g.chat.IntN(len(ghostCalm))], g.tick})
			continue
		}
		tier := min(2, int(3*g.dread(p.Room)))
		set := calmLines[tier]
		lines := set[0]
		switch {
		case asleep:
			lines = []string{"Zzz..."}
		case len(p.Guests) > 1:
			lines = slices.Concat(set[0], set[1])
		}
		p.talk = append(p.talk, line{who, lines[g.chat.IntN(len(lines))], g.tick})
	}
}

// shown: the guest is on the map.
func (g *Game) shown(guest *entities.Guest) bool { return slices.Contains(g.guests(), guest) }

// drawTalk draws, by its speaker's head, the one line each Party is saying, while the Concierge hears it:
// the last that has started (2026-11-14: a Guest never says two at once). A Party asking or complaining
// shows only its balloon.
func (g *Game) drawTalk(s *ebiten.Image, ox, oy int) {
	for _, p := range g.parties {
		if p.asking() || p.state == complaining {
			continue
		}
		if l, ok := p.saying(g.tick); ok {
			if guest := p.Guests[l.who]; g.shown(guest) && g.hears(guest) && (!roomOnly[l.text] || g.indoors(guest)) {
				g.say(s, l.text, guest, ox, oy)
			}
		}
	}
}

// saying: the Party's line now: of those begun, the last.
func (p *Party) saying(tick int) (line, bool) {
	var now line
	ok := false
	for _, l := range p.talk {
		if tick >= l.at && (!ok || l.at >= now.at) {
			now, ok = l, true
		}
	}
	return now, ok
}
