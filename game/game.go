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
)

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
)

// guestCast: Guests draw a random character from these; repeats are fine (D25). Bob is the Concierge.
var guestCast = [...]string{"Adam", "Amelia", "Alex"}

type Game struct {
	state      int
	hotel      *world.Hotel
	types      []world.RoomType // every loaded Room Type, drawn from at each Run start
	ts         *world.Tileset
	cam        Camera
	player     entities.Player
	parties    []*Party // every Party on the map
	tick       int      // simulation frames since launch
	clock      int      // game time (clock.go)
	fast       bool     // T held: the clock runs ×10
	wallet     [2]Num   // Money and Spook (wallet.go, D178)
	ghost      bool     // the whole hotel is in the ghost dimension (D141); else the real one
	haunted    []int    // per Room: the clock its haunt ends (haunt.go)
	roomSpook  []int    // per Room: its Spook level (D147, priest.go)
	serves     int      // requests answered this Run (serve.go)
	expired    int      // requests left to expire this Run
	priests    []*priest
	shades     []*shade // per Room: its Shadow Person, nil if none (D169, shadow.go)
	termCursor int      // the Terminal's selected Room
	termPick   int      // the store's selected row in its tab, or in the cart (the last row is checkout)
	walkIns    int      // Walk-ins this Run
	floaters   []floater
	locked     map[Unlock]bool  // nil: everything unlocked (unlock.go)
	taps       [][taskCount]Num // per Room and Task: taps done (tap.go)
	fog        []float64        // per Room: its veil, 0 clear .. 1 fogged (fog.go)
	message    string
	rng        *rand.Rand
	chat       *rand.Rand                  // picks Guests' lines, apart from rng so talk never changes the simulation (D165)
	in         input                       // the keys (input.go)
	headless   bool                        // no window: step skips what only drawing needs
	auto       *autopilot                  // a Strategy plays instead of a person (sim.go)
	tiles      []*ebiten.Image             // by world.TileID
	characters map[string][2]*ebiten.Image // idle_anim, run sheets by name, a Guest's recolored on first use (draw.go)
	bases      map[string][2]*image.RGBA   // each character's sheets as drawn, to recolor (D166)
	floorImg   *ebiten.Image               // the floor layer pre-rendered at 1px per world px
	props      []prop                      // every other static tile, by SortY
	dark       *ebiten.Image               // the darkness layer, the size of what drawWorld draws onto (light.go)
	pool, glow *ebiten.Image               // light sprites: a ceiling light's pool, the Terminal's glow
	palette    *ebiten.Shader              // the ghost dimension's arcade palette (ghost.go)
	transition *ebiten.Shader              // the Mirror's transitions (shift.go)
	fx         [2]*ebiten.Image            // a transition's real and ghost pictures
	shifting   *shift                      // the Mirror's transition under way, nil if none (D170)
	ink        *ebiten.Image               // scratch for tinted text (haunt.go)
	heart      *ebiten.Image               // the serve pop (serve.go)
	balloon    *ebiten.Image               // a Guest's request (serve.go)
	tail       *ebiten.Image               // the balloon's tail, pointing down
	boxes      map[int]*ebiten.Image       // speech balloons' boxes, by width in balloon px (serve.go)
	warm       bool                        // the shaders have been drawn once (shift.go warmUp)
	spill      *ebiten.Image               // the hall light through the Archway, built on first use (spill.go)
	hud        [2]counter                  // per Res: its counter on screen (hud.go)
	skull      *ebiten.Image               // the Spook counter's icon (hud.go)
	termTab    Tab                         // the store's open tab (shop.go)
	placing    int                         // the Service row waiting for its Room, or -1 (terminal.go)
	cart       []cartLine                  // the store's cart (shop.go)
	stock      map[Unlock]int              // Supplies in the hotel, by ID (shop.go; E8 delivers, E5 fetches)
	earned     [2]Num                      // lifetime earnings this Run (wallet.go): open store items
	boughtAt   map[Unlock][2]Num           // per unlock: earned when it arrived (shop.go onSale)
	icons      map[string]*ebiten.Image    // assets/sprites, loaded on first use (terminal.go)
	couriers   []*courier                  // on their way with an order (courier.go)
	pallet     *ebiten.Image               // a stock spot's pallet, made on first use (courier.go)
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
	g := &Game{state: StateTitle, hotel: h, types: types, rng: rng, chat: rand.New(rand.NewPCG(seed, seed+1)), ts: ts, in: &script{}, headless: true}
	if g.transition, err = ebiten.NewShader([]byte(transitionSrc)); err != nil { // now, not on the first flip (D190)
		return nil, err
	}
	g.palette, err = ebiten.NewShader([]byte(paletteSrc))
	return g, err
}

func (g *Game) loadImages() (err error) {
	if g.tiles, err = loadTiles(g.ts); err != nil {
		return err
	}
	g.pool, g.glow = lightPool(lightRadius), lightPool(glowRadius)
	if g.bases, err = loadCharacters(append([]string{"Bob"}, guestCast[:]...)...); err != nil {
		return err
	}
	g.characters = map[string][2]*ebiten.Image{}
	for name, b := range g.bases {
		g.characters[name] = dress(b, palettes[name], look{})
	}
	g.characters[fatherK.name] = dress(g.bases["Bob"], palettes["Bob"], fatherK.look)
	g.characters[fatherM.name] = dress(g.bases["Bob"], palettes["Bob"], fatherM.look)
	g.loadCourier()
	return nil
}

func (g *Game) mapSize() (int, int) {
	return g.hotel.Map.W * world.TileSize, g.hotel.Map.H * world.TileSize
}

// startRun begins a Run (D72): Day 1 at 08:00, every Room Vacant and given its Room Type for the Run
// (D135: no Spin after this), no money.
func (g *Game) startRun() {
	g.parties, g.floaters, g.walkIns, g.fog, g.haunted, g.priests = nil, nil, 0, nil, nil, nil
	g.roomSpook, g.shades, g.serves, g.expired = make([]int, len(g.hotel.Rooms)), make([]*shade, len(g.hotel.Rooms)), 0, 0
	g.clock, g.wallet, g.ghost, g.shifting = runStart, [2]Num{}, false, nil
	g.taps = make([][taskCount]Num, len(g.hotel.Rooms))
	g.cart, g.stock, g.earned, g.boughtAt, g.placing = nil, nil, [2]Num{}, nil, -1
	g.state = StatePlay
	g.hotel.Furnish(world.Draw(g.types, len(g.hotel.Rooms), g.rng))
	g.floorImg = nil // re-rendered on the next Draw
	g.player = entities.NewPlayer(g.hotel.ConciergeStart)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
	g.hud = [2]counter{}
	g.couriers = nil
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
			if g.placing >= 0 { // ESC backs out of the Room pick first
				g.placing = -1
				break
			}
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
		g.updateHUD()
	default:
		g.fast = g.in.Pressed(ebiten.KeyT)
		g.step(move(g.in))
		g.updateShift()
		switch {
		case !g.in.JustPressed(ebiten.KeySpace):
		case g.shifting != nil: // SPACE waits for the transition (D170)
		case g.atTerminal():
			g.openTerminal()
		case g.atMirror():
			g.startShift()
		case g.atDoor() < 0 || !g.occupied(g.atDoor()):
		case g.ghost:
			g.haunt(g.atDoor())
		default:
			g.serve(g.atDoor())
		}
	}
	return nil
}

// step advances the world one frame with the Concierge walking in direction (dx,dy). No input, no
// rendering: the headless sim drives it directly.
func (g *Game) step(dx, dy float64) {
	g.tick++
	g.advanceClock()
	g.player.Move(dx, dy, conciergeSpeed, g.conciergeBlocked)
	g.hotel.UpdateDoors(g.actors(), g.occupied)
	g.updateParties()
	g.updateTalk()
	g.updatePriests()
	g.updateShadows()
	g.separate()
	g.updateFog()
	g.updateCouriers()
	if !g.headless {
		g.player.Anim.Update(g.player.X, g.player.Y)
		for _, guest := range g.guests() {
			guest.Anim.Update(guest.X, guest.Y)
		}
		for _, pr := range g.priests {
			pr.Anim.Update(pr.X, pr.Y)
		}
		w, h := g.mapSize()
		g.cam.Follow(g.player.X, g.player.Y, w, h)
	}
	g.floaters = slices.DeleteFunc(g.floaters, func(f floater) bool { return g.tick-f.at > floaterFrames })
	g.updateHUD()
}

// blocked: a foot box would hit a wall or a shut door.
func (g *Game) blocked(r image.Rectangle) bool {
	return g.hotel.Map.BlockedBox(r) || g.hotel.DoorBlocks(r)
}

// conciergeBlocked: blocked, and also every Occupied Room's doorway, open or not (D91), so the Concierge
// can't slip in behind a Party; from inside the Room it lets him out (D21), as when a Party checks into
// the Room he's in. A bed's lane lets him lie in it (D167).
func (g *Game) conciergeBlocked(r image.Rectangle) bool {
	at := image.Pt(int(g.player.X), int(g.player.Y))
	return g.blocked(r) && !g.inLane(r) || slices.ContainsFunc(g.hotel.Doors, func(d world.Door) bool {
		return d.Room >= 0 && d.Rect.Overlaps(r) && !d.Allows(-1, at.In(g.hotel.Rooms[d.Room].Shell()), g.occupied(d.Room))
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
		give := 0.0 // idle guests (staying) stand their ground
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
	for _, pr := range g.priests { // his door pass: his own Room's door, Occupied or not
		as = append(as, world.Actor{Box: entities.FootBox(pr.X, pr.Y), Room: pr.RoomIndex})
	}
	as = append(as, g.courierActors()...)
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

// rightOfWay: guest a goes before b when they bump into each other: the one nearer its goal.
func rightOfWay(a, b *entities.Guest) bool { return a.Left() < b.Left() }

func (g *Game) atTerminal() bool {
	return image.Pt(int(g.player.X), int(g.player.Y)).In(g.hotel.Terminal)
}

func (g *Game) atMirror() bool {
	return image.Pt(int(g.player.X), int(g.player.Y)).In(g.hotel.MirrorSpot)
}

// flip shifts the whole hotel to the other dimension (D141). Only what is drawn and what the Concierge
// can do changes; the hotel runs on the same (D143).
func (g *Game) flip() { g.ghost = !g.ghost }

// Start begins a Run at once, skipping the title, in the ghost dimension if ghost: main's -shot prints.
func (g *Game) Start(ghost bool) {
	g.startRun()
	g.ghost = ghost
}

func (g *Game) Layout(_, _ int) (int, int) {
	g.sizeFx(viewW*zoom, viewH*zoom)
	return viewW * zoom, viewH * zoom
}
