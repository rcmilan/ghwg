package game

import (
	"image"
	"image/color"

	"ghosthotel/assets"
)

// Res is one of the two currencies (D178). Fear and Room Spook are stats, not currencies.
type Res int

const (
	Money Res = iota
	Spook
)

// Cost is a price in both currencies: Cost{Money: 10} or Cost{Money: 5, Spook: 3}.
type Cost [2]Num

// noFloat: earn without a floater (the haunt batches its own "+N", haunt.go).
var noFloat = image.Pt(-1, -1)

// earn adds n of r and floats "+N" from at (world px).
func (g *Game) earn(r Res, n Num, at image.Point) {
	g.wallet[r] += n
	if at == noFloat {
		return
	}
	txt := "+" + fmtNum(n)
	if r == Money {
		txt = "+$" + fmtNum(n)
	}
	g.floaters = append(g.floaters, floater{float64(at.X), float64(at.Y), txt, g.tick, floaterKind(r)}) // floatMoney, floatSpook
}

func (g *Game) afford(c Cost) bool { return g.wallet[Money] >= c[Money] && g.wallet[Spook] >= c[Spook] }

// pay takes the whole Cost, or nothing if the wallet can't cover it.
func (g *Game) pay(c Cost) bool {
	if !g.afford(c) {
		return false
	}
	g.wallet[Money], g.wallet[Spook] = g.wallet[Money]-c[Money], g.wallet[Spook]-c[Spook]
	return true
}

// resInk: a currency's color. Static for now; E2 makes it cycle (gold↔silver, green↔purple).
func resInk(r Res) color.Color {
	if r == Money {
		return assets.Money
	}
	return color.RGBA{168, 92, 230, 255}
}
