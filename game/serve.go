package game

import (
	"image"
	"image/color"

	"ghosthotel/assets"
	"ghosthotel/entities"

	"github.com/hajimehoshi/ebiten/v2"
)

// Serving (D149), Comfort (D146) and Guests asking for it (D163). Placeholders, retuned at T112.
const (
	comfortStep  = 25  // Comfort a serve gives each Guest inside when answered at once; less the longer they waited
	comfortMax   = 75  // a fully looked-after Party pays +75%
	comfortBonus = 1   // % more pay per point of the Party's mean Comfort
	askMinutes   = 180 // game minutes a request waits before it expires: a fetch and a walk (E5, was 60)
	askPenalty   = 10  // Comfort each Guest loses when its request expires
	askPerDay    = 1.5 // requests a Party makes per awake day, on average
	askGap       = 180 // game minutes at least from one request's end to the next
	wakeHour     = 7   // Guests ask only from wakeHour
	sleepHour    = 23  // to sleepHour
)

// asking: the Party wants service now (its "!!" balloon is up).
func (p *Party) asking() bool { return p.asked > 0 }

// updateAsks runs each game minute (D163). A Party in its Room, awake, askGap after its last request,
// asks at random, askPerDay times a day on average. Unanswered for askMinutes, the request expires and
// each Guest loses askPenalty Comfort. One that leaves its Room lapses unpunished.
func (g *Game) updateAsks() {
	awake := !asleep(g.clock)
	for _, p := range g.parties {
		switch {
		case p.asking() && p.state != staying:
			p.asked, p.askEnded = 0, g.clock
		case p.asking() && g.clock-p.asked >= askMinutes*minuteFrames:
			for _, guest := range p.Guests {
				guest.Comfort = max(0, guest.Comfort-askPenalty)
			}
			p.asked, p.askEnded = 0, g.clock
			g.expired++
		case !p.asking() && p.state == staying && awake && (p.askEnded == 0 || g.clock-p.askEnded >= askGap*minuteFrames) &&
			g.rng.Float64() < askPerDay/float64((sleepHour-wakeHour)*60):
			p.asked, p.want = g.clock, supplies[g.rng.IntN(len(supplies))].ID
		}
	}
}

// serve answers the Room's request (D163): every Guest of its Party inside gains Comfort, comfortStep
// if served at once, falling to 0 as the request nears expiry and as the Room's dirt nears dirtMax, up
// to comfortMax. Fear is untouched. False when the
// Room's Party isn't asking.
func (g *Game) serve(room int) bool {
	r := g.hotel.Rooms[room]
	for _, p := range g.parties {
		if !p.Occupies(room, Human) || !p.asking() || p.state != staying {
			continue
		}
		wait, calm := askMinutes*minuteFrames, max(0, dirtMax-g.dirtLevel(room))
		gain := (comfortStep*(wait-(g.clock-p.asked))*calm + wait*dirtMax - 1) / (wait * dirtMax) // rounded up: worth 0 only in a Closed Room
		for _, guest := range p.Guests {
			if image.Pt(int(guest.X), int(guest.Y)).In(r.Shell()) && gain > 0 {
				guest.Comfort = min(comfortMax, guest.Comfort+gain)
			}
		}
		p.asked, p.askEnded = 0, g.clock
		g.serves++
		return true
	}
	return false
}

// The request balloon, in pixels drawn balloonScale times: a white rounded box with a bold "!!" (3-px
// strokes) inside, and a tail on the side facing its Guest.
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

// balloonImages: the box, and its tail pointing down, whose row 0 lies over the box's edge to open it.
func balloonImages() (box, tail *ebiten.Image) {
	box = paint(balloonW, balloonH, rounded(balloonW, balloonH, balloonR), func(x, y int) bool { // two "!": a bar over rows 3..10, a dot on rows 12..14
		return (x >= 6 && x <= 8 || x >= 13 && x <= 15) && (y >= 3 && y <= 10 || y >= 12 && y <= 14)
	})
	const c = balloonTail - 1 // the tip's column
	tail = paint(2*c+1, balloonTail+1, func(x, y int) bool { return y <= 0 || max(x-c, c-x) <= balloonTail-y },
		func(int, int) bool { return false })
	return box, tail
}

// tailGeoM places the tail under the w×h box (balloon px) at (x,y), or over it when the head at (hx,hy),
// screen px, is higher than the box's middle; slid along that edge toward the head. Never on a side.
func tailGeoM(x, y, w, h, hx, hy int) ebiten.GeoM {
	const c, s = balloonTail - 1, balloonScale
	along := func(v, from, n int) float64 { // the tail's first column, kept off the round corners
		return float64(min(max((v-from)/s, balloonR+c), n-1-balloonR-c) - c)
	}
	var m ebiten.GeoM
	if hy < y+h*s/2 { // above
		m.Scale(1, -1)
		m.Translate(along(hx, x, w), 1)
	} else {
		m.Translate(along(hx, x, w), float64(h-1))
	}
	m.Scale(s, s)
	m.Translate(float64(x), float64(y))
	return m
}

// drawRequests draws a balloon with the wanted item by each asking Party's leader, in both dimensions and over the Fog
// and the dark.
func (g *Game) drawRequests(s *ebiten.Image, ox, oy int) {
	if g.balloon == nil {
		g.balloon, g.tail = balloonImages()
	}
	for _, p := range g.parties {
		if p.asking() { // its item in the balloon (show, don't tell)
			if g.askBox == nil {
				g.askBox = paint(balloonW, balloonH, rounded(balloonW, balloonH, balloonR), func(int, int) bool { return false })
			}
			x, y := g.drawBalloon(s, g.askBox, p.Guests[0], ox, oy)
			if icon := g.supplyIcon(p.want); icon != nil {
				w, h := icon.Bounds().Dx(), icon.Bounds().Dy()
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(balloonScale, balloonScale)
				op.GeoM.Translate(float64(x+(balloonW-w)*balloonScale/2), float64(y+(balloonH-h)*balloonScale/2))
				s.DrawImage(icon, op)
			}
		}
		if p.state == complaining { // a splat for "!!": the Room is dirty (dirt.go)
			if g.dirtBox == nil {
				g.dirtBox = dirtBalloonImage()
			}
			g.drawBalloon(s, g.dirtBox, p.Guests[0], ox, oy)
		}
	}
}

// drawBalloon draws box (balloon px) right of the guest's head, its tail pointing at the head. Kept on
// screen: off screen, it waits at the nearest edge, its tail still pointing at the Guest. Returns the box's
// top left, screen px.
func (g *Game) drawBalloon(s, box *ebiten.Image, guest *entities.Guest, ox, oy int) (int, int) {
	if g.tail == nil {
		g.balloon, g.tail = balloonImages()
	}
	const edge = balloonTail*balloonScale + 4 // room for the tail
	bw, bh := box.Bounds().Dx(), box.Bounds().Dy()
	w, h := s.Bounds().Dx(), s.Bounds().Dy()
	top := int((guest.Y-g.lift(guest))*zoom) - oy + (entities.FootH/2-charH)*zoom // the sprite's top, screen px
	hx, hy := int(guest.X*zoom)-ox, top+4*zoom                                    // the head
	x := min(max(hx+5*zoom, edge), w-bw*balloonScale-edge)                        // right of the head, the tail's tip just over it
	y := min(max(top+6*zoom-(bh+balloonTail)*balloonScale, edge), h-bh*balloonScale-edge)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(balloonScale, balloonScale)
	op.GeoM.Translate(float64(x), float64(y))
	s.DrawImage(box, op)
	op.GeoM = tailGeoM(x, y, bw, bh, hx, hy)
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
	x, y := g.drawBalloon(s, g.boxes[w], guest, ox, oy)
	g.inked(s, line, x+(w*balloonScale-6*len(line))/2, y+(sayH*balloonScale-16)/2, 0, 0, 0)
}
