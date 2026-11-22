package game

import (
	"image/png"
	"testing"

	"ghosthotel/assets"
)

// R20: the Wreath, the crate, the Storm jar, Gerald and the Tip jar each have their own 16x16 icon, not the key's.
func TestShopIconsOwnSprites(t *testing.T) {
	want := map[Unlock]string{rowWreath: "icon_wreath", rowCrate: "icon_crate", rowStorm: "icon_storm", rowGerald: "icon_gerald",
		shop[upRow[TipJar]].ID: "icon_tipjar", rowKey: "icon_key"}
	for _, it := range shop {
		name, ok := want[it.ID]
		if !ok {
			continue
		}
		delete(want, it.ID)
		if it.Icon != name {
			t.Errorf("%s: icon %q, want %q", it.Name, it.Icon, name)
		}
		f, err := assets.FS.Open("sprites/" + name + ".png")
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(f)
		f.Close()
		if err != nil || name != "icon_key" && (cfg.Width != 16 || cfg.Height != 16) {
			t.Errorf("%s: %dx%d, %v", name, cfg.Width, cfg.Height, err)
		}
	}
	if len(want) > 0 {
		t.Errorf("rows not in the shop: %v", want)
	}
}
