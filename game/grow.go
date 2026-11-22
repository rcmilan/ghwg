package game

import (
	"image"
	"image/color"
	"slices"
	"strings"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// Growth (07 H1): mid-Run the hotel gains a column of Rooms on its east, or a floor at its back. The map is
// built again one bigger and everything follows its place: a Room keeps its type, its state and its guests,
// and the hall, which never grows, stays centered under the Rooms with all that goes on in it.
const (
	growCol    = 1  // Item.Grow: a Room column,
	growFloor  = 2  // a floor
	growEcto   = 60 // Ectoplasm: the first expansion; each next one adds as much again, times the Rooms owned (growPrice)
	growFrames = 60 // the new part's slide in, between the pull-back out and back in
)

// The store's rows for it. Not unlocks: their IDs only name them.
const (
	GrowCol   Unlock = "grow-col"
	GrowFloor Unlock = "grow-floor"
)

// growth is the cinematic under way: the camera pulls back on the hotel as it stood, the new part slides in
// from off screen, the camera comes back. The world waits; the map is already the new one.
type growth struct {
	col   bool
	t     float64     // the slide, 0..1
	d     image.Point // how far the hall moved, world px
	cut   int         // world px: the new part lies east of this x (a column) or north of this y (a floor)
	drawn string      // the hotel view's picture (g.wide) holds the hotel, pulled all the way back, in this dimension ("real", "ghost")
}

// grown: the hotel is larger than the Run began with.
func (g *Game) grown() bool {
	c, r := g.hotel.Grid()
	return g.grid0 != [2]int{} && [2]int{c, r} != g.grid0
}

// maxGrid: the hotel's limit at a prestige level, in Rooms a side (08 P1: each prestige lifts it by one).
func maxGrid(level int) int {
	if StoryMode {
		return max(3+level, storyGrowMax)
	}
	return 3 + level
}

func growCost(g *Game) Cost {
	c, r := g.hotel.Grid()
	return Cost{Ecto: growPrice(c, r, g.ownedCount())}
}

// growPrice: the next expansion's Ectoplasm for a cols x rows hotel with n Rooms owned: it doubles with each
// step and rises with the Rooms owned (roomPrice).
func growPrice(cols, rows, n int) Num {
	return roomPrice(Num(growEcto*(cols-2+rows)), n) // a step dearer each growth, and per Room owned (08 W3 sim: doubling walled level 2 at ~70 min)
}

// growRefuse: why the hotel can't grow that way now, "" if it can.
func (g *Game) growRefuse(kind int) string {
	c, r := g.hotel.Grid()
	switch {
	case kind == growCol && c >= maxGrid(g.level):
		return "THE HOTEL IS AS WIDE AS IT GETS"
	case kind == growFloor && r >= maxGrid(g.level):
		return "THE HOTEL IS AS TALL AS IT GETS"
	case g.growNext != 0 || slices.ContainsFunc(g.cart, func(l cartLine) bool { return shop[l.item].Grow != 0 }):
		return "ONE EXPANSION AT A TIME"
	case g.questGate() != "": // 08 W5: the beat's objectives first
		return g.questGate()
	}
	return ""
}

// beginGrow: the paid-for expansion happens, as the Terminal closes: the hotel is rebuilt and the cinematic plays.
func (g *Game) beginGrow() {
	col, w := g.growNext == growCol, g.hotel.Map.W
	g.growNext = 0
	d, err := g.grow(col)
	if err != nil {
		g.refuse(strings.ToUpper(err.Error()))
		return
	}
	g.growing, g.state = &growth{col: col, d: d, cut: d.Y + world.TileSize}, StateGrow
	g.refuse("")
	g.questEvent(evGrow, -1, "") // 08 W5: a beat starts
	if col {
		g.growing.cut = (w - 1) * world.TileSize // the old east margin: the new Corridor
	}
}

// updateGrow runs the cinematic: Update pulls the camera back while the slide is not done, and in after.
func (g *Game) updateGrow() {
	switch gr := g.growing; {
	case gr.t < 1 && g.viewT == 1:
		gr.t = min(1, gr.t+1.0/growFrames)
	case gr.t == 1 && g.viewT == 0:
		g.growing, g.state = nil, StatePlay
	}
}

// DevGrow is main's -grow: the hotel grows by each of "col" and "floor" in turn, the last one as bought, its
// cinematic playing from frame 0.
func (g *Game) DevGrow(kinds string) {
	for _, k := range strings.Split(kinds, ",") {
		if g.growing, g.state, g.viewT = nil, StatePlay, 0; k != "" {
			g.growNext = map[bool]int{true: growCol, false: growFloor}[k == "col"]
			g.beginGrow()
		}
	}
}

// ungrow, at a Run's start: the hotel is the size it opened with.
func (g *Game) ungrow() {
	g.growNext, g.growing = 0, nil
	if c, r := g.hotel.Grid(); [2]int{c, r} != g.grid0 && g.grid0 != [2]int{} {
		g.hotel, _ = world.Build(g.ts, g.types, g.grid0[0], g.grid0[1]) // it built once already
		g.spill = nil
		g.repilot()
	}
}

// repilot: a Strategy playing steers by the map as it stands now, its intents for the old one dropped.
func (g *Game) repilot() {
	if g.auto != nil {
		g.auto.pilot.hotel, g.auto.pilot.queue = g.hotel, nil
	}
}

// drawGrown draws the pulled-back world, g.wide, as the growth plays: a world px p shows at p*k+(tx,ty). The
// hotel shows as it stood, in pieces of the new map set where they were, and the new part slides in: a column
// from the east as the Rooms move over above the hall, which stays put; a floor from the north.
func (g *Game) drawGrown(screen *ebiten.Image, k, tx, ty float64, bg color.Color) {
	gr := g.growing
	mw, mh := g.mapSize()
	gap := 1 - gr.t*gr.t*(3-2*gr.t)
	piece := func(r image.Rectangle, x, y float64, a float64) { // world rect r, drawn with its corner at world (x,y)
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Scale(k/zoom, k/zoom)
		op.GeoM.Translate(tx+x*k, ty+y*k)
		op.ColorScale.ScaleAlpha(float32(a))
		screen.DrawImage(g.wide.SubImage(image.Rectangle{r.Min.Mul(zoom), r.Max.Mul(zoom)}).(*ebiten.Image), op)
	}
	const ts = world.TileSize
	if !gr.col {
		piece(image.Rect(0, gr.cut, mw, mh), 0, float64(gr.cut), 1)
		piece(image.Rect(0, 0, mw, ts), 0, float64(gr.cut-ts), 1) // the old north margin, until the floor covers it
		piece(image.Rect(0, 0, mw, gr.cut), 0, -gap*screenH/k, 1)
		return
	}
	hy, lobby := g.hotel.HallY, g.hotel.Lobby
	street := lobby.Max.Y + 2*ts
	over := gap * float64(gr.d.X)  // how far east of their place the old Rooms stand: over the hall, as they were
	east := float64(gr.cut) + over // their east end
	in := over + gap*screenW/k     // how far east of its place the new column stands
	side := lobby.Max.X + ts       // the hall's east end, its old margin included
	piece(image.Rect(0, hy, side, mh), 0, float64(hy), 1)
	// beside the hall, the street and the Rooms' south wall show only under the Rooms: west, as the old ones
	// spread over them; east, they come with the new column
	ebitenutil.DrawRect(screen, tx, ty+float64(hy)*k, over*k, float64(mh-hy)*k, bg)
	piece(image.Rect(side, hy, mw, mh), float64(side)+in, float64(hy), 1)
	margin := image.Rect(0, ts, ts, ts+street-hy) // the hotel's west margin, for the ones the hall had
	piece(margin, float64(lobby.Min.X-ts), float64(hy), gap)
	piece(margin, float64(lobby.Max.X), float64(hy), gap)
	piece(image.Rect(0, 0, gr.cut, hy), over, 0, 1)
	piece(image.Rect(0, 0, ts, hy), east, 0, 1) // the old east margin, until the column covers it
	piece(image.Rect(gr.cut, 0, mw, hy), float64(gr.cut)+in, 0, 1)
}

// regrid: a per-Room slice for a hotel of n Rooms, each old Room's value at its new index, fresh elsewhere.
func regrid[T any](s []T, n int, room func(int) int, fresh T) []T {
	if s == nil {
		return nil
	}
	out := make([]T, n)
	for i := range out {
		out[i] = fresh
	}
	for i, v := range s {
		out[room(i)] = v
	}
	return out
}

// grow rebuilds the hotel with one more Room column (col) or floor. The new Rooms arrive boarded up, each
// needing its key. It returns how far the hall moved, world px.
func (g *Game) grow(col bool) (image.Point, error) {
	old := g.hotel
	oc, or := old.Grid()
	nc, nr := oc, or+1
	if col {
		nc, nr = oc+1, or
	}
	h, err := world.Build(g.ts, g.types, nc, nr)
	if err != nil {
		return image.Point{}, err
	}
	h.SetCorridorMirrors(g.corridorMirrors(old, h)) // before the Rooms land again, which hang them back (08 B1)
	h.SetPantries(g.hasPantry())                    // the shelves on the new Corridors (08 P1-U)
	// The Rooms move down by a new floor's height; the hall and the street, with them and sideways, to stay centered.
	d := image.Pt(h.Lobby.Min.X-old.Lobby.Min.X, h.HallY-old.HallY)
	at := func(p image.Point) image.Point {
		if p.Y < old.HallY {
			return p.Add(image.Pt(0, d.Y))
		}
		return p.Add(d)
	}
	room := func(i int) int { // a Room keeps its place in the grid, so its index moves
		if i < 0 {
			return i
		}
		return (i/oc+nr-or)*nc + i%oc
	}
	n := nc * nr
	for i, r := range old.Rooms {
		h.Land(room(i), r.Type, !r.Mirror.Empty())
	}
	if g.roomOwned == nil { // a Run that began with every Room open: the new ones still need their key
		g.roomOwned = make([]bool, len(old.Rooms))
		for i := range g.roomOwned {
			g.roomOwned[i] = true
		}
	}
	g.roomOwned = regrid(g.roomOwned, n, room, false)
	g.mood, g.shades, g.taps = regrid(g.mood, n, room, moodStart), regrid(g.shades, n, room, nil), regrid(g.taps, n, room, [taskCount]Num{})
	g.fog, g.dirtN = regrid(g.fog, n, room, 0), regrid(g.dirtN, n, room, 0)
	g.arrivals, g.ghostArrivals, g.mirrorSlots = regrid(g.arrivals, n, room, arrival{}), regrid(g.ghostArrivals, n, room, arrival{}), regrid(g.mirrorSlots, n, room, arrival{})
	cell := image.Pt(0, d.Y/world.TileSize) // dirt lies in the Rooms
	for _, spots := range g.dirt {
		for i := range spots {
			spots[i].cell = spots[i].cell.Add(cell)
		}
	}
	for i := range g.wiped {
		g.wiped[i].cell = g.wiped[i].cell.Add(cell)
	}
	g.dirt = regrid(g.dirt, n, room, nil)
	stains := make(map[image.Point]*ebiten.Image, len(g.stains)) // they move with their Rooms, kept: drawn again, every one shows in ghost at once (08 W3)
	for c, im := range g.stains {
		stains[c.Add(cell)] = im
	}
	g.stains = stains
	hushed := map[int]bool{}
	for i, v := range g.hushed {
		hushed[room(i)] = v
	}
	g.hushed = hushed
	g.items.regrid(room) // 08 W5: Gerald and the Ward keep their Rooms
	for _, c := range g.changes {
		c.room = room(c.room)
	}
	lines := func(ls []cartLine) {
		for i := range ls {
			if shop[ls[i].item].room() {
				ls[i].room = room(ls[i].room)
			}
		}
	}
	lines(g.cart)
	g.termCursor = room(g.termCursor)

	// the Shadows stand on walls: each keeps its wall, found again where it stood
	walls := g.walls
	g.walls, g.wallCell = wallRuns(h.Map)
	g.onRun = nil
	reseat := func(sh *shade) {
		to := at(image.Pt(int(sh.x), int(walls[sh.run].y)))
		c := h.Map.Cell(to)
		if k := g.wallCell[c.Y*h.Map.W+c.X]; k >= 0 {
			sh.x += float64(to.X - int(sh.x))
			sh.run, sh.path, sh.goal, sh.fade = k, nil, sh.x, 0
		}
	}
	for _, ss := range g.shades {
		for _, sh := range ss {
			reseat(sh)
		}
	}
	for _, sh := range g.areaShades {
		reseat(sh)
	}

	g.hotel = h
	g.openPantries()
	move := func(x, y *float64) {
		p := at(image.Pt(int(*x), int(*y)))
		*x, *y = *x+float64(p.X-int(*x)), *y+float64(p.Y-int(*y))
	}
	move(&g.player.X, &g.player.Y)
	for i := range g.floaters {
		move(&g.floaters[i].x, &g.floaters[i].y)
	}
	for _, p := range g.parties {
		p.Room = room(p.Room)
		for i := range p.goals {
			p.goals[i] = at(p.goals[i])
		}
		for i := range p.acts {
			p.acts[i].at = at(p.acts[i].at)
		}
	}
	for _, c := range g.couriers {
		for i := range c.drops {
			c.drops[i].at = at(c.drops[i].at)
			lines(c.drops[i].lines)
		}
	}
	for _, k := range g.keepers { // a housekeeper's job is dropped: it finds it again from the kitchen
		if k.job != quitting {
			g.dropJob(k)
		}
	}
	everyone := g.actors() // and the Guests off the map: a Party out for the day comes back to its Room
	for _, p := range g.parties {
		for _, guest := range p.Guests {
			if !slices.Contains(everyone, guest) {
				everyone = append(everyone, guest)
			}
		}
	}
	if c := g.items.cat; c != nil { // the Cat too: left behind, it stood in another Room (live test 5: a locked one)
		everyone = append(everyone, &c.Guest)
		c.room = room(c.room)
	}
	for _, a := range everyone { // everyone walks on from where they stood, by a way of the new map
		move(&a.X, &a.Y)
		a.RoomIndex = room(a.RoomIndex)
	}
	for _, a := range everyone {
		if len(a.Path) > 0 {
			g.reroute(a, at(a.Path[len(a.Path)-1]))
		}
	}
	g.floorImg, g.spill = nil, nil                                // re-rendered on the next Draw
	g.cam.X, g.cam.Y = g.cam.X+float64(d.X), g.cam.Y+float64(d.Y) // the hall stays where it shows; the camera eases on from there
	g.repilot()
	return d, nil
}

// reroute: the guest's way to `to` on the map as it stands; none found, it walks straight there.
func (g *Game) reroute(a *entities.Guest, to image.Point) {
	path := g.hotel.Path(image.Pt(int(a.X), int(a.Y)), to, a.RoomIndex)
	if path == nil {
		path = []image.Point{to}
	}
	a.SetPath(path)
}
