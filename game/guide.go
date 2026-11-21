package game

import (
	"image"
	"image/color"
	"math"
	"slices"
	"strings"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 W5 R1 GUIDE: holding the player's hand without coercion. The checklist reads each open objective's next step
// (circle.go objSteps, stepIdx here), and the step's target wears an outline, only in the step's
// Dimension; in the other one the nearest Mirror does (SHIFT here). Nothing here draws from an rng or changes the
// sim: it reads the quest and verb state, and draws.

// saw notes the Concierge saw something this beat (a guest's Card, the ghost's ask...), for the next step.
func (g *Game) saw(k string) {
	if k == "" {
		return
	}
	g.verbSync()
	if g.verbs.seen == nil {
		g.verbs.seen = map[string]bool{}
	}
	g.verbs.seen[k] = true
}

func (g *Game) sawn(k string) bool { return g.verbs.beat == g.quest.Beat && g.verbs.seen[k] }

// stepIdx: the index into objSteps[id] of open objective id's next step.
func (g *Game) stepIdx(id string) int {
	g.verbSync()
	held, v := g.verbs.held, g.verbs
	switch id {
	case "O1":
		switch {
		case held != "O1" && g.rollGet("O1.long") == 1:
			return 3
		case held != "O1":
			return 0
		case g.verbWait("O1"):
			return 2
		}
		return 1
	case "O2":
		switch {
		case held == "O2":
			return 2
		case g.level >= 2 && g.rollGet("O2.read") != 0:
			return 1
		}
	case "O3", "O10":
		c, who := g.roomOfKey("celestine"), "magda"
		if id == "O10" {
			who = "magenta"
		}
		switch {
		case g.together(who, "celestine"):
			return 2 + b2i(id == "O3")
		case !g.sawn("celestine"):
			return 0
		case id == "O3" && g.level >= 2 && v.stranger != nil && c >= 0 && v.stranger.Room == c && g.roomOfKey("magda") != c:
			return 1
		}
		return 1 + b2i(id == "O3")
	case "O4":
		switch {
		case g.rollGet("O4.lit") > 0:
			return 3
		case !g.nightNow() && !v.spirit.on:
			return 0
		}
		return 1 + b2i(g.level >= 2)
	case "O5":
		p := g.questParty("tim")
		switch {
		case p == nil || !p.asking():
			return 0
		case g.hands == p.want:
			return 2
		case g.sawn("O5.ask"):
			return 1
		}
	case "O6":
		room := g.porterRoom()
		if room < 0 || !g.sawn("teo") {
			return 0
		}
		if g.priestOn(room) {
			return 3
		}
		if p := g.porterParty(); p != nil && !g.revealed(p, p.Guests[0]) {
			return 1
		}
		return 2
	case "O7":
		return b2i(g.sawn("osvaldo"))
	case "O8":
		switch {
		case g.rollGet("O8.parcel") > 0:
			return 2
		case g.verbsComing(rowWreath):
			return 1
		}
	case "O9":
		switch {
		case held == "O9":
			return 3
		case !g.sawn("O9.read"):
			return 0
		}
		return 1 + b2i(g.level >= 2)
	case "O11":
		return b2i(held == "O11")
	case "O12":
		return b2i(g.sawn("O12.odd"))
	case "O13":
		return b2i(g.verbsComing(rowCrate))
	}
	return 0
}

// stepOf: open objective id's next step.
func (g *Game) stepOf(id string) (stepDef, bool) {
	ss := objSteps[id]
	if len(ss) == 0 {
		return stepDef{}, false
	}
	return ss[min(g.stepIdx(id), len(ss)-1)], true
}

// stepText: the step's row with {n} (the Room it is about) and {item} (the Supply asked for) filled in.
func (g *Game) stepText(id string, s stepDef) string {
	t := s.Text
	if strings.Contains(t, "{n}") {
		key := map[string]string{"O3": "celestine", "O10": "celestine", "O7": "osvaldo", "O6": "porter"}[id]
		n := "?"
		if p, _ := g.stepGuest(key); p != nil && p.Room >= 0 && p.Room < len(g.hotel.Rooms) {
			n = g.hotel.Rooms[p.Room].Number
		}
		t = strings.ReplaceAll(t, "{n}", n)
	}
	if strings.Contains(t, "{item}") {
		name := "SUPPLY"
		if p := g.questParty("tim"); p != nil {
			if k := supplyOf(p.want); k >= 0 {
				for _, it := range shop {
					if it.ID == supplies[k].ID {
						name = upper(it.Name)
					}
				}
			}
		}
		t = strings.ReplaceAll(t, "{item}", name)
	}
	return t
}

// stepGuest: the Party and guest a step's guest key names (nil if it is not in the hotel).
func (g *Game) stepGuest(key string) (*Party, *entities.Guest) {
	switch key {
	case "":
		return nil, nil
	case "ghostmate":
		if m := g.questParty("magenta"); m != nil {
			for _, p := range g.parties {
				if p.Realm == Ghost && p.Room == m.Room && p.state == staying {
					return p, p.Guests[0]
				}
			}
		}
		return nil, nil
	case "porter":
		if p := g.porterParty(); p != nil {
			return p, g.verbs.porter
		}
		return nil, nil
	case "stranger":
		if p := g.verbs.stranger; p != nil {
			return p, p.Guests[0]
		}
		return nil, nil
	case "ally":
		key = g.ally()
	}
	if p, i := g.questHolder(key); p != nil {
		return p, p.Guests[i]
	}
	return nil, nil
}

// guideRow is one line of the checklist.
type guideRow struct {
	ID   string
	Text string
	Step stepDef
	Done bool
	Need Unlock // the shop row the step needs and the player has not bought: the Terminal and that row wear the outline
}

// guideRows: the beat's title and its rows now: an open objective reads its next step, a done one its row; once the
// beat is done (or before the first), "GROW THE HOTEL" while another Growth is to come.
func (g *Game) guideRows() (title string, rows []guideRow) {
	q := g.quest
	grow := guideRow{ID: "O0", Text: growStep.Text, Step: growStep}
	if q.Beat < 1 {
		return "", []guideRow{grow}
	}
	bt := beats[q.Beat-1]
	open := false
	for i, o := range bt.Objs {
		if i < len(q.Done) && q.Done[i] {
			rows = append(rows, guideRow{ID: o.ID, Text: o.Row, Done: true})
			continue
		}
		open = true
		r := guideRow{ID: o.ID, Text: o.Row}
		if s, ok := g.stepOf(o.ID); ok {
			r.Step, r.Text, r.Need = s, g.stepText(o.ID, s), g.stepNeed(o.ID)
		}
		rows = append(rows, r)
	}
	if !open && q.Beat < beatsAt(g.level) {
		rows = append(rows, grow)
	}
	return bt.Title, rows
}

// stepNeed: the shop row open objective id's current step needs and is not bought yet ("" if none): the Wreath and the
// Crate, Father K's call (STAFF) for O6, the Supply O5's ghost asked for while none is in stock.
func (g *Game) stepNeed(id string) Unlock {
	i := g.stepIdx(id)
	switch {
	case id == "O8" && i == 0:
		return rowWreath
	case id == "O13" && i == 0:
		return rowCrate
	case id == "O6" && i == 2 && g.locked[unlockK]:
		return unlockK
	case id == "O5" && i == 1:
		if p := g.questParty("tim"); p != nil {
			if k := supplyOf(p.want); k >= 0 && g.stock[supplies[k].ID] == 0 {
				return supplies[k].ID
			}
		}
	}
	return ""
}

// stepNeeds: an open objective's current step needs shop row id (cheap: onSale asks it per row per frame).
func (g *Game) stepNeeds(id Unlock) bool {
	if g.quest.Beat < 1 {
		return false
	}
	for i, o := range beats[g.quest.Beat-1].Objs {
		if i < len(g.quest.Done) && !g.quest.Done[i] && g.stepNeed(o.ID) == id {
			return true
		}
	}
	return false
}

// needRows: the shop rows the Terminal outlines now.
func (g *Game) needRows() (ids []Unlock) {
	_, rows := g.guideRows()
	for _, r := range rows {
		if !r.Done && r.Need != "" {
			ids = append(ids, r.Need)
		}
	}
	return ids
}

// needInk: the outline's ink in the Dimension the Concierge is in.
func (g *Game) needInk(a float32) color.NRGBA {
	if g.ghost {
		return withAlpha(markSilver, a)
	}
	return withAlpha(markGold, a)
}

// ---- the marker ----

// mark is one outlined target: how to draw its outline, and its box (world px).
type mark struct {
	tint   []float32
	ink    color.RGBA
	guest  *entities.Guest // its sprite's silhouette
	party  *Party
	spirit bool            // the spirit's
	props  []int           // indexes in g.props: their silhouettes (a Mirror, the Terminal)
	ring   image.Rectangle // a frame round a plain shape: the book, a plate, a door, a shelf
	box    image.Rectangle
	z      float64 // sort y of its outline
	hollow bool    // only the outline, the silhouette left clear (a guest under the Fog)
}

var (
	markGold   = color.RGBA{255, 214, 70, 255}
	markSilver = color.RGBA{214, 222, 240, 255}
)

// guideMarks: the targets to outline now, one per open objective's step, deduplicated; in the step's Dimension its
// target, else the nearest Mirror. vis is the view in world px.
func (g *Game) guideMarks(vis image.Rectangle) (ms []mark) {
	if g.state != StatePlay || g.intro != nil || g.shifting != nil || g.viewT > 0 {
		return nil
	}
	_, rows := g.guideRows()
	ink, tint := markGold, []float32{1, .84, .27}
	if g.ghost {
		ink, tint = markSilver, []float32{.84, .87, .94}
	}
	for _, r := range rows {
		if r.Done || r.Step.Text == "" {
			continue
		}
		var m *mark
		tgt := r.Step.Tgt
		if r.Need != "" {
			tgt = target{tgTerminal, ""}
		}
		switch {
		case r.Step.Dim == g.realm() || r.Step.Any:
			m = g.markOf(tgt, vis)
		case !r.Step.Wait && g.unlocked(Mirror):
			m = g.mirrorMark()
		}
		if m == nil || m.box.Empty() {
			continue
		}
		m.ink, m.tint = ink, tint
		dup := false
		for _, o := range ms {
			dup = dup || o.box == m.box
		}
		if !dup {
			ms = append(ms, *m)
		}
	}
	return ms
}

const gts = world.TileSize

// markOf: the mark of target t, nil when there is nothing to outline (not in the hotel, off the view).
func (g *Game) markOf(t target, vis image.Rectangle) *mark {
	switch t.Kind {
	case tgGuest:
		p, guest := g.stepGuest(t.Key)
		if p == nil {
			return nil
		}
		at := image.Pt(int(guest.X), int(guest.Y))
		if p.shown(guest) && at.In(vis.Inset(-gts)) {
			return &mark{guest: guest, party: p, z: guest.Y - 0.01,
				box: image.Rect(at.X-8, at.Y-30, at.X+8, at.Y+2)}
		}
		if p.Room >= 0 && p.Room < len(g.hotel.Rooms) && g.isOwned(p.Room) { // inside its Room, out of sight: the plate by its door
			return g.plateMark(p.Room)
		}
	case tgDoor:
		room := -1
		if t.Key == "handprint" {
			room = g.handprintRoom()
		} else if p, _ := g.stepGuest(t.Key); p != nil {
			room = p.Room
		}
		if room < 0 || room >= len(g.hotel.Rooms) {
			return nil
		}
		d := g.hotel.Rooms[room].Door
		r := image.Rect(d.X*gts, d.Y*gts, d.X*gts+gts, d.Y*gts+2*gts)
		return &mark{ring: r, box: r, z: float64(r.Max.Y) + 1}
	case tgBook:
		b := g.bookRect()
		r := image.Rect(b.Min.X+4, b.Min.Y+3, b.Min.X+12, b.Min.Y+8)
		c := g.hotel.Map.Cell(g.hotel.Book.Min.Add(image.Pt(0, 2*gts)))
		return &mark{ring: r, box: r, z: float64(g.hotel.Map.TileSortY(world.LayerDecor, c.X, c.Y)) + 0.01}
	case tgTerminal:
		tm := g.hotel.Terminal
		return g.propsMark(image.Rect(tm.Min.X, tm.Min.Y+gts, tm.Max.X, tm.Max.Y-gts), func(n string) bool { return !maze(n) })
	case tgSupply:
		if p := g.questParty("tim"); p != nil {
			if k := supplyOf(p.want); k >= 0 {
				r := supplySpot(g.hotel, k)
				r = image.Rect(r.Min.X, r.Min.Y-gts, r.Max.X, r.Max.Y-gts)
				return &mark{ring: r, box: r, z: float64(r.Max.Y) + 1}
			}
		}
	case tgSpirit:
		if s := g.verbs.spirit; s.on && !g.ghost {
			return &mark{spirit: true, z: s.y - 0.01, box: image.Rect(int(s.x)-8, int(s.y)-30, int(s.x)+8, int(s.y)+2)}
		}
	case tgLobby:
		m := g.hotel.Mirror
		return g.mirrorProps(image.Rect(m.X-gts/2, m.Y-gts/2-2*gts, m.X+gts/2, m.Y-gts/2))
	}
	return nil
}

// plateMark: the Room's guest plate by its door, outlined (a guest inside is out of sight).
func (g *Game) plateMark(room int) *mark {
	pl := g.hotel.Rooms[room].Plate().Min
	c := pl.Sub(image.Pt(0, gts))
	r := image.Rect(c.X+gts-14, c.Y+3, c.X+gts-5, c.Y+10)
	cc := g.hotel.Map.Cell(pl).Sub(image.Pt(0, 1))
	return &mark{ring: r, box: r, z: float64(g.hotel.Map.TileSortY(world.LayerDecor, cc.X, cc.Y)) + 0.01}
}

// mirrorProps: the Mirror's glass, as the props it is.
func (g *Game) mirrorProps(glass image.Rectangle) *mark {
	return g.propsMark(glass, func(n string) bool { return strings.HasPrefix(n, "prop_mirror") })
}

// propsMark: the props whose picture lies in r and whose tile passes keep, for silhouettes.
func (g *Game) propsMark(r image.Rectangle, keep func(name string) bool) *mark {
	m := &mark{box: r, z: -math.MaxFloat64}
	for i, p := range g.props {
		o := g.hotel.Map.Origin(p.cx, p.cy)
		if !(image.Rectangle{o, o.Add(g.tiles[p.id].Bounds().Size())}).Overlaps(r) || !keep(g.hotel.Map.TS.Tiles[p.id].Name) {
			continue
		}
		m.props = append(m.props, i)
		m.z = max(m.z, float64(p.y)+0.01) // after the props it outlines, which it draws again on top
	}
	if len(m.props) == 0 {
		return &mark{ring: r, box: r, z: float64(r.Max.Y) + 1}
	}
	return m
}

// mirrorMark: the nearest Mirror to the Concierge: the Lobby's, a Room's, an open Corridor's.
func (g *Game) mirrorMark() *mark {
	me := image.Pt(int(g.player.X), int(g.player.Y))
	best, far := image.Rectangle{}, math.MaxFloat64
	try := func(r image.Rectangle) {
		c := r.Min.Add(r.Max).Div(2)
		if d := math.Hypot(float64(c.X-me.X), float64(c.Y-me.Y)); d < far {
			best, far = r, d
		}
	}
	m := g.hotel.Mirror
	try(image.Rect(m.X-gts/2, m.Y-gts/2-2*gts, m.X+gts/2, m.Y-gts/2))
	for _, r := range g.hotel.Rooms {
		if !r.Mirror.Empty() {
			try(image.Rect(r.Mirror.Min.X, r.Mirror.Min.Y-2*gts, r.Mirror.Max.X, r.Mirror.Min.Y))
		}
	}
	for _, c := range g.hotel.Corridors {
		if c.Open {
			try(c.Glass)
		}
	}
	return g.mirrorProps(best)
}

// guideActors: each mark's outline for the y-sorted pass, drawn just before what it outlines so that it shows only
// round it: the sprite's alpha mask offset a world px each way, flat in the ink (4 draws a sprite, no pixel work).
func (g *Game) guideActors(ms []mark, ox, oy int) (as []actor) {
	for _, m := range ms {
		m := m
		if g.underFog(m) {
			continue
		}
		x := 0.0
		if m.guest != nil {
			x = m.guest.X
		}
		as = append(as, actor{x: x, y: m.z, draw: func(s *ebiten.Image, _, _ int) { g.drawOutline(s, m, ox, oy) }})
	}
	return as
}

// underFog: the mark's guest stands in a fogged Room: its outline is drawn over the Fog and the darkness instead
// (drawFogOutlines), hollow, so that the sprite stays covered.
func (g *Game) underFog(m mark) bool {
	return m.guest != nil && g.fogAt(image.Pt(int(m.guest.X), int(m.guest.Y))) > 0
}

// drawFogOutlines: the hollow outlines of the guests under the Fog.
func (g *Game) drawFogOutlines(s *ebiten.Image, ox, oy int, ms []mark) {
	for _, m := range ms {
		if g.underFog(m) {
			m.hollow = true
			g.drawOutline(s, m, ox, oy)
		}
	}
}

// outlineAlpha: the outline breathes a little, from 65% to 100%.
func (g *Game) outlineAlpha() float32 { return float32(0.82 + 0.18*math.Sin(float64(g.tick)/18)) }

func (g *Game) drawOutline(s *ebiten.Image, m mark, ox, oy int) {
	a := g.outlineAlpha()
	switch {
	case m.guest != nil:
		lift := g.lift(m.guest)
		if m.party.Realm == Ghost {
			_, _, up := g.ghostLook(m.guest)
			lift += up + hover(g.tick, m.guest.Character)
		}
		an := m.guest.Anim
		g.outlineChar(s, m, m.guest.Character, an, m.guest.X, m.guest.Y-lift, ox, oy, a)
	case m.spirit:
		sp := g.verbs.spirit
		g.outlineChar(s, m, "Bob", sp.anim, sp.x, sp.y, ox, oy, a)
	case len(m.props) > 0:
		for _, i := range m.props {
			p := g.props[i]
			o := g.hotel.Map.Origin(p.cx, p.cy)
			g.outlineImg(s, m, g.tiles[p.id], g.tiles[p.id].Bounds(), float64(o.X*zoom-ox), float64(o.Y*zoom-oy), a)
		}
		for _, i := range m.props { // then the props themselves over their outline
			p := g.props[i]
			o := g.hotel.Map.Origin(p.cx, p.cy)
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(zoom, zoom)
			op.GeoM.Translate(float64(o.X*zoom-ox), float64(o.Y*zoom-oy))
			g.blit(s, g.tiles[p.id], op, modeProp, 0, nil, 0)
		}
	case !m.ring.Empty():
		r, c := m.ring, withAlpha(m.ink, a)
		x, y, w, h := r.Min.X*zoom-ox, r.Min.Y*zoom-oy, r.Dx()*zoom, r.Dy()*zoom
		rect(s, x-zoom, y-zoom, w+2*zoom, zoom, c)
		rect(s, x-zoom, y+h, w+2*zoom, zoom, c)
		rect(s, x-zoom, y, zoom, h, c)
		rect(s, x+w, y, zoom, h, c)
	}
}

func withAlpha(c color.RGBA, a float32) color.NRGBA {
	return color.NRGBA{c.R, c.G, c.B, uint8(a * 255)}
}

// outlineChar: the silhouette of a character's frame standing at (wx,wy) world px.
func (g *Game) outlineChar(s *ebiten.Image, m mark, name string, an entities.Anim, wx, wy float64, ox, oy int, a float32) {
	sheet := g.sheet(name)[0]
	if an.Moving {
		sheet = g.sheet(name)[1]
	}
	f := an.Frame()
	x, y := screenPx(wx)-ox, screenPx(wy)-oy
	g.outlineImg(s, m, sheet, image.Rect(f*charW, 0, (f+1)*charW, charH), float64(x-charW/2*zoom), float64(y+(entities.FootH/2-charH)*zoom), a)
}

// outlineSrc: a sprite's alpha mask in one flat ink (the palette shader's flat mode keeps shading, and real has none).
const outlineSrc = `//kage:unit pixels
package main

var Ink vec3

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	a := imageSrc0At(src).a * color.a
	return vec4(Ink*a, a)
}
`

var (
	outlineShader *ebiten.Shader
	outlineBuf    *ebiten.Image
)

// outlineImg draws part r of img in the ink, a world px up, down, left and right of (x,y) screen px.
func (g *Game) outlineImg(s *ebiten.Image, m mark, img *ebiten.Image, r image.Rectangle, x, y float64, a float32) {
	if outlineShader == nil {
		var err error
		if outlineShader, err = ebiten.NewShader([]byte(outlineSrc)); err != nil {
			panic(err)
		}
	}
	part := img.SubImage(r).(*ebiten.Image)
	if m.hollow { // the outline in a buffer a world px wider each way, the sprite's own pixels cut out of it
		m.hollow = false
		w, h := r.Dx()*zoom+2*zoom, r.Dy()*zoom+2*zoom
		if outlineBuf == nil || outlineBuf.Bounds().Dx() != w || outlineBuf.Bounds().Dy() != h {
			outlineBuf = ebiten.NewImage(w, h)
		}
		outlineBuf.Clear()
		g.outlineImg(outlineBuf, m, img, r, zoom, zoom, 1)
		cut := &ebiten.DrawImageOptions{Blend: ebiten.BlendDestinationOut}
		cut.GeoM.Scale(zoom, zoom)
		cut.GeoM.Translate(zoom, zoom)
		outlineBuf.DrawImage(part, cut)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(x-zoom, y-zoom)
		op.ColorScale.ScaleAlpha(a)
		s.DrawImage(outlineBuf, op)
		return
	}
	for _, d := range [...][2]float64{{-zoom, 0}, {zoom, 0}, {0, -zoom}, {0, zoom}} {
		op := &ebiten.DrawRectShaderOptions{Images: [4]*ebiten.Image{part}, Uniforms: map[string]any{"Ink": m.tint}}
		op.GeoM.Scale(zoom, zoom)
		op.GeoM.Translate(x+d[0], y+d[1])
		op.ColorScale.ScaleAlpha(a)
		s.DrawRectShader(r.Dx(), r.Dy(), outlineShader, op)
	}
}

// bookWanted: reading e now would move an open objective on (the same tests as bookRead).
func (g *Game) bookWanted(e bookEntry) bool {
	switch {
	case !e.quest():
		return false
	case e.Line == nameEntry:
		return g.questOpen("O12") && !g.sawn("O12.odd")
	case e.Line == macguffinName:
		return g.questOpen("O12")
	}
	read := g.rollGet("O2.read")
	return g.questOpen("O2") && (e.Line == celestineEntryHard && read&2 == 0 || e.Line != celestineEntryHard && read&1 == 0)
}

// bookWay: which way the open book's page-turn goes to the spread with a wanted entry: -1 A, +1 D, 0 it is on this
// spread (reading counts only there) or nothing is wanted.
func (g *Game) bookWay() int {
	es := g.shown()
	j := slices.IndexFunc(es, g.bookWanted)
	if j < 0 {
		return 0
	}
	starts := bookStarts(es)
	cur := slices.Index(starts, g.bookTop)
	want := 0
	for k, s := range starts {
		if s <= j {
			want = k
		}
	}
	switch {
	case want > cur:
		return 1
	case want < cur:
		return -1
	}
	return 0
}
