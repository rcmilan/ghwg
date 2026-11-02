package world

import (
	"encoding/json"
	"fmt"
	"image"
	"io/fs"
	"math"
	"path"
	"strings"
)

// Minimal Tiled JSON (.tsj/.tmj) reading, stdlib only: just the fields the hotel uses.

const TileSize = 16

type tiledProp struct {
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}

type tiledObject struct {
	Name   string  `json:"name"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Point  bool    `json:"point"`
}

type tiledTileset struct {
	TileWidth  int `json:"tilewidth"`
	TileHeight int `json:"tileheight"`
	Tiles      []struct {
		ID          int    `json:"id"`
		Image       string `json:"image"`
		ImageWidth  int    `json:"imagewidth"`
		ImageHeight int    `json:"imageheight"`
		ObjectGroup *struct {
			Objects []tiledObject `json:"objects"`
		} `json:"objectgroup"`
		Properties []tiledProp `json:"properties"`
	} `json:"tiles"`
}

type tiledLayer struct {
	Name     string        `json:"name"`
	Type     string        `json:"type"`
	Width    int           `json:"width"`
	Height   int           `json:"height"`
	Encoding string        `json:"encoding"`
	Data     []uint32      `json:"data"`
	Objects  []tiledObject `json:"objects"`
}

type tiledMap struct {
	Width      int         `json:"width"`
	Height     int         `json:"height"`
	TileWidth  int         `json:"tilewidth"`
	TileHeight int         `json:"tileheight"`
	Properties []tiledProp `json:"properties"`
	Tilesets   []struct {
		FirstGID int `json:"firstgid"`
	} `json:"tilesets"`
	Layers []tiledLayer `json:"layers"`
}

// prop decodes the property `name` into dst; false when it is absent.
func prop(ps []tiledProp, name string, dst any) (bool, error) {
	for _, p := range ps {
		if p.Name == name {
			if err := json.Unmarshal(p.Value, dst); err != nil {
				return true, fmt.Errorf("property %q: %w", name, err)
			}
			return true, nil
		}
	}
	return false, nil
}

// TileID indexes Tileset.Tiles; 0 is an empty cell.
type TileID uint16

type Tile struct {
	Name  string            // image file name without extension, e.g. "wall_mid_top"
	Image string            // path inside the asset FS
	Solid []image.Rectangle // collision rects, tile-local px
	Base  int               // cells down to the object's footprint (y-sort)
	Door  bool              // part of a sliding door; door state decides collision
	Flat  bool              // lies on the floor (rugs): drawn under any actor standing on it
}

type Tileset struct {
	Tiles  []Tile // indexed by TileID = Tiled tile id + 1; Tiles[0] is the empty cell
	byName map[string]TileID
}

// ID looks a tile up by name.
func (ts *Tileset) ID(name string) (TileID, bool) {
	id, ok := ts.byName[name]
	return id, ok
}

// LoadTileset reads an image-collection .tsj whose images are all TileSize×TileSize.
func LoadTileset(fsys fs.FS, file string) (*Tileset, error) {
	data, err := fs.ReadFile(fsys, file)
	if err != nil {
		return nil, err
	}
	var raw tiledTileset
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if raw.TileWidth != TileSize || raw.TileHeight != TileSize {
		return nil, fmt.Errorf("%s: tiles are %dx%d, want %dx%d", file, raw.TileWidth, raw.TileHeight, TileSize, TileSize)
	}
	ts := &Tileset{byName: map[string]TileID{}}
	for _, t := range raw.Tiles {
		if t.ImageWidth != TileSize || t.ImageHeight != TileSize {
			return nil, fmt.Errorf("%s: %s is %dx%d, want %dx%d", file, t.Image, t.ImageWidth, t.ImageHeight, TileSize, TileSize)
		}
		id := TileID(t.ID + 1)
		for len(ts.Tiles) <= int(id) { // ids can have gaps after edits in Tiled
			ts.Tiles = append(ts.Tiles, Tile{})
		}
		name := strings.TrimSuffix(path.Base(t.Image), path.Ext(t.Image))
		tile := Tile{Name: name, Image: path.Join(path.Dir(file), t.Image)}
		if t.ObjectGroup != nil {
			for _, o := range t.ObjectGroup.Objects {
				r := func(v float64) int { return int(math.Round(v)) }
				tile.Solid = append(tile.Solid, image.Rect(r(o.X), r(o.Y), r(o.X+o.Width), r(o.Y+o.Height)))
			}
		}
		if _, err := prop(t.Properties, "base", &tile.Base); err != nil {
			return nil, fmt.Errorf("%s: %s: %w", file, t.Image, err)
		}
		if _, err := prop(t.Properties, "door", &tile.Door); err != nil {
			return nil, fmt.Errorf("%s: %s: %w", file, t.Image, err)
		}
		if _, err := prop(t.Properties, "flat", &tile.Flat); err != nil {
			return nil, fmt.Errorf("%s: %s: %w", file, t.Image, err)
		}
		ts.Tiles[id] = tile
		ts.byName[name] = id
	}
	return ts, nil
}
