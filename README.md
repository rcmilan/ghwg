# Ghost Hotel — vertical slice

Run from this directory:

```powershell
go run .
```

Controls: `WASD` moves the concierge; `Space` opens the reception terminal (behind or in front of the counter), picks a room and presses `OK`; `Esc` closes the terminal, returns to title, or quits from it. Hold `T` to run the clock ×10 (dev aid).

How it plays: the hotel runs on a real-time clock, one in-game day in about 4 minutes, starting on Day 1 at 08:00. Guests walk in from the street during the day (none between 02:00 and 06:00, most from 14:00 to 22:00) and queue at reception, at most 3 parties. A party is one guest or a pair; a pair walks together, the second one a step behind, and wishes for a twin or a double room. At the terminal the world pauses: once the guest at the front stands at the counter, pick a room on the grid with `WASD` and `Space` (pick again to change it), then confirm on `OK` to book; the guest walks to its room. Rooms are Standard or Deluxe, with a single bed ($25 / $40 a night) or twin or double beds ($40 / $64). A pair doesn't fit a single, and a pair booked into the other two-bed layout than the one it wished for pays half. A guest left waiting too long gives up and walks out. Each guest stays 1–3 nights (the terminal shows it) and is a homebody, who stays in, or a tourist, who goes out each morning (08:00–10:00) and comes back at night (19:00–22:00); the room stays booked while they're out, and the concierge can never enter a booked room. Once the whole party is inside with the door shut, the room fades into darkness, and it clears as they open the door to leave. On the last morning, between 10:00 and 12:00, the guest checks out: the whole stay is paid as it steps out of its door (`+$X`, rate × nights) and the room is free again. At midnight every empty room re-draws its type in the Spin; a booked room, or the one the concierge stands in, keeps its type.

The supplied Modern Interiors source pack remains untouched at the workspace root as requested. The `assets` package is the replacement boundary for future LimeZu sprite-sheet slicing; the current clean geometric presentation keeps the project runnable until those scattered files are curated. The W95FA font source is not present locally, so Ebitengine's built-in debug glyphs are used temporarily.
