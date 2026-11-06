package game

import (
	"image"
	"math"
	"math/rand/v2"
	"slices"

	"shift/assets"
	"shift/entities"
	"shift/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// The Spin cinematic plays on top of StatePlay (spinning).
const (
	StateTitle = iota
	StatePlay
	StateTerminal
)

// Tunables. Speeds in world px per frame, times in frames (Ebitengine runs Update at 60 TPS).
const (
	tps            = 60
	conciergeSpeed = 1.0 // 3 screen px at x3
	arriveSpeed    = 0.6
	departSpeed    = 0.75
	conciergeGive  = 0.8     // share of a push the Concierge takes against a moving guest (D22b),
	guestGive      = 0.2     // and the moving guest takes; guest vs guest splits evenly; idle guests don't budge
	stuckFrames    = 2 * tps // a walking guest with no progress this long ignores actors for one tile: the anti-softlock fallback (D22c, D89)
	reelFrames     = 60      // one Room's reel roll, ~1 s (D48)
	reelStagger    = 12      // between consecutive Rooms' reel starts, so they stop ~0.2 s apart
	spinCamFrames  = 45      // the camera's glide out to all Rooms before the reels, and back after
	spinHoldFrames = 30      // all-Rooms view held after the last reel stops
	spinMargin     = 16      // world px around the Rooms in the all-Rooms view
)

// guestCast: Guests draw a random character from these; repeats are fine (D25). Bob is the Concierge.
var guestCast = [...]string{"Adam", "Amelia", "Alex"}

type Game struct {
	state           int
	hotel           *world.Hotel
	types           []world.RoomType // every loaded Room Type, drawn from at each Spin
	ts              *world.Tileset
	cam             Camera
	player          entities.Player
	parties         []*Party // every Party on the map
	queue           []*Party // the Queue, front first (D78)
	terminalCursor  int      // the selected Room
	terminalOK      bool     // the Terminal's OK button is selected instead
	terminalPick    int      // the Room picked, -1 if none; booked on OK
	terminalParty   *Party   // the Party picked for it, among those at the counter (04 D119)
	terminalList    bool     // the focus is on the list of Parties at the counter
	tick            int      // simulation frames since launch
	clock           int      // game time (clock.go)
	fast            bool     // T held: the clock runs ×10
	money           int
	autoRoom        []bool // per Room: an Auto Room, flipped at its Door Panel (04 D113)
	walkIns         int    // Walk-ins this Run
	floaters        []floater
	fog             []float64 // per Room: its veil, 0 clear .. 1 fogged (fog.go)
	message         string
	rng             *rand.Rand
	in              input                       // the keys (input.go)
	headless        bool                        // no window: step skips what only drawing needs
	auto            *autopilot                  // a Strategy plays instead of a person (sim.go)
	tiles           []*ebiten.Image             // by world.TileID
	characters      map[string][2]*ebiten.Image // idle_anim, run sheets by name
	floorImg        *ebiten.Image               // the floor layer pre-rendered at 1px per world px
	props           []prop                      // every other static tile, by SortY
	reelAt          int                         // tick the last Spin started
	spinRooms       []int                       // the Rooms rolling in the last Spin, in Room order
	landed          int                         // of spinRooms, whose reel has stopped
	reels           [][2]*ebiten.Image          // per Room: old and new footprint while it rolls
	cinematic       bool                        // the last Spin glides the camera out to the Rooms
	spinEndAt       int                         // tick the cinematic's camera is back on the Concierge
	worldImg        *ebiten.Image               // the whole map ×zoom, for the Spin cinematic
	dark            *ebiten.Image               // the darkness layer, the size of what drawWorld draws onto (light.go)
	pool, glow, dot *ebiten.Image               // light sprites: a ceiling light's pool, the Terminal's glow, a Door Panel's light
}

// New builds the shipped grid (world.Cols x world.Rows), played on the keyboard.
func New() (*Game, error) {
	g, err := newWorld(world.Cols, world.Rows, rand.Uint64())
	if err != nil {
		return nil, err
	}
	g.in, g.headless = keyboard{}, false
	return g, g.loadImages()
}

// newWorld builds a cols x rows hotel with every random draw seeded from seed, and no images: a headless
// Run, fed keys by a script.
func newWorld(cols, rows int, seed uint64) (*Game, error) {
	ts, err := world.LoadTileset(assets.FS, "tiles/hotel.tsj")
	if err != nil {
		return nil, err
	}
	types, err := world.LoadRoomTypes(assets.FS, ts)
	if err != nil {
		return nil, err
	}
	rng := rand.New(rand.NewPCG(seed, seed))
	h, err := world.Build(ts, types, cols, rows, rng)
	if err != nil {
		return nil, err
	}
	return &Game{state: StateTitle, hotel: h, types: types, rng: rng, ts: ts, in: &script{}, headless: true,
		autoRoom: make([]bool, len(h.Rooms))}, nil
}

func (g *Game) loadImages() (err error) {
	if g.tiles, err = loadTiles(g.ts); err != nil {
		return err
	}
	g.characters, err = loadCharacters(append([]string{"Bob"}, guestCast[:]...)...)
	return err
}

func (g *Game) mapSize() (int, int) {
	return g.hotel.Map.W * world.TileSize, g.hotel.Map.H * world.TileSize
}

// startRun begins a Run (D72): Day 1 at 08:00, every Room Vacant, an instant Spin (D46), no money.
func (g *Game) startRun() {
	g.parties, g.queue, g.floaters, g.walkIns, g.fog = nil, nil, nil, 0, nil
	g.autoRoom = make([]bool, len(g.hotel.Rooms)) // every Room manual (D116)
	cols, rows := g.hotel.Grid()
	g.clock, g.money, g.terminalCursor = runStart, 0, (rows-1)*cols // the cursor starts on 101
	g.state = StatePlay
	g.spin(world.Draw(g.types, len(g.hotel.Rooms), g.rng), nil, true)
	g.player = entities.NewPlayer(g.hotel.ConciergeStart)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}

func (g *Game) Update() error {
	if g.auto != nil {
		g.auto.drive(g)
	}
	if g.in.JustPressed(ebiten.KeyEscape) {
		switch g.state {
		case StateTitle:
			return ebiten.Termination
		case StateTerminal:
			g.state, g.message = StatePlay, ""
		default:
			g.state = StateTitle
		}
		return nil
	}
	switch {
	case g.state == StateTitle:
		if g.in.JustPressed(ebiten.KeySpace) {
			g.startRun()
		}
	case g.state == StateTerminal: // the world and the Clock wait (D73)
		g.updateTerminal()
	case g.spinning(): // the camera is on the Rooms: the Concierge waits; Space skips (D61)
		if g.in.JustPressed(ebiten.KeySpace) {
			g.skipSpin()
		}
		g.step(0, 0)
	default:
		g.fast = g.in.Pressed(ebiten.KeyT)
		g.step(move(g.in))
		if g.in.JustPressed(ebiten.KeySpace) {
			if room := g.atPanel(); g.atTerminal() {
				g.openTerminal()
			} else if room >= 0 {
				g.autoRoom[room] = !g.autoRoom[room]
			}
		}
	}
	return nil
}

// step advances the world one frame with the Concierge walking in direction (dx,dy). No input, no
// rendering: the headless sim drives it directly.
func (g *Game) step(dx, dy float64) {
	g.tick++
	g.landReels(false)
	g.advanceClock()
	g.player.Move(dx, dy, conciergeSpeed, g.conciergeBlocked)
	g.hotel.UpdateDoors(g.actors(), g.occupied)
	g.updateParties()
	g.autoBook()
	g.separate()
	g.updateFog()
	if !g.headless {
		g.player.Anim.Update(g.player.X, g.player.Y)
		for _, guest := range g.guests() {
			guest.Anim.Update(guest.X, guest.Y)
		}
		w, h := g.mapSize()
		g.cam.Follow(g.player.X, g.player.Y, w, h)
	}
	g.floaters = slices.DeleteFunc(g.floaters, func(f floater) bool { return g.tick-f.at > floaterFrames })
}

// blocked: a foot box would hit a wall or a shut door.
func (g *Game) blocked(r image.Rectangle) bool {
	return g.hotel.Map.BlockedBox(r) || g.hotel.DoorBlocks(r)
}

// conciergeBlocked: blocked, and also every Occupied Room's doorway, open or not (D91), so the Concierge
// can't slip in behind a Party.
func (g *Game) conciergeBlocked(r image.Rectangle) bool {
	return g.blocked(r) || slices.ContainsFunc(g.hotel.Doors, func(d world.Door) bool {
		return d.Room >= 0 && g.occupied(d.Room) && d.Rect.Overlaps(r)
	})
}

// separate pushes overlapping foot boxes apart along their shallower overlap, split by each actor's
// give (D22b). A push into a wall or a shut door is dropped. Phasing guests are skipped (D22c).
func (g *Game) separate() {
	type body struct {
		x, y    *float64
		give    float64
		blocked func(image.Rectangle) bool
		guest   *entities.Guest // nil: the Concierge
	}
	bodies := []body{{&g.player.X, &g.player.Y, conciergeGive, g.conciergeBlocked, nil}}
	for _, guest := range g.guests() {
		if guest.Phasing {
			continue
		}
		give := 0.0 // idle guests (queued, staying) stand their ground
		if len(guest.Path) > 0 {
			give = guestGive
		}
		bodies = append(bodies, body{&guest.X, &guest.Y, give, g.blocked, guest})
	}
	nudge := func(b body, dx, dy float64) {
		if !b.blocked(entities.FootBox(*b.x+dx, *b.y+dy)) {
			*b.x += dx
			*b.y += dy
		}
	}
	for i, a := range bodies {
		for _, b := range bodies[i+1:] {
			if a.guest != nil && b.guest.Mate(a.guest) { // a Party walks as one (D90)
				continue
			}
			dx, dy := *b.x-*a.x, *b.y-*a.y
			ox, oy := entities.FootW-math.Abs(dx), entities.FootH-math.Abs(dy)
			if ox <= 0 || oy <= 0 {
				continue
			}
			share := 0.5 // of the push that a takes
			if a.give+b.give > 0 {
				share = a.give / (a.give + b.give)
			}
			if ox < oy {
				nudge(a, -sign(dx)*ox*share, 0)
				nudge(b, sign(dx)*ox*(1-share), 0)
			} else {
				nudge(a, 0, -sign(dy)*oy*share)
				nudge(b, 0, sign(dy)*oy*(1-share))
			}
		}
	}
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

// actors: everyone on the map the doors react to.
func (g *Game) actors() []world.Actor {
	as := []world.Actor{{Box: entities.FootBox(g.player.X, g.player.Y), Room: -1}}
	for _, guest := range g.guests() {
		as = append(as, world.Actor{Box: entities.FootBox(guest.X, guest.Y), Room: guest.RoomIndex})
	}
	return as
}

// occupied (D91): a Room is Occupied for its whole Booking.
func (g *Game) occupied(room int) bool { return !g.vacant(room) }

// walk advances a guest along its path. It waits in place while the next step would hit a shut door,
// and sidesteps to its right (left if a wall is there) while the next step would bump into someone
// ahead, so two guests meeting head-on pass each other.
func (g *Game) walk(guest *entities.Guest, speed float64) bool {
	guest.TrackProgress(stuckFrames) // measures last frame's walk and pushes
	if !guest.Phasing && g.yields(guest) {
		return false
	}
	x, y, path := guest.X, guest.Y, guest.Path
	done := guest.FollowPath(speed)
	if g.hotel.DoorBlocks(entities.FootBox(guest.X, guest.Y)) {
		guest.X, guest.Y, guest.Path = x, y, path
		return false
	}
	hx, hy := guest.X-x, guest.Y-y
	if bx, by, bump := g.bumpsAhead(guest, hx, hy); !guest.Phasing && bump {
		guest.X, guest.Y, guest.Path = x, y, path
		sides := [...]float64{1, -1}  // right of the heading (y points down), then left,
		if (bx-x)*-hy+(by-y)*hx > 0 { // unless the one bumped is on the right
			sides = [...]float64{-1, 1}
		}
		for _, s := range sides {
			if nx, ny := x-s*hy, y+s*hx; !g.blocked(entities.FootBox(nx, ny)) {
				guest.X, guest.Y = nx, ny
				break
			}
		}
		return false
	}
	if done {
		guest.Phasing = false
	}
	return done
}

// yieldAhead: how many waypoints ahead a guest looks for a choke point. It waits a cell or two short of
// it, which leaves the way out clear.
const yieldAhead = 3

// yields (D89): a guest about to pass a 1-cell choke point (a Room doorway) waits while another guest
// coming the other way is on that stretch and nearer the choke: the nearer one goes first, so two never
// wait on each other. The Concierge is left to the sidestep and the push give (D22b).
func (g *Game) yields(guest *entities.Guest) bool {
	ahead := guest.Path[:min(len(guest.Path), yieldAhead)]
	i := slices.IndexFunc(ahead, func(p image.Point) bool { return g.hotel.Choke(p.Div(world.TileSize)) })
	if i < 0 {
		return false
	}
	c := ahead[i].Div(world.TileSize).Mul(world.TileSize).Add(image.Pt(world.TileSize/2, world.TileSize/2))
	near := func(x, y float64) float64 { return math.Hypot(float64(c.X)-x, float64(c.Y)-y) }
	hx, hy := float64(guest.Path[0].X)-guest.X, float64(guest.Path[0].Y)-guest.Y
	for _, o := range g.guests() {
		if o == guest || o.Mate(guest) || o.Phasing || len(o.Path) == 0 || near(o.X, o.Y) >= near(guest.X, guest.Y) {
			continue
		}
		on := image.Pt(int(o.X), int(o.Y)).Div(world.TileSize)
		if slices.ContainsFunc(ahead, func(p image.Point) bool { return p.Div(world.TileSize) == on }) &&
			(float64(o.Path[0].X)-o.X)*hx+(float64(o.Path[0].Y)-o.Y)*hy < 0 { // on the stretch, coming the other way
			return true
		}
	}
	return false
}

// bumpsAhead: the guest's foot box overlaps another actor, at (x,y), lying ahead along heading (hx,hy).
// When two walking guests each have the other ahead (converging on one cell), only the one without the
// right of way stops, so they never wait on each other.
func (g *Game) bumpsAhead(guest *entities.Guest, hx, hy float64) (x, y float64, bump bool) {
	box := entities.FootBox(guest.X, guest.Y).Inset(-1) // touching counts: boxes snap to whole px, so one could step into a gap and never see the bump
	hit := func(x, y float64) bool {
		return entities.FootBox(x, y).Overlaps(box) && (x-guest.X)*hx+(y-guest.Y)*hy > 0
	}
	if hit(g.player.X, g.player.Y) {
		return g.player.X, g.player.Y, true
	}
	for _, o := range g.guests() {
		if o == guest || o.Mate(guest) || o.Phasing || !hit(o.X, o.Y) {
			continue
		}
		if len(o.Path) > 0 && rightOfWay(guest, o) {
			ox, oy := float64(o.Path[0].X)-o.X, float64(o.Path[0].Y)-o.Y
			if (guest.X-o.X)*ox+(guest.Y-o.Y)*oy > 0 { // o has this guest ahead too, and waits for it
				continue
			}
		}
		return o.X, o.Y, true
	}
	return 0, 0, false
}

// rightOfWay: guest a goes before b when they bump into each other. A booked guest goes before a Walk-in
// still heading for the Queue, which would push it back into the lanes; otherwise the one nearer its goal.
func rightOfWay(a, b *entities.Guest) bool {
	if booked := a.RoomIndex >= 0; booked != (b.RoomIndex >= 0) {
		return booked
	}
	return a.Left() < b.Left()
}

func (g *Game) atTerminal() bool {
	return image.Pt(int(g.player.X), int(g.player.Y)).In(g.hotel.Terminal)
}

// midnight Spins every eligible Room with the cinematic (D85); none eligible, no cinematic.
func (g *Game) midnight() {
	next := world.Draw(g.types, len(g.hotel.Rooms), g.rng)
	var rooms []int
	for i, r := range g.hotel.Rooms {
		if g.flips(i) {
			rooms = append(rooms, i)
		} else {
			next[i] = r.Type
		}
	}
	if len(rooms) > 0 {
		g.spin(next, rooms, false)
	}
}

// flips (D85): at Midnight a Room re-draws its type if it is Vacant and the Concierge's foot point is
// outside it.
func (g *Game) flips(room int) bool {
	return g.vacant(room) && !image.Pt(int(g.player.X), int(g.player.Y)).In(g.hotel.Rooms[room].Shell())
}

// spin gives the Rooms their next types at once (D47, D49); rooms (nil: every Room) roll their reels in
// Room order, and each one's cells follow at its reel stop (D48, D50). instant stops them all now (Run
// start, D46). A Room left out keeps its type: pass that type back in next.
func (g *Game) spin(next []world.RoomType, rooms []int, instant bool) {
	g.landReels(true) // a Spin mid-roll finishes the last one first
	if rooms == nil {
		for i := range g.hotel.Rooms {
			rooms = append(rooms, i)
		}
	}
	g.hotel.Spin(next)
	g.spinRooms, g.reelAt, g.landed, g.reels, g.cinematic = rooms, g.tick, 0, nil, !instant
	if !instant {
		g.reelAt += spinCamFrames // the reels wait for the camera to frame every Room
	}
	g.spinEndAt = g.reelStop(len(rooms)-1) + spinHoldFrames + spinCamFrames
	g.landReels(instant)
}

// skipSpin lands every reel now and sends the camera back from wherever it is, as smoothly as it came (D61).
func (g *Game) skipSpin() {
	g.landReels(true)
	out := min(float64(g.tick-g.reelAt+spinCamFrames)/spinCamFrames, 1) // how far out the camera got
	g.spinEndAt = min(g.spinEndAt, g.tick+int(math.Ceil(out*spinCamFrames)))
}

// spinning: the Spin cinematic is on; the Concierge waits (camera out, reels, hold, camera back).
func (g *Game) spinning() bool { return g.cinematic && g.tick < g.spinEnd() }

func (g *Game) spinEnd() int { return g.spinEndAt }

// spinView: how far the camera is out, 0 at the Concierge, 1 framing all Rooms; smoothstep-eased.
func (g *Game) spinView() float64 {
	if !g.cinematic {
		return 0
	}
	out := float64(g.tick-g.reelAt+spinCamFrames) / spinCamFrames
	back := float64(g.spinEnd()-g.tick) / spinCamFrames
	t := min(max(min(out, back), 0), 1)
	return t * t * (3 - 2*t)
}

// roomsView: the world-px center and the zoom (never above the play zoom) that frame every Room.
func (g *Game) roomsView() (cx, cy, z float64) {
	rs := g.hotel.Rooms
	a := rs[0].Origin.Mul(world.TileSize)
	b := rs[len(rs)-1].Origin.Add(image.Pt(world.RoomW, world.RoomH)).Mul(world.TileSize)
	w, h := float64(b.X-a.X+2*spinMargin), float64(b.Y-a.Y+2*spinMargin)
	return float64(a.X+b.X) / 2, float64(a.Y+b.Y) / 2, min(viewW*zoom/w, viewH*zoom/h, zoom)
}

// reelStop: the tick the k-th rolling Room's reel stops.
func (g *Game) reelStop(k int) int { return g.reelAt + k*reelStagger + reelFrames }

// reel: how far Room i's reel has rolled, eased out from 0 to 1 (D48), and whether it is rolling.
func (g *Game) reel(i int) (float64, bool) {
	k := slices.Index(g.spinRooms, i)
	t := float64(g.tick-g.reelStop(k)+reelFrames) / reelFrames
	if k < g.landed || t < 0 {
		return 0, false
	}
	return 1 - math.Pow(1-min(t, 1), 3), true
}

// landReels stamps each rolling Room whose reel has stopped, in Room order, or all of them now.
func (g *Game) landReels(now bool) {
	for g.landed < len(g.spinRooms) && (now || g.tick >= g.reelStop(g.landed)) {
		g.hotel.Land(g.spinRooms[g.landed])
		g.landed++
		g.floorImg = nil // re-rendered on the next Draw
		g.unstick()
	}
}

// unstick moves the Concierge to the nearest cell center where the foot box fits, if a Spin left them
// inside something solid (D50).
// ponytail: BFS by steps through any cell, so a tight Room could drop them past a wall; path-aware if props need it
func (g *Game) unstick() {
	if !g.blocked(entities.FootBox(g.player.X, g.player.Y)) {
		return
	}
	m := g.hotel.Map
	start := image.Pt(int(g.player.X), int(g.player.Y)).Div(world.TileSize)
	seen := map[image.Point]bool{start: true}
	for queue := []image.Point{start}; len(queue) > 0; queue = queue[1:] {
		c := queue[0]
		x, y := float64(c.X*world.TileSize+world.TileSize/2), float64(c.Y*world.TileSize+world.TileSize/2)
		if !g.blocked(entities.FootBox(x, y)) {
			g.player.X, g.player.Y = x, y
			return
		}
		for _, d := range [...]image.Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			if n := c.Add(d); n.X >= 0 && n.Y >= 0 && n.X < m.W && n.Y < m.H && !seen[n] {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
}

func (g *Game) Layout(_, _ int) (int, int) { return viewW * zoom, viewH * zoom }
