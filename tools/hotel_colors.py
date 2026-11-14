"""Recolors the hotel's wall and floor tiles in place (07 F1, 2026-11-14): sage wallpaper, wood-toned floor, and
one dark wood top band on every wall, the kitchen's included. Exact colours only, so a second run changes nothing.

    python tools/hotel_colors.py
"""
import glob, os
from PIL import Image

TILES = os.path.join(os.path.dirname(__file__), '../assets/tiles/')
rgb = lambda h: ((h >> 16) & 255, (h >> 8) & 255, h & 255)
PAPER = {0xd18c6b: 0xc3cfad, 0xbe735d: 0xa3b68f, 0xb95159: 0x7f9c7c}  # the wallpaper and its two stripes
FLOOR = {0xac5c56: 0x9a6c50, 0xb57a6f: 0xb28665, 0x744141: 0x5c3e33}  # tile, light tile, seam
BAND = {0x5a5a66: 0x6b544a, 0xbdf1e5: 0x6b544a, 0x5e4a42: 0x6b544a}                       # a wall's top band; the kitchen's was light
for pattern, maps in (('wall_*', (PAPER, BAND)), ('border_*', (BAND,)), ('floor_clay*', (FLOOR,))):
    for f in glob.glob(TILES + pattern + '.png'):
        m = {rgb(a): rgb(b) for mp in maps for a, b in mp.items()}
        im = Image.open(f).convert('RGBA')
        im.putdata([m.get(p[:3], p[:3]) + (p[3],) for p in im.getdata()])
        im.save(f)
