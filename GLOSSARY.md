# Weird Guests

A haunted-hotel management game on a real-time clock. Parties arrive already booked and pay when they leave for good. The Concierge, a living host who can see ghosts, Shifts the whole hotel between the real and the ghost Dimension.

## Hotel

**Room**:
One of the guest-room slots in the hotel's grid. Rooms are numbered by floor and column, from the front: the row nearest the Entrance is 101, 102, …, the next 201, 202, …, and the back row the highest.
_Avoid_: slot, ROOM A

**Room Type**:
The template a Room is built from: a tier (Standard, Deluxe) times a Bed Layout. It sets the Room's look, Rate, props and Stay Spots. Authored in Tiled. Every Room opens as the Standard Single; a Room Change gives it another.
_Avoid_: prefab (in player-facing text), room template

**Bed Layout**:
A Room Type's arrangement of beds: **Single** (1 bed, 1 Guest), **Twin** (2 beds) or **Double** (1 double bed for 2 Guests).

**Capacity**:
How many Guests a Room holds: the number of Stay Spots its Room Type has.

**Rate**:
What a Party pays per night in a Room. It comes from the Room Type when the Booking is made: 2-person Bed Layouts cost 1.6 × the tier's Single Rate.
_Avoid_: price, fee

**Stay Spot**:
A point inside a Room where one Guest stays, one per bed, at its foot. From 23:00 to 07:00 the Guest sleeps in that bed, getting in and out by the bed's side, never over its foot.

**Activity Spot**:
Where a Guest stands to use one of its Room's props (a window, the TV, a bookshelf, a wardrobe, a drawer, the mirror, a cushion), facing it. Found from the Room Type's tiles: the free floor cell in front of the prop, or on it for a cushion.

**Shadow Person**:
A ghost that lives on a Room's walls. While its Room's Guests are inside it runs along the back wall from one side wall to the other, more often the lower the Room's Mood, frightening them (never into fleeing). A Strong Priest banishes it. The Terminal no longer sells one. A strong ghost Guest switches an area's Shadow People on (see Area Shadow).
_Avoid_: shadow ghost

**Lobby**:
The common area between the Hall Wall and the Entrance, where the Reception Counter stands.

**Hall**:
The Lobby and the Kitchen together, with their walls: the front part of the hotel. It is always as wide as two Room columns and never grows. It stands centered under the Rooms to the pixel, so the middle of the Entrance, the middle of the Lobby Archway and the middle of the Rooms are one line; under an odd count of Room columns the middle Room's door stands on that line, and the Hall then sits half a cell off the Rooms' grid. Where the Rooms are wider than the Hall, their south wall closes the Corridor beside it and the Street comes up to that wall.
_Avoid_: entrance hall (in code and docs), foyer

**Corridor**:
Walkable common space between Rooms and between the Rooms and the Lobby. Corridors are 2 cells wide.
_Avoid_: hallway, passage

**Hall Wall**:
The wall separating the Corridors from the Lobby.

**Top Band**:
The dark wood strip where a wall meets the ceiling, the same on every wall, the Kitchen's included. Where walls meet it is one shape, outlined along its outer edge only. It is drawn from where the walls stand, not from tile art.

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
The computer on the Reception Counter, a web store: the Concierge buys Supplies and hotel-wide unlocks there through a cart. It covers the whole screen; the world pauses while it is open; ESC closes it in one press. Its zone is the computer's own column. Its bar shows the Income rates; its footer the run timer (`RUN h:mm:ss`, real time played this Run, pauses and the title not counted, saved at each dawn) and the Clock.

**Teaser**:
A store item not on sale yet, listed with SOON; its detail shows its Hint, what unlocks it.

**Income rate**:
Money or Ectoplasm earned per real minute, averaged over the last game hour; shown only in the Terminal's bar.

**Badge**:
The steady `!` by the Terminal's screen: something on sale and affordable that the open Terminal hasn't shown yet this Run Drawn only where the Terminal is on screen, never pinned to the screen's edge.

**Reserved Room**:
Madame's Room while O3 or O10 still needs Magda or Magenta moved into it: no walk-in books it, no other Party is offered it, a retained Father K keeps out of any Room a quest needs, and Matchmaking never moves anyone into it. A closed Room is still offered to the quest guest.

**Hotel view**:
The whole hotel seen from afar: SPACE, wherever it has nothing else to do, pulls the camera back until every Room shows, each under its number and a Room Change on its way, while the world runs on in slow motion (a quarter speed). WASD picks a Room (the camera moves half way in); SPACE opens its Room ring. Everything bought for one Room is bought there and paid at once, with no cart. ESC steps back one level: beds, ring, Room, hotel, play. A Room not bought yet lies under a dark veil there.
_Avoid_: ROOMS tab, SERVICES tab, room picker, map screen

**Room ring**:
The radial menu of a Room's actions in the Hotel view, one fixed slot each: Change type on top (a boarded Room's key, alone), Deluxe right, the Room mirror lower right, Father M lower left. Only the actions that apply show; it opens on the top slot, or the next clockwise. Change type opens an inner ring of the other unlocked beds.
_Avoid_: action card, context menu

**Pause**:
ESC with nothing open: the world waits on Resume / Quit to title. ESC again, or SPACE on Resume, goes on; only SPACE on Quit ends the Run.

**Mirror**:
The mirror on the Hall Wall, 2 cells west of the Lobby Archway, behind the Reception Counter. SPACE in front of it Shifts the hotel, through one of nine random transitions (0.8 s) while the world keeps running.

**Booking**:
A Party's stay in a Room, made by its app before it arrives: the hotel gives it the first Vacant Room it fits. It lasts until the Party checks out or flees.

**Vacant**:
A Room with no Booking.

**Occupied**:
A Room with a Booking, from the moment its Party arrives until it checks out or flees. Its door opens to its own Party and to the staff (the Concierge, the Courier, a Priest), never to another Room's Guests. The Concierge's minutes inside with its Guests cost the Room Mood.
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

**Fear**:
How frightened a Guest is. It rises when the Guest sees a Shadow Person or the Concierge taps the Room (Poltergeist tap) and never falls. It shows only in the ghost Dimension, as a red glow. Each Guest has its own **Flee At** limit (100 by default).

**Poltergeist tap**, **the tap**:
The Concierge's one manual fright (09, 2026-11-22; the old Hex, back as a verb, not a store item). In the ghost Dimension SPACE on a floor tile touching a Room's outer walls from outside (any side, the door's included), with human Guests inside and nothing else for SPACE to do there: one prop in the Room (the one nearest the Concierge, the one that glows) shakes once and each human Guest inside takes a little Fear (`tapFear`, 2; a Shadow sighting is 10), and its Ectoplasm follows the same Fear rule as a Shadow's. One tap every 3 s at most (`tapCooldown`); a sooner press does nothing, with no cue. Unlike a Shadow it may push a Guest to its Flee At, so tapping too much loses the rent. Refused in the real Dimension, with no human inside or with every human already at Flee At; ghost Guests are never frightened. The Shadow Person later does the same by itself. Not the **Ghost kind** Poltergeist (the red band of the Phantasm scale).
_Avoid_: hex, haunt (a ghost's own), poke

**Flee**:
A Party leaving for good, unpaid, the moment one of its Guests reaches its Flee At. Its Room is Vacant at once. Its Guests hop once in place, screaming, then run out at triple speed.
_Avoid_: escape, check out (a fled Party never checks out)

**Ectoplasm**:
The currency Fear and ghost cleaning earn: one point for each point of Fear a Shadow adds, and half the spiritual dirt cleaned in the ghost Dimension. Shown in the ghost Dimension, on the Terminal and as "+N" over the door. It pays part of some Hotel unlocks. (Called Spook before 07.)

**Room Mood**:
One number per Room, from -100 (dread) to 100 (cozy), the Room axis. It moves only on events, never drifts: a perfect serve raises it by 25 (less as the request nears expiry), an expired request lowers it by 10, Guests' mess, the Concierge lingering inside (1 Mood every 10 game minutes past the first 30 of a visit: a clean is not lingering), a fright (by the Fear it gives) and a haunt lower it, and a clean or a Priest raises it (a clean to +50, never lowering it). Dirt spots show below +50. In the ghost Dimension the whole Room (not its Guests) shifts toward green and purple as it falls, and pulses once Closed; in the real Dimension it tints faintly green. Dirt, pay (the Mood pay factor) and the Guests' lines read it. Not the Ectoplasm currency, which the hotel keeps when a Room is cleaned. (Called Room Spook, 0 to 150 the other way, before 07; per-Guest Comfort is gone, Mood replaced it.)

**Dirt**:
Room Mood made visible: no spots from Mood +50 up, then one spot per about 3.75 points down to Mood -100 (Closed). The spots, the mess a Guest leaves and the calm lines stay tuned in 06's dirt scale (0 at +50 to 113 at -100); the code converts at one place (mood.go). A spot is normal or spiritual. Normal dirt is the Guests' mess and the Mood lost to an expired request or the Concierge lingering: Guests see it and complain of it, and SPACE cleans it in the real Dimension only, for nothing; a Housekeeper cleans it too. Spiritual dirt is a ghost's doing, a haunt or a fright: it shows only in the ghost Dimension, where SPACE cleans it and banks Ectoplasm; a Priest clears it too, and no Housekeeper does. Cleaning one kind leaves the other, and raises the Mood by what was cleaned. Any other rise of the Mood (a serve) takes normal spots only and stops at the spiritual dirt.

**Realm**:
One of the hotel's two independent layers, Human and Ghost. A Party lives in one; actors of different realms never bump, yield, push or queue for each other, and a Room is Occupied per realm. The Concierge is in Ghost while he is in the ghost Dimension. Every Party is Human until the Mirror is unlocked: then each Room also books a Ghost Party.

**Guest stats**:
Flee At (the Fear at which its Party steps out, then flees; a human's is its Nerve), Phantasm (how ghostly it is: sight, and the Mood damage of each haunt) and Haunt (x); Phantasm sets a ghost's kind and strength; Haunt sets how often it haunts.

**Ghost guest** (Ghost Party):
A Party of the Ghost realm. Once the Mirror is bought and delivered (a Run starts without it) each Room books one beside its human Party, independent of it: it walks in from the Street on the ghost layer, as one of the 4 characters in its ghost kind's flat arcade color, and a Room holds both at once. It pays no Money (no hourly rent, no tip), ignores dirt (no complaints, never flees over mess), asks for nothing, and earns only through encounters (V1). It is drawn in the ghost Dimension (at about 90% opacity) and never in real, where only its Area Shadows show. In the ghost Dimension humans draw as flat white silhouettes with a small white glow.
_Avoid_: ghost Walk-in

**Ghost kind**:
A name for a band of the Phantasm scale, with one flat arcade color (a placeholder): Wisp (cyan, the lowest, weak), Specter (pink), Wraith (orange) and Poltergeist (red, the highest); the last three are strong. The band edges and the strong threshold are placeholders (`phantasmBands`, `phantasmStrong`), tuned in the sim.

**Strong ghost**, **weak ghost**:
A ghost whose Phantasm is at or above the strong threshold (every kind but the Wisp) is strong: its Shadow People follow it into a Corridor or the entrance hall. Every ghost, weak too, switches on its own Room's (2026-11-14).

**Area Shadow**:
The Shadow People of an area (a Room, a Corridor, the entrance hall; the kitchen later), switched on by a ghost standing anywhere in it: any ghost in a Room, a strong one elsewhere. It is a flag on the area, not a figure bound to the ghost: it appears when the ghost enters, dashes at once in a Room, and fades when it leaves. It is the ordinary Shadow Person: in a Room it dashes along the back wall while humans are in, as often as its ghost haunts (the ghost's haunt frequency, so a relentless ghost shows more); in a Corridor or the hall it dashes now and then. In the real Dimension it shows only while it dashes, and only a human who sees the dash is frightened; it is hidden by a Room's Fog like its props, and Father M banishes it until that ghost leaves. Humans' sight rules are V1's.

**In Room**:
A Party is in its Room (`inRoom`) when it is staying and every Guest is inside the Room's walls, in either realm. A ghost haunts only while in Room.

**Closed**:
A Room at Mood -100. No new Party comes to it; a Party already in it stays until its checkout. The Terminal shows it in both Dimensions.
_Avoid_: blocked, locked

**Priest**:
Father M is the Priest the Concierge calls to a Room from its ring in the Hotel view, one per Room and one of each Father in the hotel at a time; his call is first unlocked once in the Terminal's STAFF tab, and every call costs Ectoplasm. He walks in, does his Ritual and leaves. He is the **Strong** Priest: he halves the Room's dirt, raises its Mood, banishes its Shadow and sends every ghost in it over, each paying at once the Ectoplasm its remaining haunts would have earned. Father K, the **Weak** Priest (halves the dirt), has no call: he is only ever the Retainer. Both wear a black cassock with a white collar.
_Avoid_: exorcist, cleaner

## Serving

**Serve**:
What the Concierge does for an Occupied Room whose Party is asking, in the real Dimension, with SPACE in front of its door: room service. The Room's Mood rises by 25 when served at once, falling to 0 as the request nears expiry. A Party that isn't asking can't be served.
_Avoid_: tend, room service (as the term)

**Mood pay factor**:
What a Room's Mood does to what its Guests pay: 0 at Mood -100, 1 at 0 and 2 at +100, linear. It multiplies the rent and the checkout tip, so a plain Room at Mood 0 pays its Rate. Rent is paid up front, per Guest, on the Guest's own clock: its share of the hour (Rate / 24, split between the Party, at the Room's Mood as it stands) the moment it steps into its Room, as a "+$N" at the door, then every game hour from that entry, in the Room or out; whole dollars, its cents carried to its next hour. A new Run's Rooms start at Mood +50.
_Avoid_: comfort bonus

**Haunt (x)**:
A ghost's haunt frequency, the second ghost stat beside Phantasm: the game minutes between its haunts. Each haunt lowers its Room's Mood by an amount that grows with the ghost's Phantasm. A ghost haunts only while in its Room: once as it settles in, then every x, until it is spent.
_Avoid_: Restlessness (a candidate name)

**Doormat**:
The 3×2 mat below a Room's door. SPACE on it delivers the asked item; elsewhere on it, with nothing to deliver, it opens the Hotel view. Its cell west of the door stands under the Room's plate, where SPACE is TALK.

**Pill**:
The balloon over a Party with a Request or a dirt complaint: the Room number, then the item's icon or a dirt splat; the same size on screen at any zoom.

**Easter egg**:
A rare pair of lines a guest says in its Room (`eggLines`, one in a hundred of its Room lines, never asleep), in two balloons: a human's Riemann proof too big for the towel, a ghost's Navier-Stokes proof it had time to read.

**Nonsense line**:
A line of nonsense humour in a line group (`odd(...)`), rarer than the others.

**Request**:
A Party in its Room asking to be served, once or twice an awake day: its Pill by its leader's head, always on screen. It ends when served, when the Party leaves the Room, or after 45 s (four and a half game hours) unanswered, when it expires and its Room loses 10 Mood.
_Avoid_: call, order

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
A Party's daily schedule. **Homebody** stays in the Room. **Tourist** goes out each morning, from the day after it arrives, and returns at night.

## Time

**Run**:
One playthrough, from the title screen until returning to it. A Run starts on Day 1 at 08:00 with every Room Vacant and a Standard Single. Money resets, and the Run never ends on its own.
_Avoid_: game, session

**Clock**:
The real-time in-game time: one day lasts about 4 minutes. It drives Walk-ins, Routines and the lighting, and it pauses only for the Terminal. The player can read it only on the Terminal.

**Time scale**:
How fast the world runs against real time, 1 normal. Every walk, animation, fade and timer, and the Clock, run by it together. Under 1 is slow motion (dev `-scale 0.25`); holding T runs it ×4. The Terminal, the hotel view, the HUD and the intro keep real time. A talk multiplies it by 0.25 to 0.08, by its depth, and an open Card by 0.06, while it is on and gives it back as it ends. The sim runs at 1 outside a talk.
_Avoid_: fast clock, game speed

**Day**:
One 24-hour turn of the Clock, counted from Day 1.

**Checkout**:
A Party leaving its Room for good on its final Day. It pays one night's Rate × the Room's Mood pay factor as a tip, whatever the stay's length, at the Room's door, the money is banked at once, and the Room becomes Vacant.

## Light

**Lightmap**:
The darkness layer over the hotel. Its tint follows the Clock, and Light Sources cut through it.

**Light Source**:
A lamp, ceiling light, street lamp, window, the kitchen's night lights, a Lantern or the Terminal screen: it cuts a glow out of the Lightmap at night.

**Lantern**:
The beam of light ahead of the Concierge and every staff member at night, on only in the dark: off where an area light already lights their spot (the Lobby while its lights are on, the kitchen, the Room the Concierge is in). It stops at the first wall.

**Glow**:
The Ectoplasm-ink light on the prop SPACE would act on, shown only while the Concierge stands in its range.

**Feet shadow**:
The dim patch under a living character; ghosts cast none and Levitate.

**Levitation**:
Ghosts, and the Concierge in the ghost Dimension, are drawn about 3 px off the floor with a slow bob.

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

## Economy

**Wallet**:
The hotel's two currencies, Money and Ectoplasm, and nothing else. Fear and Room Mood are stats, not currencies.

**Cost**:
A price in Money, Ectoplasm or both. It is paid whole or not at all.

**Unlock**:
A feature the player buys at the Terminal: the Mirror, a supply prop, Room Lock. A normal Run starts with the Mirror locked (the sim's `-mirror` flag starts it unlocked), and with it the Twin and Double Bed Layouts and the In-Room Mirror; the courier delivers a bought unlock at the stock, and the Mirror at its own wall spot (it is drawn covered, and shifts nothing, until then).

**Tap**:
One SPACE on a prop that earns +1 of something, by the Concierge or a Shadow Person alike. Each tap counts toward its job, a (Room, Task) pair: Serve or Housekeep.
_Avoid_: click

**Courier**:
The delivery person an order sends from the Entrance. He stops at the prop of each Supply ordered and fills it, drops the unlocks in the Kitchen, walks a Room key or the In-Room Mirror to its Room's door, then leaves. Priests are not carried; they walk in on their own. He shows at the Entrance once the store or the Hotel view that ordered him is closed. He and the Housekeeper never stop for anyone: like every walker they look a few cells ahead and take another way round whoever is there, and one they still run into, with no way around, they pass through. Of two walking at each other only one goes round; the other keeps its way.
_Avoid_: delivery guy, mailman

**Stock**:
The Supplies the hotel holds, each in its own prop on the Kitchen's back wall: Water in the fridge, Food in the oven, Towels on the shelf. A Food ask's balloon shows one of three food pictures, picked per ask (cosmetic); there is one Food. The props are solid; the Concierge fetches from the cell below each, and the Courier fills each from there. A Supply's icon and count show over its prop when the Concierge is within 2 cells.
_Avoid_: inventory, stock strip

**Kitchen**:
The Lobby's part east of the Lobby Archway, walled off as a room, there from the start, the same in both Dimensions. An open doorway in its west wall leads into the rest of the Lobby. It holds the Stock; its counter is decoration. Complaining Parties wait in the Lobby's bottom-left corner, clear of it.
_Avoid_: inventory

**Room key**:
An action of a boarded Room's actions in the Hotel view, paid at once, that opens it: the Courier walks the key to that Room's door, and on the drop the Room is owned. Several keys may be on their way at once, one a Room; the price rises with each Room owned and each key already paid for.
_Avoid_: unlock, license, deed

**Owned Room** / **Closed Room**:
A Room is Owned once its key has been dropped at its door; before that it is a Closed Room: drawn with the boarded door, no Party of either realm books it, its door stays shut to everyone, and it can't take a Room Change. A Run starts with every Room Closed. Not the same as a Room at Mood -100, which is Closed by Mood (a Closed Room in the sense of D140) and stays Owned.
_Avoid_: locked Room, sealed Room

**Key price rise**:
The Room key costs more with each Room already owned: `keyBase` for the first, plus `keyStep` for each one after. A Run starts with Money for exactly the first key (`startMoney`). Placeholders.

**Intro**:
The opening of a new Run: the title is the whole hotel in the ghost dimension with both Rooms boarded, no Concierge, ghosts on the street; SPACE brings three timed lines (SHIFT stressed); SPACE again and the Concierge walks in from the street while the view closes in and the world shifts to real, once. The world and the Clock wait for it. `-skipintro` skips it; the sim and the dev prints never play it unless asked (`-intro`).
_Avoid_: cutscene, tutorial

**Opener**:
The Strategy that plays the opening: it buys the first Room key, then the Mirror as soon as it can pay, then plays smart while it saves for the Twin Bed Layout, changes a Room to it and buys the In-Room Mirror for that Room. The sim's `mirror at (h)` column is the game hours from the Run start until the Mirror unlocks; `room mirror at (h)`, until the first Room has its own.

**Room Change**:
An action of a Room's actions in the Hotel view ("Change type"), paid at once, that gives the Room another Bed Layout among the unlocked ones, Single included, for that layout's fee. A Vacant Room with nobody in it changes when the Terminal closes; any other is scheduled: it takes no Booking in either realm until it is empty, then changes. A scheduled change can be cancelled on the Room's card for a full refund. The Room's Mood is untouched.
_Avoid_: upgrade, renovation, Spin A Room's actions hold only what applies to it: a Room not bought lists its key alone, a bought one never lists a key. The action is not listed while there is nothing to change to: only one bed layout unlocked, no change waiting, and no in-Room Mirror the Room could still get. A change keeps the Room's tier. Once the **Deluxe** tier is unlocked (HOTEL tab, Ectoplasm, after the Mirror) each Room's actions also hold **Deluxe**, one press, paid in Ectoplasm: an upgrade to the Deluxe of the beds it has, which waits and reels like a Room Change and can be cancelled like one. The Room gets the Deluxe look and its higher Rate from the next Booking on, and stays Deluxe through later changes of beds.

**Reel**:
How a Room Change shows: the Room's whole footprint rolls like a slot machine's reel, the old Room out the bottom, the new one in from the top, in the world and in either Dimension. The Room takes no Booking while it rolls.
_Avoid_: slot machine (as a mechanic: nothing is random), transition

**In-Room Mirror**:
A Mirror on a Room's back wall, where SPACE shifts the Dimension as at the Lobby's. Bought once, with Ectoplasm, on a first Room's actions in the Hotel view: paid at once, carried by the Courier to that Room's door and hung when he gets there, its Guests in or not (a scheduled Room Change keeps waiting); from then on every Room gets one with its next Room Change, or when its key opens it, and each Room Change also costs Ectoplasm. Once it is owned, a Room without one shows **Add mirror** in its ring (Money and Ectoplasm), and the Courier hangs it in place of the prop on that wall. A Room with a Mirror has a second ghost slot: a lone ghost books into it on its own and checks in through the Mirror; Matching reads the higher Phantasm of the two.
_Avoid_: vanity, second Mirror, Room mirror (the store row's name only)

**Growth**:
The hotel gaining Rooms mid-Run: "New column" adds a Room column on the east, "New floor" a floor at the back. Both are HOTEL rows of the Terminal, paid in Ectoplasm, on sale once the Mirror is owned; each expansion costs one step more than the last (60 Ectoplasm a step), times the per-Room price factor, one is bought at a time, and the grid stops at the Level's limit, (3 + Level) a side. No Courier brings it: the hotel grows as the Terminal closes. The world and the Clock wait while the camera pulls back on the hotel as it stood, the new part slides in from off screen, and the camera comes back. Every Room keeps its number's place, its type, its Mood and its Guests; the new Rooms arrive boarded up, each needing its Room key. A new Run starts at the size the hotel opened with.
_Avoid_: expansion (as the mechanic's name), upgrade, Spin

**Grower**:
The Strategy that plays the Opener and buys Growth whenever its Ectoplasm covers it, a column then floors, up to the grid's limit. The sim's `growth` lines give the game hour of each expansion.

**Housekeeper**:
A member of staff hired at the Terminal's STAFF tab, one for every 3 Rooms owned (rounded up, at least one); she walks in from the Entrance, no Courier, dressed as a maid (pale, black hair and dress, white apron). With nothing to do she roams the whole hotel at a stroll (the hall, the Kitchen, the Corridors), never stopping in a Room, a doorway or anyone's way. She brings a Supply the Stock holds to a Room that asks for it (the delivery pays as the Concierge's does), unless her walk there would outlast the ask and cleans a Room whose Party complains of its dirt. Idle, she sweeps the Room with the most normal dirt, empty or sleeping Rooms first; spiritual dirt is never hers. A clean takes her a while at the stains (20 game minutes, cut by the Cleaning kit); the Concierge's own clean is instant. She is paid a wage each game day, the first a day after she was hired. Short of it she stops working and is owed it, taken as soon as the Money is there; still owed at her next payday, she walks out, and hiring again costs the fee. She tells the Concierge what is wrong in her own lines, as he comes near her (owed her wage; a payday near that the wallet can't cover; leaving); with nothing wrong she says a line only now and then. It welcomes each human Party walking in past it, once.
_Avoid_: maid, cleaner, keeper (the Keeper is a Strategy)

**Staffer**:
The Strategy that opens Rooms, keeps the Stock up, hires one Housekeeper as soon as it can pay her fee and a first wage, and then stands at the Terminal. Its Runs say whether a hotel's ordinary takings keep a Housekeeper paid: the sim's `staff` lines give how many walked out unpaid.

**Busy hotel**:
A stress Run (`-busy`, `TestBusyHotel`): the hotel at 3 by 3, every Room open with Twin beds, Parties and ghosts arriving minutes apart and replaced minutes after they leave, three Housekeepers, a Courier restocking for free. It shows everyone walking at once and counts how often a walker is stopped by someone ahead.

**Notice**:
The HUD's place for words to the player: a feed on the right, under the Ectoplasm counter and TALK's cue, of one-line notices (about 4 s) drawn exactly like the Ectoplasm counter (its font, size and ink, no outline), in one left-aligned column, at most three, the oldest leaving early, the feed emptied when a Run starts; the left side's next-guest note is drawn like the Money counter and holds as long, shown again when its line changes. All hide while the Terminal is open. A notice fires when a Room reaches Mood -100 ("101 CLOSED", ghost-only Rooms too) and when a human Party flees ("GUESTS FLED 102"); staff leaving unpaid only say so. Code says it with `g.notice(text)`. The game has no message line at the bottom of the screen, and a test (`TestNoMessageLine`) fails if code brings back one, a background behind HUD words, or a text outside `g.notice`.
_Avoid_: message, message line, status bar, toast

**Room-only line**:
A line that makes sense only inside a Room (what a Guest says of its Room, of a Shadow seen there, a ghost's in-Room lines, "Zzz..."). A Guest outside the Rooms never shows one: said inside, it ends at the door. A Shadow seen in a Corridor or the hall draws a hall line instead.

**Goals Strategy** (`goals`, 2026-11-14)
The sim Strategy that plays a Run in the agreed order of first goals: Room 1, the Mirror, Room 2, then everything that earns as soon as the wallet covers it (Ectoplasm on the first Growth, the in-Room Mirror, the Deluxe tier, a Room's upgrade, more Growth; Money on the keys, then one housekeeper), buys Father K's and Father M's unlocks, calls Father M on the worst Room and keeps Father K on his Retainer; the sim's `M calls`, `retainer days` and `priest ecto` columns count it. It buys the moment it can, so when it stands idle there was nothing to buy and nothing to do: the sim's **pacing** lines (each milestone in real minutes, and the longest such wait before the first Growth, the first housekeeper and the in-Room Mirror) are read off its Runs, and `TestPacing` holds them in band.

**Spent**, **passing on** (2026-11-14)
A ghost has one haunt in it per point of Phantasm. Having done them all it is spent and passes on: it rises, pales and is gone, and its Room books the next ghost. It dims with each haunt on the way. The weakest ghost haunts once. A pair passes on together.
_Avoid_: dissolve, die, despawn

## Matching (07 X2, 2026-11-14)

**Nerve**:
How much fright a human takes before it flees, rolled per Party as it books (6 to 12, a placeholder). It sets the Guest's Flee At (Nerve x one sighting's Fear). Ghosts have none.
_Avoid_: courage, bravery, tolerance

**Fit**, **good pair**, **bad pair**:
A human fits a ghost whose Phantasm is within its Nerve. A Room holding a human Party and a ghost is a good pair when the Party fits the Room's strongest ghost, a bad pair when it does not. In a good pair the Shadow's sightings frighten and pay Ectoplasm as ever and never make the Party flee. In a bad pair the Shadow pays nothing, the Party pays half its hourly rent (a placeholder), and each haunt terrifies everyone in the Room by half their Flee At: the Party steps out on the second haunt and flees on the third. A Room with no human Party or no ghost is unpaired. The fit reads in the hotel view only: small motes rise in the Room, red for a bad pair, the Ectoplasm green and purple for a good one.
_Avoid_: match (for the pair), compatible, preference

**Auto-book by fit**:
How a human Party picks its Room as it arrives: of the Vacant, owned, open Rooms with beds enough, the one with the strongest ghost within its Nerve; with none, the one with the weakest ghost beyond it (the closest bad fit). The Room it leaves empty takes over the booking of the Room it took. Ghost booking is unchanged: each Room books its own.

**Talk** (the talk), **TALK**:
The staged conversation in which the Concierge asks a human Party to change Rooms. TALK shows on the HUD's right, in the Ectoplasm counter's style, while the Concierge stands under a Room's **plate**, the brass plaque on the wall west of its door (on the mat), with a Party of the current Dimension in the Room; SPACE there opens the guest's Card, whose MOVE starts the talk when the human Party is in, awake, not snubbed and has a target Room. Inside the Room SPACE cleans (T381 refine 2), except near a quest character (see **Quest TALK**). When the guest is a quest character the objective wants moved to a Room (O3, O7, O10), the talk offers only that Room. The camera closes in on the two, the surroundings blur into purple, the world runs in slow motion (see Time scale) and the guest stands and listens. How slow, and how heavy the blur, is the talk's **depth**: how far the Room's ghost's Phantasm is beyond the Party's Nerve, from 0 (it fits) to the most a Nerve can lack, fixed as the talk starts (a quarter speed and a light blur at 0, about a twelfth and a heavy blur at the worst; placeholders). A/D picks the target Room, W/S the offer, SPACE makes it, ESC leaves. It ends by itself if the guest leaves or no target is left, and on a refusal. Ghosts cannot be talked to or moved.
_Avoid_: dialogue, chat (a pair's chat is an activity), move menu

**Quest TALK** (08 W5 R8):
SPACE with the Concierge close (about a tile and a half) to a quest character of his Dimension (a named questline guest, human in real, ghost in ghost; also the ghost roommate, the oldest sleeper, the HAFT's ally), walking in a Corridor or in its Room, with nothing nearer to do on the spot (the plate's Card, a delivery, a Mirror, the Terminal and the shelves come first; inside a Room it comes before CLEAN). The cue is TALK on the HUD's right, as for the plate. The character stops and faces him, the camera closes in on it in the card's frame with the world stopped (a short cutscene), and it says its quest line over its head; then the word or thing he carries that it needs is handed over (TELL, GIVE), what it gives is taken (TAKE; Magenta's word for O1), and its done line follows; then the frame lets go. Each line stays up until SPACE (no prompt shows). With nothing quest-related it says one of its own lines. Not the talk (MOVE) above.
_Avoid_: TELL/TAKE/GIVE buttons (they are not on the Card any more)

**Target Room**:
A Room a talk can offer: another owned, open Room with beds enough whose ghost the Party fits, Vacant or held by a human Party that could swap (it is in or away, fits this Room's beds and fits this Room's ghost). Listed best fit first.

**Offer**:
One of the three lines the Concierge can say for a target Room. The **plain ask** is free and usually refused. The **discount** costs a fifth of a night's Rate and is likely taken. **On the house** costs a night's Rate and is always taken. Only the offers the wallet can pay show; the plain ask always does. A scared guest and a target Room of higher Mood make the first two likelier. A **refusal** costs the guest's own Room 40 Mood and ends the talk, and the guest is **snubbed**: no TALK for it for a game hour. A taken offer is paid at once. All numbers are placeholders.
_Avoid_: bribe, deal, option

**Swap**:
A taken offer for an occupied target Room: the two human Parties trade Rooms, and the price covers both (each Room's Rate times the offer's share). Either way the Parties walk over by themselves; each Guest's rent clock runs on and its next hour pays the new Room's Rate; the Moods stay with the Rooms.

**Sure offer**:
An offer whose odds are 1. Today only on the house is.

**Matchmaking**:
The HOTEL tab's upgrade (40 Ectoplasm, on sale once a Housekeeper is hired, a placeholder): an idle Housekeeper walks to a bad pair's door and makes the cheapest sure offer the wallet covers, with no talk staged. It keeps off the questline: it never moves a quest character's Party or one an open objective wants where it is, and never offers a Reserved Room or a Room an objective targets.

**Matcher**, **asker** (`matcher`, `asker`):
Two sim Strategies that play `goals` and talk to every bad pair they can: `matcher` always on the house, in the Room where it costs least; `asker`, the dumb rule it is measured against, always the plain ask, whenever the guest is not snubbed. Both buy Matchmaking as soon as they can and leave the bad pairs to the Housekeepers from then on.

**Ritual**:
A Priest's work in a Room: he walks to its middle, says an opening line, chants with a small hop while sparkles rise, a burst of light lands the effect, he says a closing line and leaves. Father K's drawing is in `game/ritual_k.go`, its knobs at the top; the dev flag `-ritual k|m|kpace` loops one for tuning.

**Retainer**:
Father K's daily wage in Ectoplasm, a STAFF row once Father K is unlocked there. Retained, he stays in the hotel doing his Ritual on the Room with the most spiritual dirt, one after another, and paces the Corridors when none has any. Short of it on payday, he leaves at once and the row is offered again.

**Running shoes**:
Three STAFF tiers in Ectoplasm, each +15% walking speed (added) for every Housekeeper, Priest and the Courier, later hires included; never the Concierge. Only the next tier is listed.

**Benevolent ghost**:
The Concierge in the ghost Dimension: he hears only ghost lines (never the living's or the staff's), triggers no Shadow or haunt, and never bothers guests by lingering.

**Corridor mirror**:
A Mirror on a Corridor made by a New floor, one per such Corridor, at a random Room's front-wall corner each Run, covered at first, like the Lobby Mirror: SPACE at a covered one does nothing. The HOTEL row buys it for Money and Ectoplasm, dearer than the Lobby's: ($100 and 60 Ectoplasm) times the number of Corridor mirrors bought plus one, then the Room key's 15% more for each Room owned; the Courier walks to the cell below it and uncovers it, and then it Shifts like the Lobby's.

## Prestige

**Prestige**:
Ending a Run on purpose to start the next one stronger: the hotel, Money, Ectoplasm, unlocks and staff all go; only the Level and the Bonus carry over. Asked of Father P.
_Avoid_: reset, rebirth, new game

**Father P**:
The Priest of the Prestige, one STAFF row priced in Ectoplasm. On sale only once the hotel has grown to its Level's limit and every Room key is bought. SPACE on his row opens a confirm card (what is gained, what is lost); SPACE on the card pays and plays his cutscene: everyone gathers in the hall, the world turns upside down, time races while everyone but him and all the furniture lifts off and spirals into the spinning clock, a white flash, and the next Run starts on Day 1 at 08:00 in the Entrance hall, with no title or intro. The first time the cutscene plays in full; later SPACE skips to the flash.

**Level**:
How many Prestiges the hotel has been through. It sets the grid's limit, (3 + Level) a side, and opens one new thing per Level (Level 1: the Pantry); a Terminal row waiting on one says "Prestige level N to unlock". Shown with the Bonus in the Terminal's header ("LV 1 - x1.53"), never on the play HUD.

**Bonus**:
The income multiplier a Prestige leaves: every Money and Ectoplasm earning is multiplied by it, in one place. Each Prestige adds to it (it never drops) the square root of the Run's Ectoplasm earned plus its unspent Money (donated, $10 counting as 1 Ectoplasm) over 6000.

**Pantry**:
A shelf of Supplies in the middle of each Corridor a New floor made, apart from its Corridor mirror: a HOTEL upgrade on sale from Level 1. It has no stock of its own: it shows the Kitchen's, each Supply's icon and `xN` side by side while the Concierge is within 2 cells. SPACE there works only while a Room asks for a Supply the Kitchen has, and takes the one asked by the asking Room nearest to it (what is in the hands goes back to the stock); a Housekeeper fetches from whichever of it and the Kitchen is nearer. Supplies are still bought in the Kitchen.
_Avoid_: shelf (alone), storeroom

**Save**:
What the game keeps between sessions: the Level and the Bonus, and a checkpoint of the Run at each dawn (08:00, from Day 2): the grid, each Room's type, ownership, Mood, dirt and Mirror, the Corridor mirrors, the staff, the wallet, the unlocks and the Ectoplasm earned. No guests: CONTINUE on the title starts that morning with an empty hotel. On the web it lives in the browser's storage, on a computer in a file; only a person's start uses it, never a dev start, a print, a test or the sim. A save it cannot read starts a new game.
_Avoid_: autosave, slot

## Personalities and economy (08 W4, 2026-11-20)

**Identity**:
Who a Guest is, rolled as it books, apart from the sim's dice (its own random stream): a human's name, age, job and line; a ghost's name, death year and cause, and line; for either, maybe a Quirk.

**Card**:
What TALK opens at the Room's plate: the Guest's Identity (a ghost's, in ghost), in the talk's frame: the camera closes in on the guest and the world runs very slow (0.06). Its line shows over its head (each guest has 3 and says the next at each opening), who it is on its left, what REVEAL read on its right, the options below (T381 refine 3). A/D flips a Party of two; its options are only those that apply: REVEAL, and MOVE (a human Party: the talk). It has no quest verbs (those are **Quest TALK**).
_Avoid_: profile, sheet

**REVEAL**:
A Card's option, paid in Ectoplasm once a Party: it shows the Guest's Nerve or Phantasm and its Quirk, and for a named ghost its fact. A named guest stays revealed on later visits (saved). A Priest guest has none.

**Quirk**:
One trait a Guest may have that bends a rule of the hotel, from the hotel's first Growth on (before it, guests have none). Human: Medium, Skeptic, Big tipper, Night owl, Neat freak. Ghost: Show-off, Shy, Jealous, Gentle, Homebody. Each one's rule is in `context/08-quests/cast.md`. A ghost's Homebody quirk is not the Homebody Routine.

**Named guest**:
One of the hand-written cast (`cast.md`: seven human entries, three ghosts) who takes a generated booking about 1 time in 6, from the first Growth on, when not in the hotel and away a game day. It keeps its own lines, quirk and (a ghost) kind, and its visits count across Runs (saved). Osvaldo comes only after Magda.

**Guest book**:
The book on the entrance counter, read with SPACE beside it. Every Party writes a line as it leaves: cozy, dread or fled; ghosts passed on or banished, shown only in ghost. A named guest writes its own. It opens as a soft-cover book: two ruled pages curving into the spine, a thin black cover bending with them. The entries are handwritten on wide ruled lines, each in its own pen and hand: some keep to the line, some tilt, some bob word by word (T381 refine 6). A/D turns its pages, a spread at a time; a guest with no words left a scrawl.
_Avoid_: notebook (in game text), log

**Priest guest**:
Father M booking a Room as a solo Guest, about 1 booking in 20 once his call is unlocked and he is not in the hotel, and never while O6 is open (it needs his call). Father K never books. He pays three times the Rate, sends the Room's ghosts on (no Ectoplasm), books none, and scrubs its spiritual dirt each hour. One Father M hotel-wide still holds.

**Upgrade**:
A row of the Terminal's UPGRADES tab: SPACE puts its next level in the cart, one at a time, paid at checkout in Money (Soundproofing, Seance table and Offering box, the ghost-side ones, in Ectoplasm too), and the Courier brings it to the stock, where it counts from the drop: Lobby decor (Rate), Brochures (the wait between Parties), Soundproofing (Mood lost to haunts), Seance table (Ectoplasm from sightings), Tip jar (tips), Cleaning kit (clean time) and Offering box (Ectoplasm each Priest ritual pays). Hidden until it applies; each level costs one step more and does 10% less than the last; reset at the Prestige, saved in the Run. Its level is not the hotel's Level.
_Avoid_: perk, boost

## Magenta's circle (08 W5, 2026-11-20)

**Questline**:
Magenta's circle (`context/08-quests/circle.md`): seven Beats played across the Growths of a Run, the same story every Run. A Level 0 Run plays 3 Beats, Level 1 plays 5, Level 2 and up all 7. What is done counts across Runs (saved).
_Avoid_: campaign; "story mode" means only the dev mode below

**Story mode**:
A dev mode (`-story`, web `?story`) to test the Questline fast: each Growth fires free as its Beat ends, all 7 Beats are open at any Level, the Mirror is unlocked and Quest guests check in at once. Its hotel may grow to 7x7 by Terminal too, so Father P (who needs the hotel at its limit) is out of reach there; a normal Run's limit stays 3 + Level. Not in the Production build.

**Production build**:
The build for players (`sh tools/web.sh prod`, `-tags prod`, `game.Dev` false): it takes no dev flags or `?args=`, has no Story mode and no frames readout. Hold `T` stays.

**Beat**:
One chapter of the Questline, started by a Growth: its Quest guests check in, its Speaker says its opening line, and the HUD shows its title and checklist of Objectives (top left in real, top right in ghost) for about 5 s whenever they change, and always in the Hotel view. Done the first time ever, it pays its purse (Ectoplasm) and gives its Item; later Runs replay it with shorter lines.

**Objective**:
One row of a Beat's checklist (O1..O13), shown in both Dimensions as its next step behind the icon of the Dimension that step is done in: a word told, a thing given, a Room chosen, a Request served, a spirit lit, a delivery made. Ticked the moment it is done. Shown briefly when it changes, then hidden; always shown in the Hotel view.
_Avoid_: task, mission

**Gate**:
While the Beat under way has an Objective open, the next Growth waits: "SHIFT'S NOT OVER". The last Beat of the Run gates nothing.

**Quest guest**:
A named guest (`cast.md`) a Beat needs: Magenta, Magda, Teo, Celestine, Tim, Osvaldo. It checks in ahead of the walk-ins, one a minute, and is held (no checkout, flight, passing on or Father M) while an Objective needs it.

**Guidance**:
The step's target (a guest or its plate, the guest book, a door, a prop, the spirit) wears a gold (real) or silver (ghost) outline, only in the Dimension the step is done in; in the other one the nearest Mirror wears it. A quest guest under a Room's Fog keeps its outline (hollow, drawn over the Fog; the sprite stays covered). A step that needs something not bought yet (Father M's call, the Wreath, the Crate, the Supply asked for) outlines the Terminal and, inside it, that row (and its tab); a row a step needs is on sale. In the guest book the questline's entries are written in a deep purple (the rest in black or dark blue), and while a step needs one, the page-turn key toward its spread wears the outline; reading counts only on the open spread.

**Quest look**:
The sheets a Quest guest wears: its base LimeZu sheet in colours of its own under one accessory (hat, tiara, lamp, cap, phone), made by `tools/questlooks`. No ordinary guest, passer-by or Shadow wears one; in ghost a quest ghost has a hue of its own. Magenta is never scared.

**Word / thing**:
What the Concierge carries for a quest guest, one at a time, across a SHIFT: TELL hands over a word, GIVE a thing, TAKE collects one (Card options shown only while an Objective wants them). Shown as a pixel icon in a bottom-left HUD slot, no text.

**Ghost Request**:
A quest ghost asks for a Supply: fetched in real, delivered with SPACE at its door in ghost. Only quest ghosts ask, and their pill shows only in ghost.

**Target override**:
While an Objective wants a guest in a given Room, the talk always offers that Room, sure, for the discount alone.

**Quest row**:
A one-off Terminal order shown only while its Objective is open: the Wreath, walked by the Courier to Madame's door (a box there in real, the wreath in ghost, where `Space` picks it up with the hands free; then `Space` on Madame gives it and she thanks for it; the wreath taken is saved, so a resumed game keeps it in hand); the Crate, carried by the Courier into Madame's room and set down on the floor at the middle of the finale's circle, after which he walks out. Buying the Crate makes everyone else in her room check out at once and books nobody into it until the finale is done. The Crate stays on the floor (both Dimensions) until the Concierge is inside the room and the Courier is out of it: then O13 is done and the finale plays (the screen fades to black and back in on its stage, where nobody but its cast is drawn in the room or round its door) (first time ever; a replay is done at delivery, no scene). It is saved with the Run's rolls, so a resumed game keeps it.

**Spirit**:
O4's spirit: at night it dashes along a Corridor; caught in the Lantern beam it shows only two small red slits for eyes ("I SERVE ANOTHER."; the big white eyes are the MacGuffin's alone) and is gone, leaving a glowing handprint on a door, found in ghost, that waves.

**Item**:
One of Magenta's six gifts, owned for good once its Beat is first done: Show, don't tell (REVEAL is free, and its option shows no price), Broom (the Concierge walks faster in ghost), Gerald (one Room's sightings pay more), the Cat (cleans spiritual dirt in ghost, two naps a shift; seen in both realms, meows in real and says rude things in ghost, silent asleep), the Ward (one Room's guests never flee), the Storm jar (tonight's thunderstorm: sightings pay double, once a day). Gerald and the Ward are placed on a Room from the hotel view's ring or the HOTEL tab, per Run.
_Avoid_: power-up, relic

**Cutscene**:
O4's two (the spirit's red slits in the dark, the handprint close-up), O9's key (a close-in on Tim's hands, the gem rising) and the finale (Madame's room alone, staged: no furniture, no Corridor, only its floor, walls, windows and mirror; the cast, the Concierge among them, walks in to points evenly round the circle, facing its centre, the lines between the points a triangle, a square, a pentagram, or for 6 a hexagon with its three long diagonals through the Crate (a wheel); the walls slide away, leaving the floor, the cast and the circle in green, purple, gold and silver in the void, and the MacGuffin rises from the Crate): the world stops, every line waits for SPACE (no prompt shows); it plays in full the first time and SPACE skips a replay, except while a line waits (then SPACE only moves that line on). The MacGuffin's eyes, in the finale's dark, are drawn at twice their first size. Money and Ecto stay off the screen during a talk, a card or a cutscene.

**MacGuffin**:
The Eternal Eldritch MacGuffin, summoned once ever by the finale (the Concierge walking into Madame's room once the Crate is on its floor and the Courier has left, where the circle is drawn around it). From then on it books the best Room every Run from the first Growth, pays 5 times the Rate, never leaves, asks or flees, is immune to Mood and Fear, and cannot be REVEALed. The finale adds 0.25 to the bonus. Every finale ends in a normal prestige (level +1, the usual gain, a fresh Run); the first one shows the credits first. It signs the guest book at the Run's end.

**Passer-by**:
Someone walking the street outside the Entrance, living in real, dead in ghost. Near the door one may say a line of gossip, which follows how far the Questline has got; a living one sometimes turns in as a walk-in.

**Weather**:
Each day may bring rain, fog or wind, drawn over the street and windows (visual only, rolled by the day). The Thunderstorm comes only from the Storm jar: lightning flashes, and every sighting pays double until the next 08:00.

**Kitchen life**:
An idle Housekeeper sometimes goes to the fridge, the counter or the stove and says a line there; any job pulls it away at once.

## Sound (09, 2026-11-22)

**Crowd volume**:
Footsteps are louder the fewer people walk in the Concierge's area: 1.0 is the loudest, alone at night.
