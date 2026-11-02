package game

import (
	"image"
	"math"
	"math/rand/v2"

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
	coinValue       = 25      // ponytail: flat until S8 (coin = Room Rate)
)

// guestCast: Guests draw a random character from these; repeats are fine (D25). Bob is the Concierge.
var guestCast = [...]string{"Adam", "Amelia", "Alex"}

type Money struct{ X, Y float64 }

type Game struct {
	state                                          int
	hotel                                          *world.Hotel
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
	mapImg, overImg                                *ebiten.Image               // pre-rendered at 1px per world px: under / over the actors
}

func New() (*Game, error) {
	ts, err := world.LoadTileset(assets.FS, "tiles/hotel.tsj")
	if err != nil {
		return nil, err
	}
	types, err := world.LoadRoomTypes(assets.FS, ts)
	if err != nil {
		return nil, err
	}
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	h, err := world.Build(ts, types, rng)
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
	return &Game{state: StateTitle, hotel: h, rng: rng, tiles: tiles, characters: chars}, nil
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
	if !keepPlayer {
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
	for i, guest := range g.guests {
		guest.State = entities.GuestLeavingRoom
		guest.SpawnAt = i * guestSpawnEvery
		guest.SetPath(g.hotel.Path(g.hotel.Rooms[guest.RoomIndex].Stay, g.hotel.PaySpot, guest.RoomIndex))
	}
}

// updateDepartures: Stay Spot -> pay spot (coin) -> street, along BFS waypoints.
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
				g.pickups = append(g.pickups, Money{X: guest.X, Y: guest.Y})
				guest.State = entities.GuestDeparting
				guest.SetPath(g.hotel.Path(g.hotel.PaySpot, g.hotel.Spawn, -1))
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
		g.state = StateBooking
		g.resetBookings()
		g.message = "New shift ready. Approach reception to book guests."
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
			g.money += coinValue
			g.message = "Payment collected."
			continue
		}
		kept = append(kept, coin)
	}
	g.pickups = kept
}

func (g *Game) Layout(_, _ int) (int, int) { return viewW * zoom, viewH * zoom }
