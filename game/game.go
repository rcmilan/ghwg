package game

import (
	"image"
	"math"
	"math/rand/v2"
	"slices"

	"ghosthotel/assets"
	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	StateTitle = iota
	StateBooking
	StateTerminal
	StateArrivals
	StateDay
	StateDepartures
)

// Tunables. Speeds in world px per frame, times in frames (Ebitengine runs Update at 60 TPS).
const (
	tps             = 60
	dayFrames       = 12 * tps
	guestSpawnEvery = 3 * tps // between guests, in Arrivals and Departures
	payFrames       = 1 * tps // a guest's pause at the pay spot
	conciergeSpeed  = 1.0     // 3 screen px at x3
	arriveSpeed     = 0.6
	toCounterSpeed  = 0.65
	departSpeed     = 0.75
	conciergeGive   = 0.8     // share of a push the Concierge takes against a moving guest (D22b),
	guestGive       = 0.2     // and the moving guest takes; guest vs guest splits evenly; idle guests don't budge
	stuckFrames     = 1 * tps // a walking guest with no progress this long ignores actors for one tile (D22c)
	coinMagnet      = 27      // px: coins slide toward the Concierge inside this radius
	coinPickup      = 7       // px
	reelFrames      = 60      // one Room's reel roll, ~1 s (D48)
	reelStagger     = 12      // between consecutive Rooms' reel starts, so they stop ~0.2 s apart
	spinCamFrames   = 45      // the camera's glide out to all Rooms before the reels, and back after
	spinHoldFrames  = 30      // all-Rooms view held after the last reel stops
	spinMargin      = 16      // world px around the Rooms in the all-Rooms view
)

// guestCast: Guests draw a random character from these; repeats are fine (D25). Bob is the Concierge.
var guestCast = [...]string{"Adam", "Amelia", "Alex"}

// Money is a coin on the floor, worth its guest's Rate at Booking (D53).
type Money struct {
	X, Y  float64
	Value int
}

type Game struct {
	state                                          int
	hotel                                          *world.Hotel
	types                                          []world.RoomType // every loaded Room Type, drawn from at each Spin
	cam                                            Camera
	player                                         entities.Player
	guests                                         []*entities.Guest
	terminalCursor                                 int
	assignments                                    []int
	confirmed                                      []bool
	tick                                           int // simulation frames since launch
	dayStarted, arrivalsStarted, departuresStarted int // ticks
	money                                          int
	pickups                                        []Money
	message                                        string
	rng                                            *rand.Rand
	tiles                                          []*ebiten.Image             // by world.TileID
	characters                                     map[string][2]*ebiten.Image // idle_anim, run sheets by name
	floorImg                                       *ebiten.Image               // the floor layer pre-rendered at 1px per world px
	props                                          []prop                      // every other static tile, by SortY
	reelAt                                         int                         // tick the last Spin started
	landed                                         int                         // Rooms whose reel has stopped, in Room order
	reels                                          [][2]*ebiten.Image          // per Room: old and new footprint while it rolls
	cinematic                                      bool                        // the last Spin glides the camera out to the Rooms
	spinEndAt                                      int                         // tick the cinematic's camera is back on the Concierge
	worldImg                                       *ebiten.Image               // the whole map ×zoom, for the Spin cinematic
}

// New builds the shipped grid (world.Cols x world.Rows).
func New() (*Game, error) { return newGame(world.Cols, world.Rows) }

func newGame(cols, rows int) (*Game, error) {
	ts, err := world.LoadTileset(assets.FS, "tiles/hotel.tsj")
	if err != nil {
		return nil, err
	}
	types, err := world.LoadRoomTypes(assets.FS, ts)
	if err != nil {
		return nil, err
	}
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	h, err := world.Build(ts, types, cols, rows, rng)
	if err != nil {
		return nil, err
	}
	tiles, err := loadTiles(ts)
	if err != nil {
		return nil, err
	}
	chars, err := loadCharacters(append([]string{"Bob"}, guestCast[:]...)...)
	if err != nil {
		return nil, err
	}
	return &Game{state: StateTitle, hotel: h, types: types, landed: len(h.Rooms), rng: rng, tiles: tiles, characters: chars}, nil
}

func (g *Game) mapSize() (int, int) {
	return g.hotel.Map.W * world.TileSize, g.hotel.Map.H * world.TileSize
}

func (g *Game) startShift(keepPlayer bool) {
	g.guests = nil
	g.pickups = nil
	g.resetBookings()
	g.terminalCursor = 0
	g.state = StateBooking
	if !keepPlayer { // a new Run starts with an instant Spin (D46)
		g.spin(world.Draw(g.types, len(g.hotel.Rooms), g.rng), true)
		g.player = entities.NewPlayer(g.hotel.ConciergeStart)
		g.money = 0
		w, h := g.mapSize()
		g.cam.Snap(g.player.X, g.player.Y, w, h)
	}
	g.message = "Approach reception and press SPACE to book incoming guests."
}

func (g *Game) resetBookings() {
	g.assignments = make([]int, len(g.hotel.Rooms))
	g.confirmed = make([]bool, len(g.hotel.Rooms))
	for i := range g.assignments {
		g.assignments[i] = i
	}
}

func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if g.state == StateTitle {
			return ebiten.Termination
		}
		if g.state == StateTerminal {
			g.state = StateBooking
			g.message = "Terminal closed."
			return nil
		}
		g.state = StateTitle
		return nil
	}
	if g.state == StateTitle {
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.startShift(false)
		}
		return nil
	}
	if g.state == StateTerminal {
		g.updateTerminal()
		return nil
	}
	if g.spinning() { // the camera is on the Rooms: the Concierge waits; Space skips (D61)
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.skipSpin()
		}
		g.step(0, 0)
		return nil
	}
	g.step(entities.ReadMove())
	if g.state == StateBooking && inpututil.IsKeyJustPressed(ebiten.KeySpace) && g.atTerminal() {
		g.state = StateTerminal
		g.message = "Choose a room for each guest."
	}
	return nil
}

// step advances the world one frame with the Concierge walking in direction (dx,dy). No input, no
// rendering: the headless sim drives it directly.
func (g *Game) step(dx, dy float64) {
	g.tick++
	g.landReels(false)
	g.player.Move(dx, dy, conciergeSpeed, g.blocked)
	g.hotel.UpdateDoors(g.actors(), g.occupied)
	switch g.state {
	case StateArrivals:
		g.updateArrivals()
	case StateDay:
		if g.tick-g.dayStarted >= dayFrames {
			g.beginDepartures()
		}
	case StateDepartures:
		g.updateDepartures()
	}
	g.separate()
	g.player.Anim.Update(g.player.X, g.player.Y)
	for _, guest := range g.guests {
		if guest.State != entities.GuestQueued {
			guest.Anim.Update(guest.X, guest.Y)
		}
	}
	w, h := g.mapSize()
	g.cam.Follow(g.player.X, g.player.Y, w, h)
	g.updateMoney()
}

// blocked: a foot box would hit a wall or a shut door.
func (g *Game) blocked(r image.Rectangle) bool {
	return g.hotel.Map.BlockedBox(r) || g.hotel.DoorBlocks(r)
}

// separate pushes overlapping foot boxes apart along their shallower overlap, split by each actor's
// give (D22b). A push into a wall or a shut door is dropped. Phasing guests are skipped (D22c).
func (g *Game) separate() {
	type body struct {
		x, y *float64
		give float64
	}
	bodies := []body{{&g.player.X, &g.player.Y, conciergeGive}}
	for _, guest := range g.guests {
		if guest.State == entities.GuestQueued || guest.Phasing {
			continue
		}
		give := 0.0 // idle guests (staying, paying) stand their ground
		if len(guest.Path) > 0 {
			give = guestGive
		}
		bodies = append(bodies, body{&guest.X, &guest.Y, give})
	}
	nudge := func(b body, dx, dy float64) {
		if !g.blocked(entities.FootBox(*b.x+dx, *b.y+dy)) {
			*b.x += dx
			*b.y += dy
		}
	}
	for i, a := range bodies {
		for _, b := range bodies[i+1:] {
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
	for _, guest := range g.guests {
		if guest.State != entities.GuestQueued {
			as = append(as, world.Actor{Box: entities.FootBox(guest.X, guest.Y), Room: guest.RoomIndex})
		}
	}
	return as
}

// occupied (D4): a Room is Occupied from its Guest reaching the Stay Spot until that Guest pays.
func (g *Game) occupied(room int) bool {
	for _, guest := range g.guests {
		if guest.RoomIndex == room && guest.State >= entities.GuestStaying && guest.State <= entities.GuestPaying {
			return true
		}
	}
	return false
}

// walk advances a guest along its path. It waits in place while the next step would hit a shut door,
// and sidesteps to its right (left if a wall is there) while the next step would bump into someone
// ahead, so two guests meeting head-on pass each other.
func (g *Game) walk(guest *entities.Guest, speed float64) bool {
	guest.TrackProgress(stuckFrames) // measures last frame's walk and pushes
	x, y, path := guest.X, guest.Y, guest.Path
	done := guest.FollowPath(speed)
	if g.hotel.DoorBlocks(entities.FootBox(guest.X, guest.Y)) {
		guest.X, guest.Y, guest.Path = x, y, path
		return false
	}
	if hx, hy := guest.X-x, guest.Y-y; !guest.Phasing && g.bumpsAhead(guest, hx, hy) {
		guest.X, guest.Y, guest.Path = x, y, path
		for _, s := range [...]float64{1, -1} { // right of the heading (y points down), then left
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

// bumpsAhead: the guest's foot box overlaps another actor lying ahead along heading (hx,hy).
func (g *Game) bumpsAhead(guest *entities.Guest, hx, hy float64) bool {
	box := entities.FootBox(guest.X, guest.Y)
	hit := func(x, y float64) bool {
		return entities.FootBox(x, y).Overlaps(box) && (x-guest.X)*hx+(y-guest.Y)*hy > 0
	}
	if hit(g.player.X, g.player.Y) {
		return true
	}
	for _, o := range g.guests {
		if o != guest && o.State != entities.GuestQueued && !o.Phasing && hit(o.X, o.Y) {
			return true
		}
	}
	return false
}

func (g *Game) atTerminal() bool {
	return image.Pt(int(g.player.X), int(g.player.Y)).In(g.hotel.Terminal)
}

func (g *Game) updateTerminal() {
	n := len(g.assignments)
	if inpututil.IsKeyJustPressed(ebiten.KeyW) {
		g.terminalCursor = (g.terminalCursor + n) % (n + 1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyS) {
		g.terminalCursor = (g.terminalCursor + 1) % (n + 1)
	}
	if g.terminalCursor < n && !g.confirmed[g.terminalCursor] && inpututil.IsKeyJustPressed(ebiten.KeyA) {
		g.cycleRoom(-1)
	}
	if g.terminalCursor < n && !g.confirmed[g.terminalCursor] && inpututil.IsKeyJustPressed(ebiten.KeyD) {
		g.cycleRoom(1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		if g.terminalCursor == n {
			for _, confirmed := range g.confirmed {
				if !confirmed {
					g.message = "Confirm every guest before starting arrivals."
					return
				}
			}
			g.beginArrivals()
			return
		}
		g.confirmed[g.terminalCursor] = !g.confirmed[g.terminalCursor]
		if !g.confirmed[g.terminalCursor] {
			g.message = "Guest unlocked. Their room is available for changes."
			return
		}
		all := true
		for _, v := range g.confirmed {
			all = all && v
		}
		if all {
			g.message = "All guests booked. Select BEGIN ARRIVALS when ready."
		} else {
			g.message = "Guest mapping confirmed. Select another guest."
		}
	}
}

func (g *Game) cycleRoom(delta int) {
	i := g.terminalCursor
	n := len(g.hotel.Rooms)
	old := g.assignments[i]
	candidate := old
	for tries := 0; tries < n; tries++ {
		candidate = (candidate + delta + n) % n
		occupant := -1
		for guest, room := range g.assignments {
			if guest != i && room == candidate {
				occupant = guest
				break
			}
		}
		// A booked guest owns its room until that guest is explicitly unlocked.
		if occupant == -1 || !g.confirmed[occupant] {
			if occupant >= 0 {
				g.assignments[occupant] = old
			}
			g.assignments[i] = candidate
			return
		}
	}
}

func (g *Game) beginArrivals() {
	g.guests = make([]*entities.Guest, len(g.assignments))
	for guest, room := range g.assignments {
		v := entities.NewGuest(room, g.hotel.Spawn, guest*guestSpawnEvery, guestCast[g.rng.IntN(len(guestCast))])
		v.Rate = g.hotel.Rooms[room].Type.Rate // Booking is final once Arrivals begin (D53)
		g.guests[guest] = &v
	}
	g.arrivalsStarted = g.tick
	g.state = StateArrivals
	g.message = "Guests are arriving one at a time."
}

// updateArrivals: street -> own door -> Stay Spot, along BFS waypoints.
func (g *Game) updateArrivals() {
	arrived := 0
	for _, guest := range g.guests {
		switch guest.State {
		case entities.GuestQueued:
			if g.tick-g.arrivalsStarted < guest.SpawnAt {
				continue
			}
			guest.State = entities.GuestArriving
			guest.SetPath(g.hotel.Path(g.hotel.Spawn, g.hotel.Rooms[guest.RoomIndex].Stay, guest.RoomIndex))
		case entities.GuestStaying:
			arrived++
			continue
		}
		if g.walk(guest, arriveSpeed) {
			guest.State = entities.GuestStaying
			arrived++
		}
	}
	if arrived == len(g.guests) {
		g.state = StateDay
		g.dayStarted = g.tick
		g.message = "Hotel occupied. Guests will check out automatically."
	}
}

func (g *Game) beginDepartures() {
	g.state = StateDepartures
	g.departuresStarted = g.tick
	g.message = "Guests are checking out."
	slices.SortFunc(g.guests, func(a, b *entities.Guest) int { return a.RoomIndex - b.RoomIndex }) // lanes need Room order (D62)
	// each guest is released so they reach the shared lane (LaneEntry) one guestSpawnEvery after the
	// one before, however long their own walk to it (D62)
	join := 0
	for i, guest := range g.guests {
		stay := g.hotel.Rooms[guest.RoomIndex].Stay
		walk := int(float64(len(g.hotel.Path(stay, g.hotel.LaneEntry, guest.RoomIndex))*world.TileSize) / toCounterSpeed)
		join = max(join+guestSpawnEvery, walk)
		if i == 0 {
			join = walk
		}
		guest.State = entities.GuestLeavingRoom
		guest.SpawnAt = join - walk
		toPay, _ := g.hotel.Checkout(guest.RoomIndex)
		guest.SetPath(toPay)
	}
}

// updateDepartures: Stay Spot -> own Pay Spot (coin) -> street, along each Room's lane (D62).
func (g *Game) updateDepartures() {
	remaining := g.guests[:0]
	for _, guest := range g.guests {
		if g.tick-g.departuresStarted < guest.SpawnAt {
			remaining = append(remaining, guest)
			continue
		}
		switch guest.State {
		case entities.GuestLeavingRoom:
			if g.walk(guest, toCounterSpeed) {
				guest.State = entities.GuestPaying
				guest.PauseUntil = g.tick + payFrames
			}
		case entities.GuestPaying:
			if g.tick >= guest.PauseUntil {
				g.pickups = append(g.pickups, Money{X: guest.X, Y: guest.Y, Value: guest.Rate})
				guest.State = entities.GuestDeparting
				_, toStreet := g.hotel.Checkout(guest.RoomIndex)
				guest.SetPath(toStreet)
			}
		case entities.GuestDeparting:
			if g.walk(guest, departSpeed) {
				continue // left through the Entrance
			}
		}
		remaining = append(remaining, guest)
	}
	g.guests = remaining
	if len(g.guests) == 0 {
		g.endShift()
	}
}

// endShift: the last guest has left the building, ending Shift 2: the Rooms Spin (D46).
func (g *Game) endShift() {
	g.state = StateBooking
	g.resetBookings()
	g.message = "New shift ready. Approach reception to book guests."
	g.spin(world.Draw(g.types, len(g.hotel.Rooms), g.rng), false)
}

// spin gives the Rooms their next types at once (D47, D49). Each Room's cells follow at its reel stop
// (D48, D50); instant stops them all now (Run start, D46).
func (g *Game) spin(next []world.RoomType, instant bool) {
	g.landReels(true) // a Spin mid-roll finishes the last one first
	g.hotel.Spin(next)
	g.reelAt, g.landed, g.reels, g.cinematic = g.tick, 0, nil, !instant
	if !instant {
		g.reelAt += spinCamFrames // the reels wait for the camera to frame every Room
	}
	g.spinEndAt = g.reelStop(len(g.hotel.Rooms)-1) + spinHoldFrames + spinCamFrames
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

// reelStop: the tick Room i's reel stops.
func (g *Game) reelStop(i int) int { return g.reelAt + i*reelStagger + reelFrames }

// reel: how far Room i's reel has rolled, eased out from 0 to 1 (D48), and whether it is rolling.
func (g *Game) reel(i int) (float64, bool) {
	t := float64(g.tick-g.reelStop(i)+reelFrames) / reelFrames
	if i < g.landed || t < 0 {
		return 0, false
	}
	return 1 - math.Pow(1-min(t, 1), 3), true
}

// landReels stamps each Room whose reel has stopped, in Room order, or all of them now.
func (g *Game) landReels(now bool) {
	for g.landed < len(g.hotel.Rooms) && (now || g.tick >= g.reelStop(g.landed)) {
		g.hotel.Land(g.landed)
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

func (g *Game) updateMoney() {
	kept := g.pickups[:0]
	for _, coin := range g.pickups {
		dx, dy := g.player.X-coin.X, g.player.Y-coin.Y
		d := math.Hypot(dx, dy)
		if d < coinMagnet && d > .1 {
			coin.X += dx / d * math.Min(1.7, .33+d*.12)
			coin.Y += dy / d * math.Min(1.7, .33+d*.12)
		}
		if d < coinPickup {
			g.money += coin.Value
			g.message = "Payment collected."
			continue
		}
		kept = append(kept, coin)
	}
	g.pickups = kept
}

func (g *Game) Layout(_, _ int) (int, int) { return viewW * zoom, viewH * zoom }
