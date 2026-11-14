"""Cuts the half-width wall faces (07 H1): under an odd count of Room columns the Rooms part ends half a cell
off the hall's grid, so the wall beside the hall ends on half a tile. Each is half of a hotel wall face, on
the cell's east side (`wall_halfw_*`, the wall's west end) or its west side (`wall_halfe_*`). Listed in
assets/tiles/hotel.tsj after the existing tiles; tiles already listed are left alone.

    python tools/half_tiles.py
"""
import copy, json, os
from PIL import Image

TILES = os.path.join(os.path.dirname(__file__), '..', 'assets/tiles/')
T, H = 16, 8
cuts = {}  # new tile -> (image, the listed tile it copies its collision and properties from)
for half in ('top', 'bottom'):
    for name, like, src, dst in (('halfw', 'left', 0, H), ('halfe', 'right', H, 0)):
        im = Image.new('RGBA', (T, T))
        im.paste(Image.open(TILES + 'wall_%s_%s.png' % (like, half)).convert('RGBA').crop((src, 0, src + H, T)), (dst, 0))
        cuts['wall_%s_%s' % (name, half)] = (im, 'wall_%s_%s' % (like, half))

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
