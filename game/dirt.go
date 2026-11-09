package game

import (
	"image"
	"math/rand/v2"
	"slices"

	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// Housekeeping (D183, redesigned 2026-11-08): dirt is Room Spook made visible, one spot per spotSpook.
// Guests leave mess; each Room books its next Party when one leaves; a Party finding too much dirt waits
// in the hall, then walks away. Placeholders.
const (
	spotSpook      = 5    // Room Spook a dirt spot stands for
	harvestRate    = 0.5  // of a spot's Room Spook, banked as Spook when cleaned in ghost
	messyOdds      = 0.15 // a Guest is messy
	messyMess      = 5    // Room Spook a messy Guest leaves a night; the rest leave 1
	spiritOdds     = 0.35 // a new spot is spiritual: only ghost shows it, only a Priest clears it (Live test 5)
	dirtLimit      = 10   // Room Spook of normal stains a Party puts up with; more and it complains
	dirtComfort    = 10   // Comfort each Guest loses per spot of either kind it settles in with
	intrudeComfort = 1    // Comfort each Guest loses per game minute the Concierge spends in its Room with it
	patience       = 120  // game minutes a complaining Party waits in the hall
	bookGap        = 120  // game minutes from a Party leaving to the next one's arrival
	fleeGap        = 360  // the same, after a flee
	firstGap       = 30   // game minutes between the Rooms' first arrivals, at Run start
	wipeFrames     = 20   // a cleaned spot's fade
	stainGrowth    = 0.75 // a new spot grows an old stain, else starts a new one
)

// spot: one dirt stain, on a cell; spiritual ones show only in ghost and only a Priest clears them.
type spot struct {
	cell   image.Point
	spirit bool
}

// arrival: a Room's next Party, booked at clock from to arrive at clock at (0: none booked).
type arrival struct{ from, at int }

// wipe: a cleaned spot fading out.
type wipe struct {
	cell image.Point
	tick int
}

// book: the Room's next Party arrives gap game minutes from now.
func (g *Game) book(room, gap int) {
	g.arrivals[room] = arrival{g.clock, g.clock + gap*minuteFrames}
}

// firstArrivals: at Run start, the Rooms' Parties arrive firstGap minutes apart.
func (g *Game) firstArrivals() []arrival {
	a := make([]arrival, len(g.hotel.Rooms))
	for i := range a {
		a[i] = arrival{g.clock, g.clock + (i+1)*firstGap*minuteFrames}
	}
	return a
}

// mess: the Room Spook one Guest leaves a night, rolled at booking.
func (g *Game) mess() int {
	if g.rng.Float64() < messyOdds {
		return messyMess
	}
	return 1
}

// leaveMess: a leaving Party's mess, for the nights it stayed (a started night counts).
func (g *Game) leaveMess(p *Party) {
	nights := min(p.Nights, (g.clock-p.arrived)/dayFrames+1)
	g.roomSpook[p.Room] += p.mess * nights
}

// spots: the Room's dirt spots: one per spotSpook Room Spook, rounded up. A new spot takes a free floor
// cell picked from its own seed, so spots stay put, most often one touching an older spot, so the dirt
// grows in stains (2026-11-08), and is spiritual spiritOdds of the time; a cleaned one leaves at once, and
// Room Spook falling otherwise (a Priest) takes spiritual spots first, newest first.
func (g *Game) spots(room int) []spot {
	if len(g.dirt) != len(g.hotel.Rooms) {
		g.dirt, g.dirtN = make([][]spot, len(g.hotel.Rooms)), make([]int, len(g.hotel.Rooms))
	}
	want := (g.roomSpook[room] + spotSpook - 1) / spotSpook
	for len(g.dirt[room]) > want {
		k := len(g.dirt[room]) - 1
		for i := k; i >= 0; i-- {
			if g.dirt[room][i].spirit {
				k = i
				break
			}
		}
		g.dirt[room] = slices.Delete(g.dirt[room], k, k+1)
	}
	for len(g.dirt[room]) < want {
		free := g.floorCells(room)
		if len(free) == 0 {
			break
		}
		taken := func(c image.Point) bool {
			return slices.ContainsFunc(g.dirt[room], func(s spot) bool { return s.cell == c })
		}
		if clean := slices.DeleteFunc(slices.Clone(free), taken); len(clean) > 0 {
			free = clean
		}
		r := rand.New(rand.NewPCG(uint64(room)+1, uint64(g.dirtN[room])))
		near := slices.DeleteFunc(slices.Clone(free), func(c image.Point) bool {
			return !slices.ContainsFunc(g.dirt[room], func(s spot) bool { return abs(c.X-s.cell.X) <= 1 && abs(c.Y-s.cell.Y) <= 1 })
		})
		if len(near) > 0 && r.Float64() < stainGrowth {
			free = near
		}
		g.dirt[room], g.dirtN[room] = append(g.dirt[room], spot{free[r.IntN(len(free))], r.Float64() < spiritOdds}), g.dirtN[room]+1
	}
	return g.dirt[room]
}

// floorCells: the Room's bare floor, inside its shell: no wall, decor or prop drawn over it, its doorway
// left clear.
func (g *Game) floorCells(room int) []image.Point {
	r, m := g.hotel.Rooms[room], g.hotel.Map
	bare := func(x, y int) bool {
		for l := world.LayerWalls; l < world.NumLayers; l++ {
			if m.At(l, x, y) != 0 {
				return false
			}
		}
		return !m.Solid(x, y)
	}
	var cells []image.Point
	for y := r.Origin.Y; y < r.Origin.Y+world.RoomH; y++ {
		for x := r.Origin.X; x < r.Origin.X+world.RoomW; x++ {
			c := image.Pt(x, y).Mul(world.TileSize).Add(image.Pt(world.TileSize/2, world.TileSize/2))
			if cell := image.Pt(x, y); bare(x, y) && c.In(r.Shell()) && cell != r.Door && cell != r.Door.Add(image.Pt(0, 1)) {
				cells = append(cells, image.Pt(x, y))
			}
		}
	}
	return cells
}

// dirtShows: the spot shows in the current dimension: every one in ghost, the normal ones in real.
func (g *Game) dirtShows(s spot) bool { return g.ghost || !s.spirit }

// normalDirt: the Room Spook its normal stains carry, what its Guests see as dirt (Live test 5).
func (g *Game) normalDirt(room int) int {
	n := 0
	for _, s := range g.spots(room) {
		if !s.spirit {
			n += spotSpook
		}
	}
	return n
}

// cleanable: the Room and the normal spot's cell under the Concierge's feet, or -1.
func (g *Game) cleanable() (int, image.Point) {
	feet := g.feet()
	for i, r := range g.hotel.Rooms {
		if c := feet.Div(world.TileSize); feet.In(r.Shell()) && slices.Contains(g.spots(i), spot{c, false}) {
			return i, c
		}
	}
	return -1, image.Point{}
}

// clean wipes the normal spot at cell: the Room loses its spotSpook of Room Spook; in ghost, harvestRate of
// it is banked as Spook. Only with no Guest inside. False if refused.
func (g *Game) clean(room int, cell image.Point) bool {
	k := slices.Index(g.spots(room), spot{cell, false})
	if _, gs := g.inside(room); k < 0 || len(gs) > 0 {
		return false
	}
	g.dirt[room] = slices.Delete(g.dirt[room], k, k+1)
	cleaned := min(spotSpook, g.roomSpook[room])
	g.roomSpook[room] -= cleaned
	at := cell.Mul(world.TileSize).Add(image.Pt(world.TileSize/2, world.TileSize/2))
	if g.ghost {
		g.earn(Spook, Num(cleaned)*harvestRate, at)
	}
	g.tapped(room, TaskHousekeep)
	g.wiped = append(g.wiped, wipe{cell, g.tick})
	return true
}

// settle: a Party reaching its Stay Spots for the first time looks at the dirt it sees: over dirtLimit
// and it walks to the hall to complain (true); else each Guest loses dirtComfort per spot of either kind.
func (g *Game) settle(p *Party) bool {
	if g.normalDirt(p.Room) > dirtLimit {
		p.state, p.complainAt = complaining, g.clock
		g.sendParty(p, g.hall(p.Room)...)
		return true
	}
	for _, guest := range p.Guests {
		guest.Comfort = max(-comfortMax, guest.Comfort-dirtComfort*len(g.spots(p.Room)))
	}
	return false
}

// complain runs a frame of a Party waiting in the hall: back to its Room once the dirt it sees is down to
// dirtLimit; gone, unpaid, after patience.
func (g *Game) complain(p *Party) {
	g.walkParty(p, arriveSpeed)
	switch {
	case g.normalDirt(p.Room) <= dirtLimit:
		g.checkIn(p)
		p.greet = greetClean
	case g.clock-p.complainAt >= patience*minuteFrames:
		p.Walked, p.state = true, leaving
		g.sendParty(p, g.hotel.Exit, g.hotel.Exit)
		g.book(p.Room, bookGap)
	}
}

// hall: where the Room's complaining Party waits, side by side in the Lobby's bottom-left corner, off the
// walk from the Entrance to the Archway: each Room its own pair of cells, a free column between Rooms, 2 Rooms a row, upward.
func (g *Game) hall(room int) []image.Point {
	const ts = world.TileSize
	m, y := g.hotel.Map, g.hotel.Lobby.Max.Y/ts-1-room/2
	x := g.hotel.Lobby.Min.X / ts
	for m.Solid(x, y) {
		x++
	}
	x += 1 + 3*(room%2)
	mid := image.Pt(ts/2, ts/2)
	return []image.Point{image.Pt(x, y).Mul(ts).Add(mid), image.Pt(x+1, y).Mul(ts).Add(mid)}
}

// waiting: a Guest of a Party complaining in the hall stands at world px (x,y).
func (g *Game) waiting(x, y float64) bool {
	for _, p := range g.parties {
		for _, o := range p.Guests {
			if p.state == complaining && len(o.Path) == 0 && o.X == x && o.Y == y {
				return true
			}
		}
	}
	return false
}

// splatRows: the dirt balloon's splat.
var splatRows = []string{
	"..X..X.",
	".XXXX..",
	"XXXXXX.",
	".XXXXXX",
	"..XXXX.",
	".X..XX.",
	"......X",
}

const stainW = 48 // a stain's bitmap, world px: 3 cells, so a few spots cover most of the floor

// stainImage: a spot's stain, a blob of pixel discs seeded by its cell, white for tinting, clipped to the
// cells bare says show their floor (a side wall's cell does, up to its line, which is drawn over it).
func stainImage(cell image.Point, bare func(image.Point) bool) *ebiten.Image {
	const ts = world.TileSize
	r := rand.New(rand.NewPCG(uint64(cell.X)+7, uint64(cell.Y)+11))
	img := ebiten.NewImage(stainW, stainW)
	o := cell.Mul(ts).Add(image.Pt(ts/2-stainW/2, ts/2-stainW/2)) // the bitmap's world origin
	for range 6 + r.IntN(4) {
		cx, cy, rad := stainW/2+r.IntN(25)-12, stainW/2+r.IntN(25)-12, 3+r.IntN(5)
		for y := cy - rad; y <= cy+rad; y++ {
			for x := cx - rad; x <= cx+rad; x++ {
				if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= rad*rad && bare(o.Add(image.Pt(x, y)).Div(ts)) {
					img.Set(x, y, opaque(0xffffff))
				}
			}
		}
	}
	return img
}

// drawDirt draws each Room's spots as stains on its floor, and the cleaned ones fading: in real, faint
// grime, 1 spot in realShows; in ghost, every one, green and purple ectoplasm.
func (g *Game) drawDirt(s *ebiten.Image, ox, oy int) {
	if g.stains == nil {
		g.stains = map[image.Point]*ebiten.Image{}
	}
	const ts = world.TileSize
	draw := func(cell image.Point, spirit bool, alpha, grow float64) {
		if !g.dirtShows(spot{cell, spirit}) {
			return
		}
		if g.stains[cell] == nil {
			m := g.hotel.Map
			g.stains[cell] = stainImage(cell, func(c image.Point) bool { // no wall tile over it; furniture is drawn over it, rugs under
				return m.At(world.LayerWalls, c.X, c.Y) == 0
			})
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(-stainW/2, -stainW/2)
		op.GeoM.Scale(grow, grow)
		op.GeoM.Translate(float64(cell.X*ts+ts/2), float64(cell.Y*ts+ts/2))
		op.GeoM.Scale(zoom, zoom)
		op.GeoM.Translate(float64(-ox), float64(-oy))
		c, a := [3]float32{0.3, 0.22, 0.15}, 0.45 // grime
		if g.ghost {
			c, a = [3]float32{0.37, 0.83, 0.37}, 0.6 // ectoplasm: green, purple where the stain's cell is odd
			if (cell.X+cell.Y)&1 == 1 {
				c = [3]float32{0.63, 0.37, 0.83}
			}
		}
		f := float32(alpha * a * (1 - g.fogAt(cell.Mul(ts).Add(image.Pt(ts/2, ts/2))))) // hidden in the Fog, as its Guests
		op.ColorScale.Scale(c[0]*f, c[1]*f, c[2]*f, f)
		s.DrawImage(g.stains[cell], op)
	}
	for i := range g.hotel.Rooms {
		for _, s := range g.spots(i) {
			draw(s.cell, s.spirit, 1, 1)
		}
	}
	kept := g.wiped[:0]
	for _, w := range g.wiped {
		if d := g.tick - w.tick; d < wipeFrames {
			f := float64(d) / wipeFrames
			draw(w.cell, false, 1-f, 1+f)
			kept = append(kept, w)
		}
	}
	g.wiped = kept
}

// nextGuest: the left column's notice while a vacant, open Room has a Party booked: "Next guest
// tomorrow" when the soonest arrives on a later day, else "Guest arriving"; "" once it walks in (Live
// test 3: a short line, no countdown).
func (g *Game) nextGuest() string {
	next := -1
	for i, a := range g.arrivals {
		if a.at > 0 && g.vacant(i) && !g.closed(i) && (next < 0 || a.at < next) {
			next = a.at
		}
	}
	switch {
	case next < 0 || g.state == StateTitle:
		return ""
	case day(next) > day(g.clock):
		return "Next guest tomorrow"
	}
	return "Guest arriving"
}

// dirtBalloonImage: the request balloon with a splat inside instead of "!!": a complaint about dirt.
func dirtBalloonImage() *ebiten.Image {
	const ox, oy = (balloonW - 7*2) / 2, (balloonH - 7*2) / 2 // the splat at 2×, centered
	return paint(balloonW, balloonH, rounded(balloonW, balloonH, balloonR), func(x, y int) bool {
		sx, sy := (x-ox)/2, (y-oy)/2
		return x >= ox && y >= oy && sy < len(splatRows) && sx < len(splatRows[0]) && splatRows[sy][sx] == 'X'
	})
}

// DevDirt is main's -dirt: every Room starts at n Room Spook, n/spotSpook spots of dirt.
func (g *Game) DevDirt(n int) {
	for i := range g.roomSpook {
		g.roomSpook[i] = n
	}
}
