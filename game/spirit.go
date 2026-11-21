package game

import (
	"image"
	"image/color"
	"math"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 W5-VERBS, O4: the spirit with no ghost. At night, in real, a Shadow-like spirit dashes along a rolled Corridor.
// It counts as lit while the Concierge's Lantern beam touches it (light.go's beam); lit, the cutscene plays, and in
// ghost a handprint glows on the rolled door: SPACE on it finishes O4. Lv2+: two dashes, one after the other, and
// both must be lit. Placeholders.
const (
	spiritSpeed = shadowSpeed
	spiritGap   = 30 * minuteFrames // frames from a dash that was not lit to the next
	spiritPause = 20                // frames between the two dashes of Lv2+
	spiritFold  = 40                // frames the lit spirit stands before the cutscene folds it away
	spiritLift  = 8                 // world px from its feet to the middle of its body
	nightFrom   = 22                // the spirit works the Corridors from this hour,
	nightTo     = 5                 // to this one
)

// spiritRun: the spirit dashing now (on), or standing lit (fold > 0).
type spiritRun struct {
	on       bool
	x, y     float64 // its feet, world px
	dir      float64 // +1 east, -1 west
	dash     int     // 0 the first dash, 1 the second (Lv2+)
	hit      [2]bool // each dash caught in the beam
	hitAt    image.Point
	next     int     // the clock before which no dash starts
	pause    float64 // frames before the second dash
	fold     float64
	anim     entities.Anim
	animInit bool
}

// dashes: how many dashes must be lit.
func dashes(level int) int { return 1 + b2i(level >= 2) }

// spiritRow: the Rooms along the rolled Corridor (the ones on its row, in map order), -1 empty before it is rolled.
func (g *Game) spiritRow() (row []int) {
	r := g.rollGet("O4.room") - 1
	if r < 0 || r >= len(g.hotel.Rooms) {
		return nil
	}
	for i, q := range g.hotel.Rooms {
		if q.Origin.Y == g.hotel.Rooms[r].Origin.Y {
			row = append(row, i)
		}
	}
	return row
}

// handprintRoom: the Room whose door shows the handprint in ghost (the spirit was lit, O4 not done), -1 if none.
func (g *Game) handprintRoom() int {
	row := g.spiritRow()
	if !g.questOpen("O4") || g.rollGet("O4.lit") == 0 || len(row) == 0 {
		return -1
	}
	return row[g.rollGet("O4.door")-1]
}

// nightNow: the Corridors are dark.
func (g *Game) nightNow() bool { h := hour(g.clock); return h >= nightFrom || h < nightTo }

// beamHits: world px point p is touched by the Concierge's Lantern beam: in real, where no area light makes the
// lantern off (light.go lit), inside its cone and no farther than its rays reach before a wall.
func (g *Game) beamHits(p image.Point) bool {
	at := g.feet()
	if g.ghost || g.lit(at) {
		return false
	}
	if g.player.Anim.Facing == entities.FaceUp {
		at.Y -= lanternLift
	}
	turn := -float64(g.player.Anim.Facing) * math.Pi / 2
	dx, dy := float64(p.X-at.X), float64(p.Y-at.Y)
	rel := math.Atan2(dy, dx) - turn
	for rel > math.Pi {
		rel -= 2 * math.Pi
	}
	for rel < -math.Pi {
		rel += 2 * math.Pi
	}
	if math.Abs(rel) > lanternHalf {
		return false
	}
	i := int(math.Round((rel/lanternHalf + 1) / 2 * (beamRays - 1)))
	return math.Hypot(dx, dy) <= g.beamReach(at, turn)[i]
}

// updateSpirit runs each step: it rolls its Corridor and door, dashes at night, and is lit or not.
func (g *Game) updateSpirit() {
	s := &g.verbs.spirit
	if !g.questOpen("O4") {
		*s = spiritRun{}
		return
	}
	if g.rollGet("O4.room") == 0 && len(g.hotel.Rooms) > 0 {
		g.roll("O4.room", len(g.hotel.Rooms))
		g.roll("O4.door", len(g.spiritRow()))
	}
	if !s.animInit {
		s.anim, s.animInit = entities.NewAnim(), true
	}
	if g.rollGet("O4.lit") > 0 { // lit: it stands a moment while the cutscene closes in, then folds away
		if s.fold -= g.dt; s.fold <= 0 {
			s.on = false
		}
		return
	}
	cells := g.spiritRow()
	if len(cells) == 0 {
		return
	}
	x0, x1, y := g.spiritLane(cells)
	switch {
	case s.pause > 0:
		if s.pause -= g.dt; s.pause <= 0 {
			g.dashFrom(1, x0, x1, y)
		}
	case !s.on:
		if g.ticked && g.nightNow() && g.clock >= s.next {
			g.dashFrom(0, x0, x1, y)
		}
	default:
		s.x += s.dir * spiritSpeed * g.dt
		if !g.headless {
			s.anim.Update(s.x, s.y, g.dt)
		}
		if p := image.Pt(int(s.x), int(s.y)-spiritLift); g.beamHits(p) {
			s.hit[s.dash], s.hitAt = true, p
			if dashes(g.level) == 1 {
				g.spiritLit()
				return
			}
		}
		if s.dir > 0 && s.x >= x1 || s.dir < 0 && s.x <= x0 {
			switch {
			case s.dash == 0 && dashes(g.level) == 2:
				s.on, s.pause = false, spiritPause
			case s.hit[0] && s.hit[1] || dashes(g.level) == 1 && s.hit[0]:
				g.spiritLit()
			default:
				*s = spiritRun{next: g.clock + spiritGap, anim: s.anim, animInit: true}
			}
		}
	}
}

// spiritLane: the Corridor the spirit dashes along: from the west end of the row of Rooms to its east end, at the
// height of their doors' fronts.
func (g *Game) spiritLane(cells []int) (x0, x1, y float64) {
	lo, hi := math.MaxInt, 0
	for _, i := range cells {
		o := g.hotel.Rooms[i].Origin.X
		lo, hi = min(lo, o), max(hi, o+world.RoomW)
	}
	return float64(lo * world.TileSize), float64(hi * world.TileSize), float64(g.doorFront(cells[0]).Y)
}

// dashFrom starts dash i from its end of the lane: the first from the west, the second back from the east.
func (g *Game) dashFrom(i int, x0, x1, y float64) {
	s := &g.verbs.spirit
	s.on, s.dash, s.y, s.pause = true, i, y, 0
	s.dir, s.x = 1, x0
	if i == 1 {
		s.dir, s.x = -1, x1
	}
	if i == 0 {
		s.hit = [2]bool{}
	}
}

// spiritLit: the beam caught it (both times at Lv2+): it stops where it was caught, the cutscene plays, and the door
// shows its handprint in ghost.
func (g *Game) spiritLit() {
	s := &g.verbs.spirit
	g.verbs.litAt = s.hitAt
	s.on, s.fold, s.pause = true, spiritFold, 0
	s.x, s.y = float64(s.hitAt.X), float64(s.hitAt.Y+spiritLift)
	g.rollSet("O4.lit", 1)
	g.questEvent(evLit, -1, "")
	g.playQuestScene("O4")
}

// spiritActor: the spirit for the y-sorted pass, seen in real only: a dim black figure, plain as it is lit.
func (g *Game) spiritActor() *actor {
	s := &g.verbs.spirit
	if !s.on || g.ghost {
		return nil
	}
	return &actor{x: s.x, y: s.y, draw: func(dst *ebiten.Image, x, y int) { // a Shadow's look: black, as drawShadow draws it in real
		a := float32(shadowOpacity)
		if s.fold > 0 {
			a = 0.95
		}
		f := s.anim.Frame()
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(zoom, zoom)
		op.GeoM.Translate(float64(x-charW/2*zoom), float64(y+(entities.FootH/2-charH)*zoom))
		op.ColorScale.ScaleAlpha(a)
		op.ColorScale.Scale(0, 0, 0, 1)
		g.blitPart(dst, g.sheet("Bob")[b2i(s.anim.Moving)], image.Rect(f*charW, 0, (f+1)*charW, charH), op, modeActor, 0, shadowTint, 0)
	}}
}

// drawHandprint: a glowing hand waving on the door, its heel at (x,y) screen px.
func (g *Game) drawHandprint(s *ebiten.Image, x, y int) {
	const u = zoom * 3 / 2 // bigger than a cell's detail: it has to be seen from the Corridor
	g.drawGlow(s, x, y-4*u, [3]float32{.4, 1, .75}, 0.6, 1.6)
	ink := resInk(Ecto)
	wave := int(math.Round(math.Sin(float64(g.tick)/12) * 2 * u))
	rect(s, x-4*u+wave/2, y-6*u, 8*u, 6*u, ink)
	for f := range 4 { // four fingers, the outer ones shorter, swaying with the wave
		rect(s, x-4*u+f*2*u+wave, y-(10+b2i(f == 1 || f == 2)*2)*u, 2*u-1, 5*u, ink)
	}
	rect(s, x+3*u+wave/2, y-7*u, 3*u, 2*u, ink) // the thumb
}

// drawParcel: what the Courier left at Madame's door: a parcel in real, in ghost the wreath it is.
func (g *Game) drawParcel(s *ebiten.Image, x, y int) {
	if !g.ghost {
		rect(s, x-5*zoom, y-6*zoom, 10*zoom, 7*zoom, color.RGBA{0xb6, 0x93, 0x7e, 255})
		rect(s, x-5*zoom, y-6*zoom, 10*zoom, zoom, color.RGBA{0x91, 0x66, 0x62, 255})
		rect(s, x-zoom, y-6*zoom, 2*zoom, 7*zoom, color.RGBA{0xd0, 0xbe, 0x9c, 255})
		return
	}
	for k := range 12 { // a ring of leaves round a red bow
		a := float64(k) / 12 * 2 * math.Pi
		rect(s, x+int(math.Round(math.Cos(a)*5*zoom))-zoom, y-4*zoom+int(math.Round(math.Sin(a)*4*zoom))-zoom, 2*zoom, 2*zoom, color.RGBA{0x5f, 0xe0, 0xb0, 255})
	}
	rect(s, x-zoom, y-9*zoom, 2*zoom, 2*zoom, color.RGBA{0xe0, 0x50, 0x60, 255})
}

// drawHeldQuest: the quest word or thing over the Concierge's head, a small pixel icon (no label).
func (g *Game) drawHeldQuest(s *ebiten.Image, x, y int) {
	if g.ghost {
		y -= int(hover(g.tick, "Bob") * zoom)
	}
	img := g.heldIcon(g.verbs.held)
	if img == nil {
		return
	}
	b := img.Bounds()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(float64(x-b.Dx()*zoom/2), float64(y+(entities.FootH/2-charH)*zoom+4*zoom-b.Dy()*zoom+int(math.Round(math.Sin(float64(g.tick)/20)*float64(zoom)))))
	s.DrawImage(img, op)
}
