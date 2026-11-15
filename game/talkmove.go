package game

import (
	"cmp"
	"fmt"
	"image/color"
	"math"
	"slices"

	"ghosthotel/entities"

	"github.com/hajimehoshi/ebiten/v2"
)

// The talk (07 X2): in a Room with its human Party in, and some other Room it fits, TALK shows on the HUD's
// right. SPACE stages it: the camera closes in on the Concierge and the guest, the rest blurs into the ghost
// purple, and the world slows down (D1's g.scale, talkSlow). A/D picks the Room to offer, W/S the offer, SPACE makes it, ESC leaves. Each
// target Room has three offers: a plain ask, free, that may be refused; a discount, some Money, likely; and the
// night on the house, a night's Rate, sure. A scared guest or a nicer target Room makes them likelier. A refusal
// costs the guest's Room some Mood and the talk goes on. Taken, both Parties walk over by themselves (match.go's
// relocate) and the Concierge is free at once. Ghosts can't be talked to. Placeholders.
const (
	talkIn      = 24   // frames the camera takes to close in; the guest speaks half way, the offers show at the end
	talkZoomMax = 1.8  // how close it gets, of the play camera
	talkSlow    = 0.25 // of the world's time scale while the talk is on
	refuseMood  = 40   // Mood a refusal costs the Room
	offerLean   = 0.3  // of what an offer's odds lack, the most a scared guest and a cozy target Room make up
)

const (
	plainAsk = iota
	discount
	onTheHouse
)

var (
	offerShare = [3]float64{0, 0.2, 1}    // of a night's Rate, for each Party that moves: on the house is the whole night
	offerBase  = [3]float64{0.05, 0.6, 1} // the odds with a calm guest and a Room at Mood -100
)

// parley: the talk under way.
type parley struct {
	p       *Party
	targets []int   // the Rooms it fits, best fit first
	to      int     // the one picked, an index into targets
	pick    int     // the offer picked
	t       int     // real frames since it began: the zoom, the blur and the lines don't slow down
	line    string  // what the guest says now
	lineAt  int     // the t it began saying it
	scale   float64 // the world's time scale as the talk found it, given back as it ends
}

var (
	talkCalm   = []string{"Yes?", "Can I help you?", "Everything all right?"}
	talkUneasy = []string{"It's cold in here, isn't it?", "Did you hear that too?", "Something feels off..."}
	talkScared = []string{"I can't stay in here!", "Something is in this room!", "Did you SEE that?!"}
	talkNo     = []string{"I'm fine here.", "No, thank you.", "We just unpacked!", "Not for that."}
	talkYes    = []string{"Deal!", "Oh, gladly!", "Lead the way!"}
	// the Concierge's offers, [calm][scared]: by the guest's state
	offerLines = [3][2]string{
		{"Could you switch to Room %s?", "I think you'd be better in Room %s."},
		{"Room %s, and a little off the bill?", "Room %s is calmer. Some off the bill?"},
		{"Room %s, tonight on the house.", "Room %s, and tonight is on us."},
	}
)

// fright: how scared the Party's most frightened member is, 0..1 of its FleeAt.
func fright(p *Party) (f float64) {
	for _, h := range p.Guests {
		f = max(f, float64(h.Fear)/float64(max(1, h.FleeAt)))
	}
	return f
}

// targets: the Rooms the Party could move to, best fit first: every other owned, open Room with Stay Spots for
// it whose ghost it fits, Vacant or held by a human Party that could swap: one that fits this Room's beds and ghost.
func (g *Game) targets(p *Party) []int {
	var ts []int
	for i, r := range g.hotel.Rooms {
		if i == p.Room || !g.isOwned(i) || g.closed(i) || g.changing(i) || len(r.Stays) < len(p.Guests) || g.roomPhantasm(i) > partyNerve(p) {
			continue
		}
		if q := g.humanIn(i); q != nil && (!movable(q) || len(g.hotel.Rooms[p.Room].Stays) < len(q.Guests) || g.roomPhantasm(p.Room) > partyNerve(q)) {
			continue
		}
		ts = append(ts, i)
	}
	slices.SortStableFunc(ts, func(a, b int) int { return cmp.Compare(g.roomPhantasm(b), g.roomPhantasm(a)) })
	return ts
}

// talker: the human Party SPACE talks to: the Concierge stands in its Room, in the real Dimension, the whole Party
// in, nothing there SPACE does first (a stain to clean, the Room's Mirror), and some Room it fits. nil if none.
func (g *Game) talker() *Party {
	if g.ghost || g.atRoomMirror() {
		return nil
	}
	for i, r := range g.hotel.Rooms {
		if !g.feet().In(r.Shell()) {
			continue
		}
		if p := g.humanIn(i); p != nil && g.inRoom(p) && len(g.targets(p)) > 0 {
			if room, _ := g.cleanable(); room < 0 {
				return p
			}
		}
		return nil
	}
	return nil
}

// offerCost: what the offer for Room to costs, in Money: its share of a night's Rate for the Party, and in a swap
// for the Party coming the other way too.
func (g *Game) offerCost(p *Party, to, offer int) Num {
	nights := float64(g.hotel.Rooms[to].Type.Rate)
	if g.humanIn(to) != nil {
		nights += float64(g.hotel.Rooms[p.Room].Type.Rate)
	}
	return Num(math.Round(nights * offerShare[offer]))
}

// offerOdds: the chance the guest takes it: its base, and more for a scared guest and a cozy target Room.
func (g *Game) offerOdds(p *Party, to, offer int) float64 {
	up := float64(g.roomMood(to)-moodMin) / (moodMax - moodMin)
	b := offerBase[offer]
	return min(1, b+(1-b)*offerLean*(fright(p)+up)/2)
}

// sureOffer: the cheapest offer the Party is sure to take, and its Room. ok false if there is none.
func (g *Game) sureOffer(p *Party) (to, offer int, c Num, ok bool) {
	for _, r := range g.targets(p) {
		for o := range offerShare {
			if k := g.offerCost(p, r, o); g.offerOdds(p, r, o) >= 1 && (!ok || k < c) {
				to, offer, c, ok = r, o, k, true
			}
		}
	}
	return to, offer, c, ok
}

// payOffer pays an offer, counted for the sim's money check. False if the wallet can't.
func (g *Game) payOffer(c Num) bool {
	if !g.pay(Cost{Money: c}) {
		return false
	}
	g.offered += c
	return true
}

// pickLine: one of the lines, from the talk's own stream.
func (g *Game) pickLine(set []string) string { return set[g.words.IntN(len(set))] }

// startTalk stages the talk with the Party: they face each other and the guest speaks by its state.
func (g *Game) startTalk(p *Party) {
	if p == nil {
		return
	}
	set := talkCalm
	switch {
	case fright(p) >= 0.5:
		set = talkScared
	case g.pairing(p.Room) == badPair:
		set = talkUneasy
	}
	g.state, g.parley = StateTalk, &parley{p: p, targets: g.targets(p), line: g.pickLine(set), lineAt: talkIn / 2, scale: g.scale}
	g.scale *= talkSlow
	lead := p.Guests[0]
	g.player.Anim.Facing = entities.Toward(lead.X-g.player.X, lead.Y-g.player.Y)
	lead.Anim.Facing = entities.Toward(g.player.X-lead.X, g.player.Y-lead.Y)
}

// endTalk leaves the talk: back to play, the world at the pace the talk found it.
func (g *Game) endTalk() {
	if g.parley != nil {
		g.scale = g.parley.scale
	}
	g.state, g.parley = StatePlay, nil
}

// updateParley runs the world on, slowed, and reads the talk's keys. The talk ends by itself if the guest leaves
// the Room or no Room is left to offer meanwhile.
func (g *Game) updateParley() {
	t := g.parley
	t.t++
	for left := g.scale; left > 0; left-- {
		g.advance(0, 0, min(left, 1))
	}
	to := t.targets[t.to]
	if t.targets = g.targets(t.p); !g.inRoom(t.p) || len(t.targets) == 0 {
		g.endTalk()
		return
	}
	t.to = max(0, slices.Index(t.targets, to))
	key, n := g.in.JustPressed, len(t.targets)
	switch {
	case key(ebiten.KeyA):
		t.to = (t.to + n - 1) % n
	case key(ebiten.KeyD):
		t.to = (t.to + 1) % n
	case key(ebiten.KeyW):
		t.pick = (t.pick + len(offerShare) - 1) % len(offerShare)
	case key(ebiten.KeyS):
		t.pick = (t.pick + 1) % len(offerShare)
	case key(ebiten.KeySpace):
		g.makeOffer()
	}
}

// makeOffer makes the picked offer for the picked Room. One the wallet can't pay does nothing. Refused, the Room
// takes refuseMood and the talk goes on; taken, it is paid, both Parties are on their way and the talk is over.
func (g *Game) makeOffer() {
	t := g.parley
	p, to := t.p, t.targets[t.to]
	c := g.offerCost(p, to, t.pick)
	if !g.afford(Cost{Money: c}) {
		return
	}
	if g.rng.Float64() >= g.offerOdds(p, to, t.pick) {
		g.addMood(p.Room, -refuseMood)
		t.line, t.lineAt = g.pickLine(talkNo), t.t
		return
	}
	g.payOffer(c)
	g.relocate(p, to)
	p.talk = []line{{0, g.pickLine(talkYes), g.tick}}
	g.endTalk()
}

// talkZoom: the talk's camera, 1 the play camera .. talkZoomMax.
func (g *Game) talkZoom() float64 {
	if g.state != StateTalk || g.parley == nil {
		return 1
	}
	return 1 + (talkZoomMax-1)*g.talkEase()
}

// talkEase: how far the talk's camera has closed in, eased, 0..1.
func (g *Game) talkEase() float64 {
	x := min(1, float64(g.parley.t)/talkIn)
	return x * x * (3 - 2*x)
}

// TalkView is the talk as a player sees it (view.go).
type TalkView struct {
	Open    bool
	Room    int      // the Party's Room
	Targets []int    // the Rooms offered
	To      int      // the one picked, an index into Targets
	Pick    int      // the offer picked
	Prices  [][3]Num // each target's three offers' prices
}

func (g *Game) talkView() TalkView {
	t := g.parley
	if g.state != StateTalk || t == nil {
		return TalkView{}
	}
	v := TalkView{Open: true, Room: t.p.Room, Targets: t.targets, To: t.to, Pick: t.pick}
	for _, r := range t.targets {
		v.Prices = append(v.Prices, [3]Num{g.offerCost(t.p, r, plainAsk), g.offerCost(t.p, r, discount), g.offerCost(t.p, r, onTheHouse)})
	}
	return v
}

// drawParley draws the talk: the world at the play camera, closed in on the two and blurred around them by the
// focus shader, cinema bars, the guest's line over its head and the Concierge's three offers below, each with its
// price. Words stand alone, outlined; the picked offer in the Ectoplasm ink.
func (g *Game) drawParley(screen *ebiten.Image, bg color.Color) {
	if g.focus == nil {
		var err error
		if g.focus, err = ebiten.NewShader([]byte(focusSrc)); err != nil {
			panic(err)
		}
	}
	t, src := g.parley, g.fx[0]
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	mw, mh := g.mapSize()
	ox, oy := g.cam.Origin(g.player.X, g.player.Y, mw, mh)
	src.Fill(bg)
	g.drawWorld(src, ox, oy)
	lead := t.p.Guests[0]
	fx := float64(screenPx(g.player.X)+screenPx(lead.X))/2 - float64(ox)
	fy := float64(screenPx(g.player.Y)+screenPx(lead.Y))/2 - float64(oy) - charH/2*zoom
	k, e := g.talkZoom(), g.talkEase()
	sharp := (math.Hypot(g.player.X-lead.X, g.player.Y-lead.Y)/2 + charH) * zoom * k
	screen.DrawRectShader(w, h, g.focus, &ebiten.DrawRectShaderOptions{Images: [4]*ebiten.Image{src},
		Uniforms: map[string]any{"Focus": []float32{float32(fx), float32(fy)}, "K": float32(k), "T": float32(e), "Sharp": float32(sharp)}})
	bar := int(float64(h) * 0.08 * e)
	rect(screen, 0, 0, w, bar, color.Black)
	rect(screen, 0, h-bar, w, bar, color.Black)
	on := func(x, y float64) (int, int) { // world px to screen px, closed in
		return int(float64(w)/2 + (float64(screenPx(x)-ox)-fx)*k), int(float64(h)/2 + (float64(screenPx(y)-oy)-fy)*k)
	}
	const scale = 2
	if t.t >= t.lineAt {
		x, y := on(lead.X, lead.Y-charH)
		outline(screen, t.line, x-len(t.line)*glyphW*scale/2, y-glyphH*scale-8, scale, color.White, 1)
	}
	if t.t < talkIn {
		return
	}
	to := t.targets[t.to]
	scared := 0
	if fright(t.p) >= 0.5 {
		scared = 1
	}
	y0 := h - bar - 12 - len(offerShare)*(glyphH*scale+8)
	for o := range offerShare {
		s := fmt.Sprintf(offerLines[o][scared], g.hotel.Rooms[to].Number)
		c := g.offerCost(t.p, to, o)
		ink, a := color.Color(color.White), float32(0.6)
		if o == t.pick {
			ink, a = resInk(Ecto), 1
		}
		if !g.afford(Cost{Money: c}) {
			a = 0.3
		}
		price := ""
		if c > 0 {
			price = "  $" + fmtNum(c)
		}
		x, y := w/2-(len(s)+len(price))*glyphW*scale/2, y0+o*(glyphH*scale+8)
		outline(screen, s, x, y, scale, ink, a)
		outline(screen, price, x+len(s)*glyphW*scale, y, scale, resInk(Money), a)
	}
	if len(t.targets) > 1 { // more Rooms to offer: A and D
		outline(screen, "<", w/2-300, y0+glyphH*scale+8, scale, color.White, 0.8)
		outline(screen, ">", w/2+300-glyphW*scale, y0+glyphH*scale+8, scale, color.White, 0.8)
	}
}

// focusSrc: image 0 is the world at the play camera. Each screen px shows it closed in K times on Focus (image
// px), sharp within Sharp screen px of the middle, blurred and sunk into the ghost purple beyond, by T (0..1).
const focusSrc = `//kage:unit pixels
package main

var Focus vec2
var K float
var T float
var Sharp float

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	size := imageDstSize()
	at := dst.xy - imageDstOrigin()
	mid := size / 2
	o := imageSrc0Origin()
	p := Focus + (at-mid)/K + o
	b := T * smoothstep(Sharp, Sharp*2.2, distance(at, mid))
	r := 1 + 5*b
	sum := imageSrc0At(p)
	for i := 0; i < 12; i++ {
		a := float(i) * 0.5236
		sum += imageSrc0At(p + vec2(cos(a), sin(a))*r)
		sum += imageSrc0At(p + vec2(cos(a+0.26), sin(a+0.26))*r*2)
	}
	c := mix(imageSrc0At(p), sum/25, b)
	c.rgb = mix(c.rgb, c.rgb*0.45+vec3(0.12, 0.05, 0.2), b)
	return vec4(c.rgb, 1)
}
`
