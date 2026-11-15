package game

import (
	"fmt"
	"image"
	"math"
	"slices"
	"strings"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// Room Mood (D147) and the Priests who clean it (D151). Placeholders, retuned at T112.
const (
	priestWeak    = 200 // $: halves a Room's dread
	priestStrong  = 500 // $: clears it
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
	voice
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
	if why := g.noFather(room, strong); why != "" {
		return why
	}
	if !g.pay(Cost{Money: Num(price)}) {
		return fmt.Sprintf("NOT ENOUGH MONEY: $%d", price)
	}
	g.sendPriest(room, strong)
	return ""
}

// sendPriest: a paid-for Priest walks in from the Entrance to the Room.
func (g *Game) sendPriest(room int, strong bool) {
	pr := &priest{Guest: entities.NewGuest(g.hotel.Spawn, fatherName(strong)), strong: strong}
	pr.RoomIndex = room
	pr.SetPath(g.hotel.Path(g.hotel.Spawn, g.workSpot(room), room))
	g.priests = append(g.priests, pr)
}

func fatherName(strong bool) string {
	if strong {
		return fatherM.name
	}
	return fatherK.name
}

// priestOn: a Priest is on his way to the Room or in it.
func (g *Game) priestOn(room int) bool {
	return slices.ContainsFunc(g.priests, func(pr *priest) bool { return pr.RoomIndex == room && !pr.done })
}

// fatherIn: that Father (K, or M if strong) is anywhere in the hotel, walking in, working or leaving. One of each at
// a time (08 F0): noFather refuses the call with it, F0-R hides the call in the ring with it.
func (g *Game) fatherIn(strong bool) bool {
	return slices.ContainsFunc(g.priests, func(pr *priest) bool { return pr.strong == strong })
}

// noFather: why that Father can't be called to the Room, "" if he can: one Priest a Room, and each Father is one
// person, so not while he is anywhere in the hotel (08 F0).
func (g *Game) noFather(room int, strong bool) string {
	switch {
	case g.fatherIn(strong):
		return strings.ToUpper(fatherName(strong)) + " IS ALREADY IN THE HOTEL"
	case g.priestOn(room):
		return "A PRIEST IS ALREADY ON ROOM " + g.hotel.Rooms[room].Number
	}
	return ""
}

// workSpot: the Room's cell just inside its door.
func (g *Game) workSpot(room int) image.Point {
	return g.hotel.Rooms[room].Door.Add(image.Pt(0, -1)).Mul(world.TileSize).Add(image.Pt(world.TileSize/2, world.TileSize/2))
}

func (g *Game) updatePriests() {
	kept := g.priests[:0]
	for _, pr := range g.priests {
		arrived := g.walk(&pr.Guest, priestSpeed*g.staffSpeed())
		g.comeNear(&pr.Guest, &pr.voice, func() []string {
			if pr.done {
				return g.sometimes(priestOut)
			}
			return g.sometimes(priestIn)
		})
		switch {
		case !arrived:
		case pr.done: // at the Exit
			continue
		case pr.until == 0:
			pr.until = g.clock + priestMinutes*minuteFrames
		case g.clock >= pr.until:
			if g.halveDirt(pr.RoomIndex); pr.strong {
				g.raiseMood(pr.RoomIndex, moodClean)
				g.shades[pr.RoomIndex] = nil  // Father M also banishes a Shadow (D169)
				g.hushed[pr.RoomIndex] = true // and a ghost's, until that ghost leaves the Room (shadow.go)
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
		s = 0.8 + 0.2*math.Cos(2*math.Pi*g.now()/(1.5*tps)) // a slow pulse, 1.5 s
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

// DevFather is main's -father: Rooms 101 and 102 open with a Party settled in each, both Fathers on sale and Money
// for both calls, Father K (M if strong) already called to Room 101 and walking in, the Concierge in the Lobby
// Archway, where SPACE opens the hotel view: Room 102's ring hides that Father's call until he has walked out.
func (g *Game) DevFather(strong bool) {
	a := slices.IndexFunc(g.hotel.Rooms, func(r world.Room) bool { return r.Number == "101" })
	g.unlock(Mirror)
	m := shop[slices.IndexFunc(shop, func(it Item) bool { return it.ID == rowFatherM })]
	g.earned[Money] = max(g.earned[Money], g.boughtAt[Mirror][Money]+m.Earned[Money]) // K needs less
	for _, room := range []int{a, (a + 1) % len(g.hotel.Rooms)} {
		g.openRoom(room)
		p := g.arrive(room)
		p.Routine = homebody
		p.schedule(g.clock)
		g.settleIn(p)
	}
	g.wallet[Money] = max(g.wallet[Money], priestWeak+priestStrong)
	g.callPriest(a, strong)
	g.player.X, g.player.Y = float64(g.hotel.Archway.X), float64(g.hotel.Archway.Y)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}
