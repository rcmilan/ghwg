package game

import (
	"image"
	"image/color"
	"slices"
	"strings"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 W5-ITEMS: Magenta's six items, owned for good once a beat first gives them (itemOn, quest.go). The Crystal
// ball is card.go's revealCost; the rest are here. Placeholders: the numbers below.
const (
	broomBoost  = 1.3 // the Concierge's walk in ghost with the Broom
	geraldMul   = 1.5 // a sighting's Ecto in Gerald's Room
	stormMul    = 2.0 // every sighting's Ecto while the storm lasts
	catClean    = 15  // game minutes the Cat takes over a Room's spiritual dirt
	catNap      = 30  // game minutes of a nap
	catNaps     = 2   // naps a shift (a game day), contractually
	catRest     = 10  // game minutes it sits at a spot before it roams on
	catSpeed    = keeperStroll
	catWorkWalk = keeperSpeed
)

const (
	rowGerald Unlock = "item-gerald"
	rowWard   Unlock = "item-ward"
)

// itemRun: the items' Run state, in the checkpoint: where Gerald and the Ward stand (Room index + 1, 0 nowhere),
// the day the Storm jar was last shaken (0 never). The rest is not saved: the Cat is back at the Entrance when a
// Run is resumed.
type itemRun struct {
	Gerald, Ward, Jar int
	last              int // the clock last seen: a clock gone back is a new Run
	cat               *catState
}

type catState struct {
	entities.Guest
	room        int // the Room it cleans, while it does
	cleaning    bool
	napping     bool
	until       int // the clock its clean or nap is done, or it roams on; 0 none
	naps, shift int // naps taken this shift (game day)
}

// regrid: a Growth renumbers the Rooms (grow.go).
func (it *itemRun) regrid(room func(int) int) {
	for _, r := range [...]*int{&it.Gerald, &it.Ward} {
		if *r > 0 {
			*r = room(*r-1) + 1
		}
	}
}

// itemRoom: the Room v (an itemRun place) names, -1 for none.
func (g *Game) itemRoom(v int) int {
	if v < 1 || v > len(g.hotel.Rooms) {
		return -1
	}
	return v - 1
}

// stormJarOn: the Storm jar's HOTEL row is on sale now (owned, not shaken today).
func (g *Game) stormJarOn() bool { return g.itemOn(itemStorm) && g.items.Jar != day(g.clock) }

// shakeJar: the jar is bought: tonight's storm, once a day.
func (g *Game) shakeJar() {
	g.items.Jar = day(g.clock)
	g.summonStorm()
}

// warded: the Ward is on Room room: its human Parties never flee.
func (g *Game) warded(room int) bool {
	return g.itemOn(itemWard) && room >= 0 && g.itemRoom(g.items.Ward) == room
}

// conciergeBoost: the Concierge's walking speed multiplier (the Broom: faster in ghost).
func (g *Game) conciergeBoost() float64 {
	if g.ghost && g.itemOn(itemBroom) {
		return broomBoost
	}
	return 1
}

// itemPay: what a sighting in a Room pays, times: Gerald's Room, and the storm.
func itemPay(gerald, storm bool) float64 {
	m := 1.0
	if gerald {
		m *= geraldMul
	}
	if storm {
		m *= stormMul
	}
	return m
}

func (g *Game) sightingPay(room int) float64 {
	return itemPay(g.itemOn(itemGerald) && g.itemRoom(g.items.Gerald) == room, g.stormOn())
}

// itemRowOn: id is one of the placing rows of a Room's ring; on: its item is owned.
func (g *Game) itemRowOn(id Unlock) (on, row bool) {
	switch id {
	case rowGerald:
		return g.itemOn(itemGerald), true
	case rowWard:
		return g.itemOn(itemWard), true
	}
	return false, false
}

// placeRefuse: why the item (Gerald or the Ward) can't be placed on the Room: it is shut or he is already there.
func placeRefuse(g *Game, room int, id Unlock) string {
	at := g.items.Gerald
	if id == rowWard {
		at = g.items.Ward
	}
	switch {
	case !g.isOwned(room):
		return "SHUT"
	case at == room+1:
		return "ALREADY HERE"
	}
	return ""
}

func init() { // the placing rows, on the Room's ring (shop.go has the rest)
	shop = append(shop,
		Item{ID: rowGerald, Tab: tabRooms, Name: "Place " + nameGerald, Blurb: nameGerald + " floats here. This Room's sightings pay 50% more.", Icon: "icon_key",
			Refuse: func(g *Game, room int) string { return placeRefuse(g, room, rowGerald) },
			Send:   func(g *Game, l cartLine) { g.items.Gerald = l.room + 1 }},
		Item{ID: rowWard, Tab: tabRooms, Name: "Place the Ward", Blurb: "Its human Parties never flee. Guests may leave. Emotionally.", Icon: "lock_red",
			Refuse: func(g *Game, room int) string { return placeRefuse(g, room, rowWard) },
			Send:   func(g *Game, l cartLine) { g.items.Ward = l.room + 1 }})
}

// updateItems runs each step of world time.
func (g *Game) updateItems() {
	it := &g.items
	if g.clock < it.last { // a new Run
		*it = itemRun{}
	}
	it.last = g.clock
	if !g.itemOn(itemCat) {
		it.cat = nil
		return
	}
	g.updateCat()
}

// catSend sets the Cat walking to `to`, through the door of room if it goes into or out of one.
func (g *Game) catSend(c *catState, to image.Point, room int) {
	c.RoomIndex = room
	g.reroute(&c.Guest, to)
}

// catRoam: to one of the housekeepers' roaming spots.
func (g *Game) catRoam(c *catState) {
	g.catSend(c, g.roamSpot(&housekeeper{Guest: c.Guest}), g.roomAt(c.X, c.Y))
}

// updateCat: the Cat walks in at the Entrance each Run, roams, takes its two naps a shift and cleans the first
// Room with spiritual dirt, which no Housekeeper does. A familiar: it ignores doors and Guests.
func (g *Game) updateCat() {
	c := g.items.cat
	if c == nil {
		c = &catState{Guest: entities.NewGuest(g.hotel.Spawn, "Cat"), room: -1}
		g.items.cat = c
		g.catRoam(c)
	}
	if d := day(g.clock); d != c.shift {
		c.shift, c.naps = d, 0
	}
	speed := catSpeed
	if c.cleaning {
		speed = catWorkWalk
	}
	arrived := c.FollowPath(speed * g.dt)
	switch {
	case c.napping:
		if g.clock >= c.until {
			c.napping, c.until = false, 0
		}
	case c.cleaning:
		if !arrived {
			break
		}
		if c.until == 0 {
			c.until = g.clock + catClean*minuteFrames
		}
		if g.clock >= c.until {
			g.scrub(c.room, true)
			c.cleaning, c.until = false, 0
			g.catRoam(c)
		}
	default:
		if room := g.spiritRoom(); room >= 0 {
			cell := slices.IndexFunc(g.spots(room), func(s spot) bool { return s.spirit })
			s := g.spots(room)[cell].cell
			c.cleaning, c.room, c.until = true, room, 0
			g.catSend(c, g.hotel.Map.Center(s.X, s.Y), room)
			break
		}
		if !arrived || g.clock < c.until {
			break
		}
		if c.naps < catNaps {
			c.napping, c.until = true, g.clock+catNap*minuteFrames
			c.naps++
			break
		}
		c.until = g.clock + catRest*minuteFrames
		g.catRoam(c)
	}
}

// spiritRoom: the first open Room with spiritual dirt, -1 if none.
func (g *Game) spiritRoom() int {
	for i := range g.hotel.Rooms {
		if g.isOwned(i) && slices.ContainsFunc(g.spots(i), func(s spot) bool { return s.spirit }) {
			return i
		}
	}
	return -1
}

// itemActors: the Cat and Gerald (ghost only), the Ward's mark (real only), for the y-sorted pass.
func (g *Game) itemActors() []actor {
	var as []actor
	if c := g.items.cat; c != nil && g.ghost {
		nap := c.napping
		as = append(as, actor{x: c.X, y: c.Y, draw: func(s *ebiten.Image, x, y int) { g.drawCat(s, x, y, nap) }})
	}
	if r := g.itemRoom(g.items.Gerald); r >= 0 && g.ghost && g.itemOn(itemGerald) {
		sh := g.hotel.Rooms[r].Shell()
		at := sh.Min.Add(sh.Max).Div(2)
		as = append(as, actor{x: float64(at.X), y: float64(at.Y), draw: func(s *ebiten.Image, x, y int) { g.drawGerald(s, x, y) }})
	}
	if r := g.itemRoom(g.items.Ward); r >= 0 && !g.ghost && g.itemOn(itemWard) {
		d := g.hotel.Rooms[r].Door // the wall east of the door, across from the guest plate
		at := image.Pt(d.X*world.TileSize+3*world.TileSize/2, (d.Y+2)*world.TileSize)
		as = append(as, actor{x: float64(at.X), y: float64(at.Y), draw: drawWard})
	}
	return as
}

var wardRows = []string{
	"..XX..",
	".X..X.",
	"XXXXXX",
	"X.XX.X",
	".X..X.",
	"..XX..",
}

var wardImg *ebiten.Image

// drawWard: a small warding sigil on the wall by the Room's door, (x,y) the foot of that wall cell.
func drawWard(s *ebiten.Image, x, y int) {
	if wardImg == nil {
		wardImg = bitmap(wardRows)
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(float64(x-3*zoom), float64(y-9*zoom))
	op.ColorScale.ScaleWithColor(color.RGBA{214, 74, 96, 255})
	s.DrawImage(wardImg, op)
}

// drawGerald: the skull, pale, bobbing as the ghosts float (hover), (x,y) the Room's centre.
func (g *Game) drawGerald(s *ebiten.Image, x, y int) {
	if g.skull == nil {
		g.skull = bitmap(skullRows)
	}
	lift := int((hover(g.tick, nameGerald) + 6) * zoom)
	g.stamp(s, g.skull, x-len(skullRows[0])*zoom/2, y-lift-len(skullRows)*zoom, zoom, color.RGBA{226, 236, 220, 255})
}

var catBody = []string{
	"X.X.......",
	"XXXX...X..",
	"XXXXXXXXX.",
	".XXXXXXXXX",
	".XXXXXXXX.",
	".X.X..X.X.",
}

var catNapBody = []string{
	"..........",
	"....XX.XX.",
	"..XXXXXXXX",
	".XXXXXXXXX",
	".XXXXXXXX.",
	"..XXXXXX..",
}

var catImgs [2]*ebiten.Image

// drawCat: a small black cat with a pale rim so it reads on the dark floor, yellow eyes awake, a Z when napping.
func (g *Game) drawCat(s *ebiten.Image, x, y int, nap bool) {
	k := 0
	rows := catBody
	if nap {
		k, rows = 1, catNapBody
	}
	if catImgs[k] == nil {
		catImgs[k] = bitmap(rows)
	}
	w, h := len(rows[0])*zoom, len(rows)*zoom
	ox, oy := x-w/2, y-h
	for _, d := range [...][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} { // the pale rim
		g.stamp(s, catImgs[k], ox+d[0]*zoom/2, oy+d[1]*zoom/2, zoom, color.RGBA{120, 120, 160, 255})
	}
	g.stamp(s, catImgs[k], ox, oy, zoom, color.RGBA{16, 16, 22, 255})
	if nap {
		g.tinted(s, "z", ox+w, oy-glyphH, color.RGBA{226, 236, 220, 255}, 1)
		return
	}
	rect(s, ox+zoom, oy+zoom, zoom, zoom, color.RGBA{240, 220, 70, 255})
	rect(s, ox+3*zoom-zoom/2, oy+zoom, zoom, zoom, color.RGBA{240, 220, 70, 255})
}

// DevItems is main's -items: all, or a comma list of ball, broom, gerald, cat, ward, storm, owned from the
// start; Gerald and the Ward stand in Room 101, the Cat naps beside the Concierge in it, every Room open.
func (g *Game) DevItems(list string) {
	if list == "" {
		return
	}
	names := [itemCount]string{"ball", "broom", "gerald", "cat", "ward", "storm"}
	for i, n := range names {
		g.questRec.Items[i] = list == "all" || slices.Contains(strings.Split(list, ","), n)
	}
	g.roomOwned = nil
	g.items.Gerald, g.items.Ward = 1, 1
	r := g.hotel.Rooms[0].Origin
	g.player.X, g.player.Y = float64((r.X+world.RoomW/2)*world.TileSize), float64((r.Y+world.RoomH-3)*world.TileSize)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
	g.items.cat = &catState{Guest: entities.NewGuest(image.Pt(int(g.player.X)+2*world.TileSize, int(g.player.Y)), "Cat"), room: -1}
}
