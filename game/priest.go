package game

import (
	"fmt"
	"image"
	"math"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// Room Mood (D147) and the Priests who clean it (D151). Placeholders, retuned at T112.
const (
	priestWeak    = 40  // $: halves a Room's dread
	priestStrong  = 100 // $: clears it
	priestMinutes = 30  // game minutes a Priest works in the Room
	priestSpeed   = arriveSpeed
)

// closed: the Room's Mood is at the floor; it takes no new Party (D140), its own stays until checkout.
func (g *Game) closed(room int) bool {
	return g.roomMood(room) <= moodMin
}

// priest walks from the Street into his Room, works priestMinutes, lowers its dread and walks out.
type priest struct {
	entities.Guest // RoomIndex is his Room: its door lets him through, Occupied or not
	strong         bool
	until          int  // the clock his work ends; 0 until he's in the Room
	done           bool // worked: walking out
}

func (pr *priest) price() int {
	if pr.strong {
		return priestStrong
	}
	return priestWeak
}

// callPriest pays for a Priest to the Room and sends him in from the Street; else why not.
func (g *Game) callPriest(room int, strong bool) string {
	price := priestWeak // D166: Father K is the Weak Priest, Father M the Strong one
	if strong {
		price = priestStrong
	}
	if g.priestOn(room) {
		return "A PRIEST IS ALREADY ON ROOM " + g.hotel.Rooms[room].Number
	}
	if !g.pay(Cost{Money: Num(price)}) {
		return fmt.Sprintf("NOT ENOUGH MONEY: $%d", price)
	}
	g.sendPriest(room, strong)
	return ""
}

// sendPriest: a paid-for Priest walks in from the Entrance to the Room.
func (g *Game) sendPriest(room int, strong bool) {
	name := fatherK.name
	if strong {
		name = fatherM.name
	}
	pr := &priest{Guest: entities.NewGuest(g.hotel.Spawn, name), strong: strong}
	pr.RoomIndex = room
	pr.SetPath(g.hotel.Path(g.hotel.Spawn, g.workSpot(room), room))
	g.priests = append(g.priests, pr)
}

// priestOn: a Priest is on his way to the Room or in it.
func (g *Game) priestOn(room int) bool {
	return slices.ContainsFunc(g.priests, func(pr *priest) bool { return pr.RoomIndex == room && !pr.done })
}

// workSpot: the Room's cell just inside its door.
func (g *Game) workSpot(room int) image.Point {
	return g.hotel.Rooms[room].Door.Add(image.Pt(0, -1)).Mul(world.TileSize).Add(image.Pt(world.TileSize/2, world.TileSize/2))
}

func (g *Game) updatePriests() {
	kept := g.priests[:0]
	for _, pr := range g.priests {
		arrived := g.walk(&pr.Guest, priestSpeed)
		switch {
		case !arrived:
		case pr.done: // at the Exit
			continue
		case pr.until == 0:
			pr.until = g.clock + priestMinutes*minuteFrames
		case g.clock >= pr.until:
			if g.halveDirt(pr.RoomIndex); pr.strong {
				g.raiseMood(pr.RoomIndex, moodClean)
				g.shades[pr.RoomIndex] = nil // Father M also banishes a Shadow (D169)
			}
			pr.done = true
			pr.SetPath(g.hotel.Path(image.Pt(int(pr.X), int(pr.Y)), g.hotel.Exit, pr.RoomIndex))
		}
		kept = append(kept, pr)
	}
	clear(g.priests[len(kept):])
	g.priests = kept
}

// halveDirt: a Priest's work, half the Room's dirt left.
func (g *Game) halveDirt(room int) { g.raiseMood(room, moodOf(g.dirtLevel(room)/2)) }

// dreadAt is the dread shift of world cell c (D162): its Room's dread, 0 outside the
// Rooms. In ghost a Closed Room pulses between 0.6 and 1.
func (g *Game) dreadAt(c image.Point) float32 {
	for i, r := range g.hotel.Rooms {
		if c.In(image.Rectangle{r.Origin, r.Origin.Add(image.Pt(world.RoomW, world.RoomH))}) {
			return g.roomShift(i)
		}
	}
	return 0
}

func (g *Game) roomShift(room int) float32 {
	if room < 0 || room >= len(g.mood) {
		return 0
	}
	s := g.dread(room)
	if g.rotting(room) {
		s = 1 // T191: a rotting Room shows full
	}
	if g.ghost && g.closed(room) {
		s = 0.8 + 0.2*math.Cos(2*math.Pi*float64(g.tick)/(1.5*tps)) // a slow pulse, 1.5 s
	}
	return float32(s)
}

// drawMoodFloor redraws each dreadful Room's floor over the hotel's, shifted by its dread.
func (g *Game) drawMoodFloor(screen *ebiten.Image, ox, oy int) {
	const ts = world.TileSize
	for i, r := range g.hotel.Rooms {
		if s := g.roomShift(i); s > 0 {
			o := r.Origin.Mul(ts)
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(o.X), float64(o.Y))
			op.GeoM.Scale(zoom, zoom)
			op.GeoM.Translate(float64(-ox), float64(-oy))
			sub := g.floorImg.SubImage(image.Rectangle{o, o.Add(image.Pt(world.RoomW*ts, world.RoomH*ts))}).(*ebiten.Image)
			g.blit(screen, sub, op, modeMaze, floorBlue, nil, s)
		}
	}
}
