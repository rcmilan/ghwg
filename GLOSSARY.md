# Ghost Hotel

A hotel-management game: a concierge books guests into rooms, waits out the day, and collects payment at checkout.

## Hotel

**Room**:
One of the guest-room slots in the hotel's grid, numbered by grid row and column: the first row is 101, 102, …, the second row 201, 202, …
_Avoid_: slot, ROOM A

**Room Type**:
The template a Room is built from: its look, its rate, and (later) its props. Authored in Tiled.
_Avoid_: prefab (in player-facing text), room template

**Rate**:
What a guest pays at checkout for one stay in a Room: the Rate of the Room's Room Type when the Booking was made.
_Avoid_: price, fee

**Stay Spot**:
The point inside a Room where its guest stands during the Day.

**Lobby**:
The common area between the Hall Wall and the Entrance, where the Reception Counter stands.

**Corridor**:
Walkable common space between Rooms and between the Rooms and the Lobby.
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
The desk in the Lobby where Guests pay; the Concierge can stand behind it.
_Avoid_: desk, reception

**Terminal**:
The computer on the Reception Counter where the Concierge books Guests into Rooms.

**Booking**:
Assigning a Guest to a Room at the Terminal, before the Guest arrives.

**Vacant**:
A Room with no checked-in guest; the concierge may enter it.

**Occupied**:
A Room whose guest has checked in; its door is locked against the concierge entering (never against leaving).
_Avoid_: locked (as a room state)

## People

**Concierge**:
The living hotel worker the player controls.
_Avoid_: player (in game text), ghost

**Ghost Mode**:
A future state of the Concierge with its own look and rules; not defined yet.

**Guest**:
A visitor booked into one Room from Shift 1 until they leave in Shift 2.

## Time

**Run**:
One playthrough, from the title screen until returning to it. Money resets and a Spin happens when a Run starts.
_Avoid_: game, session

**Shift**:
Half of the hotel's loop within a Run. **Shift 1** is Booking → Arrivals; **Shift 2** is Day → Departures (guest care). After Shift 2 comes a Spin, then Shift 1 again.

**Spin**:
The moment every Room re-draws its Room Type at random, like the reels of a slot machine. It happens when a Run starts and whenever the last Guest leaves the building at the end of Shift 2. An occupied Room keeps its Room Type.
_Avoid_: reroll, shuffle

**Day**:
The timed phase of a Shift during which guests stay in their Rooms.
