package game

import (
	"image"
	"math"

	"ghosthotel/entities"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 W5-SCENE: the Eternal Eldritch MacGuffin (circle.md). Once the finale has summoned it (questRec.MacGuffin) it books
// the best Room of every later Run, from the first Growth, and stays: a regular human Party that never checks out,
// asks, flees or is moved, pays macguffinPay times the Rate whatever the Mood, and is drawn the same in both Dimensions.
const (
	macguffinKey  = "macguffin" // its guest's Who.Key
	macguffinPay  = 5           // times the Room's Rate (placeholder)
	macguffinTall = 1.5         // of the Concierge's height
)

// isMacguffin: the Party is the MacGuffin's.
func isMacguffin(p *Party) bool { return len(p.Guests) > 0 && p.Guests[0].Who.Key == macguffinKey }

// macguffinParty: its Party this Run, nil before it has checked in.
func (g *Game) macguffinParty() *Party {
	for _, p := range g.parties {
		if isMacguffin(p) {
			return p
		}
	}
	return nil
}

// partyPay: the Mood factor on what the Party pays: the MacGuffin's is always 1.
func (g *Game) partyPay(p *Party) Num {
	if isMacguffin(p) {
		return 1
	}
	return g.moodPay(p.Room)
}

// macguffinRoom: the best Room to book: the owned, open, Vacant one with the highest Rate (the first of them), -1 if none.
func (g *Game) macguffinRoom() (best int) {
	best = -1
	for i, r := range g.hotel.Rooms {
		if g.isOwned(i) && g.vacant(i, Human) && !g.closed(i) && !g.changing(i) && len(r.Stays) > 0 &&
			(best < 0 || r.Type.Rate > g.hotel.Rooms[best].Type.Rate) {
			best = i
		}
	}
	return
}

// macguffinIn runs each game minute: once summoned, it books the best Room from the first Growth, once a Run.
func (g *Game) macguffinIn() {
	if g.questRec.MacGuffin && g.grown() && g.macguffinParty() == nil {
		g.macguffinCheckIn()
	}
}

// macguffinCheckIn books it into the best Room now, and walks it in from the Street. Nil if no Room is free.
func (g *Game) macguffinCheckIn() *Party {
	room := g.macguffinRoom()
	if room < 0 {
		return nil
	}
	guest := entities.NewGuest(g.hotel.Spawn, "Bob")
	c := macguffinCard
	guest.Who = entities.Identity{Name: c.Name, AgeNote: c.Age, Job: c.Job, Lines: c.Lines, Key: macguffinKey}
	guest.RoomIndex = room
	p := &Party{Guests: []*entities.Guest{&guest}, settled: true, arrived: g.clock,
		Booking: Booking{Room: room, Nights: 1, Routine: homebody, Rate: macguffinPay * g.hotel.Rooms[room].Type.Rate, Checkout: math.MaxInt32}}
	setNerve(p, nerveMax) // every ghost fits it
	g.parties = append(g.parties, p)
	g.checkIn(p)
	return p
}

// sceneActors: the MacGuffin, for the y-sorted pass, in either Dimension.
func (g *Game) sceneActors() []actor {
	p := g.macguffinParty()
	if p == nil || !p.shown(p.Guests[0]) {
		return nil
	}
	guest := p.Guests[0]
	a := actor{x: guest.X, y: guest.Y, name: macguffinKey}
	a.draw = func(s *ebiten.Image, x, y int) {
		fog := g.fogAt(image.Pt(int(guest.X), int(guest.Y)))
		drawFeetShadow(s, x, y, fog)
		g.drawMacguffin(s, x, y, 1, macguffinTall, guest.Anim, float32(1-fog))
	}
	return []actor{a}
}

// updateScene runs each step of world time: nothing. A cutscene stops the world and runs from Update (stepScene).
func (g *Game) updateScene() {}

// starSrc: the MacGuffin's body, from image 0 (the Concierge's frame, stretched): solid black where the sprite is,
// with stars drifting up inside the silhouette; T is the frame.
const starSrc = `//kage:unit pixels
package main

var T float

func h(p vec2) float {
	return fract(sin(dot(p, vec2(12.9898, 78.233))) * 43758.5453)
}

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	a := imageSrc0At(src).a
	if a == 0 {
		return vec4(0)
	}
	p := src - imageSrc0Origin()
	var s float
	for i := 0; i < 3; i++ {
		c := 5.0 + 4.0*float(i)
		q := (p + vec2(0, T*(0.12+0.08*float(i)))) / c
		id := floor(q)
		f := fract(q) - 0.5
		r := h(id + float(i)*19.0)
		o := (vec2(h(id+3.1), h(id+7.7)) - 0.5) * 0.6
		s += step(r, 0.5) * (1 - smoothstep(0, 0.22, length(f-o))) * (0.5 + 0.5*sin(T*0.07+r*40.0))
	}
	s = min(s, 1)
	return vec4(vec3(0.85, 0.8, 1.0)*s*a, a) * color.a
}
`

// macLook: the star shader and the stretched frame it reads, built on first draw.
type macLook struct {
	shader *ebiten.Shader
	buf    *ebiten.Image
}

const macBufW, macBufH = 2 * charW * zoom, 2 * charH * zoom // room for a frame stretched up to 2x each way

// drawMacguffin draws the Concierge's frame of anim stretched sx wide and sy tall, solid black with stars inside,
// its soles at (x,y) (a foot box center, screen px), at opacity a.
func (g *Game) drawMacguffin(s *ebiten.Image, x, y int, sx, sy float64, anim entities.Anim, a float32) {
	m := &g.scene.mac
	if m.shader == nil {
		var err error
		if m.shader, err = ebiten.NewShader([]byte(starSrc)); err != nil {
			panic(err)
		}
		m.buf = ebiten.NewImage(macBufW, macBufH)
	}
	sheet := g.sheet("Bob")[0]
	if anim.Moving {
		sheet = g.sheet("Bob")[1]
	}
	f := anim.Frame()
	m.buf.Clear()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(zoom*sx, zoom*sy)
	op.GeoM.Translate((macBufW-charW*zoom*sx)/2, macBufH-charH*zoom*sy)
	m.buf.DrawImage(sheet.SubImage(image.Rect(f*charW, 0, (f+1)*charW, charH)).(*ebiten.Image), op)
	so := &ebiten.DrawRectShaderOptions{Images: [4]*ebiten.Image{m.buf}, Uniforms: map[string]any{"T": float32(g.tick)}}
	so.GeoM.Translate(float64(x-macBufW/2), float64(y+entities.FootH/2*zoom-macBufH))
	so.ColorScale.ScaleAlpha(a)
	s.DrawRectShader(macBufW, macBufH, m.shader, so)
}

// DevMacguffin is main's -macguffin: a busy Run, the MacGuffin summoned and checked into the best Room, the Concierge
// beside it.
func (g *Game) DevMacguffin() {
	g.questRec.MacGuffin = true
	p := g.macguffinCheckIn()
	if p == nil {
		return
	}
	g.settleIn(p)
	at := g.hotel.Rooms[p.Room].Stays[0]
	g.player.X, g.player.Y = float64(at.X+18), float64(at.Y+6)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}
