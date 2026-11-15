//go:build js

package game

import (
	"fmt"
	"image/color"
	"net/url"
	"strings"
	"syscall/js"

	"github.com/hajimehoshi/ebiten/v2"
)

// fpsOn: the page URL has ?fps, a dev-only readout for live tests of the web build.
var fpsOn = func() bool {
	loc := js.Global().Get("location")
	if loc.IsUndefined() { // the Node test runner has no page
		return false
	}
	q, _ := url.ParseQuery(strings.TrimPrefix(loc.Get("search").String(), "?"))
	return q.Has("fps")
}()

func drawFPS(screen *ebiten.Image) {
	if fpsOn {
		printAt(screen, fmt.Sprintf("%.0f FPS  %.0f TPS", ebiten.ActualFPS(), ebiten.ActualTPS()), 2, 2, 1, color.White, 1)
	}
}
