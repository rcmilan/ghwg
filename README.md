# Ghost Hotel — vertical slice

Run from this directory:

```powershell
go run .
```

Controls: `WASD` moves the concierge; `Space` opens the reception terminal (behind or in front of the counter) and closes it, and at the mirror on the lobby wall shifts the whole hotel between the real and the ghost dimension; `Esc` closes the terminal, returns to title, or quits from it. Hold `T` to run the clock ×10 (dev aid).

How it plays: the hotel runs on a real-time clock, one in-game day in about 4 minutes, starting on Day 1 at 08:00. The hotel has two rooms side by side. Guests arrive already booked through an app, from the street during the day (none between 02:00 and 06:00, most from 14:00 to 22:00), and walk straight to their room; one comes only while a room it fits is empty. A party is one guest or a pair; a pair walks together, the second one a step behind, and needs a twin or double room. Rooms are Standard or Deluxe, with a single bed ($25 / $40 a night) or twin or double beds ($40 / $64), drawn when the run starts. The terminal pauses the world and shows each room's status, the money and the clock. Each guest stays 1–3 nights and is a homebody, who stays in, or a tourist, who goes out each morning (08:00–10:00) and comes back at night (19:00–22:00); the room stays booked while they're out, and the concierge can never enter a booked room. Once the whole party is inside with the door shut, the room fades into darkness; it clears only while its door is open, and stays dark while a tourist is out. The light follows the clock: clear by day, warm at dusk, deep blue at night under the corridors' ceiling lights, and pink at dawn. The lobby stays lit except from 01:00 to 06:00, and an empty room lights up while the concierge is in it. Your money shows on the terminal. On the last morning, between 10:00 and 12:00, the guest checks out: the whole stay is paid as it steps out of its door (`+$X`, rate × nights) and the room is free again.

The supplied Modern Interiors source pack remains untouched at the workspace root as requested. The `assets` package is the replacement boundary for future LimeZu sprite-sheet slicing; the current clean geometric presentation keeps the project runnable until those scattered files are curated. The W95FA font source is not present locally, so Ebitengine's built-in debug glyphs are used temporarily.

## Dev tools

Simulation mode plays many Runs headless and in parallel, each by a Strategy that uses only the keys a player has, and prints money per day, occupancy, Bookings and invariant failures per Strategy. `idle` (stand at the terminal) is the only one until haunting gives the concierge a decision:

```powershell
go run ./cmd/sim -strategy idle -runs 100 -days 7 -grid 2x1 -seed 1 -react 30 -keygap 6 -csv out.csv
```

`-react` and `-keygap` set the Pilot's pace in frames (`0` `0` is frame-perfect), `-nocheck` skips the invariants, `-csv` writes one row per Run. Run `i` uses seed `seed+i` for every Strategy. Any row replays on screen with `go run . -play idle -seed 7` (watch mode).
