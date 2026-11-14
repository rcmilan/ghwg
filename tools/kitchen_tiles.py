"""Cuts the kitchen's wall and floor tiles (07 F1) from Room_Builder_free_16x16.png, recolors them by
brightness (the wall to the sheet's own teal, the floor to the Interiors sheet's white ramp) and lists them, with the 6-px face that ends the kitchen's west wall, in
assets/tiles/hotel.tsj after the existing tiles. Tiles already listed are left alone.

    python tools/kitchen_tiles.py [sheet]
"""
import copy, json, os, sys
from PIL import Image, ImageOps

ROOT = os.path.join(os.path.dirname(__file__), '..')
SHEET = sys.argv[1] if len(sys.argv) > 1 else os.path.join(
    ROOT, '../Modern_Interiors_Free_v2.2/Modern tiles_Free/Interiors_free/16x16/Room_Builder_free_16x16.png')
TILES = os.path.join(ROOT, 'assets/tiles/')
T = 16
lum = lambda c: 0.3*c[0] + 0.59*c[1] + 0.11*c[2]
rgb = lambda h: ((h >> 16) & 255, (h >> 8) & 255, h & 255, 255)


def colors(im):
    return sorted({c for c in im.getdata() if c[3] == 255 and lum(c) >= 90}, key=lum)  # outlines stay


def ramp(im, to):
    src = colors(im)
    m = {c: to[round(i*(len(to)-1)/max(1, len(src)-1))] for i, c in enumerate(src)}
    out = im.copy()
    out.putdata([m.get(c, c) for c in im.getdata()])
    return out


rb = Image.open(SHEET).convert('RGBA')
wall = ramp(rb.crop((0, 272, 48, 304)), colors(rb.crop((176, 144, 224, 176))))
floor = ramp(rb.crop((176, 112, 224, 144)), [rgb(h) for h in (0xb2aecb, 0xc6bdd5, 0xd8d0e0, 0xebe4f2, 0xf8f8f8)])
cuts = {}  # new tile -> (image, the listed tile it copies its collision and properties from)
for c, side in enumerate(('left', 'mid', 'right')):
    for r, half in enumerate(('top', 'bottom')):
        cuts['wall_kitchen_%s_%s' % (side, half)] = (wall.crop((c*T, r*T, c*T+T, r*T+T)), 'wall_%s_%s' % (side, half))
for name, (c, r) in {'': (1, 1), '_n': (1, 0), '_w': (0, 1), '_nw': (0, 0)}.items():  # the shadow under a wall
    cuts['floor_kitchen' + name] = (floor.crop((c*T, r*T, c*T+T, r*T+T)), 'floor_clay')

for half, like in (('top', 'wall_single_top'), ('bottom', 'border_east')):  # a face as wide as a side line (6 px), in
    src = Image.open(TILES + 'wall_single_%s.png' % half).convert('RGBA')      # the hotel's wallpaper: where the
    stub = Image.new('RGBA', (T, T))                                          # kitchen's west wall ends at its doorway
    stub.paste(src.crop((0, 0, 5, T)), (0, 0))
    stub.paste(src.crop((T-1, 0, T, T)), (5, 0))
    cuts['wall_stub_' + half] = (ImageOps.mirror(stub), like)  # on the cell's east side: the kitchen's west wall

path = TILES + 'hotel.tsj'
with open(path, newline='', encoding='utf-8') as f:
    ts = json.load(f)
by = {t['image'][:-4]: t for t in ts['tiles']}
for name, (im, like) in cuts.items():
    if name in by:
        continue
    im.save(TILES + name + '.png')
    t = copy.deepcopy(by[like])
    t['id'], t['image'] = max(x['id'] for x in ts['tiles']) + 1, name + '.png'
    ts['tiles'].append(t)
ts['tilecount'] = len(ts['tiles'])
with open(path, 'w', newline='\n', encoding='utf-8') as f:
    json.dump(ts, f, indent=1)
    f.write('\n')
