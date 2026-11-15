//go:build !js

package game

import "github.com/hajimehoshi/ebiten/v2"

// drawFPS: the FPS readout is for the web build (fps_js.go).
func drawFPS(*ebiten.Image) {}
