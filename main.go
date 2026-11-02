package main

import (
	"log"

	"ghosthotel/game"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	ebiten.SetWindowSize(960, 540)
	ebiten.SetWindowTitle("Ghost Hotel")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	g, err := game.New()
	if err != nil {
		log.Fatal(err)
	}
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
