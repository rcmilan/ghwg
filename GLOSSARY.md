# Ghost Hotel

A hotel-management game. The Concierge books walk-in Parties into Rooms on a real-time clock. Each Party pays when it leaves for good.

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
The computer on the Reception Counter. Here the Concierge books the front Party of the Queue into a Room and sees every Room's status and the Clock. The world pauses while it is open.

**Queue**:
The waiting Walk-ins at the Reception Counter: 3 parallel lanes, one Party each. The Party that arrived first is booked first.

**Booking**:
Assigning a Party to a Room at the Terminal. It is instant and final, and lasts until the Party checks out.

**Vacant**:
A Room with no Booking; the Concierge may enter it.

**Occupied**:
A Room with a Booking, from the moment it is booked until its Party checks out. Its door opens only for its own Party, even while that Party is out.
_Avoid_: locked (as a room state)

**Fog**:
The dark veil over an Occupied Room's interior while its Party is inside. It fades in and out.
_Avoid_: fog of war (in player-facing text)

## People

**Concierge**:
The living hotel worker the player controls.
_Avoid_: player (in game text), ghost

**Ghost Mode**:
A future state of the Concierge with its own look and rules; not defined yet.

**Guest**:
One member of a Party.

**Party**:
1–2 Guests on one Booking. A Party arrives, queues, stays, keeps one Routine and pays together. Its leader finds the way and the follower trails it.
_Avoid_: group

**Walk-in**:
A Party that arrives at the hotel without a Booking and joins the Queue.

**Patience**:
How long a queued Party waits before it walks back out. Hidden from the player.

**Routine**:
A Party's daily schedule. **Homebody** stays in the Room. **Tourist** goes out each morning and returns at night.

**Bed Wish**:
A Party of 2's choice of Twin or Double. Booking the other 2-person layout pays less.

## Time

**Run**:
One playthrough, from the title screen until returning to it. A Run starts on Day 1 at 08:00 with an instant Spin and every Room Vacant. Money resets, and the Run never ends on its own.
_Avoid_: game, session

**Clock**:
The real-time in-game time: one day lasts about 4 minutes. It drives Walk-ins, Routines and the lighting, and it pauses only for the Terminal and the Spin. The player can read it only on the Terminal.

**Day**:
One 24-hour turn of the Clock, counted from Day 1.

**Midnight**:
00:00 on the Clock, when the Spin happens.

**Spin**:
The moment Rooms re-draw their Room Type at random, like the reels of a slot machine. It happens when a Run starts and every Midnight. At Midnight, only Vacant Rooms flip, and never the Room the Concierge stands in.
_Avoid_: reroll, shuffle

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
A rule that plays a Run in place of a human: which Room to book, when to act. It may do only what a player can do, and sees only the View unless it declares itself a cheat.

**Pilot**:
What turns a Strategy's intent into key presses at a human pace: walking the Concierge, opening the Terminal, moving its cursor. The only way a Strategy touches the game.

**View**:
Everything a player can see at one moment: the Terminal's readout, the guests on screen, the Fog, the Clock. The Strategy's input.
_Avoid_: state, snapshot
