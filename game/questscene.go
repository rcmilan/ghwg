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

// 08 W5-SCENE: the questline's three cutscenes, each built only from staging the game already has and speaking in
// one-liners (circle.md "Cutscenes: very few"). The world stops (as Father P's does: nothing steps, g.tick runs) and
// Update/Draw hand over to the scene while g.scene.id is set. Each plays in full the first time and SPACE skips it
// on a replay. When one ends it reports questEvent(evScene, -1, id).
//
//	O4      the card frame closes in on the spirit: its sign (o4Signs), then it folds into the floor
//	O9      the card frame closes in on Tim's chest and hands: his line, a click from below (he glances down, the
//	        frame nudges), the gem rises into his hands and glints, his second line
//	finale  everyone gathers in the hall, both Dimensions show split down the Mirror, the crate's lid lifts, stars pour
//	        up, a tall starry figure unfolds, a ring of light cracks round the Mirror, the vortex spins backwards, the
//	        flash; then the lines, the spirit's sign flips and blows away. First time ever: the summoning, the credits
//	        (credits.go), CONTINUE.
const (
	sceneIn   = 24  // frames the card frame takes to close in
	sceneOut  = 20  // and to let go
	sceneZoom = 1.8 // how close, of the play camera
	sceneBlur = 6   // the blur's reach, image px (talkBlur's range)

	o4Sign = 30  // the spirit holds its sign up...
	o4Fold = 150 // ...and folds it away
	o4Len  = 215

	o9Line1 = 30  // Tim's first line
	o9Click = 150 // a click from below the frame: he glances down, the frame nudges
	o9Gem   = 190 // the gem rises into his hands (over 50), then glints
	o9Line2 = 260
	o9Len   = 420
	o9Zoom  = 2.6 // closer than sceneZoom: chest and hands

	fGather = 240 // the walk in; whoever is not there yet is put there
	fSplit  = 240 // both Dimensions open down the Mirror (over splitFrames)
	fLid    = 300 // the crate's lid lifts; stars pour up
	fFigure = 360 // the tall figure unfolds (over 90)
	fRing   = 450 // the ring of light, the vortex backwards
	fFlash  = 570 // white over 60, and out over 60
	fLines  = 700 // the MacGuffin's line; Celestine's at fLines+150
	fFlip   = 960 // the spirit's sign flips to finaleSign...
	fBlow   = 1000
	fLine3  = 1080 // ...the MacGuffin's last line
	fLen    = 1220

	splitFrames = 40
	lineHold    = 140
)

// sceneState: the cutscene playing, if any (id "" is none).
type sceneState struct {
	id    string      // "O4", "O9", "finale", or "credits" (credits.go)
	t     int         // frames since it began
	at    image.Point // O4: where the spirit stands, world px, set by VERBS before playQuestScene("O4"); zero: on the Concierge
	from  string      // credits: the cutscene it ends ("finale"), "" when opened alone
	page  int         // credits: 0 the credits, 1 the epilogue
	cast  []sceneCast // finale: whoever gathers, the Concierge first
	pl    entities.Guest
	crate image.Point // finale: the crate's foot, world px
	buf   *ebiten.Image
	mac   macLook
	// talk: a quest TALK (verbs_card.go questSpeak): the card frame on tg, its line, the hand-over, its done line
	tp     *Party
	tg     *entities.Guest
	line   string // what tg says now
	lineAt int    // the t it began saying it
	did    bool   // the hand-over is done
	out    int    // the t the frame lets go, once did
}

// sceneCast: one who walks into the hall for the finale.
type sceneCast struct {
	g    *entities.Guest
	home image.Point
	slot image.Point
	pop  int // the frame a far one appears in the hall; 0 for one who walks in
}

func (s *sceneState) on() bool { return s.id != "" }

// playQuestScene starts cutscene id ("O4", "O9", "finale"); it reports evScene with id when it ends.
func (g *Game) playQuestScene(id string) {
	if g.scene.id == "talk" { // a talk's hand-over plays its cutscene (O9's GIVE): it takes the talk's place
		g.closeScene("talk")
	}
	if g.scene.on() {
		return
	}
	if id != "O4" && id != "O9" && id != "finale" && id != "talk" {
		g.questEvent(evScene, -1, id)
		return
	}
	if g.parley != nil { // a card or a talk it was started from: the world gets its pace back
		g.endTalk()
	}
	g.state, g.shifting, g.viewT, g.viewIn, g.hands, g.prestigeAsk = StatePlay, nil, 0, 0, "", false
	g.scene = sceneState{id: id, at: g.scene.at}
	if id == "finale" {
		g.finaleStart()
	}
}

// sceneSeen: this cutscene has played before, so SPACE may skip it.
func (g *Game) sceneSeen() bool {
	switch g.scene.id {
	case "O4":
		return g.timesDone(3) > 0
	case "O9":
		return g.timesDone(5) > 0
	case "finale":
		return g.questRec.MacGuffin
	}
	return false
}

// sceneLen: how long the cutscene is.
func (g *Game) sceneLen() int {
	return map[string]int{"O4": o4Len, "O9": o9Len, "finale": fLen}[g.scene.id]
}

// stepScene runs the cutscene a frame (Update's, in place of the world).
func (g *Game) stepScene() {
	s := &g.scene
	g.tick++
	switch s.id {
	case "credits":
		g.stepCredits()
		return
	case "talk":
		g.stepTalk()
		return
	}
	if s.t > 0 && g.sceneSeen() && g.in.JustPressed(ebiten.KeySpace) {
		g.endScene()
		return
	}
	switch s.id {
	case "O9":
		if tim := g.findGuest("tim"); tim != nil { // he looks off until the click, then down
			f := entities.FaceRight
			if s.t >= o9Click {
				f = entities.FaceDown
			}
			tim.Anim = entities.Posed(f, false, 0)
		}
	case "finale":
		g.stepFinale()
	}
	if s.t++; s.t >= g.sceneLen() {
		g.endScene()
	}
}

// endScene: the cutscene is over. The finale's first time ever goes on to the summoning and the credits.
func (g *Game) endScene() {
	id := g.scene.id
	if id == "finale" {
		g.restoreCast()
		if !g.questRec.MacGuffin {
			g.summon()
			g.scene = sceneState{id: "credits", from: id}
			return
		}
	}
	g.closeScene(id)
}

// closeScene hands the world back and reports the cutscene.
func (g *Game) closeScene(id string) {
	if g.scene.buf != nil {
		g.scene.buf.Deallocate()
	}
	if m := g.scene.mac.buf; m != nil {
		m.Deallocate()
	}
	g.scene = sceneState{}
	if id != "talk" { // nothing waits on a talk
		sceneDone(g, id)
	}
}

// sceneDone is what a cutscene's end calls; a test counts it.
var sceneDone = func(g *Game, id string) { g.questEvent(evScene, -1, id) }

// summon: the finale's first time ever: the MacGuffin is summoned, once ever, and the Run is saved with it.
func (g *Game) summon() {
	g.questRec.MacGuffin = true
	g.bonus += macguffinBonus
	g.saveDawn() // the checkpoint carries the record, and keeps the Run
}

func ease(x float64) float64 { x = min(1, max(0, x)); return x * x * (3 - 2*x) }

// ---- the card frame (O4, O9)

// drawFocused draws the world closed in k times on world px (wx,wy), the rest blurred into the ghost purple by e
// (0..1): the card's frame (talkmove.go drawParley), with the world stopped. It returns where a world px lands.
func (g *Game) drawFocused(screen *ebiten.Image, bg color.Color, wx, wy, k, e float64) func(x, y float64) (float64, float64) {
	if g.focus == nil {
		var err error
		if g.focus, err = ebiten.NewShader([]byte(focusSrc)); err != nil {
			panic(err)
		}
	}
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	ox, oy := screenPx(wx)-w/2, screenPx(wy)-h/2
	fx, fy := float64(screenPx(wx)-ox), float64(screenPx(wy)-oy)
	src := g.fx[0]
	src.Fill(bg)
	g.drawWorld(src, ox, oy)
	screen.DrawRectShader(w, h, g.focus, &ebiten.DrawRectShaderOptions{Images: [4]*ebiten.Image{src},
		Uniforms: map[string]any{"Focus": []float32{float32(fx), float32(fy)}, "K": float32(k), "T": float32(e), "Sharp": float32(1.5 * charH * zoom * k), "Blur": float32(sceneBlur)}})
	bar := int(float64(h) * 0.08 * e)
	rect(screen, 0, 0, w, bar, color.Black)
	rect(screen, 0, h-bar, w, bar, color.Black)
	return func(x, y float64) (float64, float64) {
		return float64(w)/2 + (float64(screenPx(x)-ox)-fx)*k, float64(h)/2 + (float64(screenPx(y)-oy)-fy)*k
	}
}

// sayText: the line, centred over (x,y) in outlined words, wrapped, its bottom line at y.
func sayText(s *ebiten.Image, line string, x, y float64, a float32) {
	ls := wrapAt(line, 40)
	for i, l := range ls {
		outline(s, l, int(x)-len(l)*glyphW, int(y)-(len(ls)-i)*(glyphH*2+2), 2, color.White, a)
	}
}

// drawSpirit: the spirit with its sign at (x,y), screen px, u screen px a world px. flat (0..1) folds the sign
// away, then sinks it; the sign's lines are in sign.
func (g *Game) drawSpirit(s *ebiten.Image, x, y, u float64, sign []string, flat, a float64) {
	sink := ease(flat*2 - 1)
	sy := 1 - sink
	a *= 1 - sink*0.7
	g.glowAt(s, x, y-10*u*sy, 34*u, mEdge, .5*a)
	vector.FillCircle(s, float32(x), float32(y-17*u*sy), float32(5*u), color.NRGBA{8, 6, 14, uint8(255 * a)}, true)
	var p vector.Path // the hooded body, a trapezoid that sinks into the floor
	p.MoveTo(float32(x-4*u), float32(y-14*u*sy))
	p.LineTo(float32(x+4*u), float32(y-14*u*sy))
	p.LineTo(float32(x+8*u), float32(y))
	p.LineTo(float32(x-8*u), float32(y))
	p.Close()
	dp := &vector.DrawPathOptions{AntiAlias: true}
	dp.ColorScale.ScaleWithColor(color.NRGBA{8, 6, 14, uint8(255 * a)})
	vector.FillPath(s, &p, &vector.FillOptions{}, dp)
	for _, d := range []float64{-2, 2} {
		vector.FillCircle(s, float32(x+d*u), float32(y-17*u*sy), float32(u*.8), color.NRGBA{255, 255, 255, uint8(255 * a)}, true)
	}
	if len(sign) == 0 {
		return
	}
	const sc = 2
	bw, bh := 0, len(sign)*(glyphH*sc)+10
	for _, l := range sign {
		bw = max(bw, len(l)*glyphW*sc)
	}
	bw += 16
	f := ease(flat * 2) // the board folds shut first
	bh = int(float64(bh) * (1 - f))
	top := y - 26*u*sy - float64(bh)
	if bh > 0 {
		rect(s, int(x)+int(7*u), int(top)+bh, int(u), int(26*u*sy)-int(float64(bh)), color.NRGBA{120, 90, 60, uint8(255 * a)}) // the pole
		rect(s, int(x)-bw/2-2, int(top)-2, bw+4, bh+4, color.NRGBA{20, 16, 30, uint8(255 * a)})
		rect(s, int(x)-bw/2, int(top), bw, bh, color.NRGBA{240, 236, 226, uint8(255 * a)})
		if f < .4 {
			for i, l := range sign {
				printAt(s, l, int(x)-len(l)*glyphW*sc/2, int(top)+5+i*glyphH*sc, sc, color.Black, float32(a*(1-f/.4)))
			}
		}
	}
}

func (g *Game) drawO4(screen *ebiten.Image, bg color.Color) {
	t := g.scene.t
	spot := g.scene.at
	if spot == (image.Point{}) { // none given: on the Concierge
		spot = image.Pt(int(g.player.X), int(g.player.Y))
	}
	e := ease(float64(t)/sceneIn) * (1 - ease(float64(t-o4Len+sceneOut)/sceneOut))
	k := 1 + (sceneZoom-1)*e
	on := g.drawFocused(screen, bg, float64(spot.X), float64(spot.Y)-charH/2, k, e)
	x, y := on(float64(spot.X), float64(spot.Y))
	var sign []string
	if t >= o4Sign {
		sign = wrapAt(o4Signs[0], 22)
	}
	g.drawSpirit(screen, x, y, zoom*k, sign, float64(t-o4Fold)/50, ease(float64(t)/12))
}

// ---- talk

// sayLen: frames a talk's line stays up, by its length; 0 for none.
func sayLen(line string) int {
	if line == "" {
		return 0
	}
	return max(lineHold, 4*len(line))
}

// stepTalk: the frame closes in, the line, the hand-over (questHand) and its done line, the frame lets go. SPACE,
// once closed in, cuts the line under way short.
func (g *Game) stepTalk() {
	s := &g.scene
	space := s.t >= sceneIn && g.in.JustPressed(ebiten.KeySpace)
	switch {
	case !s.did && (space || s.t >= s.lineAt+sayLen(s.line)):
		pre := s.line
		done := g.questHand(s.tp, s.tg)
		if g.scene.id != "talk" { // it played a cutscene in the talk's place
			return
		}
		if done == "" && pre == "" {
			done = ownLine(s.tp, s.tg)
		}
		s.did, s.line, s.lineAt = true, done, s.t
		s.out = s.t + sayLen(done)
	case s.did && space && s.t < s.out:
		s.out = s.t
	}
	if s.t++; s.did && s.t >= s.out+sceneOut {
		s.tp.hold = g.tick + talkHold
		g.endScene()
	}
}

func (g *Game) drawTalkScene(screen *ebiten.Image, bg color.Color) {
	s := &g.scene
	gu := s.tg
	e := ease(float64(s.t) / sceneIn)
	if s.did {
		e *= 1 - ease(float64(s.t-s.out)/sceneOut)
	}
	on := g.drawFocused(screen, bg, gu.X, gu.Y-charH/2, 1+(sceneZoom-1)*e, e)
	if s.line != "" && s.t >= s.lineAt && (!s.did || s.t < s.out) {
		x, y := on(gu.X, gu.Y-charH)
		sayText(screen, s.line, x, y-6, float32(ease(float64(s.t-s.lineAt)/8)))
	}
}

// ---- O9

// findGuest: the named guest's, nil if none is in the hotel.
func (g *Game) findGuest(key string) *entities.Guest {
	for _, gu := range g.guests() {
		if gu.Who.Key == key {
			return gu
		}
	}
	return nil
}

var gemRows = []string{
	"..KKKKKKK..",
	".KWWCCCBBK.",
	"KWCCCCCCBBK",
	"KKKKKKKKKKK",
	".KCCCCCBBK.",
	"..KCCCBBK..",
	"...KCCBK...",
	"....KCK....",
	".....K.....",
}

// gemIcon: the change Tim has carried since 1923, a small cut gem.
func (g *Game) gemIcon() *ebiten.Image {
	if g.icons == nil {
		g.icons = map[string]*ebiten.Image{}
	}
	if img, ok := g.icons["gem"]; ok {
		return img
	}
	pal := map[rune]color.RGBA{'K': {26, 20, 40, 255}, 'W': {255, 255, 255, 255}, 'C': {170, 235, 245, 255}, 'B': {90, 170, 210, 255}}
	img := ebiten.NewImage(len(gemRows[0]), len(gemRows))
	for y, row := range gemRows {
		for x, ch := range row {
			if c, ok := pal[ch]; ok {
				img.Set(x, y, c)
			}
		}
	}
	g.icons["gem"] = img
	return img
}

func (g *Game) drawO9(screen *ebiten.Image, bg color.Color) {
	t := g.scene.t
	nx, ny := g.player.X, g.player.Y
	if tim := g.findGuest("tim"); tim != nil {
		nx, ny = tim.X, tim.Y
	}
	e := ease(float64(t)/sceneIn) * (1 - ease(float64(t-o9Len+sceneOut)/sceneOut))
	k := 1 + (o9Zoom-1)*e
	wy := ny - charH*.3 // a little below his face
	if d := t - o9Click; d >= 0 && d < 8 {
		wy += .4 * float64(d%4/2*2-1) // the click: no audio, so the frame jolts
	}
	on := g.drawFocused(screen, bg, nx, wy, k, e)
	x, y := on(nx, ny)
	u := zoom * k
	if t >= o9Line1 && t < o9Click {
		sayText(screen, o9Lines[0], x, y-(charH+2)*u, 1)
	}
	if t >= o9Gem {
		img := g.gemIcon()
		b := img.Bounds()
		r := ease(float64(t-o9Gem) / 50)
		hx, hy := x, y-charH*.25*u // his hands, at the belt
		from := float64(screen.Bounds().Dy()) + 10
		cy := from + (hy-from)*r
		sc := u * .7
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(-float64(b.Dx())/2, -float64(b.Dy())/2)
		op.GeoM.Scale(sc, sc)
		op.GeoM.Translate(hx, cy)
		screen.DrawImage(img, op)
		if d := float64(t - o9Gem - 50); d > 0 && d < 40 { // the glint
			a := math.Sin(d / 40 * math.Pi)
			gx, gy, l := hx-sc*3, cy-sc*2, sc*4*a
			var p vector.Path
			p.MoveTo(float32(gx-l), float32(gy))
			p.LineTo(float32(gx+l), float32(gy))
			p.MoveTo(float32(gx), float32(gy-l))
			p.LineTo(float32(gx), float32(gy+l))
			strokeP(screen, &p, 3, [3]float32{1, 1, .9}, a)
		}
	}
	if t >= o9Line2 && t < o9Len-sceneOut {
		sayText(screen, o9Lines[1], x, y-(charH+2)*u, 1)
	}
}

// ---- the finale

// finaleStart: the Concierge and everyone walk into the hall, the living on the Mirror's left, the dead on its right.
func (g *Game) finaleStart() {
	s := &g.scene
	hall, sp := g.hotel.Lobby, g.hotel.MirrorSpot
	mx := sp.Min.X + sp.Dx()/2
	s.crate = image.Pt(mx, sp.Max.Y-4)
	front := image.Pt(mx, sp.Max.Y+20)
	dist := func(a, b image.Point) float64 { return math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y)) }
	var slots []image.Point // walkable spots of the hall, off the kitchen, none behind the Mirror or on the crate or the spirit
	for y := hall.Min.Y + 8; y < hall.Max.Y-4; y += 13 {
		for x := hall.Min.X + 8; x < hall.Max.X-8; x += 13 {
			p := image.Pt(x, y)
			if !p.In(g.hotel.Kitchen) && !g.blocked(entities.FootBox(float64(x), float64(y))) && p.Y > s.crate.Y+4 && dist(p, s.crate) > 14 && dist(p, spiritAt(s.crate)) > 14 {
				slots = append(slots, p)
			}
		}
	}
	if len(slots) == 0 {
		slots = []image.Point{front}
	}
	slices.SortFunc(slots, func(a, b image.Point) int { return int(dist(a, front) - dist(b, front)) })
	side := func(left bool) (out []image.Point) {
		for _, p := range slots {
			if left == (p.X < mx) {
				out = append(out, p)
			}
		}
		return orAll(out, slots)
	}
	left, right := side(true), side(false)
	mw, mh := g.mapSize()
	ox, oy := g.cam.Origin(g.player.X, g.player.Y, mw, mh)
	seen := image.Rect(ox/zoom, oy/zoom, ox/zoom+viewW, oy/zoom+viewH)
	s.pl = entities.NewGuest(image.Pt(int(g.player.X), int(g.player.Y)), "Bob")
	s.pl.X, s.pl.Y = g.player.X, g.player.Y
	nl, nr := 0, 0
	for i, gu := range append([]*entities.Guest{&s.pl}, g.actors()...) {
		c := sceneCast{g: gu, home: image.Pt(int(gu.X), int(gu.Y))}
		if i == 0 || g.realmOf(gu) == Human {
			c.slot, nl = left[nl%len(left)], nl+1
		} else {
			c.slot, nr = right[nr%len(right)], nr+1
		}
		if !c.home.In(seen) {
			c.pop = 20 + i*37%180 // not on screen: appears in the hall
		} else {
			g.reelRoute(gu, c.slot)
		}
		s.cast = append(s.cast, c)
	}
	g.cam.Snap(float64(mx), float64(front.Y), mw, mh)
}

// orAll: out, or all if out is empty.
func orAll(out, all []image.Point) []image.Point {
	if len(out) == 0 {
		return all
	}
	return out
}

// spiritAt: where the spirit stands for the finale, beside the crate on the dead's side.
func spiritAt(crate image.Point) image.Point { return crate.Add(image.Pt(70, 24)) }

// restoreCast puts everyone back where the cutscene found them.
func (g *Game) restoreCast() {
	for _, c := range g.scene.cast {
		c.g.X, c.g.Y, c.g.Path = float64(c.home.X), float64(c.home.Y), nil
	}
	if len(g.scene.cast) > 0 {
		g.player.X, g.player.Y = float64(g.scene.cast[0].home.X), float64(g.scene.cast[0].home.Y)
	}
}

func (g *Game) stepFinale() {
	s := &g.scene
	t := s.t
	for i := range s.cast {
		c := &s.cast[i]
		switch {
		case t == fGather:
			c.g.X, c.g.Y, c.g.Path = float64(c.slot.X), float64(c.slot.Y), nil
		case t < fGather && t >= c.pop:
			if c.pop > 0 && t == c.pop {
				c.g.X, c.g.Y = float64(c.slot.X), float64(c.slot.Y)
			}
			if c.pop == 0 {
				c.g.FollowPath(reelWalk)
			}
		}
	}
	g.player.X, g.player.Y = s.pl.X, s.pl.Y
	g.player.Anim.Update(g.player.X, g.player.Y, 1)
	for _, c := range s.cast[1:] {
		c.g.Anim.Update(c.g.X, c.g.Y, 1)
	}
	mw, mh := g.mapSize()
	g.cam.Follow(float64(s.crate.X), float64(s.crate.Y+20), mw, mh)
}

// finaleLine: which line of finaleLines is up at frame t, and how far into it (frames); -1 for none.
func finaleLine(t int) (i, age int) {
	for j, at := range [...]int{fLines, fLines + 150, fLine3} {
		if t >= at && t < at+lineHold {
			return j, t - at
		}
	}
	return -1, 0
}

func (g *Game) drawFinale(screen *ebiten.Image, bg color.Color) {
	s := &g.scene
	t := s.t
	b := screen.Bounds()
	if s.buf == nil {
		s.buf = ebiten.NewImage(b.Dx(), b.Dy())
	}
	mw, mh := g.mapSize()
	ox, oy := g.cam.Origin(g.player.X, g.player.Y, mw, mh)
	at := func(x, y float64) (float64, float64) { return x*zoom - float64(ox), y*zoom - float64(oy) }
	mx, my := at(float64(s.crate.X), float64(s.crate.Y)-16) // the Mirror's middle
	was := g.ghost
	if f := float64(t-fSplit) / splitFrames; t >= fSplit && t < fFlash+60 { // both Dimensions, split down the Mirror
		g.ghost = true
		screen.Fill(color.Black)
		g.drawWorld(screen, ox, oy)
		g.ghost = false
		s.buf.Fill(assets.Night)
		g.drawWorld(s.buf, ox, oy)
		g.ghost = was
		half := int(mx * min(1, f)) // the real half opens from the seam outward
		x0 := int(mx) - half
		screen.DrawImage(s.buf.SubImage(image.Rect(x0, 0, int(mx), b.Dy())).(*ebiten.Image), &ebiten.DrawImageOptions{GeoM: translate(x0, 0)})
		rect(screen, int(mx)-1, 0, 3, b.Dy(), color.NRGBA{255, 255, 255, uint8(200 * min(1, f))})
		g.glowAt(screen, mx, float64(b.Dy())/2, float64(b.Dy())*1.2, moonLight, .25*min(1, f))
	} else {
		screen.Fill(bg)
		g.drawWorld(screen, ox, oy)
	}
	cx, cy := at(float64(s.crate.X), float64(s.crate.Y))
	flashed := t >= fFlash+60
	if !flashed { // the crate
		lift := ease(float64(t-fLid) / 60)
		rect(screen, int(cx-8*zoom), int(cy-10*zoom), 16*zoom, 10*zoom, color.NRGBA{96, 62, 34, 255})
		for _, d := range []int{-4, 0, 4} {
			rect(screen, int(cx)+d*zoom, int(cy-10*zoom), 1, 10*zoom, color.NRGBA{58, 36, 20, 255})
		}
		var p vector.Path // the lid, lifting and tilting off
		ang, rise := lift*.7, lift*12*zoom
		for i, c := range [...][2]float64{{-9, -10}, {9, -10}, {9, -13}, {-9, -13}} {
			px := cx + (c[0]*math.Cos(ang)-(c[1]+10)*math.Sin(ang))*zoom
			py := cy - 10*zoom + ((c[0])*math.Sin(ang)+(c[1]+10)*math.Cos(ang))*zoom - rise
			if i == 0 {
				p.MoveTo(float32(px), float32(py))
			} else {
				p.LineTo(float32(px), float32(py))
			}
		}
		p.Close()
		dp := &vector.DrawPathOptions{AntiAlias: true}
		dp.ColorScale.ScaleWithColor(color.NRGBA{128, 84, 46, 255})
		vector.FillPath(screen, &p, &vector.FillOptions{}, dp)
		if lift > 0 {
			g.glowAt(screen, cx, cy-10*zoom, 40*zoom*lift, moonLight, .7)
		}
	}
	for i := range 70 { // stars pour up out of it
		j := jitter(i)
		age := float64(t - fLid - 10 - i*3)
		if age < 0 || age > 110 || flashed {
			continue
		}
		x := cx + (j.ph-math.Pi)*4*zoom + math.Sin(age*.08+j.ph)*8*zoom*(.4+j.wob)
		y := cy - 12*zoom - age*(.9+j.speed*.5)*zoom
		a := min(1, age/10) * (1 - age/110)
		rect(screen, int(x), int(y), 2*zoom/2+1, 2*zoom/2+1, color.NRGBA{235, 230, 255, uint8(255 * a)})
		g.glowAt(screen, x, y, 9*zoom, moonLight, .5*a)
	}
	if u := ease(float64(t-fFigure) / 90); u > 0 { // a tall figure unfolds: the Concierge's shape, stretched
		g.drawMacguffin(screen, int(cx), int(cy), 1+.7*(1-u), math.Max(.05, macguffinTall*u), entities.NewAnim(), 1)
	}
	if t >= fRing && !flashed { // a ring of light cracks round the Mirror; the vortex spins backwards
		ph := float64(t - fRing)
		ramp := ease(ph / 70)
		r := 2.4 * 16 * zoom
		g.glowAt(screen, mx, my, r*2.6, moonLight, .3*ramp)
		drawRing(screen, mx, my, r, ramp, 3, kTint, .95)
		drawRing(screen, mx, my, r*1.12, ramp, 1.5, mEdge, .7)
		for i := range 10 {
			if (t/3+i)%5 < 3 {
				a := float64(i)*.628 + .3
				var p vector.Path
				p.MoveTo(float32(mx+math.Cos(a)*r*.94), float32(my+math.Sin(a)*r*.94))
				p.LineTo(float32(mx+math.Cos(a+.05)*r*1.22), float32(my+math.Sin(a+.05)*r*1.22))
				strokeP(screen, &p, 2, [3]float32{1, 1, 1}, ramp)
			}
		}
		spin := -(ph*.02 + ph*ph*.0004)
		for i := range 90 {
			q := float64(i) / 90
			a := spin*(1.6-q) + float64(i%3)*2*math.Pi/3 + q*6
			rr := r*.15 + q*r*2.2
			x, y := mx+rr*math.Cos(a), my+rr*math.Sin(a)*.8
			al := ramp * (1 - q)
			rect(screen, int(x), int(y), 3, 3, color.NRGBA{235, 230, 255, uint8(255 * al)})
			g.glowAt(screen, x, y, 12*zoom, mEdge, .4*al)
		}
	}
	if t >= fFlash+60 { // it stands where the crate was
		g.drawMacguffin(screen, int(cx), int(cy), 1, macguffinTall, entities.NewAnim(), 1)
	}
	sx, sy := at(float64(spiritAt(s.crate).X), float64(spiritAt(s.crate).Y))
	if t >= fSplit-60 && t < fBlow+60 {
		sign, a := wrapAt(o4Signs[0], 18), 1.0
		if t >= fFlip {
			sign = wrapAt(finaleSign, 18)
		}
		if t >= fBlow { // it blows away
			f := float64(t-fBlow) / 60
			sx, sy, a = sx+f*f*500, sy-f*60, 1-f
		}
		g.drawSpirit(screen, sx, sy, zoom, sign, 0, a*ease(float64(t-fSplit+60)/20))
	}
	if i, age := finaleLine(t); i >= 0 { // the lines, as subtitles: the MacGuffin's, then Celestine's
		sayText(screen, finaleLines[i], float64(b.Dx())/2, float64(b.Dy())-36, float32(ease(float64(age)/10)*(1-ease(float64(age-lineHold+15)/15))))
	}
	if f := float64(t-fFlash) / 60; f > 0 { // the flash: white, then out
		a := min(1, f)
		if t >= fFlash+60 {
			a = 1 - float64(t-fFlash-60)/60
		}
		rect(screen, 0, 0, b.Dx(), b.Dy(), color.NRGBA{255, 255, 255, uint8(255 * max(0, a))})
	}
}

func translate(x, y int) (g ebiten.GeoM) {
	g.Translate(float64(x), float64(y))
	return
}

// drawScene draws the cutscene playing, over everything else.
func (g *Game) drawScene(screen *ebiten.Image) {
	g.bakes = 0
	var bg color.Color = assets.Night
	if g.ghost {
		bg = color.Black
	}
	switch g.scene.id {
	case "O4":
		g.drawO4(screen, bg)
	case "O9":
		g.drawO9(screen, bg)
	case "talk":
		g.drawTalkScene(screen, bg)
	case "finale":
		g.drawFinale(screen, bg)
	case "credits":
		g.drawCredits(screen)
	}
}

// DevScene is main's -scene: a busy hotel a few hours in, with Tim and Celestine in it, the cutscene playing.
func (g *Game) DevScene(id string) {
	for range 3 * hourFrames {
		g.step(0, 0)
	}
	for i, key := range []string{"tim", "celestine"} {
		if p := g.arriveGhost(i); p != nil {
			g.nameGhost(p.Guests[0], namedGhosts[slices.IndexFunc(namedGhosts, func(n namedGhost) bool { return n.Key == key })])
			g.settleIn(p)
		}
	}
	g.scene.at = image.Pt(int(g.player.X)+28, int(g.player.Y)-4)
	g.playQuestScene(id)
}
