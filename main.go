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
	if err := ebiten.RunGame(game.New()); err != nil {
		log.Fatal(err)
	}
}
