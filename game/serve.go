package game

import (
	"image"
	"image/color"

	"ghosthotel/assets"
	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// Serving (D149), Comfort (D146) and Guests asking for it (D163). Placeholders, retuned at T112.
const (
	comfortStep  = 25  // Comfort a serve gives each Guest inside when answered at once; less the longer they waited
	comfortMax   = 75  // a fully looked-after Party pays +75%
	comfortBonus = 1   // % more pay per point of the Party's mean Comfort
	askMinutes   = 60  // game minutes a request waits before it expires
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
	awake := hour(g.clock) >= wakeHour && hour(g.clock) < sleepHour
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
			p.asked = g.clock
		}
	}
}

// serve answers the Room's request (D163): every Guest of its Party inside gains Comfort, comfortStep
// if served at once, falling to 0 as the request nears expiry, up to comfortMax, and a heart pops over
// the door for each. Fear is untouched. False when the Room's Party isn't asking.
func (g *Game) serve(room int) bool {
	r := g.hotel.Rooms[room]
	for _, p := range g.parties {
		if !p.Occupies(room) || !p.asking() || p.state != staying {
			continue
		}
		wait := askMinutes * minuteFrames
		gain := (comfortStep*(wait-(g.clock-p.asked)) + wait - 1) / wait // rounded up: a serve is never worth 0
		n := 0
		for _, guest := range p.Guests {
			if image.Pt(int(guest.X), int(guest.Y)).In(r.Shell()) {
				guest.Comfort = min(comfortMax, guest.Comfort+gain)
				n++
			}
		}
		door := r.Door.Mul(world.TileSize)
		for i := range n { // one heart per Guest served, side by side over the door: the Room's inside is off screen from its door
			g.floaters = append(g.floaters, floater{float64(door.X+world.TileSize/2) + float64(2*i-n+1)*5, float64(door.Y), "", g.tick, floatHeart})
		}
		p.asked, p.askEnded = 0, g.clock
		g.serves++
		return true
	}
	return false
}

// heartRows is the serve pop, 7x6 px.
var heartRows = [...]string{
	".XX.XX.",
	"XXXXXXX",
	"XXXXXXX",
	".XXXXX.",
	"..XXX..",
	"...X...",
}

// drawHeart draws the pop centered on (x,y), screen px.
func (g *Game) drawHeart(s *ebiten.Image, x, y int) {
	if g.heart == nil {
		g.heart = ebiten.NewImage(len(heartRows[0]), len(heartRows))
		for j, row := range heartRows {
			for i, c := range row {
				if c == 'X' {
					g.heart.Set(i, j, color.RGBA{255, 90, 140, 255})
				}
			}
		}
	}
	const scale = 2
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(float64(x-len(heartRows[0])*scale/2), float64(y-len(heartRows)*scale/2))
	s.DrawImage(g.heart, op)
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

// balloonImages: the box, and its tail pointing down, whose row 0 lies over the box's edge to open it.
func balloonImages() (box, tail *ebiten.Image) {
	box = paint(balloonW, balloonH, func(x, y int) bool {
		cx, cy := min(max(x, balloonR), balloonW-1-balloonR), min(max(y, balloonR), balloonH-1-balloonR)
		return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= balloonR*balloonR
	}, func(x, y int) bool { // two "!": a bar over rows 3..10, a dot on rows 12..14
		return (x >= 6 && x <= 8 || x >= 13 && x <= 15) && (y >= 3 && y <= 10 || y >= 12 && y <= 14)
	})
	const c = balloonTail - 1 // the tip's column
	tail = paint(2*c+1, balloonTail+1, func(x, y int) bool { return y <= 0 || max(x-c, c-x) <= balloonTail-y },
		func(int, int) bool { return false })
	return box, tail
}

// tailGeoM places the tail on the side of the box at (x,y) that faces the head at (hx,hy), all screen px,
// slid along that side toward the head.
func tailGeoM(x, y, hx, hy int) ebiten.GeoM {
	const c, s = balloonTail - 1, balloonScale
	along := func(v, from, n int) float64 { // the tail's first column, kept off the round corners
		return float64(min(max((v-from)/s, balloonR+c), n-1-balloonR-c) - c)
	}
	var m ebiten.GeoM
	switch {
	case hy >= y+balloonH*s: // below
		m.Translate(along(hx, x, balloonW), balloonH-1)
	case hy < y: // above
		m.Scale(1, -1)
		m.Translate(along(hx, x, balloonW), 1)
	default: // beside: the tail turned on its side
		m.SetElement(0, 0, 0)
		m.SetElement(0, 1, 1)
		m.SetElement(1, 0, 1)
		m.SetElement(1, 1, 0)
		if hx < x+balloonW*s/2 {
			m.Scale(-1, 1)
			m.Translate(1, along(hy, y, balloonH))
		} else {
			m.Translate(balloonW-1, along(hy, y, balloonH))
		}
	}
	m.Scale(s, s)
	m.Translate(float64(x), float64(y))
	return m
}

// drawRequests draws a "!!" balloon by the head of each asking Party's leader, in both dimensions and
// over the Fog and the dark. Kept on screen: off screen, it waits at the nearest edge, its tail still
// pointing at the Guest.
func (g *Game) drawRequests(s *ebiten.Image, ox, oy int) {
	if g.balloon == nil {
		g.balloon, g.tail = balloonImages()
	}
	const bw, bh, edge = balloonW * balloonScale, balloonH * balloonScale, balloonTail*balloonScale + 4 // room for the tail
	w, h := s.Bounds().Dx(), s.Bounds().Dy()
	for _, p := range g.parties {
		if !p.asking() {
			continue
		}
		lead := p.Guests[0]
		top := int(lead.Y*zoom) - oy + (entities.FootH/2-charH)*zoom // the sprite's top, screen px
		hx, hy := int(lead.X*zoom)-ox, top+4*zoom                     // the head
		x := min(max(hx+5*zoom, edge), w-bw-edge)                     // right of the head, the tail's tip just over it
		y := min(max(top+6*zoom-(balloonH+balloonTail)*balloonScale, edge), h-bh-edge)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(balloonScale, balloonScale)
		op.GeoM.Translate(float64(x), float64(y))
		s.DrawImage(g.balloon, op)
		op.GeoM = tailGeoM(x, y, hx, hy)
		s.DrawImage(g.tail, op)
	}
}
