package world

import (
	"encoding/json"
	"fmt"
	"image"
	"io/fs"
	"slices"
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
	NumLayers
)

var layerNames = [NumLayers]string{"floor", "walls", "decor"}

// RoomType is the template a Room is built from, authored in Tiled (assets/rooms/*.tmj).
type RoomType struct {
	Name    string
	Rate    int
	Tiles   [NumLayers][]TileID // RoomW*RoomH each, row-major
	Stay    image.Point         // Stay Spot, room-local px
	DoorCol int
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
		if len(types) > 0 && rt.DoorCol != types[0].DoorCol { // D45: Spins never move doors
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

	// exactly one "stay" point, on walkable floor
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
	if len(stays) != 1 {
		return rt, fmt.Errorf("found %d \"stay\" points in the markers layer, want exactly 1", len(stays))
	}
	rt.Stay = stays[0]
	if !rt.walkable(ts, rt.Stay) {
		return rt, fmt.Errorf("\"stay\" point at (%d,%d) px is not on walkable floor", rt.Stay.X, rt.Stay.Y)
	}
	return rt, nil
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
