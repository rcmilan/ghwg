package game

import (
	"fmt"
	"ghosthotel/assets"
	"ghosthotel/entities"
	"ghosthotel/world"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"image/color"
	"math"
	"time"
)

const (
	StateTitle = iota
	StateBooking
	StateTerminal
	StateArrivals
	StateDay
	StateDepartures
)
const dayDuration = 12 * time.Second

type Money struct{ X, Y float64 }
type Game struct {
	state                                          int
	player                                         entities.Player
	layout                                         world.Layout
	rooms                                          []world.Room
	guests                                         []*entities.Guest
	terminalCursor                                 int
	assignments                                    []int
	confirmed                                      []bool
	dayStarted, arrivalsStarted, departuresStarted time.Time
	money                                          int
	pickups                                        []Money
	message                                        string
}

func New() *Game {
	l := world.DefaultLayout()
	return &Game{state: StateTitle, layout: l, rooms: l.Rooms}
}
func (g *Game) startShift(keepPlayer bool) {
	g.layout = world.DefaultLayout()
	g.rooms = g.layout.Rooms
	g.guests = nil
	g.pickups = nil
	g.assignments = make([]int, len(g.rooms))
	g.confirmed = make([]bool, len(g.rooms))
	for i := range g.assignments {
		g.assignments[i] = i
	}
	g.terminalCursor = 0
	g.state = StateBooking
	if !keepPlayer {
		g.player = entities.NewPlayer()
		g.money = 0
	}
	g.message = "Approach reception and press SPACE to book incoming guests."
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
	playerX, playerY := g.player.X, g.player.Y
	g.player.Move(g.layout, g.rooms)
	g.resolvePlayerCollisions()
	if g.layout.IsBlocked(g.player.X, g.player.Y, g.rooms) {
		g.player.X, g.player.Y = playerX, playerY
	}
	g.updateMoney()
	switch g.state {
	case StateBooking:
		cx, cy := g.layout.CounterCenter()
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) && g.player.Near(cx, cy, 100) {
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
	n := len(g.rooms)
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
	ex, ey := g.layout.ExitCenter()
	g.guests = make([]*entities.Guest, len(g.assignments))
	for guest, room := range g.assignments {
		v := entities.NewGuest(room, ex, ey+18, float64(guest)*3)
		g.guests[guest] = &v
	}
	g.arrivalsStarted = time.Now()
	g.state = StateArrivals
	g.message = "Guests are arriving one at a time."
}
func (g *Game) updateArrivals() {
	arrived := 0
	px, py := g.layout.PassageCenter()
	for _, guest := range g.guests {
		if guest.State == entities.GuestQueued {
			if time.Since(g.arrivalsStarted).Seconds() >= guest.SpawnAt {
				guest.State = entities.GuestArriving
			} else {
				continue
			}
		}
		if guest.State == entities.GuestStaying {
			arrived++
			continue
		}
		room := &g.rooms[guest.RoomIndex]
		points := g.roomDoorRoute(*room, px, py)
		if g.moveGuest(guest, points[guest.Waypoint][0], points[guest.Waypoint][1], 1.7) {
			guest.Waypoint++
			if guest.Waypoint == len(points) {
				guest.State = entities.GuestStaying
				room.Locked = true
				arrived++
			}
		}
	}
	if arrived == len(g.guests) {
		g.state = StateDay
		g.dayStarted = time.Now()
		g.message = "Hotel occupied. Guests will check out automatically."
	}
}

// roomDoorRoute reaches the south side of a room without crossing any room
// tile. Top rooms are approached through the central vertical corridor.
func (g *Game) roomDoorRoute(room world.Room, passageX, passageY float64) [][2]float64 {
	doorX := float64(room.Door.Min.X + room.Door.Dx()/2)
	belowWall := float64(g.layout.Passage.Min.Y - 15)
	doorSide := float64(room.Door.Max.Y + 10)
	if room.Bounds.Min.Y == g.layout.Config.OriginY {
		betweenRows := float64(room.Bounds.Max.Y + g.layout.Config.RoomGap/2)
		return [][2]float64{{passageX, passageY}, {passageX, belowWall}, {passageX, betweenRows}, {doorX, betweenRows}, {doorX, doorSide}}
	}
	return [][2]float64{{passageX, passageY}, {passageX, belowWall}, {doorX, belowWall}, {doorX, doorSide}}
}
func (g *Game) beginDepartures() {
	g.state = StateDepartures
	g.departuresStarted = time.Now()
	g.message = "Guests are checking out."
	for i, guest := range g.guests {
		guest.State = entities.GuestLeavingRoom
		guest.Waypoint = 0
		guest.SpawnAt = float64(i) * 3
		g.rooms[guest.RoomIndex].Locked = false
	}
}
func (g *Game) updateDepartures() {
	remaining := g.guests[:0]
	cx, cy := g.layout.CounterCenter()
	ex, ey := g.layout.ExitCenter()
	for _, guest := range g.guests {
		room := g.rooms[guest.RoomIndex]
		if time.Since(g.departuresStarted).Seconds() < guest.SpawnAt {
			remaining = append(remaining, guest)
			continue
		}
		switch guest.State {
		case entities.GuestLeavingRoom:
			// Door -> central open wall gap -> counter. No post-payment upper-hall route.
			px, py := g.layout.PassageCenter()
			arrivalRoute := g.roomDoorRoute(room, px, py)
			points := make([][2]float64, len(arrivalRoute), len(arrivalRoute)+1)
			for i := range arrivalRoute {
				points[i] = arrivalRoute[len(arrivalRoute)-1-i]
			}
			// Clear the wall gap vertically before crossing the open hotel hall.
			points = append(points, [2]float64{px, cy}, [2]float64{cx, cy})
			if g.moveGuest(guest, points[guest.Waypoint][0], points[guest.Waypoint][1], 1.9) {
				guest.Waypoint++
				if guest.Waypoint == len(points) {
					guest.State = entities.GuestPaying
					guest.PauseUntil = time.Now().Add(time.Second).Sub(g.dayStarted).Seconds()
				}
			}
		case entities.GuestPaying:
			if time.Since(g.dayStarted).Seconds() >= guest.PauseUntil {
				g.pickups = append(g.pickups, Money{X: cx, Y: cy + 20})
				guest.State = entities.GuestDeparting
				guest.Waypoint = 0
			}
		case entities.GuestDeparting:
			// Direct counter-to-exit trajectory after dropping the payment.
			if g.moveGuest(guest, ex, ey+22, 2.2) {
				continue
			}
		}
		remaining = append(remaining, guest)
	}
	g.guests = remaining
	if len(g.guests) == 0 {
		g.state = StateBooking
		g.rooms = g.layout.Rooms
		g.assignments = make([]int, len(g.rooms))
		g.confirmed = make([]bool, len(g.rooms))
		for i := range g.assignments {
			g.assignments[i] = i
		}
		g.message = "New shift ready. Approach reception to book guests."
	}
}
func (g *Game) moveGuest(guest *entities.Guest, x, y, speed float64) bool {
	oldX, oldY := guest.X, guest.Y
	reached := guest.WalkTo(x, y, speed)
	// Routes deliberately cross the gap; solid wall tiles still stop every guest.
	if g.layout.IsBlocked(guest.X, guest.Y, g.rooms) {
		guest.X, guest.Y = oldX, oldY
		return false
	}
	g.resolveGuestCollisions(guest)
	if g.layout.IsBlocked(guest.X, guest.Y, g.rooms) {
		guest.X, guest.Y = oldX, oldY
		return false
	}
	return reached
}
func (g *Game) resolvePlayerCollisions() {
	for _, guest := range g.guests {
		if guest.State != entities.GuestQueued {
			g.separate(&g.player.X, &g.player.Y, entities.PlayerRadius, &guest.X, &guest.Y, entities.GuestRadius)
		}
	}
}
func (g *Game) resolveGuestCollisions(active *entities.Guest) {
	g.separate(&active.X, &active.Y, entities.GuestRadius, &g.player.X, &g.player.Y, entities.PlayerRadius)
	for _, other := range g.guests {
		if other != active && other.State != entities.GuestQueued {
			g.separate(&active.X, &active.Y, entities.GuestRadius, &other.X, &other.Y, entities.GuestRadius)
		}
	}
}
func (g *Game) separate(ax, ay *float64, ar float64, bx, by *float64, br float64) {
	dx, dy := *ax-*bx, *ay-*by
	d := math.Hypot(dx, dy)
	min := ar + br
	if d >= min {
		return
	}
	if d < .01 {
		dx, d = 1, 1
	}
	push := (min - d) / 2
	*ax += dx / d * push
	*ay += dy / d * push
	*bx -= dx / d * push
	*by -= dy / d * push
}
func (g *Game) updateMoney() {
	kept := g.pickups[:0]
	for _, coin := range g.pickups {
		dx, dy := g.player.X-coin.X, g.player.Y-coin.Y
		d := math.Hypot(dx, dy)
		if d < 80 && d > .1 {
			coin.X += dx / d * math.Min(5, 1+d*.12)
			coin.Y += dy / d * math.Min(5, 1+d*.12)
		}
		if d < entities.PlayerRadius+8 {
			g.money += 25
			g.message = "Payment collected."
			continue
		}
		kept = append(kept, coin)
	}
	g.pickups = kept
}
func (g *Game) Layout(_, _ int) (int, int) { return 960, 540 }
func rect(s *ebiten.Image, x, y, w, h int, c color.Color) {
	ebitenutil.DrawRect(s, float64(x), float64(y), float64(w), float64(h), c)
}
func text(s *ebiten.Image, v string, x, y int, _ color.Color) { ebitenutil.DebugPrintAt(s, v, x, y) }
func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(assets.Night)
	if g.state == StateTitle {
		g.drawTitle(screen)
		return
	}
	g.drawWorld(screen)
	if g.state == StateTerminal {
		g.drawTerminal(screen)
	}
	text(screen, g.message, 28, 518, assets.Accent)
	text(screen, fmt.Sprintf("$%02d", g.money), 880, 518, assets.Money)
}
func (g *Game) drawTitle(s *ebiten.Image) {
	text(s, "G H O S T   H O T E L", 345, 195, assets.Ghost)
	text(s, "[ SPACE ] begin shift", 370, 335, assets.Money)
	text(s, "WASD move / SPACE interact / ESC title or quit", 275, 485, assets.Floor)
}
func (g *Game) drawWorld(s *ebiten.Image) {
	rect(s, 25, 35, 910, 475, assets.Wall)
	rect(s, 35, 45, 890, 455, assets.Floor)
	for x := 35; x < 925; x += 32 {
		rect(s, x, 45, 1, 455, assets.FloorLine)
	}
	for y := 45; y < 500; y += 32 {
		rect(s, 35, y, 890, 1, assets.FloorLine)
	}
	for i, r := range g.rooms {
		g.drawRoom(s, r, i)
	}
	for _, wall := range g.layout.Walls {
		rect(s, wall.Min.X, wall.Min.Y, wall.Dx(), wall.Dy(), assets.Ink)
	}
	rect(s, g.layout.Counter.Min.X, g.layout.Counter.Min.Y, g.layout.Counter.Dx(), g.layout.Counter.Dy(), assets.Ink)
	rect(s, g.layout.Counter.Min.X+8, g.layout.Counter.Min.Y+8, g.layout.Counter.Dx()-16, g.layout.Counter.Dy()-16, assets.Accent)
	text(s, "RECEPTION", 165, 415, assets.Ink)
	rect(s, g.layout.Exit.Min.X, g.layout.Exit.Min.Y, g.layout.Exit.Dx(), g.layout.Exit.Dy(), assets.Ink)
	rect(s, g.layout.Exit.Min.X+10, g.layout.Exit.Min.Y+5, g.layout.Exit.Dx()-20, 23, assets.Accent)
	text(s, "EXIT", 457, 475, assets.Ghost)
	for _, coin := range g.pickups {
		rect(s, int(coin.X)-6, int(coin.Y)-4, 12, 8, assets.Money)
	}
	for _, guest := range g.guests {
		if guest.State != entities.GuestQueued {
			g.drawPerson(s, guest.X, guest.Y, assets.Guest)
		}
	}
	g.drawPerson(s, g.player.X, g.player.Y, assets.Ghost)
	cx, cy := g.layout.CounterCenter()
	if g.state == StateBooking && g.player.Near(cx, cy, 100) {
		text(s, "[SPACE] OPEN TERMINAL", 110, 360, assets.Money)
	}
	if g.state == StateDay {
		left := dayDuration - time.Since(g.dayStarted)
		text(s, fmt.Sprintf("DAY %02ds", int(left.Seconds())+1), 430, 350, assets.Money)
	}
}
func (g *Game) drawRoom(s *ebiten.Image, r world.Room, _ int) {
	b := r.Bounds
	rect(s, b.Min.X, b.Min.Y, b.Dx(), b.Dy(), assets.Ink)
	rect(s, b.Min.X+8, b.Min.Y+8, b.Dx()-16, b.Dy()-16, assets.Wall)
	rect(s, b.Min.X+65, b.Min.Y+42, b.Dx()-130, 40, color.RGBA{115, 80, 82, 255})
	door, label := assets.Accent, "OPEN"
	if r.Locked {
		door, label = assets.Locked, "LOCKED"
	}
	rect(s, r.Door.Min.X, r.Door.Min.Y, r.Door.Dx(), r.Door.Dy(), door)
	text(s, r.Name, b.Min.X+165, b.Min.Y+24, assets.Ghost)
	text(s, label, r.Door.Min.X+6, r.Door.Max.Y+12, door)
}
func (g *Game) drawTerminal(s *ebiten.Image) {
	rect(s, 220, 85, 520, 340, assets.Ink)
	rect(s, 228, 93, 504, 324, assets.Wall)
	text(s, "RECEPTION TERMINAL", 365, 125, assets.Ghost)
	text(s, "W/S SELECT   A/D UNLOCKED ROOM   SPACE LOCK/UNLOCK", 255, 155, assets.Accent)
	for i, room := range g.assignments {
		mark, status := " ", "UNBOOKED"
		if i == g.terminalCursor {
			mark = ">"
		}
		if g.confirmed[i] {
			status = "BOOKED"
		}
		text(s, fmt.Sprintf("%s GUEST %d -> %s  [%s]", mark, i+1, g.rooms[room].Name, status), 280, 205+i*42, assets.Money)
	}
	mark := " "
	if g.terminalCursor == len(g.assignments) {
		mark = ">"
	}
	text(s, mark+" BEGIN ARRIVALS", 280, 375, assets.Ghost)
}
func (g *Game) drawPerson(s *ebiten.Image, x, y float64, c color.Color) {
	rect(s, int(x)-11, int(y)-14, 22, 28, c)
	rect(s, int(x)-6, int(y)-21, 12, 10, c)
}
