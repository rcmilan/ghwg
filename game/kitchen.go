package game

import (
	"fmt"
	"image"

	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// The kitchen (07 F1): the Lobby's part east of the Archway, walled off, there from the start, the same in
// both dimensions.
// Its back wall carries the Supplies' props (supply.go) and the decoration below; the courier stops at each
// ordered Supply's prop. A Supply's icon and count show over its prop when the Concierge is near.

const stockNear = 2 // cells from a Supply's prop within which its icon and count show

// kitchenDecor: the props that only stand there, on the cells the Supplies leave free.
var kitchenDecor = []Supply{
	{Prop: "drawer_a", X: 4, W: 1},
}

// supplyDrop: where the courier stands to fill supply k, the cell in front of its prop.
func (g *Game) supplyDrop(k int) image.Point {
	r := supplySpot(g.hotel, k)
	return image.Pt((r.Min.X+r.Max.X)/2, r.Max.Y-world.TileSize/2)
}

// stockShown: supply k's count shows, the Concierge within stockNear cells of its prop.
func (g *Game) stockShown(k int) bool { return g.nearStock(supplySpot(g.hotel, k)) }

// nearStock: the Concierge is within stockNear cells of the prop r (world px): its stock shows.
func (g *Game) nearStock(r image.Rectangle) bool {
	near := stockNear * world.TileSize
	return g.feet().In(image.Rect(r.Min.X-near, r.Min.Y-near, r.Max.X+near, r.Max.Y+near))
}

// drawStockMark draws supply id's icon and "xN" over a prop: centered on world x mid, just above world y top. The
// kitchen's shelves and the pantries both draw their stock with it.
func (g *Game) drawStockMark(screen *ebiten.Image, id Unlock, n, mid, top, ox, oy int) {
	icon := g.supplyIcon(id)
	label := fmt.Sprintf("x%d", n)
	iw, ih := icon.Bounds().Dx()*zoom, icon.Bounds().Dy()*zoom
	ix, iy := mid*zoom-ox-(iw+2+6*len(label))/2, (top-2)*zoom-oy-ih
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(float64(ix), float64(iy))
	screen.DrawImage(icon, op)
	g.inked(screen, label, ix+iw+2, iy+ih-15, 0, 0, 0) // black, on the walls' light paint
}

// drawKitchen draws the kitchen's props on their cells, against the back wall, and, with the Concierge near a
// Supply's prop, its icon and count over it. drawWorld calls it in the y-sorted pass, after the wall's face.
func (g *Game) drawKitchen(screen *ebiten.Image, ox, oy int) {
	stock := g.hotel.Stock
	op := &ebiten.DrawImageOptions{}
	for _, s := range append(kitchenDecor, supplies...) {
		img := g.sprite(s.Prop)
		if img == nil {
			continue
		}
		w, h := img.Bounds().Dx(), img.Bounds().Dy()
		mid, top := stock.Min.X+s.X*world.TileSize+s.W*world.TileSize/2, stock.Max.Y-h
		op.GeoM.Reset()
		op.GeoM.Scale(zoom, zoom)
		op.GeoM.Translate(float64((mid-w/2)*zoom-ox), float64(top*zoom-oy))
		g.blit(screen, img, op, modeProp, 0, nil, 0)
	}
	for k, s := range supplies { // over every prop: a neighbour never hides a count
		if !g.stockShown(k) {
			continue
		}
		mid, top := stock.Min.X+s.X*world.TileSize+s.W*world.TileSize/2, stock.Max.Y-g.sprite(s.Prop).Bounds().Dy()
		g.drawStockMark(screen, s.ID, g.stock[s.ID], mid, top, ox, oy)
	}
}

// DevKitchen starts the Run for a print (-kitchen): the Water full, the Food low, the Towels out, and you in
// front of the oven.
func (g *Game) DevKitchen() {
	g.stock = map[Unlock]int{SupplyWater: 5, SupplyFood: 2, SupplyVeggie: 3, SupplyGourmet: 1}
	at := g.supplyDrop(supplyOf(SupplyFood))
	g.player.X, g.player.Y = float64(at.X), float64(at.Y)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}
