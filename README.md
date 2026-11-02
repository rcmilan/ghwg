# Ghost Hotel — vertical slice

Run from this directory:

```powershell
go run .
```

Controls: `WASD` moves the ghost concierge; `Space` starts a shift and performs the contextual booking/payment interaction; `Esc` returns to title (or quits from it).

The slice runs at a crisp 960×540 windowed resolution. Its strict top-down layout has three north-side rooms with south-facing doors, divided partition walls, and a central passage aligned with the south exit. At reception, use the in-world terminal (`W`/`S` and `Space`) to assign all three incoming guests before confirming bookings. One shared day clock begins once every guest reaches a room. Checkout, the brief counter payment pause, payment collection, passage traversal, and guest despawn are all automatic. Beginning the next day preserves the concierge's exact location.

The supplied Modern Interiors source pack remains untouched at the workspace root as requested. The `assets` package is the replacement boundary for future LimeZu sprite-sheet slicing; the current clean geometric presentation keeps the project runnable until those scattered files are curated. The W95FA font source is not present locally, so Ebitengine's built-in debug glyphs are used temporarily.
