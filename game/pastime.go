package game

import (
	"image"
	"slices"

	"ghosthotel/entities"
)

// Guest activities (D168). Awake in its Room, a Guest uses a prop, rests on its bed, idles at its Stay Spot
// glancing around, or chats face to face with its mate.
const (
	actMin, actMax       = 10 * minuteFrames, 40 * minuteFrames // how long it stays at one (placeholder)
	glanceMin, glanceMax = 2 * minuteFrames, 6 * minuteFrames   // between glances while idle (placeholder)
	idleOdds             = 0.4                                  // after an activity, idle rather than pick another (placeholder)
	chatOdds             = 0.2                                  // a pair's pick is a chat (placeholder)
)

// act: what a Guest is doing. It walks to `at`, then faces `face` (or glances around, idle) for `dur`.
type act struct {
	at          image.Point
	face        entities.Facing
	idle        bool
	dur         int
	until, next int // the clock it's done; idle, its next glance
}

// pastime runs one frame of a staying Party's day.
func (g *Game) pastime(p *Party) {
	if len(p.acts) != len(p.Guests) {
		p.acts = make([]act, len(p.Guests))
	}
	for i, guest := range p.Guests {
		a := &p.acts[i]
		switch {
		case len(guest.Path) > 0:
			if g.walk(guest, arriveSpeed) {
				a.until = g.clock + a.dur
			}
		case g.clock >= a.until:
			g.pick(p, i)
		case !a.idle:
			guest.Anim.Facing = a.face
		case g.clock >= a.next:
			guest.Anim.Facing, a.next = entities.Facing(g.chat.IntN(4)), g.clock+glanceMin+g.chat.IntN(glanceMax-glanceMin)
		}
	}
}

// pick starts Guest i's next activity: after one, sometimes idling at its Stay Spot; else a chat (a pair),
// its bed, or a Spot its mate isn't using.
func (g *Game) pick(p *Party, i int) {
	r, guest := g.hotel.Rooms[p.Room], p.Guests[i]
	dur := actMin + g.chat.IntN(actMax-actMin+1)
	switch {
	case !p.acts[i].idle && g.chat.Float64() < idleOdds:
		g.idle(p, i)
	case len(p.Guests) == 2 && g.chat.Float64() < chatOdds: // both to their Stay Spots, facing each other
		for j, mate := range p.Guests {
			d := r.Stays[1-j].Sub(r.Stays[j])
			p.acts[j] = act{at: r.Stays[j], face: entities.Toward(float64(d.X), float64(d.Y)), dur: dur}
			mate.SetPath(g.route(mate, r.Stays[j]))
		}
	default:
		var spots []int
		for s, spot := range r.Spots {
			if !slices.ContainsFunc(p.acts, func(o act) bool { return o.at == spot.At }) {
				spots = append(spots, s)
			}
		}
		if n := g.chat.IntN(len(spots) + 1); n < len(spots) {
			s := r.Spots[spots[n]]
			d := s.Prop.Sub(s.At)
			p.acts[i] = act{at: s.At, face: entities.Toward(float64(d.X), float64(d.Y)), dur: dur}
			guest.SetPath(g.route(guest, s.At))
		} else if p.Realm == Ghost { // a ghost has no use for a bed
			g.idle(p, i)
		} else { // rest on its bed
			p.acts[i] = act{at: sleepSpot(r.Lanes[i]), face: entities.FaceDown, dur: dur}
			guest.SetPath(g.toBed(guest, r, i))
		}
	}
}

// idle sends Guest i to its Stay Spot to stand glancing around for a while.
func (g *Game) idle(p *Party, i int) {
	if len(p.acts) != len(p.Guests) {
		p.acts = make([]act, len(p.Guests))
	}
	at := g.stayOf(p, i) // the Stay Spot (a mirror's ghost has its own, ghostguest.go)
	p.acts[i] = act{at: at, idle: true, dur: actMin + g.chat.IntN(actMax-actMin+1)}
	p.Guests[i].SetPath(g.route(p.Guests[i], at))
}
