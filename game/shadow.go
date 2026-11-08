package game

import (
	"fmt"
	"image"
	"math"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// The Shadow Person (D169): a ghost bought with Spook that runs along a Room's back wall. Placeholders.
const (
	shadowPrice   = 80 // Spook
	shadowSpeed   = 4 * conciergeSpeed
	shadowEvery   = 20 // game minutes between its steady 1 Fear to each Guest inside
	shadowSeen    = 10 // Fear a run gives a Guest who sees it
	shadowUnseen  = 2  // and one who doesn't
	startleHeight = 4  // world px: a Guest's hop on seeing it (the flee's is hopHeight)
)

var (
	sightLines = []string{"What was that?!", "A shadow!", "Eek!", "Something moved!"}
	shadowTint = []float32{.45, .2, .7} // ghost: a black figure would vanish on the black maze walls
	shadowCast = append([]string{"Bob"}, guestCast[:]...)
)

// shade is a Room's Shadow Person: between runs dir is 0; running, x moves by dir × shadowSpeed.
type shade struct {
	name string // the character it's the silhouette of
	next int    // the clock its next run is due
	x    float64
	dir  float64
	anim entities.Anim
	seen map[*entities.Guest]int // this run's sightings: the tick each Guest saw it
}

func (g *Game) shaded(room int) bool { return room < len(g.shades) && g.shades[room] != nil }

// buyShadow pays shadowPrice Spook for a Shadow Person in the Room; else why not.
func (g *Game) buyShadow(room int) string {
	switch {
	case g.shaded(room):
		return "ROOM " + g.hotel.Rooms[room].Number + " HAS A SHADOW ALREADY"
	case !g.pay(Cost{Spook: shadowPrice}):
		return fmt.Sprintf("NOT ENOUGH SPOOK: %d", shadowPrice)
	}
	g.shades[room] = &shade{name: shadowCast[g.chat.IntN(len(shadowCast))], next: g.clock + g.shadowGap(room), anim: entities.NewAnim()}
	return ""
}

// shadowGap: frames between runs, from 1 every 4 game hours at 0 Room Spook to 1 an hour at spookLimit (S12 review: 4 was too often).
func (g *Game) shadowGap(room int) int {
	s := min(1, float64(g.roomSpook[room])/spookLimit)
	return int(60 / (0.25 + 0.75*s) * minuteFrames)
}

// wallRun: a Room's back wall, where a Shadow runs: from x0 to x1 (world px, the side walls' inner edges)
// with its feet at y.
func wallRun(r world.Room) (x0, x1, y float64) {
	o := r.Origin.Mul(world.TileSize)
	return float64(o.X + world.TileSize), float64(o.X + (world.RoomW-1)*world.TileSize), float64(o.Y+2*world.TileSize) - entities.FootH/2
}

// inside: the Guests of the Room's Party inside it.
func (g *Game) inside(room int) (*Party, []*entities.Guest) {
	for _, p := range g.parties {
		if p.Occupies(room) && p.state != out {
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

// updateShadows runs each frame: a due Shadow starts a run if its Room has a Guest inside, from outside
// one side wall to outside the other. A Guest who sees it gets shadowSeen Fear, hops and speaks; at the
// end, each one inside who didn't gets shadowUnseen.
func (g *Game) updateShadows() {
	for i, sh := range g.shades {
		if sh == nil {
			continue
		}
		p, in := g.inside(i)
		x0, x1, y := wallRun(g.hotel.Rooms[i])
		if sh.dir == 0 {
			if g.clock < sh.next {
				continue
			}
			if sh.next = g.clock + g.shadowGap(i); len(in) == 0 {
				continue
			}
			sh.dir, sh.x, sh.seen = 1, x0-charW/2, map[*entities.Guest]int{}
			if g.chat.IntN(2) == 0 {
				sh.dir, sh.x = -1, x1+charW/2
			}
		}
		sh.x += sh.dir * shadowSpeed
		sh.anim.Update(sh.x, y)
		sh.anim.Facing = entities.Toward(sh.dir, 0)
		for _, guest := range in {
			if _, ok := sh.seen[guest]; !ok && g.sees(guest, sh.x, y) {
				sh.seen[guest] = g.tick
				g.frighten(p, guest, shadowSeen, fearLimit-1)
				who := slices.Index(p.Guests, guest)
				p.talk = append(slices.DeleteFunc(p.talk, func(l line) bool { return l.who == who }), line{who, sightLines[g.chat.IntN(len(sightLines))], g.tick})
			}
		}
		if sh.x < x0-charW/2 || sh.x > x1+charW/2 {
			sh.dir = 0
			for _, guest := range in {
				if _, ok := sh.seen[guest]; !ok {
					g.frighten(p, guest, shadowUnseen, fearLimit-1)
				}
			}
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

// frighten: n Fear to a Guest of p, but not past limit; the hotel and its Room gain the Spook (D148).
func (g *Game) frighten(p *Party, guest *entities.Guest, n, limit int) {
	n = max(0, min(n, limit-guest.Fear))
	guest.Fear += n
	g.earn(Spook, Num(n), noFloat) // the haunt floats its own batched "+N"
	g.roomSpook[p.Room] += n
	p.spookShown += n
}

// startle: how high a Guest is in its hop on seeing a Shadow, world px.
func (g *Game) startle(guest *entities.Guest) float64 {
	for _, sh := range g.shades {
		if t, ok := sh.seenAt(guest); ok && g.tick-t < hopFrames {
			return startleHeight * math.Sin(math.Pi*float64(g.tick-t)/hopFrames)
		}
	}
	return 0
}

func (sh *shade) seenAt(guest *entities.Guest) (int, bool) {
	if sh == nil {
		return 0, false
	}
	t, ok := sh.seen[guest]
	return t, ok
}

// drawShadow draws any running Shadow over back-wall tile p, clipped to its cell: the side walls hide
// where it steps out, and the windows and props drawn after cover it. Fogged with the wall in real.
func (g *Game) drawShadow(s *ebiten.Image, p prop, ox, oy int) {
	const ts = world.TileSize
	for i, sh := range g.shades {
		o := g.hotel.Rooms[i].Origin
		if sh == nil || sh.dir == 0 || p.cy-o.Y > 1 || p.cy < o.Y || p.cx <= o.X || p.cx >= o.X+world.RoomW-1 {
			continue
		}
		_, _, y := wallRun(g.hotel.Rooms[i])
		cell := image.Rect(p.cx*ts*zoom-ox, p.cy*ts*zoom-oy, (p.cx+1)*ts*zoom-ox, (p.cy+1)*ts*zoom-oy)
		f := sh.anim.Frame()
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(zoom, zoom)
		op.GeoM.Translate(float64(screenPx(sh.x)-ox-charW/2*zoom), float64(screenPx(y)-oy+(entities.FootH/2-charH)*zoom))
		op.ColorScale.ScaleAlpha(float32(1 - g.propFog(p)))
		if !g.ghost {
			op.ColorScale.Scale(0, 0, 0, 1)
		}
		frame := g.sheet(sh.name)[1].SubImage(image.Rect(f*charW, 0, (f+1)*charW, charH)).(*ebiten.Image)
		g.blit(s.SubImage(cell).(*ebiten.Image), frame, op, modeActor, 0, shadowTint, 0)
	}
}
