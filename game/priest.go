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
	priestStrong = 40 // Ecto: a call of Father M, clears a Room (B1: calls cost Ecto, the unlocks 30 and 80)
	unlockKEcto  = 30 // the STAFF unlock that lets Father K be retained
	unlockMEcto  = 80
	retainerEcto = 150 // Ecto a game day: Father K stays on rounds; dear on purpose (08 B1 live test 4)

	priestMinutes = 30 // game minutes a Priest chants in the Room
	priestSpeed   = arriveSpeed
	priestStroll  = priestSpeed / 2 // his pace on the corridors with nothing to clean
	burstFrames   = 30              // the burst of light that lands the effect
	chantHop      = 24              // frames of one hop in the chant
)

// The Fathers' STAFF unlocks (one-time, Ecto); the retainer is a row of its own.
const (
	unlockK   Unlock = "call-father-k"
	unlockM   Unlock = "call-father-m"
	rowRetain Unlock = "retain-father-k"
)

// closed: the Room's Mood is at the floor; it takes no new Party (D140), its own stays until checkout.
func (g *Game) closed(room int) bool {
	return g.roomMood(room) <= moodMin
}

// A Priest's ritual (B1, 2026-11-15): walk to the Room's middle, an opening line, a chant with a hop while
// sparkles rise, a burst of light that lands the effect, a closing line, out. Father K on retainer then
// goes on to the next Room, or paces the corridors.
type phase int

const (
	rWalk  phase = iota // to the Room's middle
	rOpen               // his opening line
	rChant              // hops, sparkles rising, until the chant's time is up
	rBurst              // the burst of light; the effect lands at its end
	rClose              // his closing line
	rLeave              // to the Exit
	rPace               // retained, no spiritual dirt anywhere: a slow loop of the corridors
)

// priest works a Room: he walks in from the Street, performs the ritual and walks out.
type priest struct {
	entities.Guest // RoomIndex is his Room: its door lets him through, Occupied or not; -1 on the corridors
	strong         bool
	until          int  // the clock his chant ends; 0 until he starts it
	done           bool // the effect landed: closing, then on
	phase          phase
	t              float64 // frames in the phase
	retained       bool    // Father K on his daily retainer (B1)
	loop           bool    // dev -ritual: the dirt comes back after each ritual (ritual_dev.go)
	due            int     // the clock of his next payday
	lap            int     // the corridor stop he paces to
	voice
}

// price: the Money a Priest cost, none since B1 (calls are paid in Ecto).
func (pr *priest) price() int { return 0 }

// callPriest pays for Father M to the Room and sends him in from the Street; else why not. Father K is only ever
// retained (retainK), he has no call (08 W5 R11).
func (g *Game) callPriest(room int) string {
	if why := g.noFather(room, true); why != "" {
		return why
	}
	if !g.pay(Cost{Ecto: priestStrong}) {
		return fmt.Sprintf("NOT ENOUGH ECTOPLASM: %d", priestStrong)
	}
	g.sendPriest(room, true)
	return ""
}

// sendPriest: a paid-for Priest walks in from the Entrance to the Room.
func (g *Game) sendPriest(room int, strong bool) {
	pr := &priest{Guest: entities.NewGuest(g.hotel.Spawn, fatherName(strong)), strong: strong}
	g.priests = append(g.priests, pr)
	g.goRitual(pr, room)
}

// retainK: Father K comes on his retainer: in from the Entrance, his first payday a day away.
func (g *Game) retainK() {
	if g.fatherIn(false) {
		return
	}
	pr := &priest{Guest: entities.NewGuest(g.hotel.Spawn, fatherK.name), retained: true, phase: rPace, due: g.clock + dayFrames}
	pr.RoomIndex = -1
	g.priests = append(g.priests, pr)
}

// goRitual: the Priest walks to the Room's middle to work it.
func (g *Game) goRitual(pr *priest, room int) {
	pr.RoomIndex, pr.phase, pr.t, pr.until, pr.done = room, rWalk, 0, 0, false
	g.priestRoute(pr, g.ritualSpot(room), room)
}

// priestRoute: the Priest's way to a point, out of the Room he is in by its door first (one door is open to a
// path at a time, world/path.go); room is the one he goes into, -1 for none.
func (g *Game) priestRoute(pr *priest, to image.Point, room int) {
	from := image.Pt(int(pr.X), int(pr.Y))
	var path []image.Point
	if in := g.roomAt(pr.X, pr.Y); in >= 0 && in != room {
		out := g.doorFront(in)
		path = g.hotel.Path(from, out, in)
		from = out
	}
	pr.SetPath(append(path, g.hotel.Path(from, to, room)...))
}

// doorFront: the corridor cell in front of the Room's door, world px.
func (g *Game) doorFront(room int) image.Point {
	return g.hotel.Rooms[room].Door.Add(image.Pt(0, 2)).Mul(world.TileSize).Add(image.Pt(world.TileSize/2, world.TileSize/2))
}

func fatherName(strong bool) string {
	if strong {
		return fatherM.name
	}
	return fatherK.name
}

// priestOn: a Priest is on his way to the Room or in it.
func (g *Game) priestOn(room int) bool {
	return slices.ContainsFunc(g.priests, func(pr *priest) bool {
		return pr.RoomIndex == room && pr.phase != rLeave && pr.phase != rPace
	})
}

// fatherIn: that Father (K, or M if strong) is anywhere in the hotel, walking in, working or leaving. One of each at
// a time (08 F0): noFather refuses the call with it, F0-R hides the call in the ring with it.
func (g *Game) fatherIn(strong bool) bool {
	return slices.ContainsFunc(g.priests, func(pr *priest) bool { return pr.strong == strong }) || g.fatherGuest(strong)
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

// spiritDirt: the dirt of the Room's spiritual spots, what a retained Father K goes after.
func (g *Game) spiritDirt(room int) (n int) {
	for _, s := range g.spots(room) {
		if s.spirit {
			n += s.size
		}
	}
	return n
}

// worstRoom: the owned Room with the most spiritual dirt that no Priest is on, -1 if none has any.
func (g *Game) worstRoom() int {
	best, most := -1, 0
	for i := range g.hotel.Rooms {
		if d := g.spiritDirt(i); g.isOwned(i) && d > most && !g.priestOn(i) && !g.questRoom(i) {
			best, most = i, d
		}
	}
	return best
}

// ritualSpot: the bare floor cell nearest the Room's middle, world px.
func (g *Game) ritualSpot(room int) image.Point {
	r := g.hotel.Rooms[room]
	mid := r.Origin.Add(image.Pt(world.RoomW/2, world.RoomH/2))
	best, d := g.workSpot(room), math.MaxInt
	for _, c := range g.floorCells(room) {
		if e := (c.X-mid.X)*(c.X-mid.X) + (c.Y-mid.Y)*(c.Y-mid.Y); e < d {
			best, d = c.Mul(world.TileSize).Add(image.Pt(world.TileSize/2, world.TileSize/2)), e
		}
	}
	return best
}

// workSpot: the Room's cell just inside its door.
func (g *Game) workSpot(room int) image.Point {
	return g.hotel.Rooms[room].Door.Add(image.Pt(0, -1)).Mul(world.TileSize).Add(image.Pt(world.TileSize/2, world.TileSize/2))
}

func (g *Game) updatePriests() {
	g.updatePriestGuests()
	kept := g.priests[:0]
	for _, pr := range g.priests {
		if !g.updatePriest(pr) {
			kept = append(kept, pr)
		}
	}
	clear(g.priests[len(kept):])
	g.priests = kept
}

// priestSays: the Priest speaks one of the lines, always (a ritual's lines are not the staff's chat).
func (g *Game) priestSays(pr *priest, lines []string) {
	pr.said = line{0, g.pickLine(lines), g.tick}
}

// updatePriest runs a Priest a frame; true once he is out the Exit.
func (g *Game) updatePriest(pr *priest) bool {
	speed := priestSpeed
	if pr.phase == rPace {
		speed = priestStroll
	}
	arrived := g.walk(&pr.Guest, speed*g.staffSpeed())
	pr.t += g.dt
	if pr.retained && g.clock >= pr.due {
		pr.due += dayFrames
		if !g.payStaff(Cost{Ecto: retainerEcto}) { // short on payday: he leaves at once, his line only, no notice
			pr.retained, pr.phase, pr.done = false, rLeave, false
			g.priestSays(pr, fatherKUnpaid)
			g.priestRoute(pr, g.hotel.Exit, -1)
			return false
		}
	}
	switch pr.phase {
	case rWalk:
		if arrived {
			pr.phase, pr.t = rOpen, 0
			g.priestSays(pr, g.openers(pr))
		}
	case rOpen:
		if pr.t >= sayFrames {
			pr.phase, pr.t, pr.until = rChant, 0, g.clock+priestMinutes*minuteFrames
		}
	case rChant:
		if g.clock >= pr.until {
			pr.phase, pr.t = rBurst, 0
		}
	case rBurst:
		if pr.t >= burstFrames {
			g.ritualEffect(pr)
			if pr.loop {
				g.ritualDirt(pr.RoomIndex)
			}
			pr.phase, pr.t, pr.done = rClose, 0, true
			g.priestSays(pr, g.closers(pr))
		}
	case rClose:
		switch {
		case pr.t < sayFrames:
		case pr.retained:
			g.nextRound(pr)
		default:
			pr.phase = rLeave
			g.priestRoute(pr, g.hotel.Exit, -1)
		}
	case rPace:
		switch room := g.worstRoom(); {
		case room >= 0:
			g.goRitual(pr, room)
		case arrived:
			g.pace(pr)
		}
	case rLeave:
		return arrived
	}
	return false
}

func (g *Game) openers(pr *priest) []string {
	if l := g.verbOpener(pr); l != nil { // 08 W5: K's retirement party, the memory lines
		return l
	}
	if pr.strong {
		return mOpen
	}
	return kOpen
}

func (g *Game) closers(pr *priest) []string {
	if pr.strong {
		return mClose
	}
	return kClose
}

// ritualEffect: the burst lands. Father K halves the Room's dirt; Father M also clears its Mood, banishes a
// Shadow and sends every ghost in it over (seams_b1.go).
func (g *Game) ritualEffect(pr *priest) {
	room := pr.RoomIndex
	g.halveDirt(room)
	g.verbRitual(pr) // 08 W5
	if pr.strong {
		g.raiseMood(room, moodClean)
		g.shades[room] = nil  // Father M also banishes a Shadow (D169)
		g.hushed[room] = true // until its ghost is gone: sendOver has it passing on, but its Shadow would come back meanwhile
		g.sendOver(room)
	}
	if e := g.ritualEcto(pr); e > 0 { // Offering box (T374)
		g.earn(Ecto, e, image.Pt(int(pr.X), int(pr.Y)))
	}
}

// nextRound: the retained Father K, done in a Room, goes to the one with the most spiritual dirt, else paces.
func (g *Game) nextRound(pr *priest) {
	if room := g.worstRoom(); room >= 0 {
		g.goRitual(pr, room)
		return
	}
	pr.RoomIndex, pr.phase, pr.done = -1, rPace, false
	g.pace(pr)
}

// pace: the retained Father K walks on to the next Room's door along the corridors, round and round.
func (g *Game) pace(pr *priest) {
	n := len(g.hotel.Rooms)
	pr.lap = (pr.lap + 1) % n
	g.priestRoute(pr, g.doorFront(pr.lap), -1)
}

// halveDirt: a Priest's work, half the Room's dirt left.
func (g *Game) halveDirt(room int) { g.raiseMood(room, moodOf(g.dirtLevel(room)/2)) }

// dreadAt is the dread shift of world cell c (D162): its Room's dread, 0 outside the
// Rooms. In ghost a Closed Room pulses between 0.6 and 1.
func (g *Game) dreadAt(c image.Point) float32 { return g.roomShift(g.roomOfCell(c)) }

// roomOfCell: the Room whose RoomW x RoomH box holds world cell c, -1 if none.
func (g *Game) roomOfCell(c image.Point) int {
	for i, r := range g.hotel.Rooms {
		if c.In(image.Rectangle{r.Origin, r.Origin.Add(image.Pt(world.RoomW, world.RoomH))}) {
			return i
		}
	}
	return -1
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
			g.shade(screen, sub, op, modeMaze, floorBlue, nil, s)
		}
	}
}

// DevFather is main's -father: Rooms 101 and 102 open with a Party settled in each, both Fathers on sale and Ecto
// for one call and a bit, Father M (K if not strong) already sent to Room 101 and walking in, the Concierge in the Lobby
// Archway, where SPACE opens the hotel view: Room 102's ring hides that Father's call until he has walked out.
func (g *Game) DevFather(strong bool) {
	a := slices.IndexFunc(g.hotel.Rooms, func(r world.Room) bool { return r.Number == "101" })
	g.unlock(Mirror)
	g.unlock(unlockK)
	g.unlock(unlockM)
	m := shop[slices.IndexFunc(shop, func(it Item) bool { return it.ID == rowFatherM })]
	g.earned[Money] = max(g.earned[Money], g.boughtAt[Mirror][Money]+m.Earned[Money]) // K needs less
	for _, room := range []int{a, (a + 1) % len(g.hotel.Rooms)} {
		g.openRoom(room)
		p := g.arrive(room)
		p.Routine = homebody
		p.schedule(g.clock)
		g.settleIn(p)
	}
	g.wallet[Ecto] = max(g.wallet[Ecto], priestStrong-1) // one short of a call: its row shows red
	g.sendPriest(a, strong)
	g.player.X, g.player.Y = float64(g.hotel.Archway.X), float64(g.hotel.Archway.Y)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}
