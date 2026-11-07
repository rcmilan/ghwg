# Ghost Hotel

A haunted-hotel management game on a real-time clock. Parties arrive already booked and pay when they leave for good. The Concierge, a living host who can see ghosts, Shifts the whole hotel between the real and the ghost Dimension.

## Hotel

**Room**:
One of the guest-room slots in the hotel's grid. Rooms are numbered by floor and column, from the front: the row nearest the Entrance is 101, 102, …, the next 201, 202, …, and the back row the highest.
_Avoid_: slot, ROOM A

**Room Type**:
The template a Room is built from: a tier (Standard, Deluxe) times a Bed Layout. It sets the Room's look, Rate, props and Stay Spots. Authored in Tiled.
_Avoid_: prefab (in player-facing text), room template

**Bed Layout**:
A Room Type's arrangement of beds: **Single** (1 bed, 1 Guest), **Twin** (2 beds) or **Double** (1 double bed for 2 Guests).

**Capacity**:
How many Guests a Room holds: the number of Stay Spots its Room Type has.

**Rate**:
What a Party pays per night in a Room. It comes from the Room Type when the Booking is made: 2-person Bed Layouts cost 1.6 × the tier's Single Rate.
_Avoid_: price, fee

**Stay Spot**:
A point inside a Room where one Guest stays, one per bed.

**Lobby**:
The common area between the Hall Wall and the Entrance, where the Reception Counter stands.

**Corridor**:
Walkable common space between Rooms and between the Rooms and the Lobby. Corridors are 2 cells wide.
_Avoid_: hallway, passage

**Hall Wall**:
The wall separating the Corridors from the Lobby.

**Lobby Archway**:
The open gap in the Hall Wall that joins the Lobby to the Corridors.
_Avoid_: passage, hall door

**Entrance**:
The hotel's sliding glass front door, between the Lobby and the Street.
_Avoid_: exit

**Street**:
The strip outside the Entrance where Guests appear and leave.

**Reception Counter**:
The desk in the Lobby that holds the Terminal; the Concierge can stand behind it.
_Avoid_: desk, reception

**Terminal**:
The computer on the Reception Counter. Here the Concierge reads every Room's status, the money and the Clock. The world pauses while it is open.

**Mirror**:
The mirror on the Hall Wall, west of the Lobby Archway. SPACE in front of it Shifts the hotel.

**Booking**:
A Party's stay in a Room, made by its app before it arrives: the hotel gives it the first Vacant Room it fits. It lasts until the Party checks out or flees.

**Vacant**:
A Room with no Booking; the Concierge may enter it.

**Occupied**:
A Room with a Booking, from the moment its Party arrives until it checks out or flees. Its door opens only for its own Party, even while that Party is out.
_Avoid_: locked (as a room state)

**Fog**:
The dark veil over an Occupied Room's interior while its Party is inside, in the real Dimension only; the ghost Dimension sees through it. It fades in and out.
_Avoid_: fog of war (in player-facing text)

## Dimensions

**Dimension**:
The state the whole hotel is in: **real** (the hotel as Guests see it) or **ghost** (the same hotel in an arcade palette, where the Fog hides nothing). The hotel runs the same in both; only what is drawn and what the Concierge can do differ.
_Avoid_: mode, world, ghost mode

**Shift**:
Flipping the whole hotel to the other Dimension, at the Mirror, at any time. The game's one meaning of the word.
_Avoid_: flip (in player-facing text), toggle

## Haunting

**Haunt**:
What the Concierge does to an Occupied Room in the ghost Dimension, with SPACE in front of its door. A haunt lasts an hour of the Clock; haunting again restarts it.
_Avoid_: scare (as the verb), attack

**Poltergeist**:
The haunt there is so far: the Room's furniture shakes, and its Guests gain Fear while they're inside.

**Fear**:
How frightened a Guest is. It rises while the Guest is inside a haunted Room and never falls. It shows only in the ghost Dimension, as a red glow.

**Flee**:
A Party leaving for good, unpaid, the moment one of its Guests reaches the Fear limit. Its Room is Vacant at once.
_Avoid_: escape, check out (a fled Party never checks out)

**Spook**:
The currency haunting earns: one point for each point of Fear a haunt adds. Shown in the ghost Dimension, on the Terminal and as cyan "+N" over the door. It buys nothing yet.
_Avoid_: scare points, ectoplasm

**Room Spook**:
How haunted a Room is: it rises by the Fear each haunt adds to its Guests and never falls on its own. In the ghost Dimension the whole Room (not its Guests) shifts toward green and purple with it, and pulses once Closed; in the real Dimension it tints faintly green. Not the Spook currency, which the hotel keeps when a Room is cleaned.

**Closed**:
A Room at the Room Spook limit. No new Party comes to it; a Party already in it stays until its checkout. The Terminal shows it in both Dimensions.
_Avoid_: blocked, locked

**Priest**:
A visitor the Concierge calls and pays for at the Terminal, one per Room at a time. He walks in from the Street, works half an hour in the Room, lowers its Room Spook and leaves. A **Weak** Priest halves it; a **Strong** one clears it.
_Avoid_: exorcist, cleaner

## People

**Concierge**:
The living host the player controls. Guests arrive already booked; the Concierge looks after them.
_Avoid_: player (in game text), ghost

**Guest**:
One member of a Party.

**Party**:
1–2 Guests on one Booking. A Party arrives, stays, keeps one Routine and pays together. Its leader finds the way and the follower trails it.
_Avoid_: group

**Walk-in**:
A Party arriving from the Street, already booked, and walking straight to its Room. One comes only while a Room it fits is Vacant.

**Routine**:
A Party's daily schedule. **Homebody** stays in the Room. **Tourist** goes out each morning and returns at night.

## Time

**Run**:
One playthrough, from the title screen until returning to it. A Run starts on Day 1 at 08:00 with every Room Vacant and given its Room Type for the whole Run. Money resets, and the Run never ends on its own.
_Avoid_: game, session

**Clock**:
The real-time in-game time: one day lasts about 4 minutes. It drives Walk-ins, Routines and the lighting, and it pauses only for the Terminal. The player can read it only on the Terminal.

**Day**:
One 24-hour turn of the Clock, counted from Day 1.

**Checkout**:
A Party leaving its Room for good on its final Day. It pays Rate × nights at the Room's door, the money is banked at once, and the Room becomes Vacant.

## Light

**Lightmap**:
The darkness layer over the hotel. Its tint follows the Clock, and Light Sources cut through it.

**Light Source**:
A lamp, ceiling light, street lamp or Terminal screen that cuts a glow out of the Lightmap at night.

## Development

**Simulation**:
Many Runs played headless and in parallel by Strategies, to compare outcomes across seeds. A development tool, not part of the game.

**Strategy**:
A rule that plays a Run in place of a human: where to stand, when to act. It may do only what a player can do, and sees only the View unless it declares itself a cheat.

**Pilot**:
What turns a Strategy's intent into key presses at a human pace: walking the Concierge, opening and closing the Terminal. The only way a Strategy touches the game.

**View**:
Everything a player can see at one moment: the Terminal's readout, the guests on screen, the Fog, the Clock. The Strategy's input.
_Avoid_: state, snapshot
