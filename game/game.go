package game

import (
	"image"
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"ghosthotel/assets"
	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	StateTitle = iota
	StatePlay
	StateTerminal
	StateView  // the hotel view: the world in slow motion, the camera pulled back (hotelview.go)
	StatePause // ESC with nothing open: Resume / Quit to title
	StateGrow  // the hotel grows: the world waits for the cinematic (grow.go)
	StateTalk  // the talk (talkmove.go, 07 X2): closed in on the Concierge and a guest
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
	steered       map[*entities.Guest]steerMemo // steerRoute
	state         int
	hotel         *world.Hotel
	types         []world.RoomType // every loaded Room Type, drawn from at each Run start
	ts            *world.Tileset
	cam           Camera
	player        entities.Player
	parties       []*Party // every Party on the map
	tick          int      // simulation frames since launch
	clock         int      // game time (clock.go)
	fast          bool     // T held: the world runs ×fastScale (clock.go)
	wallet        [2]Num   // Money and Ecto (wallet.go, D178)
	ghost         bool     // the whole hotel is in the ghost dimension (D141); else the real one
	mood          []int    // per Room: its Mood, -100..100 (mood.go)
	serves        int      // requests answered this Run (serve.go)
	expired       int      // requests left to expire this Run
	priests       []*priest
	shades        []shades // per Room: its Shadow People (D169, D185, shadow.go)
	termCursor    int      // the Terminal's selected Room
	termPick      int      // the store's selected row in its tab, or in the cart (the last row is checkout)
	walkIns       int      // Walk-ins this Run
	floaters      []floater
	locked        map[Unlock]bool  // nil: everything unlocked (unlock.go)
	pantryStock   []map[Unlock]int // per pantry, its Supplies (pantry.go)
	taps          [][taskCount]Num // per Room and Task: taps done (tap.go)
	fog           []float64        // per Room: its veil, 0 clear .. 1 fogged (fog.go)
	rng           *rand.Rand
	chat          *rand.Rand                     // picks Guests' lines, apart from rng so talk never changes the simulation (D165)
	linger        struct{ room, from, last int } // the Concierge's visit to an occupied Room: which, since when, and the last frame of it (party.go's intrude)
	words         *rand.Rand                     // the lines themselves (2026-11-14, 4c): chat came to roll ghosts' bookings and pastimes too, so a text edit moved the sim
	in            input                          // the keys (input.go)
	headless      bool                           // no window: step skips what only drawing needs
	auto          *autopilot                     // a Strategy plays instead of a person (sim.go)
	tiles         []*ebiten.Image                // by world.TileID
	characters    map[string][2]*ebiten.Image    // idle_anim, run sheets by name, a Guest's recolored on first use (draw.go)
	bases         map[string][2]*image.RGBA      // each character's sheets as drawn, to recolor (D166)
	floorImg      *ebiten.Image                  // the floor layer pre-rendered at 1px per world px
	props         []prop                         // every other static tile, by SortY
	dark          *ebiten.Image                  // the darkness layer, the size of what drawWorld draws onto (light.go)
	pool, glow    *ebiten.Image                  // light sprites: a ceiling light's pool, the Terminal's glow
	palette       *ebiten.Shader                 // the ghost dimension's arcade palette (ghost.go)
	transition    *ebiten.Shader                 // the Mirror's transitions (shift.go)
	fx            [2]*ebiten.Image               // a transition's real and ghost pictures
	shifting      *shift                         // the Mirror's transition under way, nil if none (D170)
	ink           *ebiten.Image                  // scratch for tinted text (fear.go)
	balloon       *ebiten.Image                  // a Guest's request (serve.go)
	tail          *ebiten.Image                  // the balloon's tail, pointing down
	boxes         map[int]*ebiten.Image          // speech balloons' boxes, by width in balloon px (serve.go)
	warm          bool                           // the shaders have been drawn once (shift.go warmUp)
	spill         *ebiten.Image                  // the hall light through the Archway, built on first use (spill.go)
	hud           [2]counter                     // per Res: its counter on screen (hud.go)
	skull         *ebiten.Image                  // the Ecto counter's icon (hud.go)
	termTab       Tab                            // the store's open tab (shop.go)
	placing       int                            // the Service row waiting for its Room, or -1 (terminal.go)
	cart          []cartLine                     // the store's cart (shop.go)
	stock         map[Unlock]int                 // Supplies in the hotel, by ID (shop.go; E8 delivers, E5 fetches)
	earned        [2]Num                         // lifetime earnings this Run (wallet.go): open store items
	boughtAt      map[Unlock][2]Num              // per unlock: earned when it arrived (shop.go onSale)
	seen          map[Unlock]bool                // per Run: store items the open Terminal showed fresh (the "!", hud.go)
	icons         map[string]*ebiten.Image       // assets/sprites, loaded on first use (terminal.go)
	couriers      []*courier                     // on their way with an order (courier.go)
	walls         []wallRun                      // the wall graph Shadows travel (D185, wall.go)
	wallCell      []int                          // each map cell's run, -1 if none (wall.go)
	onRun         [][]*shade                     // this frame's Shadows by run, for drawShadow
	dirt          [][]spot                       // per Room: its dirt spots (dirt.go)
	dirtN         []int                          // per Room: spots ever placed, the next one's seed
	byGhost       bool                           // the spots being placed are a ghost's doing: spiritual (mood.go's ghostDirt)
	wiped         []wipe                         // cleaned spots, fading
	arrivals      []arrival                      // per Room: its next Party's arrival (dirt.go)
	stains        map[image.Point]*ebiten.Image  // dirt spots' stains, by cell (dirt.go)
	guestNote     slot                           // the next-guest notice in the left column (hud.go)
	guestText     string                         // its text, kept while it slides out
	hands         Unlock                         // the supply the Concierge carries, "" for none (supply.go)
	delivered     Num                            // Money deliveries paid this Run (supply.go; the sim's money check)
	hushed        map[int]bool                   // Rooms whose ghost Shadow Father M banished, until its ghost leaves (shadow.go)
	cover         *ebiten.Image                  // the covered Mirror's sprite (ghostguest.go)
	mirrorAtStart bool                           // the sim's -mirror: a Run starts with the Mirror unlocked (startRun)
	ghostArrivals []arrival                      // per Room: its next ghost Party's arrival (ghostguest.go)
	areaShades    []*shade                       // the Shadow People strong ghosts switch on, one per area (shadow.go)
	roomOwned     []bool                         // per Room: owned, so open for business; nil owns all (owned.go, 07 B1)
	openAtStart   bool                           // the sim's Open and most tests: a Run starts with every Room owned
	introOn       bool                           // a Run starts with the opening intro (intro.go); off for the sim, the tests and prints
	intro         *intro                         // the intro under way, nil if none
	wide          *ebiten.Image                  // the whole hotel at zoom, the intro's first shots (intro.go)
	changes       []*change                      // Rooms' changes of type, waiting or rolling (roomtype_change.go, 07 K1)
	termBeds      int                            // the bed layout picked for a change at the Terminal, an index into bedKinds
	bedsAtStart   string                         // the sim's Beds: a Run starts with every Room of this bed layout; "" is Single
	termDepth     int                            // how far into the hotel view: 1 the hotel, 2 a Room picked, 3 its ring, 4 its beds (hotelview.go)
	viewT         float64                        // the hotel view's pull-back, 0 the play camera .. 1 the whole hotel
	pausePick     int                            // the pause's picked row
	growNext      int                            // the expansion paid for, played once the Terminal is closed: growCol or growFloor (grow.go, 07 H1)
	growing       *growth                        // its cinematic under way, nil if none
	grid0         [2]int                         // the Room grid a Run starts with, cols and rows: a grown hotel is rebuilt to it
	note          notice                         // the HUD's text notice (hud.go, 07 U2): the only place for words to the player; the game says none today
	keepers       []*housekeeper                 // the housekeepers on the map (keeper.go, 07 H1 step 3)
	staffPaid     Num                            // Money paid to hire and keep them this Run (the sim's money check)
	hired, quits  int                            // housekeepers hired, and walked out unpaid, this Run
	busy          bool                           // the busy hotel, a stress Run (busy.go)
	bumps, paces  int                            // frames a walker was stopped by someone ahead, and frames walked, this Run (busy.go)
	refused       string                         // why the last SPACE did nothing, "" when it did: the View's, never drawn (view.go refuse)
	parley        *parley                        // the talk under way (talkmove.go, 07 X2)
	talkCue       slot                           // TALK on the HUD's right (hud.go)
	offered       Num                            // Money paid for offers this Run (the sim's money check)
	focus         *ebiten.Shader                 // the talk's close-in and blur (talkmove.go)
	scale         float64                        // the world time scale (D1, clock.go): 1 normal, under 1 slow motion
	dt            float64                        // this step's frames of world time, at most 1; a whole frame (and ticked) outside a step, for the tests
	acc           float64                        // world time since the last whole frame, 0..1
	ticked        bool                           // a whole frame of world time ended this step: its per-frame logic runs
	recent        []earning                      // the last game hour's earnings: the Terminal's rates (wallet.go)
	viewIn        float64                        // the hotel view's camera from the whole hotel (0) to the picked Room (1), eased (hotelview.go)
	shake         int                            // frames left of the Room ring's "no" shake (hotelview.go)
	mirrorSlots   []arrival                      // per Room with a mirror: its second ghost's arrival (ghostguest.go, 08 B1)
	guestAge      int                            // frames the next-guest notice has shown its text: it holds noticeFrames, like the feed (08 B1 live test 5)
	reel          *prestigeReel                  // Father P's cutscene while it plays (prestige_reel.go)
	level         int                            // prestige level: the max grid is (3+level)² (08 P1, prestige.go)
	bonus         float64                        // the permanent income multiplier, 1 at first, cumulative over prestiges (earn, wallet.go)
	runEcto       Num                            // Ectoplasm earned this Run, before the bonus: grows the bonus at a prestige
	prestigeAsk   bool                           // the Terminal's Father P confirm card is open (prestige.go)
	persist       bool                           // this Game reads and writes the save (save.go): only main's plain start sets it; dev starts, prints, tests and the sim never do
	baked         map[bakeKey]*ebiten.Image      // each sprite's ghost-dimension look, painted once (ghost.go)
	coverHalves   [2]*ebiten.Image               // coverHalf's two tiles, kept for blit's bake
	bakes         int                            // looks baked in the frame being drawn (ghost.go bakeBudget)
	sheets        bakeSheets                     // the pages the looks are baked on (ghost.go)
	propSheet     propSheet                      // every prop's picture on one image, and the run of props being gathered (props.go)
	propShader    *ebiten.Shader                 // draws a run of props: fog, dread, the ghost palette (props.go)
	wideBuf       *ebiten.Image                  // g.wide's texture, kept across growths (props.go reuse)
	darkBuf       *ebiten.Image                  // g.dark's
	floorBuf      *ebiten.Image                  // g.floorImg's
}

// New builds the shipped grid (world.Cols x world.Rows), played on the keyboard.
func New() (*Game, error) { return NewSeed(rand.Uint64()) }

// NewSeed is New with the Run's seed given (dev prints: the same hotel again).
func NewSeed(seed uint64) (*Game, error) {
	g, err := newWorld(world.Cols, world.Rows, seed)
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
	h, err := world.Build(ts, types, cols, rows)
	if err != nil {
		return nil, err
	}
	g := &Game{state: StateTitle, hotel: h, grid0: [2]int{cols, rows}, types: types, rng: rng, chat: rand.New(rand.NewPCG(seed, seed+1)), words: rand.New(rand.NewPCG(seed, seed+2)), ts: ts, in: &script{}, bonus: 1, headless: true, scale: 1, dt: 1, ticked: true}
	if g.transition, err = ebiten.NewShader([]byte(transitionSrc)); err != nil { // now, not on the first flip (D190)
		return nil, err
	}
	if g.palette, err = ebiten.NewShader([]byte(paletteSrc)); err != nil {
		return nil, err
	}
	g.propShader, err = ebiten.NewShader([]byte(propSrc))
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
	g.characters[fatherP.name] = dress(g.bases["Adam"], palettes["Adam"], fatherP.look)
	g.loadCourier()
	g.loadKeeper()
	return nil
}

func (g *Game) mapSize() (int, int) {
	return g.hotel.Map.W * world.TileSize, g.hotel.Map.H * world.TileSize
}

// startRun begins a Run (D72): Day 1 at 08:00, every Room Vacant and a Standard Single (K1: the Run-start
// draw is gone; Rooms change type one by one, roomtype_change.go), no money.
func (g *Game) startRun() {
	g.note = notice{} // a Run starts on a clean feed (a load's broken-save notice comes after, in beginIntro)
	g.ungrow()
	g.parties, g.floaters, g.walkIns, g.fog, g.priests = nil, nil, 0, nil, nil
	g.mood, g.shades, g.serves, g.expired = newMoods(len(g.hotel.Rooms)), make([]shades, len(g.hotel.Rooms)), 0, 0
	g.clock, g.wallet, g.ghost, g.shifting = runStart, [2]Num{}, false, nil
	g.taps = make([][taskCount]Num, len(g.hotel.Rooms))
	g.cart, g.stock, g.earned, g.boughtAt, g.placing = nil, nil, [2]Num{}, nil, -1
	g.seen = nil
	g.state = StatePlay
	first, ok := world.TypeOf(g.types, g.bedsAtStart)
	if !ok {
		first, _ = world.TypeOf(g.types, "Single")
	}
	for i := range g.hotel.Rooms {
		g.hotel.Land(i, first, false)
	}
	for _, c := range g.changes {
		c.pics.free()
	}
	g.changes, g.termBeds = nil, 0
	g.floorImg = nil // re-rendered on the next Draw
	g.player = entities.NewPlayer(g.hotel.ConciergeStart)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
	g.hud, g.guestNote, g.guestText = [2]counter{}, slot{}, ""
	g.couriers, g.hands, g.delivered = nil, "", 0
	g.keepers, g.staffPaid, g.hired, g.quits = nil, 0, 0, 0
	g.busy, g.bumps, g.paces = false, 0, 0
	g.walls, g.wallCell = wallRuns(g.hotel.Map)
	g.dirt, g.dirtN, g.wiped, g.arrivals, g.stains = nil, nil, nil, g.firstArrivals(), nil // stains clip to this Run's furniture
	g.ghostArrivals, g.areaShades = g.firstArrivals(), nil
	g.hushed = map[int]bool{}
	// a normal Run starts with the Mirror locked: humans only, until it's bought and delivered (07 G1, B1's
	// T272); and with K1's bed layouts and in-Room Mirror to buy
	g.locked = map[Unlock]bool{Mirror: !g.mirrorAtStart, BedsTwin: true, BedsDouble: true, RoomMirror: true, Deluxe: true}
	g.locked[Matchmaking], g.parley, g.offered = true, nil, 0 // 07 X2
	g.locked[unlockK], g.locked[unlockM] = true, true         // 08 B1-P: the Fathers' calls
	g.locked[Pantry] = true                                   // 08 P1-U
	g.roomOwned = nil                                         // 07 B1: a normal Run starts with every Room closed and Money for one key
	if !g.openAtStart {
		g.closeRooms()
	}
	g.beginIntro()
}

func (g *Game) Update() error {
	if g.auto != nil {
		g.auto.drive(g)
	}
	if g.state == StateView || g.growing != nil && g.growing.t < 1 { // the hotel view's camera: out while it is open (or the hotel grows), back in once closed
		g.viewT = min(1, g.viewT+1.0/viewFrames)
	} else {
		g.viewT = max(0, g.viewT-1.0/viewFrames)
	}
	if g.reel == nil && g.in.JustPressed(ebiten.KeyEscape) { // closes what is open, in one press; with nothing open it pauses (U1)
		switch {
		case g.state == StateTitle:
			return ebiten.Termination
		case g.prestigeAsk: // Father P's card closes first
			g.prestigeAsk = false
		case g.state == StateGrow: // it plays out
		case g.state == StateTalk: // the world gets its pace back (talkmove.go, X2)
			g.endTalk()
		case g.state == StateView && g.termDepth > 1: // the hotel view steps back one level (08 F0)
			g.termDepth--
			g.refuse("")
		case g.state != StatePlay:
			g.state = StatePlay
			g.refuse("")
		case g.intro != nil || g.auto != nil: // the opening, or a Strategy playing: back to the title
			g.toTitle()
		default:
			g.state, g.pausePick = StatePause, 0
		}
		return nil
	}
	if g.growNext != 0 && g.state == StatePlay && g.shifting == nil && g.intro == nil { // paid for, the Terminal closed
		g.beginGrow()
	}
	switch {
	case g.reel != nil: // Father P's cutscene: no input but SPACE to skip (prestige_reel.go)
		g.updateReel()
	case g.intro != nil: // the opening, the title included: the world waits for it
		g.updateIntro()
	case g.state == StateGrow:
		g.updateGrow()
	case g.state == StateTitle:
		if g.in.JustPressed(ebiten.KeySpace) {
			g.startRun()
		}
	case g.state == StateTerminal: // the world and the Clock wait (D73)
		g.updateTerminal()
		g.updateHUD()
	case g.state == StateView:
		g.updateView()
		g.updateHUD()
	case g.state == StatePause:
		g.updatePause()
	case g.state == StateTalk: // the world runs on in slow motion, stepped by updateParley (X2, D1)
		g.updateParley()
		g.updateHUD()
	default:
		g.fast = g.in.Pressed(ebiten.KeyT)
		g.step(move(g.in))
		g.updateShift()
		if g.in.JustPressed(ebiten.KeySpace) {
			g.space() // what actionable says (actionable.go)
		}
	}
	return nil
}

// step advances the world one real frame, by the time scale (clock.go), with the Concierge walking in
// direction (dx,dy). No input, no rendering: the headless sim drives it directly.
func (g *Game) step(dx, dy float64) {
	start := time.Now()
	for left := g.timeScale(); left > 0; left-- {
		g.advance(dx, dy, min(left, 1))
		if !g.headless && time.Since(start) > stepBudget { // a window never spends more of a frame than this on the world, whatever the scale: it runs slower instead
			break
		}
	}
	if !g.headless {
		w, h := g.mapSize()
		g.cam.Follow(g.player.X, g.player.Y, w, h)
	}
	g.updateHUD()
}

// stepBudget: the most of a real frame (16.7 ms) a window's step spends on the world.
const stepBudget = 4 * time.Millisecond

// advance runs dt (at most 1) frames of world time.
func (g *Game) advance(dx, dy, dt float64) {
	g.dt, g.acc = dt, g.acc+dt
	if g.ticked = g.acc >= 1; g.ticked {
		g.acc--
		g.tick++
		g.advanceClock()
		if g.clock%dayFrames == runStart { // dawn: the checkpoint (save.go)
			g.saveDawn()
		}
	}
	g.player.Move(dx, dy, conciergeSpeed*dt, g.conciergeBlocked)
	g.hotel.UpdateDoors(g.doorActors(), g.boarded, dt)
	g.updateChanges()
	g.updateParties()
	g.updateTalk()
	g.updatePriests()
	g.updateShadows()
	g.separate()
	g.updateFog()
	g.updateCouriers()
	g.updateKeepers()
	if g.busy {
		g.busyStep()
	}
	if !g.headless {
		g.player.Anim.Update(g.player.X, g.player.Y, dt)
		for _, guest := range g.guests() {
			guest.Anim.Update(guest.X, guest.Y, dt)
		}
		for _, pr := range g.priests {
			pr.Anim.Update(pr.X, pr.Y, dt)
		}
	}
	g.floaters = slices.DeleteFunc(g.floaters, func(f floater) bool { return g.tick-f.at > floaterFrames })
}

// blocked: a foot box would hit a wall or a shut door.
func (g *Game) blocked(r image.Rectangle) bool {
	return g.hotel.Map.BlockedBox(r) || g.hotel.DoorBlocks(r)
}

// conciergeBlocked: blocked, but for a bed's side, which lets him lie in it (D167). He walks into any open
// Room, its guests in or not (K1, 2026-11-11: D91's shut doorway is gone); only a boarded door stays shut.
func (g *Game) conciergeBlocked(r image.Rectangle) bool { return g.blocked(r) && !g.inLane(r) }

// separate pushes overlapping foot boxes apart along their shallower overlap, split by each actor's
// give (D22b). A push into a wall or a shut door is dropped. Phasing guests are skipped (D22c).
func (g *Game) separate() {
	for _, realm := range [...]Realm{Human, Ghost} {
		g.separateIn(realm)
	}
}

// separateIn: separate for the actors of one realm; the Concierge is in the one he stands in.
func (g *Game) separateIn(realm Realm) {
	type body struct {
		x, y    *float64
		give    float64
		blocked func(image.Rectangle) bool
		guest   *entities.Guest // nil: the Concierge
	}
	var bodies []body
	if g.realm() == realm {
		bodies = append(bodies, body{&g.player.X, &g.player.Y, conciergeGive, g.conciergeBlocked, nil})
	}
	for guest := range g.walkers(realm) {
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

// doorActors: everyone on the map the doors react to, of either realm: a door is one structure. The staff
// (the Concierge, a Priest, a courier) open any door, a Guest its own.
func (g *Game) doorActors() []world.Actor {
	as := []world.Actor{{Box: entities.FootBox(g.player.X, g.player.Y), Room: -1}}
	for _, guest := range g.guests() {
		as = append(as, world.Actor{Box: entities.FootBox(guest.X, guest.Y), Room: guest.RoomIndex})
	}
	for _, pr := range g.priests {
		as = append(as, world.Actor{Box: entities.FootBox(pr.X, pr.Y), Room: -1})
	}
	as = append(as, g.courierActors()...)
	as = append(as, g.keeperActors()...)
	return as
}

// boarded: the Room isn't owned yet: its door stays shut to everyone (07 B1). An owned Room's door opens to
// the Concierge whoever is in; his minutes in there with its Guests cost the Room Mood (intrude).
func (g *Game) boarded(room int) bool { return !g.isOwned(room) }

// walk advances a guest along its path. It waits in place while the next step would hit a shut door,
// and sidesteps to its right (left if a wall is there) while the next step would bump into someone
// ahead, so two guests meeting head-on pass each other.
func (g *Game) walk(guest *entities.Guest, speed float64) bool {
	if g.ticked {
		guest.TrackProgress(stuckFrames) // measures last frame's walk and pushes
	}
	g.steer(guest)
	if !guest.Phasing && g.yields(guest) {
		return false
	}
	x, y, path := guest.X, guest.Y, guest.Path
	if len(path) > 0 {
		g.paces++
	}
	done := guest.FollowPath(speed * g.dt)
	if g.hotel.DoorBlocks(entities.FootBox(guest.X, guest.Y)) {
		guest.X, guest.Y, guest.Path = x, y, path
		return false
	}
	hx, hy := guest.X-x, guest.Y-y
	if bx, by, bump := g.bumpsAhead(guest, hx, hy); !guest.Phasing && bump {
		guest.X, guest.Y, guest.Path = x, y, path
		g.bumps++
		// A Party waiting in the hall stays for hours: go around it (dirt.go). A courier goes around anyone:
		// he is faster than a Party walking in and tailgated it up the Corridor (Live test 7, 2026-11-08).
		if staff(guest) && len(path) > 0 {
			// The staff never stop for anyone (2026-11-14): they steer around whoever is ahead before they
			// meet (steer); one they still run into, with no way around, they pass through.
			guest.Phase()
			return false
		}
		if g.waiting(bx, by) && len(path) > 0 {
			g.send(guest, path[len(path)-1], image.Pt(int(bx), int(by)))
			return false
		}
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

// staff: a courier or a housekeeper: they have somewhere to be.
func staff(guest *entities.Guest) bool {
	return guest.Character == courierName || guest.Character == keeperName
}

// steerAhead: the waypoints ahead a member of staff watches; steerEvery: frames between its looks.
const steerAhead, steerEvery = 3, 6

// steerMax: the detours one errand allows; past it the walker keeps its way, as before it could steer.
const steerMax = 40

// steer: a member of staff looks ahead as it walks: someone on the next cells of its way, it takes another way
// round them now, well before they meet. With no other way it walks on (walk lets it through).
func (g *Game) steer(guest *entities.Guest) {
	if len(guest.Path) == 0 || guest.Phasing || !g.ticked || g.tick%steerEvery != 0 || guest.Detours() >= steerMax {
		return
	}
	m := g.hotel.Map
	here := m.Cell(image.Pt(int(guest.X), int(guest.Y)))
	ahead := guest.Path[:min(len(guest.Path), steerAhead)]
	var avoid []image.Point
	see := func(x, y float64) {
		at := image.Pt(int(x), int(y))
		if c := m.Cell(at); c != here && slices.ContainsFunc(ahead, func(p image.Point) bool { return m.Cell(p) == c }) {
			avoid = append(avoid, at)
		}
	}
	realm := g.realmOf(guest)
	if g.realm() == realm {
		see(g.player.X, g.player.Y)
	}
	for o := range g.walkers(realm) {
		if o == guest || o.Phasing || o.Mate(guest) {
			continue
		}
		// Two walking at each other: only one goes round, the other keeps its way (2026-11-14: two housekeepers
		// at the kitchen each took the other's side, again and again, and never passed).
		if (guest.X < o.X || guest.X == o.X && guest.Y < o.Y) && slices.ContainsFunc(o.Path[:min(len(o.Path), steerAhead)], func(p image.Point) bool { return m.Cell(p) == here }) {
			continue
		}
		see(o.X, o.Y)
	}
	if len(avoid) == 0 {
		return
	}
	if way := g.steerRoute(guest, here, avoid); way != nil {
		guest.Detour(way)
	}
}

// steerMemo: a walker's last way round the ones ahead. One held up looks again every steerEvery frames from the
// same cell for the same goal around the same ones: the way is the same until the map changes.
type steerMemo struct {
	h           *world.Hotel
	rev, n, own int // the map's Rev and the Rooms' count when it was found, and whose Room door is open
	here, to    image.Point
	avoid       [4]image.Point // cells
	nAvoid      int
	way         []image.Point
}

func (g *Game) steerRoute(guest *entities.Guest, here image.Point, avoid []image.Point) []image.Point {
	m := g.hotel.Map
	k := steerMemo{h: g.hotel, rev: m.Rev, n: len(g.hotel.Rooms), own: guest.RoomIndex, here: here, to: guest.Path[len(guest.Path)-1], nAvoid: len(avoid)}
	if len(avoid) > len(k.avoid) {
		return g.route(guest, k.to, avoid...)
	}
	for i, p := range avoid {
		k.avoid[i] = m.Cell(p)
	}
	if old, ok := g.steered[guest]; ok && old.h == k.h && old.rev == k.rev && old.n == k.n && old.own == k.own && old.here == k.here && old.to == k.to && old.avoid == k.avoid && old.nAvoid == k.nAvoid {
		return slices.Clone(old.way)
	}
	if k.way = g.route(guest, k.to, avoid...); k.way == nil {
		return nil
	}
	if g.steered == nil || len(g.steered) > 1024 { // ponytail: wiped now and then, not per departed guest
		g.steered = map[*entities.Guest]steerMemo{}
	}
	g.steered[guest] = k
	return slices.Clone(k.way)
}

// yieldAhead: how many waypoints ahead a guest looks for a choke point. It waits a cell or two short of
// it, which leaves the way out clear.
const yieldAhead = 3

// yields (D89): a guest about to pass a 1-cell choke point (a Room doorway) waits while another guest
// coming the other way is on that stretch and nearer the choke: the nearer one goes first, so two never
// wait on each other. The Concierge is left to the sidestep and the push give (D22b).
func (g *Game) yields(guest *entities.Guest) bool {
	ahead := guest.Path[:min(len(guest.Path), yieldAhead)]
	m := g.hotel.Map
	i := slices.IndexFunc(ahead, func(p image.Point) bool { return g.hotel.Choke(m.Cell(p)) })
	if i < 0 {
		return false
	}
	c := m.Center(m.Cell(ahead[i]).X, m.Cell(ahead[i]).Y)
	near := func(x, y float64) float64 { return math.Hypot(float64(c.X)-x, float64(c.Y)-y) }
	hx, hy := float64(guest.Path[0].X)-guest.X, float64(guest.Path[0].Y)-guest.Y
	for o := range g.walkers(g.realmOf(guest)) {
		if o == guest || o.Mate(guest) || o.Phasing || len(o.Path) == 0 || near(o.X, o.Y) >= near(guest.X, guest.Y) {
			continue
		}
		on := m.Cell(image.Pt(int(o.X), int(o.Y)))
		if slices.ContainsFunc(ahead, func(p image.Point) bool { return m.Cell(p) == on }) &&
			(float64(o.Path[0].X)-o.X)*hx+(float64(o.Path[0].Y)-o.Y)*hy < 0 { // on the stretch, coming the other way
			return true
		}
	}
	return false
}

// bumpsAhead: the guest's foot box overlaps another actor, at (x,y), lying ahead along heading (hx,hy).
// When two walking guests each have the other ahead (converging on one cell), only the one without the
// right of way stops, so they never wait on each other; one stuck for half of stuckFrames is bumped like a
// wall, so the other sidesteps around it (hall jam, 2026-11-08: it was pinned under the Hall Wall).
func (g *Game) bumpsAhead(guest *entities.Guest, hx, hy float64) (x, y float64, bump bool) {
	box := entities.FootBox(guest.X, guest.Y).Inset(-1) // touching counts: boxes snap to whole px, so one could step into a gap and never see the bump
	hit := func(x, y float64) bool {
		return entities.FootBox(x, y).Overlaps(box) && (x-guest.X)*hx+(y-guest.Y)*hy > 0
	}
	realm := g.realmOf(guest)
	if g.realm() == realm && hit(g.player.X, g.player.Y) {
		return g.player.X, g.player.Y, true
	}
	for o := range g.walkers(realm) {
		if o == guest || o.Mate(guest) || o.Phasing || !hit(o.X, o.Y) {
			continue
		}
		if len(o.Path) > 0 && o.Stuck() < stuckFrames/2 && rightOfWay(guest, o) { // a stuck one can't make room: go around it
			ox, oy := float64(o.Path[0].X)-o.X, float64(o.Path[0].Y)-o.Y
			if (guest.X-o.X)*ox+(guest.Y-o.Y)*oy > 0 { // o has this guest ahead too, and waits for it
				continue
			}
		}
		return o.X, o.Y, true
	}
	return 0, 0, false
}

// rightOfWay: guest a goes before b when they bump into each other: one stuck for half of stuckFrames first,
// pinned against a wall or corner where only the other can make room (hall jam, 2026-11-08: a Guest walking
// in was shoved into the hall corner a complaining Party was walking to); else the one nearer its goal.
func rightOfWay(a, b *entities.Guest) bool {
	if sa, sb := a.Stuck() >= stuckFrames/2, b.Stuck() >= stuckFrames/2; sa != sb {
		return sa
	}
	return a.Left() < b.Left()
}

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

// NewGrid is NewSeed on a cols x rows hotel: main's -grid without -play.
func NewGrid(cols, rows int, seed uint64) (*Game, error) {
	g, err := newWorld(cols, rows, seed)
	if err != nil {
		return nil, err
	}
	g.in, g.headless = keyboard{}, false
	return g, g.loadImages()
}
