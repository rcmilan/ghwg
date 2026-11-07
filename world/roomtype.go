package world

import (
	"encoding/json"
	"fmt"
	"image"
	"io/fs"
	"slices"
	"strings"
)

// Room Type maps are exactly RoomW×RoomH tiles; the door sits on the south face rows doorRow and doorRow+1.
const (
	RoomW   = 9
	RoomH   = 9
	doorRow = 7
)

// Tile layers, in draw order. Room Types and the Hotel tilemap share them.
const (
	LayerFloor = iota
	LayerWalls // faces and lintels
	LayerDecor // drawn over walls: side lines, windows, doors, counter
	LayerProps // furniture and rugs (D65), over the decor: a bookshelf may cover a window
	NumLayers
)

var layerNames = [NumLayers]string{"floor", "walls", "decor", "props"}

// Bed layouts (D80) and the Stay Spots each holds: a Room's capacity.
var bedCapacity = map[string]int{"Single": 1, "Twin": 2, "Double": 2}

// RoomType is the template a Room is built from, authored in Tiled (assets/rooms/*.tmj): a tier (Name)
// times a bed layout (Beds, D80). Several files may share both: they are variants, one prop layout each (D65).
type RoomType struct {
	Name    string // the tier: Standard or Deluxe
	Beds    string // Single, Twin or Double
	Rate    int
	Tiles   [NumLayers][]TileID // RoomW*RoomH each, row-major
	Stays   []image.Point       // Stay Spots, room-local px: one per bed, so the Room's capacity (D80)
	Lanes   []image.Rectangle   // each Stay Spot's bed, room-local px: its cells straight above the Spot (D167)
	Spots   []Spot              // one in front of each activity prop (D168)
	DoorCol int
}

// Spot is where a Guest stands to use a prop (D168), room-local px (world px on a Room): its feet At,
// facing Prop (the prop's middle; At itself for a prop it stands on, so it faces down), doing Verb.
type Spot struct {
	At, Prop image.Point
	Verb     string
}

// activityVerbs: the activity props by tile name prefix, and what a Guest does at each (D168).
var activityVerbs = [...][2]string{
	{"window_", "look out"}, {"prop_tv_", "watch TV"}, {"prop_bookshelf_", "browse"}, {"prop_wardrobe_", "open"},
	{"prop_drawer_", "open"}, {"prop_mirror_", "look in"}, {"prop_cushion_", "sit"},
}

// LoadRoomTypes loads every rooms/*.tmj, sorted by file name.
func LoadRoomTypes(fsys fs.FS, ts *Tileset) ([]RoomType, error) {
	files, err := fs.Glob(fsys, "rooms/*.tmj")
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no rooms/*.tmj found")
	}
	types := make([]RoomType, 0, len(files))
	for _, f := range files {
		data, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		rt, err := ParseRoomType(data, ts)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		if len(types) > 0 && rt.DoorCol != types[0].DoorCol { // D45: doors never move
			return nil, fmt.Errorf("%s: door column %d, want %d like %s", f, rt.DoorCol, types[0].DoorCol, files[0])
		}
		types = append(types, rt)
	}
	return types, nil
}

// ParseRoomType reads one .tmj and validates it against the hotel's rules.
func ParseRoomType(data []byte, ts *Tileset) (RoomType, error) {
	var m tiledMap
	if err := json.Unmarshal(data, &m); err != nil {
		return RoomType{}, err
	}
	var rt RoomType
	if m.Width != RoomW || m.Height != RoomH || m.TileWidth != TileSize || m.TileHeight != TileSize {
		return rt, fmt.Errorf("map is %dx%d tiles of %dx%d px, want %dx%d tiles of %dx%d px",
			m.Width, m.Height, m.TileWidth, m.TileHeight, RoomW, RoomH, TileSize, TileSize)
	}
	if len(m.Tilesets) != 1 {
		return rt, fmt.Errorf("uses %d tilesets, want only hotel.tsj", len(m.Tilesets))
	}
	if _, err := prop(m.Properties, "name", &rt.Name); err != nil || rt.Name == "" {
		return rt, fmt.Errorf("map property \"name\" must be a non-empty string")
	}
	if _, err := prop(m.Properties, "rate", &rt.Rate); err != nil || rt.Rate <= 0 {
		return rt, fmt.Errorf("map property \"rate\" must be a positive int")
	}
	if _, err := prop(m.Properties, "beds", &rt.Beds); err != nil || bedCapacity[rt.Beds] == 0 {
		return rt, fmt.Errorf("map property \"beds\" must be Single, Twin or Double")
	}

	first := uint32(m.Tilesets[0].FirstGID)
	for i, name := range layerNames {
		l := findLayer(m.Layers, name, "tilelayer")
		if l == nil {
			return rt, fmt.Errorf("missing tile layer %q", name)
		}
		if l.Encoding != "" {
			return rt, fmt.Errorf("layer %q is %s-encoded; set Tile Layer Format to CSV in Tiled", name, l.Encoding)
		}
		if len(l.Data) != RoomW*RoomH {
			return rt, fmt.Errorf("layer %q has %d cells, want %d", name, len(l.Data), RoomW*RoomH)
		}
		rt.Tiles[i] = make([]TileID, len(l.Data))
		for c, gid := range l.Data {
			gid &= 0x0FFFFFFF // ponytail: flip/rotate flags dropped; flipped tiles draw unflipped
			if gid == 0 {
				continue
			}
			id := gid - first + 1
			if gid < first || int(id) >= len(ts.Tiles) || ts.Tiles[id].Name == "" {
				return rt, fmt.Errorf("layer %q cell (%d,%d): unknown tile gid %d", name, c%RoomW, c/RoomW, gid)
			}
			rt.Tiles[i][c] = TileID(id)
		}
	}

	// exactly one door column, on the south face rows
	var cols []int
	for _, layer := range rt.Tiles {
		for c, id := range layer {
			if !ts.Tiles[id].Door {
				continue
			}
			x, y := c%RoomW, c/RoomW
			if y != doorRow && y != doorRow+1 {
				return rt, fmt.Errorf("door tile at (%d,%d): doors must be on rows %d-%d", x, y, doorRow, doorRow+1)
			}
			if !slices.Contains(cols, x) {
				cols = append(cols, x)
			}
		}
	}
	if len(cols) != 1 {
		return rt, fmt.Errorf("found %d door columns %v, want exactly 1", len(cols), cols)
	}
	rt.DoorCol = cols[0]

	// one "stay" point per bed, each on walkable floor
	markers := findLayer(m.Layers, "markers", "objectgroup")
	if markers == nil {
		return rt, fmt.Errorf("missing object layer \"markers\"")
	}
	var stays []image.Point
	for _, o := range markers.Objects {
		if o.Name == "stay" && o.Point {
			stays = append(stays, image.Pt(int(o.X), int(o.Y)))
		}
	}
	if len(stays) != bedCapacity[rt.Beds] {
		return rt, fmt.Errorf("found %d \"stay\" points in the markers layer, want %d for %s (one per guest)", len(stays), bedCapacity[rt.Beds], rt.Beds)
	}
	for _, p := range stays {
		if !rt.walkable(ts, p) {
			return rt, fmt.Errorf("\"stay\" point at (%d,%d) px is not on walkable floor", p.X, p.Y)
		}
	}
	rt.Stays = stays
	for _, p := range stays { // D167: the bed in the Stay Spot's column, up from the cell above it
		c := p.Div(TileSize)
		top := c.Y
		for top > 0 && strings.HasPrefix(ts.Tiles[rt.Tiles[LayerProps][(top-1)*RoomW+c.X]].Name, "prop_bed_") {
			top--
		}
		if top == c.Y {
			return rt, fmt.Errorf("\"stay\" point at (%d,%d) px has no bed straight above it", p.X, p.Y)
		}
		rt.Lanes = append(rt.Lanes, image.Rect(c.X*TileSize, top*TileSize, (c.X+1)*TileSize, c.Y*TileSize))
	}
	rt.spots(ts)
	return rt, nil
}

// spots finds each activity prop (its 4-connected cells of one kind, on the decor or props layer) and puts
// its Spot on the free floor cell nearest the middle of the row in front of it, or on the prop itself when
// it lies flat on the floor. A cell is free when it's Room floor with nothing on it but a rug, above the
// doorway, and isn't a Stay Spot's or an earlier Spot's (D168).
func (rt *RoomType) spots(ts *Tileset) {
	kind := func(l, i int) int {
		name := ts.Tiles[rt.Tiles[l][i]].Name
		return slices.IndexFunc(activityVerbs[:], func(v [2]string) bool { return strings.HasPrefix(name, v[0]) })
	}
	taken := map[image.Point]bool{}
	for _, p := range rt.Stays {
		taken[p.Div(TileSize)] = true
	}
	free := func(c image.Point) bool {
		i := c.Y*RoomW + c.X
		if c.Y >= doorRow || rt.Tiles[LayerFloor][i] == 0 || rt.Tiles[LayerWalls][i] != 0 || taken[c] { // not a wall's walkable top row
			return false
		}
		if p := ts.Tiles[rt.Tiles[LayerProps][i]]; p.Name != "" && !p.Flat { // under a tall prop's top
			return false
		}
		return !slices.ContainsFunc(rt.Tiles[:], func(l []TileID) bool { return len(ts.Tiles[l[i]].Solid) > 0 })
	}
	mid := func(c image.Point) image.Point { return c.Mul(TileSize).Add(image.Pt(TileSize/2, TileSize/2)) }
	seen := map[[2]int]bool{}
	for _, l := range []int{LayerDecor, LayerProps} {
		for i := range rt.Tiles[l] {
			k := kind(l, i)
			if k < 0 || seen[[2]int{l, i}] {
				continue
			}
			var box image.Rectangle // the prop, in cells
			seen[[2]int{l, i}] = true
			for stack := []int{i}; len(stack) > 0; {
				j := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				c := image.Pt(j%RoomW, j/RoomW)
				box = box.Union(image.Rectangle{c, c.Add(image.Pt(1, 1))})
				for _, d := range [...]image.Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					n := c.Add(d)
					if m := n.Y*RoomW + n.X; n.In(image.Rect(0, 0, RoomW, RoomH)) && !seen[[2]int{l, m}] && kind(l, m) == k {
						seen[[2]int{l, m}] = true
						stack = append(stack, m)
					}
				}
			}
			prop := box.Min.Add(box.Max).Mul(TileSize).Div(2)
			aim := image.Pt(prop.X, box.Max.Y*TileSize+TileSize/2) // the row in front of it
			if ts.Tiles[rt.Tiles[l][i]].Flat {                     // on the floor: used from on top
				aim = prop
			}
			best, dist := image.Pt(-1, -1), 0
			for c := range RoomW * RoomH {
				cell := image.Pt(c%RoomW, c/RoomW)
				if d := mid(cell).Sub(aim); free(cell) && (best.X < 0 || d.X*d.X+d.Y*d.Y < dist) {
					best, dist = cell, d.X*d.X+d.Y*d.Y
				}
			}
			if best.X < 0 {
				continue
			}
			taken[best] = true
			s := Spot{mid(best), prop, activityVerbs[k][1]}
			if aim == prop {
				s.Prop = s.At
			}
			rt.Spots = append(rt.Spots, s)
		}
	}
}

// walkable: p (room-local px) is inside the room, over a floor tile, and outside every collision rect.
func (rt RoomType) walkable(ts *Tileset, p image.Point) bool {
	if !p.In(image.Rect(0, 0, RoomW*TileSize, RoomH*TileSize)) {
		return false
	}
	c := p.Y/TileSize*RoomW + p.X/TileSize
	if rt.Tiles[LayerFloor][c] == 0 {
		return false
	}
	local := image.Pt(p.X%TileSize, p.Y%TileSize)
	for _, layer := range rt.Tiles {
		for _, r := range ts.Tiles[layer[c]].Solid {
			if local.In(r) {
				return false
			}
		}
	}
	return true
}

func findLayer(ls []tiledLayer, name, typ string) *tiledLayer {
	for i := range ls {
		if ls[i].Name == name && ls[i].Type == typ {
			return &ls[i]
		}
	}
	return nil
}
