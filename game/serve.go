package game

import (
	"image/color"
	"math"

	"ghosthotel/assets"
	"ghosthotel/entities"

	"github.com/hajimehoshi/ebiten/v2"
)

// Serving (D149) and Guests asking for it (D163): a serve raises the Room's Mood, an expired ask lowers it
// (moodServe, moodExpire). Placeholders, retuned at T112.
const (
	askMinutes = 270 // game minutes a request waits before it expires: 45 s, the worst 3x3 trip and detours (08 F0, was 180)
	askPerDay  = 1.5 // requests a Party makes per awake day, on average
	askGap     = 180 // game minutes at least from one request's end to the next
	wakeHour   = 7   // Guests ask only from wakeHour
	sleepHour  = 23  // to sleepHour
)

// asking: the Party wants service now (its "!!" balloon is up).
func (p *Party) asking() bool { return p.asked > 0 }

// updateAsks runs each game minute (D163). A Party in its Room, awake, askGap after its last request,
// asks at random, askPerDay times a day on average. Unanswered for askMinutes, the request expires and
// the Room loses moodExpire Mood. One that leaves its Room lapses unpunished.
func (g *Game) updateAsks() {
	awake := !asleep(g.clock)
	for _, p := range g.parties {
		if p.Realm != Human { // a ghost asks for nothing
			continue
		}
		switch {
		case p.asking() && p.state != staying:
			p.asked, p.askEnded = 0, g.clock
		case p.asking() && g.clock-p.asked >= askMinutes*minuteFrames:
			g.addMood(p.Room, -moodExpire)
			p.asked, p.askEnded = 0, g.clock
			g.expired++
		case !p.asking() && p.state == staying && awake && (p.askEnded == 0 || g.clock-p.askEnded >= askGap*minuteFrames) &&
			g.rng.Float64() < askPerDay/float64((sleepHour-wakeHour)*60):
			p.asked, p.want = g.clock, supplies[g.rng.IntN(len(supplies))].ID
		}
	}
}

// serve answers the Room's request (D163): its Mood rises by moodServe if served at once, falling to 0 as
// the request nears expiry. Fear is untouched. False when the Room's Party isn't asking.
func (g *Game) serve(room int) bool {
	for _, p := range g.parties {
		if !p.Occupies(room, Human) || !p.asking() || p.state != staying {
			continue
		}
		wait := askMinutes * minuteFrames
		g.addMood(room, (moodServe*(wait-(g.clock-p.asked))+wait-1)/wait) // rounded up
		p.asked, p.askEnded = 0, g.clock
		g.serves++
		return true
	}
	return false
}

// The balloons, in pixels drawn balloonScale times: a white rounded box balloonH tall (the ask pill's, as wide as
// it needs; a speech balloon's sayH) and a tail on the side facing its Guest.
const balloonW, balloonH, balloonR, balloonTail, balloonScale = 22, 18, 4, 4, 3

// paint builds a w×h bitmap of the shape in: white inside, Night on its edge, black where ink.
func paint(w, h int, in, ink func(x, y int) bool) *ebiten.Image {
	img := ebiten.NewImage(w, h)
	for y := range h {
		for x := range w {
			switch {
			case !in(x, y):
			case ink(x, y):
				img.Set(x, y, color.Black)
			case in(x-1, y) && in(x+1, y) && in(x, y-1) && in(x, y+1):
				img.Set(x, y, color.White)
			default:
				img.Set(x, y, assets.Night)
			}
		}
	}
	return img
}

// rounded: a w×h box with corners of radius r.
func rounded(w, h, r int) func(x, y int) bool {
	return func(x, y int) bool {
		cx, cy := min(max(x, r), w-1-r), min(max(y, r), h-1-r)
		return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r
	}
}

// tailImage: the balloon's tail pointing down, whose row 0 lies over the box's edge to open it.
func tailImage() *ebiten.Image {
	const c = balloonTail - 1 // the tip's column
	return paint(2*c+1, balloonTail+1, func(x, y int) bool { return y <= 0 || max(x-c, c-x) <= balloonTail-y },
		func(int, int) bool { return false })
}

// tailGeoM places the tail under the w×h box (balloon px, sc screen px each) at (x,y), or over it when the head at
// (hx,hy), screen px, is higher than the box's middle; slid along that edge toward the head. Never on a side.
func tailGeoM(x, y, w, h, hx, hy int, sc float64) ebiten.GeoM {
	const c = balloonTail - 1
	along := func(v, from, n int) float64 { // the tail's first column, kept off the round corners
		return float64(min(max(int(float64(v-from)/sc), balloonR+c), n-1-balloonR-c) - c)
	}
	var m ebiten.GeoM
	if float64(hy) < float64(y)+float64(h)*sc/2 { // above
		m.Scale(1, -1)
		m.Translate(along(hx, x, w), 1)
	} else {
		m.Translate(along(hx, x, w), float64(h-1))
	}
	m.Scale(sc, sc)
	m.Translate(float64(x), float64(y))
	return m
}

// The ask pill (08 F0 "Pills"), in balloon px: the Room's number, then the item asked for or the dirt splat.
const pillPad, pillGap = 4, 3

var (
	pills = map[int]*ebiten.Image{} // the pill's box, by width in balloon px
	splat *ebiten.Image             // the dirt complaint's, white (splatRows)
)

// drawRequests draws a pill by each asking or complaining Party's leader, in
// both dimensions and over the Fog and the dark. The world shows at kz screen px a world px, its top left at (ox,oy).
func (g *Game) drawRequests(s *ebiten.Image, ox, oy int, kz float64) {
	if splat == nil {
		splat = bitmap(splatRows)
	}
	for _, p := range g.parties {
		switch icon := g.supplyIcon(p.want); {
		case p.state == complaining: // the Room is dirty (dirt.go)
			g.drawPill(s, p, splat, 2*balloonScale, ox, oy, kz)
		case p.asking() && icon != nil: // its item (show, don't tell)
			g.drawPill(s, p, icon, balloonScale, ox, oy, kz)
		}
	}
}

// drawPill draws the Party's pill: its Room's number in black, then icon at iconK screen px a pixel (the splat in
// black); the box as wide as they need, sc screen px a balloon px.
func (g *Game) drawPill(s *ebiten.Image, p *Party, icon *ebiten.Image, iconK float64, ox, oy int, kz float64) {
	const sc float64 = balloonScale
	num := g.hotel.Rooms[p.Room].Number
	text := sc * 2 / 3 // the debug font at 2× beside a 3× icon
	tw, iw, ih := float64(len(num)*glyphW)*text, float64(icon.Bounds().Dx())*iconK, float64(icon.Bounds().Dy())*iconK
	w := int(math.Ceil((tw+iw)/sc)) + 2*pillPad + pillGap
	if pills[w] == nil {
		pills[w] = paint(w, balloonH, rounded(w, balloonH, balloonR), func(int, int) bool { return false })
	}
	x, y := g.drawBalloon(s, pills[w], p.Guests[0], ox, oy, kz, sc)
	left, mid := float64(x)+pillPad*sc, float64(y)+balloonH*sc/2
	printAt(s, num, int(left), int(mid-7.5*text), text, color.Black, 1) // the digits sit on rows 3..12 of the 16
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(iconK, iconK)
	op.GeoM.Translate(math.Round(left+tw+pillGap*sc), math.Round(mid-ih/2))
	if icon == splat {
		op.ColorScale.Scale(0, 0, 0, 1)
	}
	s.DrawImage(icon, op)
}

// balloonAt places a bw×bh box (balloon px, sc screen px each) right of the head of a guest at world px (x,y), the world
// shown at kz screen px a world px with its top left at (ox,oy), on a w×h screen; kept on it: off screen, it waits at the
// nearest edge. Returns the box's top left and the head, screen px.
func balloonAt(x, y, kz float64, ox, oy, bw, bh int, sc float64, w, h int) (bx, by, hx, hy int) {
	edge := int(balloonTail*sc) + 4                                 // room for the tail
	top := int(y*kz) - oy + int(float64(entities.FootH/2-charH)*kz) // the sprite's top, screen px
	hx, hy = int(x*kz)-ox, top+int(4*kz)                            // the head
	bx = min(max(hx+int(5*kz), edge), w-int(float64(bw)*sc)-edge)   // right of the head, the tail's tip just over it
	by = min(max(top+int(6*kz)-int(float64(bh+balloonTail)*sc), edge), h-int(float64(bh)*sc)-edge)
	return
}

// drawBalloon draws box (balloon px, sc screen px each) right of the guest's head, its tail pointing at the head, the
// world at kz screen px a world px (balloonAt). Returns the box's top left, screen px.
func (g *Game) drawBalloon(s, box *ebiten.Image, guest *entities.Guest, ox, oy int, kz, sc float64) (int, int) {
	if g.tail == nil {
		g.tail = tailImage()
	}
	bw, bh := box.Bounds().Dx(), box.Bounds().Dy()
	x, y, hx, hy := balloonAt(guest.X, guest.Y-g.lift(guest), kz, ox, oy, bw, bh, sc, s.Bounds().Dx(), s.Bounds().Dy())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(sc, sc)
	op.GeoM.Translate(float64(x), float64(y))
	s.DrawImage(box, op)
	op.GeoM = tailGeoM(x, y, bw, bh, hx, hy, sc)
	s.DrawImage(g.tail, op)
	return x, y
}

// A speech balloon (D165) is the request balloon's box sized to its line, the text black at 1×, 6 px a
// character in the debug font.
const sayH, sayPad = 8, 8 // balloon px tall; screen px each side of the text, its 3-px edge included

// say draws line in a speech balloon by the guest's head.
func (g *Game) say(s *ebiten.Image, line string, guest *entities.Guest, ox, oy int) {
	w := max((6*len(line)+2*sayPad+balloonScale-1)/balloonScale, 2*(balloonR+balloonTail)-1) // room for the tail
	if g.boxes == nil {
		g.boxes = map[int]*ebiten.Image{}
	}
	if g.boxes[w] == nil {
		g.boxes[w] = paint(w, sayH, rounded(w, sayH, 3), func(int, int) bool { return false })
	}
	x, y := g.drawBalloon(s, g.boxes[w], guest, ox, oy, zoom, balloonScale)
	g.inked(s, line, x+(w*balloonScale-6*len(line))/2, y+(sayH*balloonScale-16)/2, 0, 0, 0)
}
