package game

import (
	"image"

	"shift/world"
)

// View is what a player sees, nothing more (D105): a Strategy decides from it and the Pilot steers by it.
type View struct {
	Title     bool // the title screen is up
	Spinning  bool // the Spin cinematic is on
	Clock     string
	Day       int
	Money     int
	Rooms     []RoomView    // in the Terminal's order
	Front     *Front        // the front Party, standing at the counter; nil if nobody is
	Guests    []image.Point // every guest's cell not hidden by the Fog
	Concierge image.Point   // world px
	Terminal  TerminalView
	Hotel     *world.Hotel // the map as drawn: walls, doors, anchors. Read it, never write it.
}

type RoomView struct {
	Number, Type, Beds string
	Vacant             bool
	Capacity, Rate     int
	Auto               bool // an Auto Room: its Door Panel is green, as the Terminal shows (04 D114)
}

// Front is the Party at the counter: its size is on screen; the rest is on the Terminal's card, so it is
// filled only while the Terminal is open.
type Front struct {
	Size    int
	Nights  int
	Routine string
	Wish    string // a pair's bed wish; "" solo or while the Terminal is shut
}

type TerminalView struct {
	Open    bool
	Cursor  int  // the selected Room
	OK      bool // the OK button is selected instead
	Pick    int  // the picked Room, -1 if none
	Message string
}

func (g *Game) view() View {
	v := View{
		Title: g.state == StateTitle, Spinning: g.spinning(),
		Clock: clockText(g.clock), Day: day(g.clock), Money: g.money,
		Concierge: image.Pt(int(g.player.X), int(g.player.Y)), Hotel: g.hotel,
		Terminal: TerminalView{g.state == StateTerminal, g.terminalCursor, g.terminalOK, g.terminalPick, g.message},
	}
	for i, r := range g.hotel.Rooms {
		v.Rooms = append(v.Rooms, RoomView{r.Number, r.Type.Name, r.Type.Beds, g.vacant(i), len(r.Stays), r.Type.Rate, g.autoRoom[i]})
	}
	if g.atCounter() {
		p := g.queue[0]
		v.Front = &Front{Size: len(p.Guests)}
		if v.Terminal.Open {
			v.Front.Nights, v.Front.Routine, v.Front.Wish = p.Nights, p.Routine.String(), p.Wish
		}
	}
	for _, guest := range g.guests() {
		if at := image.Pt(int(guest.X), int(guest.Y)); g.fogAt(at) < 1 {
			v.Guests = append(v.Guests, at.Div(world.TileSize))
		}
	}
	return v
}
