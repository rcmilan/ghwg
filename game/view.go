package game

import (
	"image"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// View is what a player sees, nothing more (D105): a Strategy decides from it and the Pilot steers by it.
type View struct {
	Title     bool // the title screen is up
	Ghost     bool // the hotel is in the ghost dimension
	Clock     string
	Day       int
	Money     int
	Spook     int
	Rooms     []RoomView    // in the Terminal's order
	Guests    []image.Point // every guest's cell not hidden by the Fog (none is, in the ghost dimension)
	Concierge image.Point   // world px
	Terminal  TerminalView
	Hotel     *world.Hotel // the map as drawn: walls, doors, anchors. Read it, never write it.
}

type RoomView struct {
	Number, Type, Beds string
	Vacant             bool
	Capacity, Rate     int
	Closed             bool // at the Spook limit: no new Party (D147)
	Priest             bool // one is on his way or at work
	Spook              int  // its Room Spook, read off its colors (D162)
	Away               bool // Occupied, but its Party isn't inside: the player saw it walk out
	Asking             bool // its Guests want service: the "!!" balloon, seen from anywhere (D163)
	// Seen only in the ghost dimension: its props shake, and its Guests glow with Fear.
	Haunted bool
	Fear    int // its most frightened Guest in sight
}

type TerminalView struct {
	Open    bool
	Message string
	Cursor  int  // the selected Room
	Menu    int  // the Room whose Priest menu is open, or -1
	Strong  bool // the menu has the Strong Priest selected
}

func (g *Game) view() View {
	v := View{
		Title: g.state == StateTitle, Ghost: g.ghost,
		Clock: clockText(g.clock), Day: day(g.clock), Money: g.money, Spook: g.spook,
		Concierge: image.Pt(int(g.player.X), int(g.player.Y)), Hotel: g.hotel,
		Terminal: TerminalView{g.state == StateTerminal, g.message, g.termCursor, g.termRoom, g.termStrong},
	}
	for i, r := range g.hotel.Rooms {
		rv := RoomView{Number: r.Number, Type: r.Type.Name, Beds: r.Type.Beds, Vacant: g.vacant(i), Capacity: len(r.Stays), Rate: r.Type.Rate,
			Closed: g.closed(i), Priest: g.priestOn(i), Spook: g.roomSpook[i]}
		rv.Asking = slices.ContainsFunc(g.parties, func(p *Party) bool { return p.Occupies(i) && p.asking() })
		rv.Away = !rv.Vacant && !slices.ContainsFunc(g.guests(), func(guest *entities.Guest) bool {
			return guest.RoomIndex == i && image.Pt(int(guest.X), int(guest.Y)).In(r.Shell())
		})
		if g.ghost {
			rv.Haunted = g.haunting(i)
			for _, guest := range g.guests() {
				if guest.RoomIndex == i {
					rv.Fear = max(rv.Fear, guest.Fear)
				}
			}
		}
		v.Rooms = append(v.Rooms, rv)
	}
	for _, guest := range g.guests() {
		if at := image.Pt(int(guest.X), int(guest.Y)); g.fogAt(at) < 1 {
			v.Guests = append(v.Guests, at.Div(world.TileSize))
		}
	}
	return v
}
