package game

import (
	"image"
	"math"
	"math/rand/v2"
	"slices"

	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// Housekeeping (D183, redesigned 2026-11-08): dirt is dirt made visible, the n-th spot standing for
// spotUnit up to the bigFrom-th, then each one more (fewer, bigger stains as it grows, E6 close-out).
// Guests leave mess; each Room books its next Party when one leaves; a Party finding too much dirt waits
// in the hall, then walks away. Placeholders.
const (
	spotUnit    = 5    // dirt a Room's first bigFrom dirt spots each stand for
	bigFrom     = 4    // spots past this many grow: the next stands for 2 × spotUnit, then 3 ×, …
	harvestRate = 0.5  // of a spot's dirt, banked as Ecto when cleaned in ghost
	messyOdds   = 0.15 // a Guest is messy
	messyMess   = 5    // dirt a messy Guest leaves a night; the rest leave 1
	dirtLimit   = 10   // dirt of normal stains a Party puts up with; more and it complains
	patience    = 120  // game minutes a complaining Party waits in the hall
	bookGap     = 120  // game minutes from a Party leaving to the next one's arrival
	fleeGap     = 360  // the same, after a flee
	firstGap    = 30   // game minutes between the Rooms' first arrivals, at Run start
	wipeFrames  = 20   // a cleaned spot's fade
	stainGrowth = 0.75 // a new spot grows an old stain, else starts a new one
)

// spot: one dirt stain, on a cell, standing for size dirt. A spiritual one is a ghost's doing (a haunt, a
// fright: mood.go's ghostDirt): it shows only in ghost and is cleaned only there, for Ecto, or by a Priest
// (2026-11-14). A normal one is cleaned only in real.
type spot struct {
	cell   image.Point
	spirit bool
	size   int
}

// arrival: a Room's next Party, booked at clock from to arrive at clock at (0: none booked).
type arrival struct{ from, at int }

// wipe: a cleaned spot fading out.
type wipe struct {
	spot
	tick int
}

// book: the Room's next Party arrives gap game minutes from now.
func (g *Game) book(room, gap int) {
	if g.busy {
		gap = busyGap
	}
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

// mess: the dirt one Guest leaves a night, rolled at booking.
func (g *Game) mess() int {
	if g.rng.Float64() < messyOdds {
		return messyMess
	}
	return 1
}

// leaveMess: a leaving Party's mess, for the nights it stayed (a started night counts).
func (g *Game) leaveMess(p *Party) {
	nights := min(p.Nights, (g.clock-p.arrived)/dayFrames+1)
	g.addDirtLevel(p.Room, p.mess*nights)
}

// spots: the Room's dirt spots, enough to cover its dirt, past bigFrom each a spotUnit bigger (a
// Room at 200 has 12, not 40: E6 close-out, 2026-11-08). A new spot takes a free floor
// cell picked from its own seed, so spots stay put, most often one touching an older spot, so the dirt
// grows in stains (2026-11-08), and is spiritual when a ghost made it (g.byGhost); a cleaned one leaves at
// once, and dirt falling otherwise takes the normal spots first, newest first: only a clean in ghost or a
// Priest takes the spiritual (08 B1 live test 4: a serve wiped them).
func (g *Game) spots(room int) []spot {
	if len(g.dirt) != len(g.hotel.Rooms) {
		g.dirt, g.dirtN = make([][]spot, len(g.hotel.Rooms)), make([]int, len(g.hotel.Rooms))
	}
	total := func() (n int) {
		for _, s := range g.dirt[room] {
			n += s.size
		}
		return n
	}
	for len(g.dirt[room]) > 0 {
		k := len(g.dirt[room]) - 1
		for i := k; i >= 0; i-- {
			if !g.dirt[room][i].spirit {
				k = i
				break
			}
		}
		if total()-g.dirt[room][k].size < g.dirtLevel(room) {
			break
		}
		g.dirt[room] = slices.Delete(g.dirt[room], k, k+1)
	}
	for total() < g.dirtLevel(room) {
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
		g.dirt[room], g.dirtN[room] = append(g.dirt[room], spot{free[r.IntN(len(free))], g.byGhost, spotUnit * max(1, len(g.dirt[room])+2-bigFrom)}), g.dirtN[room]+1
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

// normalDirt: the dirt its normal stains carry, what its Guests see as dirt (Live test 5).
func (g *Game) normalDirt(room int) int {
	n := 0
	for _, s := range g.spots(room) {
		if !s.spirit {
			n += s.size
		}
	}
	return n
}

// cleanable: the Room the Concierge stands in and the cell of its nearest spot this Dimension cleans (normal
// in real, spiritual in ghost), or -1 with none. Anywhere in the Room will do (2026-11-14: SPACE beside a
// stain opened the hotel view instead; a stain has no hitbox to miss).
func (g *Game) cleanable() (int, image.Point) {
	feet := g.feet()
	for i, r := range g.hotel.Rooms {
		if !feet.In(r.Shell()) {
			continue
		}
		best, cell := math.MaxFloat64, image.Point{}
		for _, s := range g.spots(i) {
			c := s.cell.Mul(world.TileSize).Add(image.Pt(world.TileSize/2, world.TileSize/2))
			if d := math.Hypot(float64(feet.X-c.X), float64(feet.Y-c.Y)); s.spirit == g.ghost && d < best {
				best, cell = d, s.cell
			}
		}
		if best < math.MaxFloat64 {
			return i, cell
		}
	}
	return -1, image.Point{}
}

// clean wipes the Room's dirt of this Dimension's kind (the spot at cell is the one tapped): the normal in
// real, the spiritual in ghost, where harvestRate of it is banked as Ecto. Its Guests may be inside
// (2026-11-11). False if refused.
func (g *Game) clean(room int, cell image.Point) bool {
	if !slices.ContainsFunc(g.spots(room), func(s spot) bool { return s.cell == cell && s.spirit == g.ghost }) {
		return false
	}
	cleaned := g.scrub(room, g.ghost)
	if g.ghost {
		g.earn(Ecto, Num(cleaned)*harvestRate, cell.Mul(world.TileSize).Add(image.Pt(world.TileSize/2, world.TileSize/2)))
	}
	return true
}

// scrub: the Room's spots of one kind are cleaned, by the Concierge or a housekeeper (keeper.go): they fade
// and its Mood rises by what they weighed, never past moodClean. It returns the dirt cleaned.
func (g *Game) scrub(room int, spirit bool) int {
	level, left := g.dirtLevel(room), 0
	for _, s := range g.spots(room) {
		if s.spirit == spirit {
			g.wiped = append(g.wiped, wipe{s, g.tick})
		} else {
			left += s.size
		}
	}
	left = min(level, left)
	if left == level {
		return 0
	}
	g.dirt[room] = slices.DeleteFunc(g.dirt[room], func(s spot) bool { return s.spirit == spirit })
	g.setDirtLevel(room, left)
	g.tapped(room, TaskHousekeep)
	return level - left
}

// settle: a Party reaching its Stay Spots for the first time looks at the dirt it sees: over dirtLimit
// and it walks to the hall to complain (true).
func (g *Game) settle(p *Party) bool {
	if g.normalDirt(p.Room) > dirtLimit {
		p.state, p.complainAt = complaining, g.clock
		g.sendParty(p, g.hall(p.Room)...)
		return true
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
		g.logStay(p, outFled)
		g.sendParty(p, g.hotel.Exit, g.hotel.Exit)
		g.book(p.Room, g.bookGapNow())
	}
}

// hall: where the Room's complaining Party waits, side by side in the Lobby's bottom-left corner (the
// kitchen fills its east side, F1), off the
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

const (
	stainW     = 48 // the first spot's stain bitmap, world px: 3 cells
	stainScale = 3  // a stain grows with the root of its size, up to this many times stainW
)

// stainWidth: a spot's stain bitmap, world px: stainW × √(size/spotUnit), capped at stainScale.
func stainWidth(size int) int {
	return int(stainW * min(stainScale, math.Sqrt(float64(max(size, spotUnit))/spotUnit)))
}

// stainImage: a spot's stain, a blob of pixel discs seeded by its cell, as many and as spread as its
// width allows, white for tinting, clipped to the cells bare says show their floor (a side wall's cell
// does, up to its line, which is drawn over it).
func stainImage(cell image.Point, w int, bare func(image.Point) bool) *ebiten.Image {
	const ts = world.TileSize
	r := rand.New(rand.NewPCG(uint64(cell.X)+7, uint64(cell.Y)+11))
	img := image.NewRGBA(image.Rect(0, 0, w, w))        // drawn on the CPU and uploaded once: a pixel at a time, a growth in ghost redrawing every stain stalled ~0.5 s in the browser (08 W3)
	o := cell.Mul(ts).Add(image.Pt(ts/2-w/2, ts/2-w/2)) // the bitmap's world origin
	k := w * w / (stainW * stainW)                      // its area in first stains
	for range (6 + r.IntN(4)) * max(1, k) {
		cx, cy, rad := w/2+r.IntN(w/2+1)-w/4, w/2+r.IntN(w/2+1)-w/4, 3+r.IntN(5)
		for y := cy - rad; y <= cy+rad; y++ {
			for x := cx - rad; x <= cx+rad; x++ {
				if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= rad*rad && bare(o.Add(image.Pt(x, y)).Div(ts)) {
					img.Set(x, y, opaque(0xffffff))
				}
			}
		}
	}
	return ebiten.NewImageFromImage(img)
}

// drawDirt draws each Room's spots as stains on its floor, and the cleaned ones fading: in real, faint
// grime, 1 spot in realShows; in ghost, every one, green and purple ectoplasm.
func (g *Game) drawDirt(s *ebiten.Image, ox, oy int) {
	if g.stains == nil {
		g.stains = map[image.Point]*ebiten.Image{}
	}
	const ts = world.TileSize
	draw := func(sp spot, alpha, grow float64) {
		cell, w := sp.cell, stainWidth(sp.size)
		if !g.dirtShows(sp) {
			return
		}
		if im := g.stains[cell]; im == nil || im.Bounds().Dx() != w { // a new spot on the cell may be bigger
			m := g.hotel.Map
			g.stains[cell] = stainImage(cell, w, func(c image.Point) bool { // no wall tile over it nor a doorway (Live test 12); furniture is drawn over it, rugs under
				return m.At(world.LayerWalls, c.X, c.Y) == 0 && !slices.ContainsFunc(g.hotel.Rooms, func(r world.Room) bool {
					return c == r.Door || c == r.Door.Add(image.Pt(0, 1))
				})
			})
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(-w/2), float64(-w/2))
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
		if g.reeling(i) { // its floor is rolling
			continue
		}
		for _, s := range g.spots(i) {
			draw(s, 1, 1)
		}
	}
	kept, fade := g.wiped[:0], float64(g.cleanFrames(wipeFrames)) // the kit quickens the fade too; a housekeeper's work is keeperClean (keeper.go)
	for _, w := range g.wiped {
		if d := g.now() - float64(w.tick); d < fade {
			f := d / fade
			draw(w.spot, 1-f, 1+f)
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
		if a.at > 0 && g.isOwned(i) && g.vacant(i, Human) && !g.closed(i) && (next < 0 || a.at < next) {
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

// DevDirt is main's -dirt: every Room starts at n dirt (5 a spot).
func (g *Game) DevDirt(n int) {
	for i := range g.mood {
		g.setDirtLevel(i, n)
	}
}

// DevMood is main's -mood: every Room starts at Mood m, its spots of it, and the Concierge stands in Room 0.
func (g *Game) DevMood(m int) {
	for i := range g.mood {
		g.mood[i] = max(moodMin, min(moodMax, m))
	}
	r := g.hotel.Rooms[0].Origin // and the Concierge stands in Room 0, the camera on it
	g.player.X, g.player.Y = float64((r.X+world.RoomW/2)*world.TileSize), float64((r.Y+world.RoomH-3)*world.TileSize)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}
