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
	shadowUnseen   = 2           // and one who doesn't
	startleHeight  = 4           // world px: a Guest's hop on seeing it (the flee's is hopHeight)
	shadowEssence  = 24.0        // (placeholder) what it's born with; a patrol pass spends 1
	shadowSunDrain = 1.0 / 60    // (placeholder) essence lost per game minute in full daylight
	shadowOpacity  = 0.5         // (placeholder) a full Shadow's alpha: a dimness on the wall, not a hole (2026-11-08)
	shadowEdgeFade = charW       // (placeholder) world px from a wall's end over which a Shadow fades into it
	shadowMinWall  = 3 * charW   // (placeholder) a narrower wall, such as the Corridor's back wall, never shows a Shadow (2026-11-08)
)

var (
	sightLines = []string{"What was that?!", "A shadow!", "Eek!", "Something moved!"}
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
	lo, hi := g.walls[sh.run].ends()
	t := clamp(min(sh.x-lo, hi-sh.x)/shadowEdgeFade, 0, 1)
	a *= t * t * (3 - 2*t)
	if sh.fade > 0 {
		a *= math.Abs(1 - 2*float64(sh.fade)/teleportFrames)
	}
	return a
}

// inside: the Guests of the Room's Party inside it.
func (g *Game) inside(room int) (*Party, []*entities.Guest) {
	for _, p := range g.parties {
		if p.Occupies(room, Human) && p.state != out {
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

// updateShadows runs each frame. Daylight drains each Shadow; one away from its Room's back wall runs
// home, to the nearer end. At home, a due Shadow patrols if its Room has a Guest inside: to the other
// end, spending 1 essence. A Guest who sees it gets shadowSeen Fear, hops and speaks; at the end, each
// one inside who didn't gets shadowUnseen. At 0 essence it's gone.
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
			arrived := g.move(sh)
			if sh.patrol {
				g.sighting(p, in, sh)
				if arrived {
					sh.patrol = false
					sh.essence--
					for _, guest := range in {
						if _, ok := sh.seen[guest]; !ok {
							g.frighten(p, guest, shadowUnseen, guest.FleeAt-1)
						}
					}
				}
				continue
			}
			switch {
			case !arrived:
			case sh.run != run || sh.x != xa && sh.x != xb:
				g.sendShade(sh, run, slices.MinFunc([]float64{xa, xb}, func(a, b float64) int { return cmp.Compare(math.Abs(a-sh.x), math.Abs(b-sh.x)) }))
			case g.clock >= sh.next:
				if sh.next = g.clock + g.shadowGap(i); len(in) > 0 {
					sh.patrol, sh.seen = true, map[*entities.Guest]int{}
					g.sendShade(sh, run, xa+xb-sh.x)
				}
			}
		}
		g.shades[i] = slices.DeleteFunc(list, func(sh *shade) bool { return sh.essence <= 0 })
	}
}

// sighting: each Guest inside who sees a patrolling Shadow for the first time this pass gets shadowSeen
// Fear, hops and speaks.
func (g *Game) sighting(p *Party, in []*entities.Guest, sh *shade) {
	y := g.walls[sh.run].y
	for _, guest := range in {
		if _, ok := sh.seen[guest]; !ok && g.sees(guest, sh.x, y) {
			sh.seen[guest] = g.tick
			g.frighten(p, guest, shadowSeen, guest.FleeAt-1)
			who := slices.Index(p.Guests, guest)
			p.talk = append(slices.DeleteFunc(p.talk, func(l line) bool { return l.who == who }), line{who, sightLines[g.chat.IntN(len(sightLines))], g.tick})
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
	g.addDirtLevel(p.Room, n)
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
	x0, x1 := max(p.cx*ts, int(r.x0)), min((p.cx+1)*ts, int(r.x1))
	clip := image.Rect(x0*zoom-ox, p.cy*ts*zoom-oy, x1*zoom-ox, (p.cy+1)*ts*zoom-oy)
	for _, sh := range g.onRun[k] {
		if math.Abs(float64(p.cx*ts+ts/2)-sh.x) >= (ts+charW)/2 {
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
