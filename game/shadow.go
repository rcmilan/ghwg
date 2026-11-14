package game

import (
	"cmp"
	"image"
	"math"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// The Shadow Person (D169, D185): a ghost that lives on the walls (wall.go) and patrols its
// Room's back wall. Placeholders.
const (
	shadowSpeed    = 4 * conciergeSpeed
	patrolSpeed    = shadowSpeed // a patrol runs as fast as its travel (Live test 5, 2026-11-08: a crawl read as broken)
	shadowSeen     = 10          // Fear a patrol gives a Guest who sees it
	startleHeight  = 4           // world px: a Guest's hop on seeing it (the flee's is hopHeight)
	shadowEssence  = 24.0        // (placeholder) what it's born with; a patrol pass spends 1
	shadowSunDrain = 1.0 / 60    // (placeholder) essence lost per game minute in full daylight
	shadowOpacity  = 0.5         // (placeholder) a full Shadow's alpha: a dimness on the wall, not a hole (2026-11-08)
	shadowEdgeFade = charW       // (placeholder) world px from a wall's end over which a Shadow fades into it
	shadowMinWall  = 3 * charW   // (placeholder) a narrower wall, such as the Corridor's back wall, never shows a Shadow (2026-11-08)
)

var (
	// sightLines: what a Guest says on seeing a Shadow dash, by its ghost's kind (kindOf: the stronger the
	// ghost, the worse the fright); one at random. Placeholders.
	sightLines = [len(ghostKinds)][]string{ // in its Room
		{"Hm?", "Did something move?", "A draft?", "Huh.", "Just the curtain."},
		{"What was that?!", "A shadow!", "Something moved!", "Who's there?", "Behind the lamp!"},
		{"Eek!", "It ran along the wall!", "I saw it! I SAW it!", "Not again!", "Under the covers!"},
		{"EEEEK!", "IT'S IN THE WALLS!", "GET ME OUT!", "NO NO NO!", "OPEN THE DOOR!"},
	}
	// hallSights: the same in a Corridor or the hall, where there is no Room to want out of (2026-11-14).
	hallSights = [len(ghostKinds)][]string{
		{"Odd.", "A flicker?", "Hm. Nothing.", "Was that a moth?"},
		{"Did you see that?", "Something's on the wall!", "Who's over there?", "That wasn't me."},
		{"Yikes!", "The wall moved!", "It's following me!", "Walk faster!"},
		{"IT'S OUT HERE TOO!", "DON'T LOOK BACK!", "THE WALLS!", "KEEP WALKING!"},
	}
	shadowTint = []float32{.45, .2, .7} // ghost: a black figure would vanish on the black maze walls
	shadowCast = append([]string{"Bob"}, guestCast[:]...)
)

// shade is a Shadow Person, always on a wall run: at x on g.walls[run], on its way along path to goal.
type shade struct {
	name    string // the character it's the silhouette of
	next    int    // the clock its next patrol is due
	essence float64
	minute  int // the last game minute daylight drained it
	run     int
	x       float64
	path    []int   // the runs it still teleports onto; the last holds its goal
	goal    float64 // x on its last run
	fade    int     // frames into a teleport, 0 if none (wall.go)
	dir     float64 // the way it last ran: its facing
	patrol  bool    // on a pass along its Room's back wall
	anim    entities.Anim
	seen    map[*entities.Guest]int // the last pass's sightings: the tick each Guest saw it
	roam    bool                    // dev -haunt: wanders the walls forever, unspent
	area    bool                    // an area's Shadow, switched on by a strong ghost (G1): it stays put, fading with lvl
	lvl     float64                 // an area Shadow's opacity, 0..1
	on      bool                    // a strong ghost is in its area now
}

type shades []*shade // a Room's Shadow People

func (g *Game) shaded(room int) bool { return room < len(g.shades) && len(g.shades[room]) > 0 }

// shadowCap: how many Shadows a Room holds (placeholder; R1 raises it with Room level).
func (g *Game) shadowCap(room int) int { return 1 }

// placeShadow: a Shadow Person joins the Room, born mid-way along the wall nearest where the
// courier leaves its box, the Room's front wall (Live test 4, 2026-11-08), then off to its back wall.
func (g *Game) placeShadow(room int) {
	at := g.doorDrop(room)
	sh := &shade{name: shadowCast[g.chat.IntN(len(shadowCast))], next: g.clock + g.shadowGap(room), essence: shadowEssence,
		minute: g.clock / minuteFrames, run: g.runAt(float64(at.X), float64(at.Y)), x: float64(at.X), anim: entities.NewAnim()}
	lo, hi := g.walls[sh.run].ends()
	sh.x, sh.goal = (lo+hi)/2, (lo+hi)/2
	g.shades[room] = append(g.shades[room], sh)
}

// shadowGap: frames from a patrol's start to the next, from 1 an hour at a calm Room to 1 every 20 game
// minutes at a Closed one (2026-11-08: a Shadow scares only while seen, so it patrols more).
func (g *Game) shadowGap(room int) int {
	s := g.dread(room)
	return int(60 / (1 + 2*s) * minuteFrames)
}

// dashGap: frames to a Shadow's next dash. A ghost's Shadow (area) dashes as often as its ghost haunts: the
// Haunt minutes of the most relentless strong ghost in the Room (Z1, 2026-11-13). A free Shadow keeps shadowGap.
func (g *Game) dashGap(room int, sh *shade) int {
	gap := 0
	if _, in := g.insideIn(room, Ghost); sh.area {
		for _, ghost := range in {
			if strong(ghost) && ghost.Haunt > 0 && (gap == 0 || ghost.Haunt < gap) {
				gap = ghost.Haunt
			}
		}
	}
	if gap == 0 {
		return g.shadowGap(room)
	}
	return gap * minuteFrames
}

// backWall: the run of a Room's back wall, and its patrol's two ends: touching either side wall.
func (g *Game) backWall(room int) (run int, xa, xb float64) {
	o := g.hotel.Rooms[room].Origin
	run = g.wallCell[(o.Y+1)*g.hotel.Map.W+o.X+world.RoomW/2]
	xa, xb = g.walls[run].ends()
	return run, xa, xb
}

// daylight: 0 at full night .. 1 in full day, from the darkness layer's alpha.
func daylight(c int) float64 { return 1 - tint(c)[3]/tint(0)[3] }

// shadowAlpha: how opaque a Shadow is drawn: its essence, its nearness to its wall's ends (it fades into
// them, to nothing touching one), and its teleport's fade.
func (g *Game) shadowAlpha(sh *shade) float64 {
	a := shadowOpacity * math.Sqrt(max(0, sh.essence)/shadowEssence) // sqrt: a nearly spent Shadow still shows, faintly
	if sh.area {
		if !sh.patrol && !g.ghost { // in real a ghost's Shadow shows only while it dashes (Z1)
			return 0
		}
		a *= sh.lvl
	}
	lo, hi := g.walls[sh.run].ends()
	t := clamp(min(sh.x-lo, hi-sh.x)/shadowEdgeFade, 0, 1)
	a *= t * t * (3 - 2*t)
	if sh.fade > 0 {
		a *= math.Abs(1 - 2*float64(sh.fade)/teleportFrames)
	}
	return a
}

// inside: the Guests of the Room's Party inside it.
func (g *Game) inside(room int) (*Party, []*entities.Guest) { return g.insideIn(room, Human) }

// insideIn: the Guests of the Room's Party of the realm inside it.
func (g *Game) insideIn(room int, realm Realm) (*Party, []*entities.Guest) {
	for _, p := range g.parties {
		if p.Occupies(room, realm) && p.state != out {
			var in []*entities.Guest
			for _, guest := range p.Guests {
				if image.Pt(int(guest.X), int(guest.Y)).In(g.hotel.Rooms[room].Shell()) {
					in = append(in, guest)
				}
			}
			return p, in
		}
	}
	return nil, nil
}

// updateShadows runs each frame. A strong ghost's Shadow (area) dashes as often as its ghost haunts (dashGap)
// and in real shows only while it dashes; only a Guest who sees it is frightened (Z1). Daylight drains each Shadow; one away from its Room's back wall runs
// home, to the nearer end. At home, a due Shadow patrols if its Room has a Guest inside: to the other
// end, spending 1 essence. A Guest who sees it gets shadowSeen Fear, hops and speaks; one who
// doesn't gets nothing. At 0 essence it's gone.
func (g *Game) updateShadows() {
	for i, list := range g.shades {
		p, in := g.inside(i)
		run, xa, xb := g.backWall(i)
		for _, sh := range list {
			if sh.roam {
				if g.move(sh) {
					g.wander(sh)
				}
				continue
			}
			if m := g.clock / minuteFrames; m > sh.minute {
				sh.essence -= float64(m-sh.minute) * shadowSunDrain * daylight(g.clock)
				sh.minute = m
			}
			if sh.area { // a ghost's Shadow is spent only by the ghost leaving
				sh.essence = shadowEssence
			}
			arrived := g.move(sh)
			if sh.patrol {
				g.sighting(p, in, sh)
				if arrived {
					sh.patrol = false
					sh.essence--
				}
				continue
			}
			switch {
			case !arrived:
			case sh.run != run || sh.x != xa && sh.x != xb:
				g.sendShade(sh, run, slices.MinFunc([]float64{xa, xb}, func(a, b float64) int { return cmp.Compare(math.Abs(a-sh.x), math.Abs(b-sh.x)) }))
			case g.clock >= sh.next:
				if sh.next = g.clock + g.dashGap(i, sh); len(in) > 0 {
					sh.patrol, sh.seen = true, map[*entities.Guest]int{}
					g.sendShade(sh, run, xa+xb-sh.x)
				}
			}
		}
		g.shades[i] = slices.DeleteFunc(list, func(sh *shade) bool { return sh.essence <= 0 })
	}
	g.updateAreaShades()
}

// areaFade: frames an area's Shadows take to appear or fade (placeholder).
const areaFade = 60

// areaRun: the area (07 G1) the world point is in, named by its upper wall: the run of the first wall face
// above it. A Room's is its back wall; a Corridor's or the entrance hall's, the wall above that stretch.
// -1 if none.
func (g *Game) areaRun(x, y float64) int {
	m := g.hotel.Map
	for cy := int(y) / world.TileSize; cy >= 0; cy-- {
		if k := g.wallCell[cy*m.W+m.Cell(image.Pt(int(x), cy*world.TileSize)).X]; k >= 0 {
			return k
		}
	}
	return -1
}

// roomOfRun: the Room whose back wall is the run, -1 if none.
func (g *Game) roomOfRun(run int) int {
	for i := range g.hotel.Rooms {
		if r, _, _ := g.backWall(i); r == run {
			return i
		}
	}
	return -1
}

// areaOn: the area's Shadow People are switched on: a strong ghost stands somewhere in it.
func (g *Game) areaOn(run int) bool {
	if i := g.roomOfRun(run); i >= 0 { // a Room's Shadow moves between walls: any of its own counts
		return slices.ContainsFunc(g.shades[i], func(sh *shade) bool { return sh.area && sh.on })
	}
	return slices.ContainsFunc(g.areaShades, func(sh *shade) bool { return sh.run == run && sh.on })
}

// areaGap: game minutes between the dashes of a Corridor's or the hall's Shadow (placeholder).
const areaGap = 20

// updateAreaShades runs each frame: every area holding a strong ghost of the map has its Shadow People on,
// faded in; the ones whose ghost left fade out and go. In a Room it is the old Shadow (updateShadows): it
// dashes along the back wall while a human is in, frightens one who sees it, a Strong Priest banishes it
// (until the ghost leaves); kept at full essence while the ghost stays. In a Corridor or the hall it is the
// same figure dashing along that wall now and then, frightening a human who sees it there. A weak ghost
// switches nothing on. A flag on the area, not a figure bound to the ghost.
func (g *Game) updateAreaShades() {
	on := map[int]bool{}
	for _, guest := range g.guestsIn(Ghost) {
		if k := g.areaRun(guest.X, guest.Y); k >= 0 && strong(guest) {
			on[k] = true
		}
	}
	fade := func(sh *shade, up bool) {
		sh.on = up
		if up {
			sh.lvl = min(1, sh.lvl+1.0/areaFade)
		} else {
			sh.lvl = max(0, sh.lvl-1.0/areaFade)
		}
	}
	gone := func(sh *shade) bool { return sh.area && !sh.on && sh.lvl <= 0 }
	for i := range g.shades {
		run, _, _ := g.backWall(i)
		if !on[run] {
			delete(g.hushed, i)
		}
		if on[run] && len(g.shades[i]) == 0 && !g.hushed[i] {
			g.placeShadow(i)
			g.shades[i][0].area = true
			g.shades[i][0].next = g.clock + g.dashGap(i, g.shades[i][0])
		}
		for _, sh := range g.shades[i] {
			if sh.area {
				fade(sh, on[run] && !g.hushed[i])
			}
		}
		g.shades[i] = slices.DeleteFunc(g.shades[i], gone)
	}
	for k := range on {
		if g.roomOfRun(k) < 0 && !slices.ContainsFunc(g.areaShades, func(sh *shade) bool { return sh.run == k }) {
			lo, hi := g.walls[k].ends()
			g.areaShades = append(g.areaShades, &shade{name: shadowCast[g.chat.IntN(len(shadowCast))], essence: shadowEssence,
				run: k, x: (lo + hi) / 2, goal: (lo + hi) / 2, area: true, anim: entities.NewAnim()})
		}
	}
	for _, sh := range g.areaShades {
		fade(sh, on[sh.run])
		arrived := g.move(sh)
		if sh.patrol {
			for _, p := range g.parties {
				if p.Realm != Human || p.state == out {
					continue
				}
				var in []*entities.Guest
				for _, guest := range p.Guests {
					if g.areaRun(guest.X, guest.Y) == sh.run {
						in = append(in, guest)
					}
				}
				g.sighting(p, in, sh)
			}
			sh.patrol = !arrived
		} else if arrived && g.clock >= sh.next {
			lo, hi := g.walls[sh.run].ends()
			sh.next, sh.patrol, sh.seen = g.clock+areaGap*minuteFrames, true, map[*entities.Guest]int{}
			g.sendShade(sh, sh.run, lo+hi-sh.x)
		}
	}
	g.areaShades = slices.DeleteFunc(g.areaShades, gone)
}

// areaPhantasm: the Phantasm of the strongest ghost in the area named by the run, 0 with none.
func (g *Game) areaPhantasm(run int) (most int) {
	for _, ghost := range g.guestsIn(Ghost) {
		if g.areaRun(ghost.X, ghost.Y) == run {
			most = max(most, ghost.Phantasm)
		}
	}
	return most
}

// sighting: each Guest inside who sees a patrolling Shadow for the first time this pass gets shadowSeen
// Fear, hops and speaks a line picked at random from its ghost's kind's.
func (g *Game) sighting(p *Party, in []*entities.Guest, sh *shade) {
	y := g.walls[sh.run].y
	kind := kindOf(g.areaPhantasm(sh.run))
	for _, guest := range in {
		lines := hallSights[kind]
		if g.indoors(guest) {
			lines = sightLines[kind]
		}
		if _, ok := sh.seen[guest]; !ok && g.sees(guest, sh.x, y) {
			sh.seen[guest] = g.tick
			g.frighten(p, guest, shadowSeen, guest.FleeAt-1)
			who := slices.Index(p.Guests, guest)
			p.talk = append(slices.DeleteFunc(p.talk, func(l line) bool { return l.who == who }), line{who, lines[g.chat.IntN(len(lines))], g.tick})
		}
	}
}

// sees: (x,y) is in the half-plane ahead of an awake Guest: its heading while walking, else its facing.
// Asleep in bed it sees nothing.
func (g *Game) sees(guest *entities.Guest, x, y float64) bool {
	if _, abed := g.bedAt(guest.X, guest.Y); abed && asleep(g.clock) {
		return false
	}
	var fx, fy float64
	if len(guest.Path) > 0 {
		fx, fy = float64(guest.Path[0].X)-guest.X, float64(guest.Path[0].Y)-guest.Y
	} else {
		a := float64(guest.Anim.Facing) * math.Pi / 2 // FaceRight, FaceUp, FaceLeft, FaceDown: counterclockwise, y down
		fx, fy = math.Cos(a), -math.Sin(a)
	}
	return (x-guest.X)*fx+(y-guest.Y)*fy > 0
}

// frighten: n Fear to a Guest of p, but not past limit; the hotel and its Room gain the Ecto (D148).
func (g *Game) frighten(p *Party, guest *entities.Guest, n, limit int) {
	n = max(0, min(n, limit-guest.Fear))
	guest.Fear += n
	g.earn(Ecto, Num(n), noFloat) // the haunt floats its own batched "+N"
	g.ghostDirt(p.Room, dirtMood(n))
	p.ectoShown += n
}

// startle: how high a Guest is in its hop on seeing a Shadow, world px.
func (g *Game) startle(guest *entities.Guest) float64 {
	for _, list := range g.shades {
		for _, sh := range list {
			if t, ok := sh.seen[guest]; ok && g.tick-t < hopFrames {
				return startleHeight * math.Sin(math.Pi*float64(g.tick-t)/hopFrames)
			}
		}
	}
	return 0
}

// bucketShadows sorts this frame's Shadows by run, so each wall cell draws only its own run's.
func (g *Game) bucketShadows() {
	if len(g.onRun) != len(g.walls) {
		g.onRun = make([][]*shade, len(g.walls))
	}
	for k := range g.onRun {
		g.onRun[k] = g.onRun[k][:0]
	}
	for _, list := range g.shades {
		for _, sh := range list {
			g.onRun[sh.run] = append(g.onRun[sh.run], sh)
		}
	}
	for _, sh := range g.areaShades {
		g.onRun[sh.run] = append(g.onRun[sh.run], sh)
	}
}

// drawShadow draws the Shadows of wall tile p's run over it, clipped to the cell and the run, at their
// alpha. Not over a lintel: it passes behind doors, as the windows and props drawn after cover it, and
// the side lines are off its run. Fogged with the wall in real.
func (g *Game) drawShadow(s *ebiten.Image, p prop, ox, oy int) {
	const ts = world.TileSize
	k := g.wallCell[p.cy*g.hotel.Map.W+p.cx]
	if k < 0 || g.walls[k].hidden() || g.ts.Tiles[p.id].Name == "wall_lintel" {
		return
	}
	r := g.walls[k]
	px := g.hotel.Map.Origin(p.cx, p.cy).X // the cell's west edge
	x0, x1 := max(px, int(r.x0)), min(px+ts, int(r.x1))
	clip := image.Rect(x0*zoom-ox, p.cy*ts*zoom-oy, x1*zoom-ox, (p.cy+1)*ts*zoom-oy)
	for _, sh := range g.onRun[k] {
		if math.Abs(float64(px+ts/2)-sh.x) >= (ts+charW)/2 {
			continue
		}
		f := sh.anim.Frame()
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(zoom, zoom)
		op.GeoM.Translate(float64(screenPx(sh.x)-ox-charW/2*zoom), float64(screenPx(r.y)-oy+(entities.FootH/2-charH)*zoom))
		op.ColorScale.ScaleAlpha(float32((1 - g.propFog(p)) * g.shadowAlpha(sh)))
		if !g.ghost {
			op.ColorScale.Scale(0, 0, 0, 1)
		}
		frame := g.sheet(sh.name)[1].SubImage(image.Rect(f*charW, 0, (f+1)*charW, charH)).(*ebiten.Image)
		g.blit(s.SubImage(clip).(*ebiten.Image), frame, op, modeActor, 0, shadowTint, 0)
	}
}

// DevShadow is main's -shot -shadow, a dev print: a free Shadow for the first Room, with essence e, watched
// by the Concierge from the Corridor beside it, where the view takes in the whole north wall.
func (g *Game) DevShadow(e float64) {
	g.placeShadow(0)
	g.shades[0][0].essence = e
	o := g.hotel.Rooms[0].Origin
	g.player.X, g.player.Y = float64((o.X+world.RoomW+1)*world.TileSize), float64((o.Y+3)*world.TileSize)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}

// DevHaunt is main's -haunt, a dev view of the wall movement: n Shadows spread over the runs and the Rooms'
// lists, wandering the walls forever, unspent and harmless.
func (g *Game) DevHaunt(n int) {
	var shown []int
	for k, r := range g.walls {
		if !r.hidden() {
			shown = append(shown, k)
		}
	}
	for i := range n {
		run := shown[i%len(shown)]
		lo, hi := g.walls[run].ends()
		sh := &shade{name: shadowCast[g.chat.IntN(len(shadowCast))], essence: shadowEssence, run: run, x: (lo + hi) / 2, roam: true, anim: entities.NewAnim()}
		sh.goal = sh.x
		g.shades[i%len(g.shades)] = append(g.shades[i%len(g.shades)], sh)
	}
}
