package game

import (
	"slices"

	"ghosthotel/entities"
)

// Realm is one of the hotel's two independent layers (07 "Ghost entry; realms as layers"): the humans' and
// the ghosts'. Actors of different realms never bump, yield, push or queue for each other; they share a
// Room only through its Mood.
type Realm int

const (
	Human Realm = iota
	Ghost
)

// realm: the layer the Concierge is in: Ghost while he is in the ghost dimension.
func (g *Game) realm() Realm {
	if g.ghost {
		return Ghost
	}
	return Human
}

// realmOf: the layer of a Party's member; couriers and Priests are Human.
func (g *Game) realmOf(guest *entities.Guest) Realm {
	for _, p := range g.parties {
		if slices.Contains(p.Guests, guest) {
			return p.Realm
		}
	}
	return Human
}

// actors: every actor on the map but the Concierge and the Shadows, who are handled apart: the Parties'
// members on the map, the couriers and the Priests. A slice that adds a kind of actor registers it here.
func (g *Game) actors() []*entities.Guest {
	gs := g.guests()
	for _, c := range g.couriers {
		gs = append(gs, &c.Guest)
	}
	for _, pr := range g.priests {
		gs = append(gs, &pr.Guest)
	}
	return gs
}
