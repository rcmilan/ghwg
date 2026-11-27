package game

import (
	"image"

	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// spaceAct: what SPACE would do right now at the Concierge's spot (08 F0).
type spaceAct int

const (
	actNone spaceAct = iota
	actTerminal
	actBook
	actMirror
	actRoomMirror
	actSupply
	actDeliver
	actCard
	actClean
	actView
	actQuestTalk   // 08 W5 R8: SPACE talks to a quest character close by (verbs_card.go)
	actPoltergeist // 09: SPACE beside a Room's walls in ghost, humans inside: the tap (poltergeist.go)
)

// actionable: what SPACE would do now and the world rect of the prop it acts on (for the glow; empty when there is
// no prop: a talk, the hotel view, nothing). space() acts on it, so the glow and SPACE never disagree.
func (g *Game) actionable() (spaceAct, image.Rectangle) {
	const ts = world.TileSize
	feet := g.feet()
	if g.shifting != nil { // SPACE waits for the transition (D170)
		return actNone, image.Rectangle{}
	}
	if g.atTerminal() { // the computer: its screen over the counter
		t := g.hotel.Terminal
		return actTerminal, image.Rect(t.Min.X, t.Min.Y+ts, t.Max.X, t.Max.Y-ts)
	}
	if g.atBook() { // the guest book on the counter
		return actBook, g.bookRect()
	}
	if k := g.atCorridorMirror(); k >= 0 { // a Corridor Mirror shifts like the Lobby's once the courier uncovers it
		if m := g.hotel.Corridors[k]; m.Open {
			return actMirror, m.Glass
		}
		return actNone, image.Rectangle{}
	}
	if g.atMirror() {
		if !g.unlocked(Mirror) { // covered until the courier installs it
			return actNone, image.Rectangle{}
		}
		m := g.hotel.Mirror // the spot below it; the glass is the two cells over that
		return actMirror, image.Rect(m.X-ts/2, m.Y-ts/2-2*ts, m.X+ts/2, m.Y-ts/2)
	}
	for _, r := range g.hotel.Rooms { // a Room's own Mirror shifts too (K1)
		if feet.In(r.Mirror) {
			return actRoomMirror, image.Rect(r.Mirror.Min.X, r.Mirror.Min.Y-2*ts, r.Mirror.Max.X, r.Mirror.Min.Y)
		}
	}
	if k := g.atSupply(); k >= 0 {
		if id := supplies[k].ID; g.ghost || g.stock[id] == 0 && g.hands != id { // nothing for it here, and no hotel view either
			return actNone, image.Rectangle{}
		}
		r := supplySpot(g.hotel, k) // the prop, two cells tall, over its front row
		return actSupply, image.Rect(r.Min.X, r.Min.Y-ts, r.Max.X, r.Max.Y-ts)
	}
	if p := g.atPantry(); p >= 0 && !g.ghost && g.pantryPick(p) >= 0 { // the same, from a pantry (08 P1-U); nothing for it there falls through
		return actSupply, g.pantryRect(p)
	}
	if i := g.atMat(); i >= 0 {
		r := g.hotel.Rooms[i]
		if g.ghost && g.ghostDoor(i) { // 08 W5: a quest ghost's Request, the wreath, the handprint
			return actDeliver, image.Rect(r.Door.X*ts-ts, r.Door.Y*ts, r.Door.X*ts+2*ts, r.Door.Y*ts+4*ts)
		}
		if !g.ghost && g.wants(i, g.hands) { // the door and its mat
			return actDeliver, image.Rect(r.Door.X*ts-ts, r.Door.Y*ts, r.Door.X*ts+2*ts, r.Door.Y*ts+4*ts)
		}
		if g.cardParty(i) != nil && feet.In(r.Plate()) { // the guest card, from the plate by the door, in either dimension (T375)
			return actCard, r.Plate().Sub(image.Pt(0, ts))
		}
		if feet.In(r.Plate()) {
			return actNone, image.Rectangle{}
		}
	}
	if p, _ := g.questTalk(); p != nil { // before cleaning, the walkers' and the Room's alike
		return actQuestTalk, image.Rectangle{}
	}
	if room, cell := g.cleanable(); room >= 0 {
		return actClean, image.Rect(cell.X*ts, cell.Y*ts, cell.X*ts+ts, cell.Y*ts+ts)
	}
	if room, cell := g.tapTarget(); room >= 0 { // after everything else: the tap is only where nothing else applies
		if cell.X < 0 {
			return actPoltergeist, image.Rectangle{}
		}
		return actPoltergeist, image.Rect(cell.X*ts, cell.Y*ts, cell.X*ts+ts, cell.Y*ts+ts)
	}
	return actView, image.Rectangle{} // SPACE where nothing else is its to do: the hotel view (U2)
}

// atMat: the Room whose doormat the Concierge stands on, or -1.
func (g *Game) atMat() int {
	for i, r := range g.hotel.Rooms {
		if g.feet().In(r.Mat()) {
			return i
		}
	}
	return -1
}

// space: SPACE in play does what actionable says.
func (g *Game) space() {
	switch act, _ := g.actionable(); act {
	case actTerminal:
		g.openTerminal()
	case actBook:
		g.openBook()
	case actMirror, actRoomMirror:
		g.startShift()
	case actSupply:
		if p := g.atPantry(); p >= 0 {
			g.fetch(g.pantryPick(p))
		} else {
			g.fetch(g.atSupply())
		}
	case actDeliver:
		if g.ghost {
			g.ghostDeliver(g.atMat())
		} else {
			g.deliver(g.atMat(), g.hands)
		}
	case actCard:
		g.startCard(g.atMat())
	case actQuestTalk:
		g.questSpeak(g.questTalk())
	case actClean:
		g.clean(g.cleanable())
	case actPoltergeist:
		g.poltTapNow()
	case actView:
		g.openView()
	default:
		if g.shifting == nil && (g.atMirror() || g.atCorridorMirror() >= 0) {
			g.refuse("THE MIRROR IS COVERED")
		}
	}
}

const actGlow = 0.45 // the lit prop's glow at its centre (placeholder)

// drawActGlow lights the prop SPACE would act on, in the Ectoplasm ink, over the dark (08 F0 "Glow"): only in its
// range and only while SPACE there does something.
func (g *Game) drawActGlow(s *ebiten.Image, ox, oy int) {
	if g.state != StatePlay || g.intro != nil {
		return
	}
	_, r := g.actionable()
	if r.Empty() {
		return
	}
	r = r.Inset(-world.TileSize / 2)
	d := float64(g.glow.Bounds().Dx())
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, Blend: ebiten.BlendLighter}
	op.GeoM.Scale(float64(r.Dx()*zoom)/d, float64(r.Dy()*zoom)/d)
	op.GeoM.Translate(float64(r.Min.X*zoom-ox), float64(r.Min.Y*zoom-oy))
	op.ColorScale.ScaleWithColor(resInk(Ecto))
	op.ColorScale.ScaleAlpha(actGlow)
	s.DrawImage(g.glow, op)
}
