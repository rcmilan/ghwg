package main

import (
	"image"
	"strings"
)

// The Cat (08 W5 R2): a small black cat, assets/sprites/cat.png: 16x16 frames, a row of 4 walk frames per facing
// (right, up, left, down, as the characters' sheets), then a row with the nap's 2 frames (breathing). It lives
// in the ghost dimension, on a black floor, so its outline is the light one: a rim drawn around the body.
const (
	catW      = 16
	catFrames = 4
)

var catInks = map[rune]uint32{
	'B': 0x1c1c28, 'h': 0x34344a, // the coat, and the light on its back
	'e': 0xf2de50, // the eyes
	'n': 0xc88a9a, // the nose
}

const catRim = 0x585880

// catSide, catFront, catBack: the rows every walk frame of a facing shares, from the frame's top; the legs' two
// rows follow, one pair per frame.
var catSide = []string{
	"................",
	"................",
	"................",
	"................",
	"................",
	"................",
	"...........B.B..",
	"..B........BBB..",
	"..B........BeB..",
	"...B.hhhhhBBBn..",
	"...BBBBBBBBBB...",
	"....BBBBBBBBB...",
	"....BBBBBBBBB...",
}

var catSideLegs = [catFrames][2]string{
	{"....B..B.B..B...", "....B..B.B..B..."},
	{".....BB...BB....", ".....B.....B...."},
	{"....B..B.B..B...", "...B...B.B...B.."},
	{".....BB...BB....", "......B...B....."},
}

var catFront = []string{
	"................",
	"................",
	"................",
	"................",
	"................",
	".....B....B.....",
	".....BBBBBB.....",
	".....BeBBeB.....",
	".....BBnnBB.....",
	"......BBBB......",
	".....BBBBBB.....",
	".....BBBBBB..B..",
	".....BBBBBBBB...",
}

// catBack: the front's outline, the light down the back of its head and along its spine.
var catBack = []string{
	"................",
	"................",
	"................",
	"................",
	"................",
	".....B....B.....",
	".....BBBBBB.....",
	".....BBhhBB.....",
	".....BBBBBB.....",
	"......BBBB......",
	".....BBhhBB.....",
	".....BBhhBB..B..",
	".....BBBBBBBB...",
}

var catFrontLegs = [catFrames][2]string{
	{".....BB..BB.....", ".....BB..BB....."},
	{".....BB..BB.....", ".........BB....."},
	{".....BB..BB.....", ".....BB..BB....."},
	{".....BB..BB.....", ".....BB........."},
}

var catNap = [2][]string{
	{
		"................",
		"................",
		"................",
		"................",
		"................",
		"................",
		"................",
		"................",
		"................",
		"..........B.B...",
		"....hhhhhhBBB...",
		"...BBBBBBBBhBB..",
		"..BBBBBBBBBBBB..",
		"..BBBBBBBBBBBB..",
		"...BhhhhhhhBB...",
	},
	{
		"................",
		"................",
		"................",
		"................",
		"................",
		"................",
		"................",
		"................",
		"................",
		".....hhhh.B.B...",
		"....BBBBBBBBB...",
		"...BBBBBBBBhBB..",
		"..BBBBBBBBBBBB..",
		"..BBBBBBBBBBBB..",
		"...BhhhhhhhBB...",
	},
}

func mirror(rows []string) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		b := []byte(r)
		for l, h := 0, len(b)-1; l < h; l, h = l+1, h-1 {
			b[l], b[h] = b[h], b[l]
		}
		out[i] = string(b)
	}
	return out
}

// catSheet: the frames drawn in their inks, each then rimmed: every clear pixel beside the body takes the rim.
func catSheet() *image.NRGBA {
	var rows [5][][]string
	for f := range catFrames {
		side := append(append([]string{}, catSide...), catSideLegs[f][:]...)
		if f == 2 { // the tail sways
			side[7], side[8] = strings.Replace(side[7], "..B.", "...B", 1), strings.Replace(side[8], "..B.", "...B", 1)
		}
		rows[0] = append(rows[0], side)
		rows[1] = append(rows[1], append(append([]string{}, catBack...), catFrontLegs[f][:]...))
		rows[2] = append(rows[2], mirror(side))
		rows[3] = append(rows[3], append(append([]string{}, catFront...), catFrontLegs[f][:]...))
	}
	rows[4] = catNap[:]
	out := image.NewNRGBA(image.Rect(0, 0, catFrames*catW, len(rows)*catW))
	for j, row := range rows {
		for i, frame := range row {
			at := func(x, y int) rune {
				if y < 0 || y >= len(frame) || x < 0 || x >= catW {
					return '.'
				}
				return rune(frame[y][x])
			}
			for y := range catW {
				for x := range catW {
					if c, ok := catInks[at(x, y)]; ok {
						out.SetNRGBA(i*catW+x, j*catW+y, rgb(c))
					} else if at(x-1, y) != '.' || at(x+1, y) != '.' || at(x, y-1) != '.' || at(x, y+1) != '.' {
						out.SetNRGBA(i*catW+x, j*catW+y, rgb(catRim))
					}
				}
			}
		}
	}
	return out
}
