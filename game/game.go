package game

import (
	"image"
	"math"
	"math/rand/v2"
	"time"

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

// Tunables, world px per frame unless noted.
const (
	dayDuration     = 12 * time.Second
	arriveSpeed     = 0.6
	toCounterSpeed  = 0.65
	departSpeed     = 0.75
	coinMagnet      = 27  // px: coins slide toward the Concierge inside this radius
	coinPickup      = 7   // px
	coinValue       = 25  // ponytail: flat until S8 (coin = Room Rate)
	guestSpawnEvery = 3.0 // seconds between guests
)

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
	dayStarted, arrivalsStarted, departuresStarted time.Time
	money                                          int
	pickups                                        []Money
	message                                        string
	tiles                                          []*ebiten.Image // by world.TileID
	mapImg                                         *ebiten.Image   // pre-rendered at 1px per world px
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
	h, err := world.Build(ts, types, rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
	if err != nil {
		return nil, err
	}
	tiles, err := loadTiles(ts)
	if err != nil {
		return nil, err
	}
	return &Game{state: StateTitle, hotel: h, tiles: tiles}, nil
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
	g.player.Move(g.hotel.Map)
	w, h := g.mapSize()
	g.cam.Follow(g.player.X, g.player.Y, w, h)
	g.updateMoney()
	switch g.state {
	case StateBooking:
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) && g.atTerminal() {
			g.state = StateTerminal
			g.message = "Choose a room for each guest."
		}
	case StateArrivals:
		g.updateArrivals()
	case StateDay:
		if time.Since(g.dayStarted) >= dayDuration {
			g.beginDepartures()
		}
	case StateDepartures:
		g.updateDepartures()
	}
	return nil
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
		v := entities.NewGuest(room, g.hotel.Spawn, float64(guest)*guestSpawnEvery)
		g.guests[guest] = &v
	}
	g.arrivalsStarted = time.Now()
	g.state = StateArrivals
	g.message = "Guests are arriving one at a time."
}

// updateArrivals: street -> own door -> Stay Spot, along BFS waypoints.
func (g *Game) updateArrivals() {
	arrived := 0
	for _, guest := range g.guests {
		switch guest.State {
		case entities.GuestQueued:
			if time.Since(g.arrivalsStarted).Seconds() < guest.SpawnAt {
				continue
			}
			guest.State = entities.GuestArriving
			guest.Path = g.hotel.Path(g.hotel.Spawn, g.hotel.Rooms[guest.RoomIndex].Stay, guest.RoomIndex)
		case entities.GuestStaying:
			arrived++
			continue
		}
		if guest.FollowPath(arriveSpeed) {
			guest.State = entities.GuestStaying
			arrived++
		}
	}
	if arrived == len(g.guests) {
		g.state = StateDay
		g.dayStarted = time.Now()
		g.message = "Hotel occupied. Guests will check out automatically."
	}
}

func (g *Game) beginDepartures() {
	g.state = StateDepartures
	g.departuresStarted = time.Now()
	g.message = "Guests are checking out."
	for i, guest := range g.guests {
		guest.State = entities.GuestLeavingRoom
		guest.SpawnAt = float64(i) * guestSpawnEvery
		guest.Path = g.hotel.Path(g.hotel.Rooms[guest.RoomIndex].Stay, g.hotel.PaySpot, guest.RoomIndex)
	}
}

// updateDepartures: Stay Spot -> pay spot (coin) -> street, along BFS waypoints.
func (g *Game) updateDepartures() {
	remaining := g.guests[:0]
	for _, guest := range g.guests {
		if time.Since(g.departuresStarted).Seconds() < guest.SpawnAt {
			remaining = append(remaining, guest)
			continue
		}
		switch guest.State {
		case entities.GuestLeavingRoom:
			if guest.FollowPath(toCounterSpeed) {
				guest.State = entities.GuestPaying
				guest.PauseUntil = time.Since(g.departuresStarted).Seconds() + 1
			}
		case entities.GuestPaying:
			if time.Since(g.departuresStarted).Seconds() >= guest.PauseUntil {
				g.pickups = append(g.pickups, Money{X: guest.X, Y: guest.Y})
				guest.State = entities.GuestDeparting
				guest.Path = g.hotel.Path(g.hotel.PaySpot, g.hotel.Spawn, -1)
			}
		case entities.GuestDeparting:
			if guest.FollowPath(departSpeed) {
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
