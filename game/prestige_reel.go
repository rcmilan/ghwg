package game

import (
	"image"
	"image/color"
	"math"
	"slices"

	"ghosthotel/assets"
	"ghosthotel/entities"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// 08 W3 (P1-X): Father P's prestige cutscene, in the Entrance hall. An homage in spirit only (time speeding up,
// gravity turning over); the lines and the staging are our own. The world waits: nothing steps, the cutscene
// moves the actors itself, then calls startPrestige (prestige.go) once, behind a white flash.
//
//	gather   everyone walks into the hall; the far ones appear there; ghosts show though the view is real
//	appear   Father P appears; three lines
//	flip     the view turns over and everyone but him falls "up"
//	race     the clock spins faster and faster, day and night flicker with it
//	flash    white, then startPrestige
const (
	reelGather  = 360 // frames: the walk in; whoever is not there yet is put there
	reelLines   = 420 // the first line; one every reelGap
	reelGap     = 130
	reelFlip    = 810 // gravity turns over...
	turnFrames  = 90  // ...in this many frames
	reelRace    = 930
	reelFlash   = 1260 // the white comes in over flashRamp frames; the whole is about 22 s
	flashRamp   = 60
	reelEnd     = 1340
	popFrames   = 45 // the ring of a guest appearing
	reelWalk    = 1.0
	spinStart   = 24.0  // clock frames a frame at the start of the race...
	spinEnd     = 600.0 // ...and at its end: a day in 24 frames, under 3 flickers a second
	fallPull    = 0.05  // px/frame² "up"
	clockRadius = 110   // screen px
)

// fatherP: Adam, the one sheet no Father wears, in Pucci's colors (08 W3 live test): white hair, dark skin, a black
// cassock.
var fatherP = struct {
	name string
	look look
}{"Father P", look{hair: 0xeceaf2, skin: 0x7a4e36, top: 0x1c1a22, bottom: 0x141218}}

var pLines = [...]string{"Welcome, all. Living, dead, and late.", "Nothing ends. It only begins again.", "Mind your feet. The floor is leaving."}

// prestigeGo is what the cutscene calls at its end; a test counts it.
var prestigeGo = func(g *Game) { g.startPrestige() }

type cast struct {
	g    *entities.Guest
	slot image.Point
	pop  int // the frame a far guest appears in the hall; 0 for one who walks in
	vy   float64
}

type prestigeReel struct {
	t     int
	cast  []cast
	pl    entities.Guest // the Concierge, walked like the rest, then copied back
	p     *priest
	ppos  image.Point // where Father P stands
	ceil  float64     // the hall's "up" end, world px
	flip  float64     // 0 upright .. 1 turned over
	skip  bool
	buf   *ebiten.Image // the world, drawn once then laid on the screen turned over
	says  []reelSay     // the speech balloons the world pass left out, drawn upright after the flip
	hallC image.Point
}

// playPrestige starts the cutscene (P1-C's confirm card calls it); when it ends it calls startPrestige.
func (g *Game) playPrestige() {
	if g.reel != nil {
		return
	}
	g.state, g.ghost, g.shifting, g.viewT, g.hands = StatePlay, false, nil, 0, ""
	r := &prestigeReel{}
	g.reel = r
	hall := g.hotel.Lobby
	r.hallC = hall.Min.Add(hall.Max).Div(2)
	var slots []image.Point // walkable spots of the hall, off the kitchen
	for y := hall.Min.Y + 8; y < hall.Max.Y-4; y += 13 {
		for x := hall.Min.X + 8; x < hall.Max.X-8; x += 13 {
			if p := image.Pt(x, y); !p.In(g.hotel.Kitchen) && !g.blocked(entities.FootBox(float64(x), float64(y))) {
				slots = append(slots, p)
			}
		}
	}
	if len(slots) == 0 {
		slots = []image.Point{g.hotel.Archway}
	}
	dist := func(a, b image.Point) float64 { return math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y)) }
	front := image.Pt(r.hallC.X, hall.Min.Y+hall.Dy()*2/5)
	r.ppos = slots[0]
	for _, s := range slots {
		if dist(s, front) < dist(r.ppos, front) {
			r.ppos = s
		}
	}
	r.ceil = float64(slots[0].Y) - 4
	for _, s := range slots {
		r.ceil = math.Min(r.ceil, float64(s.Y)-4)
	}
	slices.SortFunc(slots, func(a, b image.Point) int { // the crowd fills in from just below him
		return int(dist(a, r.ppos.Add(image.Pt(0, 24))) - dist(b, r.ppos.Add(image.Pt(0, 24))))
	})
	slots = slices.DeleteFunc(slots, func(s image.Point) bool { return dist(s, r.ppos) < 12 || s.Y <= r.ppos.Y+4 }) // nobody behind him
	if len(slots) == 0 {
		slots = []image.Point{r.ppos.Add(image.Pt(0, 14))}
	}
	mw, mh := g.mapSize()
	ox, oy := g.cam.Origin(g.player.X, g.player.Y, mw, mh)
	seen := image.Rect(ox/zoom, oy/zoom, ox/zoom+viewW, oy/zoom+viewH)
	r.pl = entities.NewGuest(image.Pt(int(g.player.X), int(g.player.Y)), "Bob")
	r.pl.X, r.pl.Y = g.player.X, g.player.Y
	for i, gu := range append([]*entities.Guest{&r.pl}, g.actors()...) {
		c := cast{g: gu, slot: slots[i%len(slots)]}
		if !image.Pt(int(gu.X), int(gu.Y)).In(seen) {
			c.pop = 20 + i*37%180 // not on screen: appears in the hall
		} else {
			g.reelRoute(gu, c.slot)
		}
		r.cast = append(r.cast, c)
	}
}

// reelRoute: the way from where the guest is to the hall, out of its Room by the door first (priestRoute's).
func (g *Game) reelRoute(gu *entities.Guest, to image.Point) {
	from := image.Pt(int(gu.X), int(gu.Y))
	var path []image.Point
	if in := g.roomAt(gu.X, gu.Y); in >= 0 {
		out := g.doorFront(in)
		path = g.hotel.Path(from, out, in)
		from = out
	}
	gu.SetPath(append(path, g.hotel.Path(from, to, -1)...))
}

// updateReel runs the cutscene a frame.
func (g *Game) updateReel() {
	r := g.reel
	if r.t < reelFlash && g.level > 0 && g.in.JustPressed(ebiten.KeySpace) { // later times: skip to the white
		r.t, r.skip = reelFlash, true
	}
	g.tick++
	switch {
	case r.t < reelGather:
		for i := range r.cast {
			if c := &r.cast[i]; r.t >= c.pop {
				if c.pop > 0 && r.t == c.pop { // appears
					c.g.X, c.g.Y = float64(c.slot.X), float64(c.slot.Y)
				}
				if c.pop == 0 {
					c.g.FollowPath(reelWalk)
				}
			}
		}
	case r.t == reelGather:
		for _, c := range r.cast { // whoever is not there is
			c.g.X, c.g.Y, c.g.Path = float64(c.slot.X), float64(c.slot.Y), nil
		}
		pr := &priest{Guest: entities.NewGuest(r.ppos, fatherP.name)}
		pr.RoomIndex = -1
		r.p = pr
		g.priests = append(g.priests, pr)
	}
	if i := (r.t - reelLines) / reelGap; r.t >= reelLines && (r.t-reelLines)%reelGap == 0 && i < len(pLines) {
		r.p.said = line{0, pLines[i], g.tick}
	}
	if r.t >= reelFlip && !r.skip {
		r.flip = min(1, r.flip+1.0/turnFrames)
		for i := range r.cast { // gravity is up: they rise off the floor, faster and faster, and pile on the ceiling
			c := &r.cast[i]
			c.vy += fallPull * (1 + float64(i%3)/3)
			c.g.X += math.Sin(float64(r.t)/9+float64(i)) * .3
			c.g.Y = math.Max(r.ceil, c.g.Y-c.vy)
		}
	}
	if r.t >= reelRace && !r.skip {
		k := float64(r.t-reelRace) / (reelFlash - reelRace)
		g.clock += int(spinStart * math.Pow(spinEnd/spinStart, k))
	}
	g.player.X, g.player.Y = r.pl.X, r.pl.Y
	g.player.Anim.Update(g.player.X, g.player.Y, 1)
	for _, c := range r.cast[1:] {
		c.g.Anim.Update(c.g.X, c.g.Y, 1)
	}
	mw, mh := g.mapSize()
	g.cam.Follow(float64(r.hallC.X), float64(r.hallC.Y), mw, mh)
	if r.t++; r.t >= reelEnd {
		g.endReel()
	}
}

// endReel: the white is full; hand over to the prestige.
func (g *Game) endReel() {
	r := g.reel
	if r.p != nil {
		g.priests = slices.DeleteFunc(g.priests, func(pr *priest) bool { return pr == r.p })
	}
	if r.buf != nil {
		r.buf.Deallocate()
	}
	g.reel = nil
	prestigeGo(g)
}

// drawReel draws the cutscene: the world into one offscreen, laid on the screen turned over by flip, the
// clock over it, then the white.
func (g *Game) drawReel(screen *ebiten.Image) {
	r := g.reel
	b := screen.Bounds()
	if r.buf == nil {
		r.buf = ebiten.NewImage(b.Dx(), b.Dy())
	}
	r.buf.Fill(assets.Night)
	r.says = r.says[:0]
	mw, mh := g.mapSize()
	ox, oy := g.cam.Origin(g.player.X, g.player.Y, mw, mh)
	g.drawWorld(r.buf, ox, oy)
	at := func(x, y float64) (float64, float64) { return x*zoom - float64(ox), (y-8)*zoom - float64(oy) }
	for _, c := range r.cast { // a far guest's arrival: a ring and a flash
		if f := float64(r.t-c.pop) / popFrames; c.pop > 0 && f >= 0 && f < 1 {
			x, y := at(c.g.X, c.g.Y)
			drawRing(r.buf, x, y+8*zoom, 8*zoom*(1+2*f), 1, 1.5, kTint, 1-f)
			g.glowAt(r.buf, x, y, 40*zoom*math.Sin(math.Pi*f), kTint, .8)
		}
	}
	if p := r.p; p != nil {
		x, y := at(p.X, p.Y)
		f := float64(r.t-reelGather) / popFrames
		g.glowAt(r.buf, x, y, 36*zoom*(1+.1*math.Sin(float64(r.t)/20)), mEdge, .35*math.Min(1, f))
		if f < 1 {
			drawRing(r.buf, x, y+8*zoom, 10*zoom*(1+4*f), 1, 2, mTint, 1-f)
		}
		if p.talking(g.tick) {
			g.say(r.buf, p.said.text, &p.Guest, ox, oy)
		}
	}
	screen.Fill(assets.Night)
	e := r.flip * r.flip * (3 - 2*r.flip)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, -float64(b.Dy())/2)
	op.GeoM.Scale(1, math.Cos(math.Pi*e)) // turns through edge-on to upside down
	op.GeoM.Translate(0, float64(b.Dy())/2)
	screen.DrawImage(r.buf, op)
	c := math.Cos(math.Pi * e)
	for _, s := range r.says { // the speaker's flipped place, the balloon itself upright
		at := s.guest
		at.Y = (flipY(s.y, float64(b.Dy()), c) + float64(s.oy)) / zoom
		g.sayAt(screen, s.line, &at, s.ox, s.oy)
	}
	if k := min(1, float64(r.t-reelRace)/60); k > 0 && !r.skip { // the clock, spinning
		cx, cy := float64(b.Dx())/2, float64(b.Dy())/2
		g.glowAt(screen, cx, cy, 3*clockRadius*zoom/2, moonLight, .3*k)
		drawRing(screen, cx, cy, clockRadius, 1, 3, moonLight, .9*k)
		for i, hand := range [...]struct{ turn, len float64 }{{float64(g.clock%hourFrames) / hourFrames, .85}, {float64(g.clock%(12*hourFrames)) / (12 * hourFrames), .55}} {
			a := 2*math.Pi*hand.turn - math.Pi/2
			var p vector.Path
			p.MoveTo(float32(cx), float32(cy))
			p.LineTo(float32(cx+math.Cos(a)*clockRadius*hand.len), float32(cy+math.Sin(a)*clockRadius*hand.len))
			strokeP(screen, &p, float32(4-i), moonLight, k)
		}
	}
	if f := float64(r.t-reelFlash) / flashRamp; f > 0 {
		rect(screen, 0, 0, b.Dx(), b.Dy(), color.NRGBA{255, 255, 255, uint8(255 * min(1, f))})
	}
}

// reelSay: a balloon of the world pass, kept for after the flip: the line, a copy of its speaker and the screen y of
// his head (lift in), in the world's own view.
type reelSay struct {
	line   string
	guest  entities.Guest
	y      float64
	ox, oy int
}

// flipY: where screen y lands when the screen of height h is turned through scale c on its middle line.
func flipY(y, h, c float64) float64 { return h/2 + (y-h/2)*c }

// DevPrestige is main's -prestige: a busy hotel a few hours in, the cutscene running.
func (g *Game) DevPrestige() {
	for range 3 * hourFrames {
		g.step(0, 0)
	}
	g.playPrestige()
}
