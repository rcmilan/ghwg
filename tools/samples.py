"""Builds sound/data/*.ogg from CC0 source recordings (sources and licenses: README, Assets).

    python -I tools/samples.py <dir holding the downloaded sources>

Needs numpy and soundfile (pip install soundfile numpy). The sources are not committed; the
paths in ROWS are relative to the directory given on the command line. Naming: the MIDI note
actually sounding (checked by FFT; VSCO and VCSL label C3 = MIDI 60, an octave above the
usual, so their file names are not the MIDI number).
"""
import os
import sys

import numpy as np
import soundfile as sf

RATE = 22050
OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "sound", "data")

# (instrument, midi, source file, trim start s, length s, fade s, tune to exact pitch, shift semitones)
V = "vsco/"  # VSCO 2 Community Edition, CC0
C = "vcsl/"  # Versilian Community Sample Library, CC0
F = "fs-"  # Freesound previews, CC0
T = "BrettTenor_NV_Main_"
ROWS = [
    ("bass", 31, V + "BKCtbss_Pizz_G0_v1_rr1.wav", 0, 1.2, 0.15, False, 0),
    ("bass", 44, V + "BKCtbss_Pizz_G#1_v1_rr1.wav", 0, 1.2, 0.15, False, 0),
    ("bass", 56, V + "BKCtbss_Pizz_G#2_v1_rr1.wav", 0, 1.2, 0.15, False, 0),
    ("sax", 54, C + T + "F#2_vl2_rr1.wav", 0, 1.5, 0.1, False, 0),
    ("sax", 66, C + T + "F#3_vl2_rr1.wav", 0, 1.5, 0.1, False, 0),
    ("sax", 78, C + T + "F#4_vl2_rr1.wav", 0, 1.5, 0.1, False, 0),
    ("vibes", 57, C + "Vibes_hard_A2_v2_rr1_Main.wav", 0, 2.5, 0.4, False, 0),
    ("vibes", 67, C + "Vibes_hard_G3_v2_rr1_Main.wav", 0, 2.5, 0.4, False, 0),
    ("vibes", 77, C + "Vibes_hard_F4_v2_rr1_Main.wav", 0, 2.5, 0.4, False, 0),
    ("vibes", 88, C + "Vibes_hard_E5_v2_rr1_Main.wav", 0, 2.5, 0.4, False, 0),
    ("celesta", 68, F + "celeste-96054/s.mp3", 0, 2.5, 0.4, True, 0),
    ("celesta", 80, F + "celeste-96056/s.mp3", 0, 2.5, 0.4, True, 0),
    ("celesta", 88, F + "celeste-96051/s.mp3", 0, 2.5, 0.4, True, 0),
    ("celesta", 100, F + "celeste-96053/s.mp3", 0, 2.5, 0.4, True, 0),
    ("trumpet", 58, V + "Sum_SHTrumpet_harmonM-sus_A#2_v1_rr1.wav", 0, 1.5, 0.1, False, 0),
    ("trumpet", 70, V + "Sum_SHTrumpet_harmonM-sus_A#3_v1_rr1.wav", 0, 1.5, 0.1, False, 0),
    ("trumpet", 81, V + "Sum_SHTrumpet_harmonM-sus_A4_v1_rr1.wav", 0, 1.5, 0.1, False, 0),
    # the whistle source is one note near G6 (91); 79 is it an octave down
    ("whistle", 91, F + "vrymaa-797587/s.mp3", 0.3, 1.5, 0.1, True, 0),
    ("whistle", 79, F + "vrymaa-797587/s.mp3", 0.3, 1.5, 0.1, True, -12),
]
STEPS = [  # (source, trim start s): Freesound OwlStorm, single shoe steps on a hard floor
    (F + "owl-151222/s.mp3", 0.2),
    (F + "owl-151209/s.mp3", 0.27),
    (F + "owl-151209/s.mp3", 1.52),
]
PEAK = 10 ** (-1 / 20)  # -1 dBFS
RMS = 10 ** (-16 / 20)  # perceived level target for notes, measured over the first 0.6 s


def load(path):
    x, sr = sf.read(path, dtype="float64")
    return (x.mean(1) if x.ndim > 1 else x), sr


def resample(x, sr, shift=0.0, ratio=1.0):
    """Read x at sr * 2^(shift/12) * ratio, out at RATE: lowpass first, then linear interpolation."""
    step = sr / RATE * 2 ** (shift / 12) * ratio
    if step > 1:  # keep what the target rate can hold
        sp = np.fft.rfft(x)
        sp[int(len(sp) * 0.45 / step * 2) :] = 0
        x = np.fft.irfft(sp, len(x))
    t = np.arange(0, len(x) - 1, step)
    return np.interp(t, np.arange(len(x)), x)


def f0_near(x, midi):
    """Measured pitch of the note at the start of x, searched within 1.5 semitones of midi."""
    s = x[: RATE // 2] * np.hanning(min(len(x), RATE // 2))
    sp = np.abs(np.fft.rfft(s, 1 << 18))
    fr = np.fft.rfftfreq(1 << 18, 1 / RATE)
    nom = 440 * 2 ** ((midi - 69) / 12)
    m = (fr > nom * 2 ** (-1.5 / 12)) & (fr < nom * 2 ** (1.5 / 12))
    i = np.argmax(np.where(m, sp, 0))
    a, b, c = np.log(sp[i - 1 : i + 2] + 1e-12)
    return fr[i] + (a - c) / (2 * (a - 2 * b + c)) * (fr[1] - fr[0]), nom


def trim(x, start, length, fade):
    x = x[int(start * RATE) :]
    on = max(0, np.argmax(np.abs(x) > 0.02 * np.abs(x).max()) - int(0.005 * RATE))
    x = x[on : on + int(length * RATE)].copy()
    n = min(len(x), int(fade * RATE))
    x[len(x) - n :] *= np.linspace(1, 0, n) ** 2
    return x


def level(x, rms=True):
    pk = np.abs(x).max()
    g = PEAK / pk
    if rms:
        g = min(g, RMS / np.sqrt((x[: int(0.6 * RATE)] ** 2).mean()))
    return x * g


def reverb(x, tail=3.0):
    """Dark Freeverb-style hall: 8 damped feedback combs and 4 allpasses, about 2.5 s to -60 dB, mostly wet."""
    n = int(tail * RATE)
    x = np.pad(x, (0, n - len(x))) if len(x) < n else x[:n]
    wet = np.zeros(n)
    for d in (1116, 1188, 1277, 1356, 1422, 1491, 1557, 1617):
        d = int(d * RATE / 44100 * 1.15)
        g = 10 ** (-3 * d / RATE / 2.5)
        buf, lp, y = np.zeros(d), 0.0, np.zeros(n)
        for i in range(n):
            o = buf[i % d]
            lp = o * 0.45 + lp * 0.55  # damping: each trip darkens the tail
            buf[i % d] = x[i] + lp * g
            y[i] = o
        wet += y
    for d in (556, 441, 341, 225):
        d = int(d * RATE / 44100)
        buf, y = np.zeros(d), np.zeros(n)
        for i in range(n):
            o = buf[i % d]
            v = wet[i] + o * 0.5
            buf[i % d] = v
            y[i] = o - wet[i] * 0.5
        wet = y
    sp = np.fft.rfft(wet)  # the wet signal itself is dark too: lowpass about 3 kHz
    sp *= 1 / (1 + (np.fft.rfftfreq(n, 1 / RATE) / 3000) ** 4)
    wet = np.fft.irfft(sp, n)
    wet *= np.abs(x).max() / np.abs(wet).max() * 1.6
    out = x * 0.35 + wet
    out[-int(0.5 * RATE) :] *= np.linspace(1, 0, int(0.5 * RATE)) ** 2
    return out


def write(name, x):
    path = os.path.join(OUT, name)
    sf.write(path, x.astype("float32"), RATE, format="OGG", subtype="VORBIS", compression_level=0.35)
    print(f"{name:20s} {len(x) / RATE:4.2f}s {os.path.getsize(path):6d} B")


def main(src):
    os.makedirs(OUT, exist_ok=True)
    dry = {}
    for inst, midi, f, start, length, fade, tune, shift in ROWS:
        x, sr = load(os.path.join(src, f))
        x = resample(x, sr, shift)
        if tune:  # land exactly on the semitone (the source is a few cents off)
            f0, nom = f0_near(trim(x, start, length, 0), midi)
            print(f"  {inst}_{midi}: measured {69 + 12 * np.log2(f0 / 440):.2f}")
            x = resample(x, RATE, 0, nom / f0)
        x = trim(x, start, length, fade)
        dry[inst, midi] = x
        write(f"{inst}_{midi}.ogg", level(x))
    for (inst, midi), x in dry.items():
        if inst == "bass":
            write(f"basswet_{midi}.ogg", level(reverb(x)))
    for i, (f, start) in enumerate(STEPS, 1):
        x, sr = load(os.path.join(src, f))
        write(f"step_{i}.ogg", level(trim(resample(x, sr), start, 0.45, 0.12), rms=False) * 0.7)
    print("total", sum(os.path.getsize(os.path.join(OUT, f)) for f in os.listdir(OUT) if f.endswith(".ogg")), "B")


if __name__ == "__main__":
    main(sys.argv[1])
