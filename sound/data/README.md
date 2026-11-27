# sound/data

Embedded into the game (`sound.Data`). Every file is mono Ogg Vorbis at 22050 Hz (the mixer's `Rate`; other rates are resampled at load).

| File | What |
|---|---|
| `<instrument>_<midi>.ogg` | One note of an instrument, its pitch the MIDI note number (`sax_62.ogg` is a D4). Instruments: `bass`, `basswet` (the bass with long reverb: the Concierge in ghost), `sax`, `vibes`, `celesta`, `trumpet`, `whistle`. About one file per octave of each voice's range. **Not shipped while the game has no music** (10, 2026-11-27): the music PR (#16) brings them back; `python -I tools/samples.py <downloads>` cuts them. Without them every instrument plays a sine placeholder. |
| `step_<n>.ogg` | A footstep, `n` from 1. |

Sources and licenses: DEVELOPMENT.md's Assets section.
