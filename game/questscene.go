package game

import (
	"image"
	"image/color"
	"math"
	"slices"

	"ghosthotel/assets"
	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// 08 W5-SCENE: the questline's three cutscenes, each built only from staging the game already has and speaking in
// one-liners (circle.md "Cutscenes: very few"). The world stops (as Father P's does: nothing steps, g.tick runs) and
// Update/Draw hand over to the scene while g.scene.id is set. Each plays in full the first time and SPACE skips it
// on a replay. When one ends it reports questEvent(evScene, -1, id).
//
//	O4      the card frame closes in on the spirit: its sign (o4Signs), then it folds into the floor
//	O9      the card frame on Tim: the suitcase opens (a case, a clipboard), then the hotel view pulls back and the
//	        clipboard's 5x5 hotel with a circle round it is drawn over it
//	finale  in Madame's Room, the ghosts faint over the real Room: everyone walks in, Magenta traces a circle on the
//	        floor, five take a point on it, the Courier sets the crate in the middle, the circle lights, the lid
//	        lifts, a tall starry figure unfolds and speaks, Tim takes off his cap and fades, the dark: the spirit's
//	        eyes open and close, Celestine's line. First time ever: the summoning, the credits (credits.go), CONTINUE.
const (
	sceneIn   = 24  // frames the card frame takes to close in
	sceneOut  = 20  // and to let go
	sceneZoom = 1.8 // how close, of the play camera
	sceneBlur = 6   // the blur's reach, image px (talkBlur's range)

	o4Sign = 30  // the spirit holds its sign up...
	o4Fold = 150 // ...and folds it away
	o4Len  = 215

	o9Line1 = 30 // Tim's first line
	o9Case  = 70 // the suitcase opens (30 frames), the small one rises at o9Small, the clipboard at o9Board
	o9Small = 130
	o9Board = 170
	o9Hotel = 230 // the dissolve into the pulled-back hotel
	o9Grid  = 290 // the clipboard's grid is drawn over 70 frames, the circle after it over 60
	o9Ring  = 360
	o9Line2 = 400
	o9Len   = 540

	finaleFaint = .45 // the cast's ghosts' opacity
	fCircleDy   = 8   // the circle's centre, below the Room's centre (the back wall is thick)
	fRadius     = 30  // the circle's, world px
	fWalk       = 10  // everyone walks in by the door
	fRingAt     = 90  // Magenta traces the circle
	fPoints     = 240 // the five take their points (over 50)
	fCourier    = 320 // the Courier walks in with the crate...
	fSet        = 380 // ...sets it down in the middle, and walks out
	fLight      = 450 // the circle lights up
	fLid        = 480 // the lid lifts; stars pour up
	fFigure     = 540 // the starry figure unfolds (over 90)
	fLines      = 680 // the MacGuffin's line
	fCap        = 840 // Tim takes his cap off (over fCapLift), then fades (over fFade)
	fCapLift    = 40
	fFade       = 50
	fGone       = fCap + fCapLift + fFade
	fDark       = 930  // the screen goes dark (over 40)
	fEyes       = 1000 // the spirit's eyes open (30), stay (fEyesHold), close for good (30)
	fEyesHold   = 50
	fLine2      = 1140 // Celestine's line, over the dark
	fLen        = 1300

	lineHold = 140
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
	crate image.Point // finale: the circle's centre and the crate's foot, world px
	door  image.Point // finale: Madame's door, where they walk in
	cour  entities.Guest
	timAt image.Point
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
		g.viewT = min(1, max(0, float64(s.t-o9Hotel)/(viewFrames*3)))
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

func (g *Game) drawO9(screen *ebiten.Image, bg color.Color) {
	s := &g.scene
	t := s.t
	nx, ny := g.player.X, g.player.Y
	if tim := g.findGuest("tim"); tim != nil {
		nx, ny = tim.X, tim.Y
	}
	e := ease(float64(t)/sceneIn) * (1 - ease(float64(t-o9Hotel-30)/30))
	k := 1 + (sceneZoom-1)*e
	on := g.drawFocused(screen, bg, nx, ny-charH/2, k, e)
	x, y := on(nx, ny)
	u := zoom * k
	if t >= o9Line1 && t < o9Case+70 {
		sayText(screen, o9Lines[0], x, y-(charH+6)*k*zoom, 1)
	}
	if t >= o9Case && t < o9Hotel { // the suitcase at his feet: lid up, a smaller one rises out of it, then a clipboard
		cx, cy := x+14*u, y+2*u
		open := ease(float64(t-o9Case) / 30)
		rect(screen, int(cx-9*u), int(cy-8*u), int(18*u), int(8*u), color.NRGBA{110, 70, 40, 255})
		rect(screen, int(cx-9*u), int(cy-8*u), int(18*u), int(u), color.NRGBA{60, 36, 20, 255})
		lid := int(4 * u * (1 - open))
		rect(screen, int(cx-9*u), int(cy-8*u)-lid-int(2*u*open), int(18*u), lid+int(u), color.NRGBA{140, 90, 50, 255})
		if t >= o9Small {
			r := ease(float64(t-o9Small) / 30)
			rect(screen, int(cx-5*u), int(cy-8*u-9*u*r), int(10*u), int(6*u), color.NRGBA{150, 100, 60, 255})
		}
		if t >= o9Board {
			r := ease(float64(t-o9Board) / 30)
			rect(screen, int(cx-3*u), int(cy-8*u-20*u*r), int(7*u), int(10*u), color.NRGBA{240, 236, 226, 255})
			rect(screen, int(cx-1*u), int(cy-8*u-18*u*r), int(3*u), int(u*.6), color.NRGBA{60, 60, 80, 255})
		}
	}
	if t < o9Hotel {
		return
	}
	// the hotel view pulled back, dissolving in over the card frame
	b := screen.Bounds()
	if s.buf == nil {
		s.buf = ebiten.NewImage(b.Dx(), b.Dy())
	}
	s.buf.Fill(bg)
	g.drawView(s.buf, bg)
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(float32(ease(float64(t-o9Hotel) / 40)))
	screen.DrawImage(s.buf, op)
	if t < o9Grid-30 {
		return
	}
	// the clipboard's drawing over it: a 5x5 hotel with a circle round it
	vk, tx, ty := g.viewAt()
	mw, mh := g.mapSize()
	cx, cy := float64(mw)/2*vk+tx, float64(mh)/2*vk+ty
	rect(screen, 0, 0, b.Dx(), b.Dy(), color.NRGBA{0, 0, 0, uint8(150 * ease(float64(t-o9Grid+30)/30))})
	side := min(math.Max(float64(mw), float64(mh))*vk, 340)
	ink := [3]float32{.95, .93, .85}
	lines := int(11 * min(1, float64(t-o9Grid)/70)) // the 6 + 6 lines, one after another
	for i := 0; i < min(lines, 11)+1 && t >= o9Grid; i++ {
		var p vector.Path
		f := float64(i/2) / 5
		if i%2 == 0 {
			p.MoveTo(float32(cx-side/2+f*side), float32(cy-side/2))
			p.LineTo(float32(cx-side/2+f*side), float32(cy+side/2))
		} else {
			p.MoveTo(float32(cx-side/2), float32(cy-side/2+f*side))
			p.LineTo(float32(cx+side/2), float32(cy-side/2+f*side))
		}
		strokeP(screen, &p, 2, ink, .9)
	}
	drawRing(screen, cx, cy, side/2*math.Sqrt2+8, ease(float64(t-o9Ring)/60), 3, ink, .95)
	if t >= o9Line2 && t < o9Line2+lineHold+60 {
		sayText(screen, o9Lines[1], float64(b.Dx())/2, 70, 1)
	}
}

// ---- the finale

// finaleKeys: who takes a point on the circle, in order; the Concierge stands outside it.
var finaleKeys = [...]string{"magenta", "celestine", "tim", "magda", "osvaldo"}

// finaleRoom: Madame's Room (the first Room if she is not in the hotel).
func (g *Game) finaleRoom() world.Room {
	i := g.roomOfKey("celestine")
	if c := g.findGuest("celestine"); i < 0 && c != nil {
		i = g.roomAt(c.X, c.Y)
	}
	return g.hotel.Rooms[max(0, i)]
}

// finaleStart: the cast (the Concierge first, then whoever of finaleKeys is in the hotel) is set to walk into
// Madame's Room by its door, the circle's centre on the Room's floor.
func (g *Game) finaleStart() {
	s := &g.scene
	r := g.finaleRoom()
	sh := r.Shell()
	s.crate = image.Pt((sh.Min.X+sh.Max.X)/2, (sh.Min.Y+sh.Max.Y)/2+fCircleDy)
	s.door = image.Pt(r.Door.X*gts+gts/2, r.Door.Y*gts+gts/2)
	if i := slices.IndexFunc(g.hotel.Rooms, func(x world.Room) bool { return x.Origin == r.Origin }); i < len(g.fog) {
		g.fog[i] = 0 // the Room is clear for the staging
	}
	for _, c := range g.couriers {
		c.out = false // his own walk is shown by the scene
	}
	s.cour = entities.NewGuest(s.door, courierBox)
	home := image.Pt(int(g.player.X), int(g.player.Y))
	s.pl = entities.NewGuest(home, "Bob")
	s.pl.X, s.pl.Y = g.player.X, g.player.Y
	s.cast = []sceneCast{{g: &s.pl, home: home, slot: s.crate.Add(image.Pt(0, fRadius+22))}}
	for _, k := range finaleKeys {
		if gu := g.findGuest(k); gu != nil {
			s.cast = append(s.cast, sceneCast{g: gu, home: image.Pt(int(gu.X), int(gu.Y))})
		}
	}
	n := len(s.cast) - 1
	for i := 1; i <= n; i++ { // the points of the circle, from the top, round
		a := 2*math.Pi*float64(i-1)/float64(n) - math.Pi/2
		s.cast[i].slot = s.crate.Add(image.Pt(int(fRadius*math.Cos(a)), int(fRadius*math.Sin(a))))
	}
	mw, mh := g.mapSize()
	g.ghost = false
	g.cam.Snap(float64(s.crate.X), float64(s.crate.Y), mw, mh)
}

// restoreCast puts everyone back where the cutscene found them.
func (g *Game) restoreCast() {
	for _, c := range g.scene.cast {
		c.g.X, c.g.Y, c.g.Path = float64(c.home.X), float64(c.home.Y), nil
	}
	if len(g.scene.cast) > 0 {
		g.player.X, g.player.Y = float64(g.scene.cast[0].home.X), float64(g.scene.cast[0].home.Y)
	}
}

// lerpPt: a to b, f (0..1) of the way.
func lerpPt(a, b image.Point, f float64) (float64, float64) {
	f = min(1, max(0, f))
	return float64(a.X) + float64(b.X-a.X)*f, float64(a.Y) + float64(b.Y-a.Y)*f
}

func (g *Game) stepFinale() {
	s := &g.scene
	t := s.t
	shell := g.finaleRoom().Shell()
	for i := range s.cast {
		c := &s.cast[i]
		from := s.door // in through the door, unless already in the Room
		if i > 0 && c.home.In(shell) {
			from = c.home
		}
		wait := c.slot.Add(image.Pt(0, fRadius/2+8)) // below its point, off the circle, till they take their points
		if i == 0 {
			wait = c.slot
		}
		if t < fPoints {
			c.g.X, c.g.Y = lerpPt(from, wait, float64(t-fWalk-i*20)/70)
		} else {
			c.g.X, c.g.Y = lerpPt(wait, c.slot, float64(t-fPoints)/50)
		}
		if t >= fWalk+i*20 {
			c.g.Anim.Update(c.g.X, c.g.Y, 1)
		}
	}
	g.player.X, g.player.Y = s.pl.X, s.pl.Y
	if tim := g.findGuest("tim"); tim != nil && t >= fCap { // the scene draws him from here
		if t == fCap {
			s.timAt = image.Pt(int(tim.X), int(tim.Y))
		}
		tim.X, tim.Y = -1000, -1000
	}
	if t >= fSet { // the Courier: in with the crate, sets it down, out
		s.cour.X, s.cour.Y = lerpPt(s.crate, s.door, float64(t-fSet)/60)
	} else {
		s.cour.X, s.cour.Y = lerpPt(s.door, s.crate.Add(image.Pt(0, 14)), float64(t-fCourier)/60)
	}
	if t >= fCourier {
		s.cour.Anim.Update(s.cour.X, s.cour.Y, 1)
	}
	mw, mh := g.mapSize()
	g.cam.Follow(float64(s.crate.X), float64(s.crate.Y), mw, mh)
}

// finaleLine: which line of finaleLines is up at frame t, and how far into it (frames); -1 for none.
func finaleLine(t int) (i, age int) {
	for j, at := range [...]int{fLines, fLine2} {
		if t >= at && t < at+lineHold {
			return j, t - at
		}
	}
	return -1, 0
}

// drawEyes: the spirit's two eyes at (x,y), u screen px a world px; open 0..1.
func drawEyes(s *ebiten.Image, x, y, u, open float64) {
	if open <= 0 {
		return
	}
	for _, d := range []float64{-7, 7} {
		var p vector.Path
		for i := 0; i < 24; i++ {
			a := 2 * math.Pi * float64(i) / 24
			px, py := float32(x+(d+4.4*math.Cos(a))*u), float32(y+3*open*math.Sin(a)*u)
			if i == 0 {
				p.MoveTo(px, py)
			} else {
				p.LineTo(px, py)
			}
		}
		p.Close()
		dp := &vector.DrawPathOptions{AntiAlias: true}
		dp.ColorScale.ScaleWithColor(color.White)
		vector.FillPath(s, &p, &vector.FillOptions{}, dp)
	}
}

// drawCrate: the crate at foot (cx,cy), screen px, its lid lifted by lift (0..1).
func (g *Game) drawCrate(screen *ebiten.Image, cx, cy, lift float64) {
	rect(screen, int(cx)-8*zoom, int(cy)-10*zoom, 16*zoom, 10*zoom, color.NRGBA{96, 62, 34, 255})
	for _, d := range []int{-4, 0, 4} {
		rect(screen, int(cx)+d*zoom, int(cy)-10*zoom, 1, 10*zoom, color.NRGBA{58, 36, 20, 255})
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

func (g *Game) drawFinale(screen *ebiten.Image, bg color.Color) {
	s := &g.scene
	t := s.t
	b := screen.Bounds()
	mw, mh := g.mapSize()
	ox, oy := g.cam.Origin(g.player.X, g.player.Y, mw, mh)
	at := func(x, y float64) (float64, float64) { return x*zoom - float64(ox), y*zoom - float64(oy) }
	screen.Fill(bg)
	g.drawWorld(screen, ox, oy) // the real Room; the cast's ghosts show faint over it
	cx, cy := at(float64(s.crate.X), float64(s.crate.Y))
	lit := ease(float64(t-fLight) / 40)
	grow := ease(float64(t-fRingAt) / (fPoints - fRingAt - 20))
	drawRing(screen, cx, cy, fRadius*zoom, grow, 3, [3]float32{.95, .93, .85}, .8-.3*lit)
	if grow > 0 && grow < 1 { // Magenta's staff at the circle's growing end
		a := 2*math.Pi*grow - math.Pi/2
		g.glowAt(screen, cx+fRadius*zoom*math.Cos(a), cy+fRadius*zoom*math.Sin(a), 14*zoom, mEdge, .8)
	}
	if lit > 0 {
		drawRing(screen, cx, cy, fRadius*zoom, 1, 4, kTint, lit)
		g.glowAt(screen, cx, cy, fRadius*zoom*2.6, moonLight, .35*lit)
	}
	if t >= fCourier && t < fSet+60 { // the Courier, the crate in his arms till he sets it down
		x, y := at(s.cour.X, s.cour.Y)
		name := courierBox
		if t >= fSet {
			name = courierName
		}
		g.drawCharacter(screen, int(x), int(y), name, s.cour.Anim, 0)
	}
	if t >= fSet && t < fDark {
		fall := 8 * (1 - ease(float64(t-fSet)/10)) * zoom
		g.drawCrate(screen, cx, cy-fall, ease(float64(t-fLid)/60))
	}
	for i := range 70 { // stars pour up out of it
		j := jitter(i)
		age := float64(t - fLid - 10 - i*3)
		if age < 0 || age > 110 || t >= fDark {
			continue
		}
		x := cx + (j.ph-math.Pi)*4*zoom + math.Sin(age*.08+j.ph)*8*zoom*(.4+j.wob)
		y := cy - 12*zoom - age*(.9+j.speed*.5)*zoom
		a := min(1, age/10) * (1 - age/110)
		rect(screen, int(x), int(y), 2*zoom/2+1, 2*zoom/2+1, color.NRGBA{235, 230, 255, uint8(255 * a)})
		g.glowAt(screen, x, y, 9*zoom, moonLight, .5*a)
	}
	if u := ease(float64(t-fFigure) / 90); u > 0 && t < fDark { // a tall figure unfolds: the Concierge's shape, stretched
		g.drawMacguffin(screen, int(cx), int(cy), 1+.7*(1-u), math.Max(.05, macguffinTall*u), entities.NewAnim(), 1)
	}
	for _, c := range s.cast[1:] { // the ghosts of the cast, faint over the Room (Tim fades out after his cap)
		if g.realmOf(c.g) != Ghost {
			continue
		}
		x, y, a := c.g.X, c.g.Y, float32(finaleFaint)
		cap := c.g.Who.Key == "tim" && t >= fCap
		if cap {
			x, y, a = float64(s.timAt.X), float64(s.timAt.Y), a*float32(1-ease(float64(t-fCap-fCapLift)/fFade))
		}
		if cap && t >= fGone {
			continue
		}
		tint, _, _ := g.ghostLook(c.g)
		sx, sy := at(x, y)
		g.drawCharacterAs(screen, int(sx), int(sy), c.g.Character, c.g.Anim, 0, tint, a, modeFlat)
		if cap { // his cap, lifted off
			w, up := 8*zoom, int(ease(float64(t-fCap)/fCapLift)*14*zoom)+zoom
			top := int(sy) + (entities.FootH/2-charH+10)*zoom - up
			rect(screen, int(sx)-w/2, top, w, 3*zoom, color.NRGBA{110, 70, 170, uint8(255 * a)})
			rect(screen, int(sx)-w/2, top+3*zoom, w, zoom, color.NRGBA{60, 36, 100, uint8(255 * a)})
		}
	}
	if d := ease(float64(t-fDark) / 40); d > 0 { // the dark, and the spirit's eyes in it
		rect(screen, 0, 0, b.Dx(), b.Dy(), color.NRGBA{0, 0, 0, uint8(255 * d)})
		open := ease(float64(t-fEyes)/30) * (1 - ease(float64(t-fEyes-fEyesHold)/30))
		drawEyes(screen, float64(b.Dx())/2, float64(b.Dy())/2, zoom, open)
	}
	if i, age := finaleLine(t); i >= 0 { // the lines, as subtitles: the MacGuffin's, then Celestine's
		sayText(screen, finaleLines[i], float64(b.Dx())/2, float64(b.Dy())-36, float32(ease(float64(age)/10)*(1-ease(float64(age-lineHold+15)/15))))
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

// DevScene is main's -scene: a busy hotel a few hours in, with the finale's cast in it, the cutscene playing.
func (g *Game) DevScene(id string) {
	for range 3 * hourFrames {
		g.step(0, 0)
	}
	g.roomOwned = nil // every Room open, as the other dev starts
	room := 0         // each into the next Room that takes them
	for _, key := range []string{"celestine", "tim", "osvaldo"} {
		for ; room < len(g.hotel.Rooms); room++ {
			if p := g.arriveGhost(room); p != nil {
				g.nameGhost(p.Guests[0], namedGhosts[slices.IndexFunc(namedGhosts, func(n namedGhost) bool { return n.Key == key })])
				g.settleIn(p)
				room++
				break
			}
		}
	}
	for i, key := range []string{"magenta", "magda"} {
		if p := g.arrive(room + i); p != nil {
			g.nameHuman(p, namedHumans[slices.IndexFunc(namedHumans, func(h namedHuman) bool { return h.Key == key })])
			g.settleIn(p)
		}
	}
	g.scene.at = image.Pt(int(g.player.X)+28, int(g.player.Y)-4)
	g.playQuestScene(id)
}
