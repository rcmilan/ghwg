package game

// 08 W5: Magenta's circle as data (context/08-quests/circle.md, approved 2026-11-20). Seven beats, one per Growth
// from the 2x1 start; 13 objectives. Every number is a placeholder the W5 sim tunes. The engine is quest.go, the
// verbs verbs.go, the cutscenes questscene.go. Lines by times done: [0] the 1st time, [1] the 2nd, [2] the 3rd and on
// (an empty one falls back to the one before).

// beatCount: the questline's beats. Level 0 plays beats 1-3, Level 1 beats 1-5, Level 2 and up all 7 (beatsAt).
const beatCount = 7

// beatsAt: how many beats a Run at prestige level plays.
func beatsAt(level int) int {
	if StoryMode {
		return beatCount
	}
	return [...]int{3, 5, beatCount}[min(level, 2)]
}

// objective is one row of a beat's checklist.
type objective struct {
	ID    string // "O1".."O13"
	Row   string // the checklist row
	Side  Realm  // where it is finished: Human, a row on the left shown in real; Ghost, on the right shown in ghost
	Scene string // its cutscene's id ("O4", "O9", "finale"), "" for none
	Ask   string // the line that sets it, "" for none
	Done  string // the line when it is done, "" for none
	Wait  string // the row while it can't be done yet (O1: no ghost in her Room), "" for none
	Hard  string // what changes from Level 2 (Lv2+), for the engine; "" for nothing
}

// beat is one Growth's chapter.
type beat struct {
	Title   string
	Guests  []string // the quest guests checking in when it starts: namedHumans/namedGhosts keys
	Objs    []objective
	Purse   Num       // Ecto, the first time ever only
	Item    itemID    // given the first time ever only; noItem for beat 7
	Speaker string    // who says Lines: a cast key
	Lines   [3]string // by times done, said when the beat starts (beat 3, 5: its cliffhanger, when it ends)
	AtEnd   bool      // Lines are said when the beat ends, not when it starts
}

var beats = [beatCount]beat{
	{Title: "SHIFT HAPPENS", Guests: []string{"magenta"}, Purse: 20, Item: itemBall, Speaker: "magenta",
		Lines: [3]string{"Be not afraid. The hotel is full of noises. Most of them are the plumbing.",
			"Again, Concierge? Lovely to see you.",
			"You know the steps. Do them with more feeling."},
		Objs: []objective{{ID: "O1", Row: "TELL HER ROOMMATE SHE'S HERE", Side: Ghost,
			Ask:  "Tell whoever shares my room on the other side that the witch has arrived. Use those words. Say it with a cape in your voice.",
			Done: "A witch! Finally, someone who knows where the good towels are.",
			Wait: "WAIT FOR HER ROOMMATE",
			Hard: "the word must arrive before the next dawn, or she sends a second, longer word"}}},
	{Title: "A GIFT, DELAYED", Guests: []string{"celestine"}, Purse: 30, Item: itemBroom, Speaker: "celestine",
		Lines: [3]string{"This is my hotel, darling. Do keep it. I'm only haunting it part-time.",
			"You again! And the witch! It's like a family reunion, but nobody brought a casserole.",
			"Every time the same witch, the same you. Every time I'm still dead. Consistency is a virtue."},
		Objs: []objective{{ID: "O2", Row: "FIND OUT WHAT " + upper(nameMadame) + " ORDERED", Side: Human,
			Ask:  "The lady of the house left a note in the guest book. Read it to me. Do the voices.",
			Done: "Ninety-seven years on hold. That woman has the patience of a mountain.",
			Hard: "the entry continues on another spread"}}},
	{Title: "A SHAFT OF LIGHT", Guests: []string{"magda"}, Purse: 40, Item: itemGerald, Speaker: "magenta", AtEnd: true,
		Lines: [3]string{"Two eyes in the dark? That's mine. Bound to me till the job's done.",
			"Same eyes, same words. It's loyal. To me, mostly.",
			"It waved at me. We're on waving terms now. It still wants its freedom. Fair."},
		Objs: []objective{
			{ID: "O3", Row: "PUT " + upper(nameMagda) + " IN " + upper(nameMadame) + "'S ROOM", Side: Human,
				Ask:  "A medium in the lady's room. Mediums are the customer service of the dead.",
				Done: "We talked all night. Mostly about curtains.",
				Hard: nameMagda + " arrives with a stranger, who has to be moved somewhere less important first"},
			{ID: "O4", Row: "CATCH THE SPIRIT IN A SHAFT OF LIGHT", Side: Ghost, Scene: "O4",
				Hard: "two dashes, and both must be lit"}}},
	{Title: "NIGHT SHIFT", Guests: []string{"tim", "teo"}, Purse: 50, Item: itemCat, Speaker: "tim",
		Lines: [3]string{"Night shift since 1959, sir. I could murder a towel. Not literally. Mostly."},
		Objs: []objective{
			{ID: "O5", Row: "BRING " + upper(nameTim) + " WHAT HE CAN'T CARRY", Side: Ghost,
				Ask:  "Night shift since 1959, sir. I could murder a towel. Not literally. Mostly.",
				Hard: "two Supplies, one after the other"},
			{ID: "O6", Row: "CLOCK OUT THE OTHER NIGHT PORTER", Side: Human,
				Ask: "Chat says a ghost in Room {n} keeps whispering 'is my shift over'. Chat, honestly."}}},
	{Title: "SIFT", Guests: []string{"osvaldo"}, Purse: 60, Item: itemWard, Speaker: "tim", AtEnd: true,
		Lines: [3]string{"", "Paid in full. Again. The change keeps turning up in my pocket.",
			"Same change, every time. I've stopped counting it."},
		Objs: []objective{
			{ID: "O7", Row: "LET " + upper(nameMagda) + " AND " + upper(nameOsvaldo) + " MEET", Side: Human,
				Ask:  "The circle needs a knot of love that outlasts death. Also string. I've got the string.",
				Done: "They said full fathom five he lies. He's in {n}. Same thing, really."},
			{ID: "O8", Row: "A GIFT FOR " + upper(nameMadame), Side: Ghost,
				Done: "Flowers! For waiting! Nobody has ever thanked me for waiting. I'll wait even harder."},
			{ID: "O9", Row: "SIFT FOR THE KEY TO " + upper(nameTim) + "'S SUITCASE", Side: Ghost, Scene: "O9",
				Ask:  "The suitcase key is with the oldest soul sleeping here. Living soul. I've sifted the dead ones.",
				Hard: "the oldest is a Tourist, out until night"}}},
	{Title: "GET A HAFT", Guests: nil, Purse: 80, Item: itemStorm, Speaker: "magenta",
		Lines: [3]string{"Every circle needs a staff. Mine's lost its HAFT. Don't ask where the rest of it is."},
		Objs: []objective{
			{ID: "O10", Row: "STRANGE BEDFELLOWS", Side: Human,
				Done: "She snores in Latin. Misery acquaints a woman with strange bedfellows. This was not misery. This was a sleepover."},
			{ID: "O11", Row: "THE HAFT, FROM THE OTHER SIDE", Side: Human,
				Ask:  nameTim + "'s old luggage-trolley handle. Slightly haunted. Excellent grip.",
				Done: "Tim's trolley handle. Fits my staff like it grew there. HAFT found."},
			{ID: "O12", Row: "READ THE NAME", Side: Ghost,
				Done: "Don't say it aloud. It RSVPs."}}},
	{Title: "MELTED INTO AIR", Guests: nil, Purse: 0, Item: noItem, Speaker: "magenta",
		Lines: [3]string{"Thank you, Concierge. Truly. Stand back. Further. Behind the towel.",
			"It's already here, love. It never left. It's in the minibar."},
		Objs: []objective{{ID: "O13", Row: "THE CRATE WITH NO SENDER", Side: Human, Scene: "finale"}}},
}

// objLines: the quest guests' other lines, by objective.
var (
	o11Magenta         = "Fetch me " + nameTim + "'s old trolley handle. I'd go myself, but then what would you do all day?" // her Card while O11 is open (a fetch-quest joke, live test 5)
	celestineEntry     = "Ordered: one (1) MacGuffin, eternal, eldritch. Room service, 1923. Still waiting. Lovely hotel otherwise. Room {n}."
	celestineEntryHard = "cont'd, still waiting, still lovely"
	o4Lines            = [2]string{"I SERVE ANOTHER.", "STILL SERVING."}
	porterQuirk        = "NIGHT PORTER, 1923"
	porterFact         = "Clocked in 1923. The clock broke. Still in."
	keyGive            = "Found this in the bedside drawer. It hummed. I didn't like it."
	nameEntry          = "MAGnificent stay, Couldn't Get Up For breakfast. Fine."
	o9Lines            = [2]string{"Delivery for " + nameArbogast + ", night shift, 1959. Signature required.", "Paid in full, 1923. I've been carrying the change."}
	wreathRow          = "WREATH, FOR " + upper(nameCelestineShort) + " (A THANK-YOU, 97 YEARS PENDING)"
	crateRow           = "A CRATE. NO SENDER. ORDERED 1923. EXPRESS."
	finaleLines        = [2]string{"I AM THE ETERNAL ELDRITCH MACGUFFIN. I WOULD LIKE A ROOM WITH A VIEW. OF EVERYTHING.", "Ninety-seven years awake. Maybe we're all just dreams, rounded off with a nap."}
	gateRefuse         = "SHIFT'S NOT OVER"
)

// Father M's retirement party (O6) and the Fathers' memory lines, by times done.
var (
	fatherMParty = [3]string{"I don't do exorcisms. I do retirement parties. There's cake. Spiritual cake.",
		"Didn't I retire you last prestige? Fine. Speech, cake, light.",
		"Same porter, same room, same cake. Bless the cake."}
	fatherKQuest = "The witch's room again. Smells of sage and opinions." // 2nd and on, a Ritual in a questline Room
	fatherMQuest = [2]string{"I've sent this one over before. It came back with a tan.", "Same ghost, same room, same Tuesday. Bless the Tuesday."}
	fatherPQuest = [2]string{"Her circle isn't closed yet. Shall we go round again? Round is the whole point of circles.",
		"It's closed. The MacGuffin's in. Let's go round anyway, for the cardio."}
)

// itemID is one of Magenta's six items (items.go), one per beat 1-6, owned for good once its beat first gives it.
type itemID int

const (
	itemBall itemID = iota
	itemBroom
	itemGerald
	itemCat
	itemWard
	itemStorm
	itemCount
	noItem itemID = -1
)

var itemNames = [itemCount][2]string{ // name, blurb
	{nameShowDontTell + " (a crystal ball)", "REVEAL costs nothing. Previous owner saw this coming."},
	{"Broom, sport model", "You walk faster in ghost. Does not sweep. Has never swept. SHIFTs gears."},
	{nameGerald + ", a floating skull", "One Room's sightings pay more. " + nameGerald + " is not for sale. " + nameGerald + " is for rent."},
	{"Cat (black, unionized)", "Cleans spiritual dirt in ghost. Works the night shift. Two naps per shift, contractually."},
	{"Ward against leaving", "One Room's guests never flee. Guests may leave. Emotionally."},
	{"Storm in a jar (shake well)", "Tonight's thunderstorm: sightings pay double. A tempest. Pocket-sized. Do not open indoors. Do not open outdoors."},
}

// The Eternal Eldritch MacGuffin's card (macguffin.go).
var macguffinCard = struct {
	Name, Age, Job, Reveal, Book string
	Lines                        []string
}{nameMacguffin, "yes", "plot device", "REVEAL: IT REVEALS YOU",
	"Lovely stay. Did nothing. Will do nothing again. SHIFTed nobody.",
	[]string{"I am very important. Do not ask why.", "Everyone wanted me. Now I want a mint on my pillow.", "We are such stuff as receipts are made on."}}

// Street gossip (street.go), said near the Entrance by a passer-by, by how far the questline has got (questStage).
var (
	gossipBefore = []string{"That place grew a wing overnight. Hotels shouldn't do that.", "My cousin stayed there. Now he alphabetizes his soup."}
	gossipEarly  = []string{"Heard a witch checked in. Pointy hat. Very polite.", "Saw a lady buying sage. Enough sage to season a cathedral."}
	gossipLate   = []string{"The streamer guy's in there again. Says he saw nothing. Twice.", "Something works that corridor at night. It had a sign.",
		"Saw the Concierge carry a towel to nobody, flowers to a door and a key to a ghost. Odd job. Good legs."}
	gossipAfter      = []string{"Don't look at the tall one. It looks back with stars.", "Five stars on every review. Literally. They fall out of the page."}
	gossipAny        = []string{"Hell is empty. All the devils are on holiday in there.", "Whoever runs that place works a very long SHIFT."}
	gossipGhost      = []string{"The old lady's hotel. She's fussier dead.", "A living man who walks with us. In this economy."}
	gossipGhostAfter = "Even we don't go near Room {n}. It hums in a key that doesn't exist."
)

// Idle kitchen life (kitchenlife.go): Housekeeper lines at the fridge, the sink and the stove.
var (
	kitchenFridge = "Who keeps buying this much water? Ghosts don't even drink. Allegedly."
	kitchenSink   = "This mug has been haunted since Tuesday. It only haunts on Tuesdays."
	kitchenStove  = "Soup for the living. A smell of soup for everyone else."
	kitchenAny    = "Twelve hours on my feet. What a SHIFT."
	kitchenAfter  = "There's a star in the sugar bowl. Again. It's very sweet about it."
)

// The credits (credits.go), after the finale; then CONTINUE.
var credits = []string{
	"Made for GitHub Game Off 2026, theme SHIFT",
	"",
	"A GAME BY",
	"RICARDO MILAN     GABRIEL RODRIGUES",
	"",
	"Engine: Ebitengine. Language: Go.",
	"",
	"SPECIAL THANKS",
	"LimeZu, for the free Modern Interiors pack",
	"",
	"No MacGuffins were harmed. Nobody knows how to harm one.",
}

const creditsEpilogue = "The night shift's over. Thank you for staying."

// 08 W5-VERBS: lines the objectives need that circle.md leaves to the code.
var (
	o1LongAsk     = "Tell them the witch has arrived, with her book, her staff, her opinions and one small towel. All of it, in that order, with the cape."
	macguffinName = "Mag'ugh-Ghaal Uf-Fhinn the Eternal, Eldritch, Unclaimed and Unreturned, Third of Its Name, Keeper of the Thing Nobody Ordered, Guest of Rooms Not Yet Built, Who Is Owed A Mint"
)

// 08 W5 R1 GUIDE: the checklist's next-step rows (context/08-quests/copy.md "Next-step rows"). Each objective is an
// ordered list of steps; guide.go's stepIdx picks the current one from the quest and verb state. Dim is where the step
// is done (Any: the Terminal, either); Wait: nothing to do but wait (it never sends the player to a Mirror). The
// target is what the guidance marker outlines in Dim. Texts are not final (copy pass R3): swap them here only. {n} is a
// Room number, {item} the rolled Supply.
type tgKind int

const (
	tgNone     tgKind = iota
	tgGuest           // a quest guest by Key (its sprite, else its Room's plate)
	tgDoor            // the door of the Room of the guest Key ("handprint": the glowing door)
	tgBook            // the guest book on the counter
	tgTerminal        // the Terminal's screen
	tgSupply          // the Supply the ghost asked for
	tgSpirit          // O4's spirit
)

// Guest keys beyond the cast's: "ghostmate" (the ghost in Magenta's Room), "porter" (O6's ghost), "stranger" (O3 Lv2+),
// "ally" (O11's rolled ghost).
type target struct {
	Kind tgKind
	Key  string
}

type stepDef struct {
	Dim       Realm
	Any, Wait bool
	Text      string
	Tgt       target
}

var growStep = stepDef{Dim: Human, Any: true, Text: "GROW THE HOTEL", Tgt: target{tgTerminal, ""}}

var objSteps = map[string][]stepDef{
	"O1": {
		{Dim: Human, Text: "TALK TO MAGENTA", Tgt: target{tgGuest, "magenta"}},
		{Dim: Ghost, Text: "TELL HER ROOMMATE", Tgt: target{tgGuest, "ghostmate"}},
		{Dim: Ghost, Wait: true, Text: "WAIT FOR HER ROOMMATE"},
		{Dim: Human, Text: "TALK TO MAGENTA AGAIN", Tgt: target{tgGuest, "magenta"}},
	},
	"O2": {
		{Dim: Ghost, Text: "READ " + upper(nameMadame) + "'S NOTE", Tgt: target{tgBook, ""}},
		{Dim: Ghost, Text: "FIND THE REST OF THE NOTE", Tgt: target{tgBook, ""}},
		{Dim: Human, Text: "TELL " + upper(nameMagenta) + " WHAT IT SAYS", Tgt: target{tgGuest, "magenta"}},
	},
	"O3": {
		{Dim: Ghost, Text: "FIND " + upper(nameMadame) + "'S ROOM", Tgt: target{tgGuest, "celestine"}},
		{Dim: Human, Text: "MOVE THE STRANGER OUT", Tgt: target{tgGuest, "stranger"}},
		{Dim: Human, Text: "MOVE " + upper(nameMagda) + " TO ROOM {n}", Tgt: target{tgGuest, "magda"}},
		{Dim: Human, Wait: true, Text: "LET THEM TALK UNTIL DAWN"},
	},
	"O4": {
		{Dim: Human, Wait: true, Text: "WAIT FOR NIGHT"},
		{Dim: Human, Text: "LIGHT THE SPIRIT", Tgt: target{tgSpirit, ""}},
		{Dim: Human, Text: "LIGHT THE SPIRIT TWICE", Tgt: target{tgSpirit, ""}},
		{Dim: Ghost, Text: "ANSWER THE HANDPRINT", Tgt: target{tgDoor, "handprint"}},
	},
	"O5": {
		{Dim: Ghost, Text: "SEE WHAT " + upper(nameTim) + " NEEDS", Tgt: target{tgGuest, "tim"}},
		{Dim: Human, Text: "FETCH THE {item}", Tgt: target{tgSupply, ""}},
		{Dim: Ghost, Text: "TAKE IT TO " + upper(nameTim) + "'S DOOR", Tgt: target{tgDoor, "tim"}},
	},
	"O6": {
		{Dim: Human, Text: "LISTEN TO " + upper(nameTeo), Tgt: target{tgGuest, "teo"}},
		{Dim: Ghost, Text: "REVEAL ROOM {n}'S GHOST", Tgt: target{tgGuest, "porter"}},
		{Dim: Human, Text: "CALL " + upper(nameFatherM) + " TO ROOM {n}", Tgt: target{tgDoor, "porter"}},
		{Dim: Human, Wait: true, Text: "WAIT FOR " + upper(nameFatherM)},
	},
	"O7": {
		{Dim: Ghost, Text: "FIND " + upper(nameOsvaldo) + "'S ROOM", Tgt: target{tgGuest, "osvaldo"}},
		{Dim: Human, Text: "MOVE " + upper(nameMagda) + " TO ROOM {n}", Tgt: target{tgGuest, "magda"}},
	},
	"O8": {
		{Dim: Human, Any: true, Text: "BUY THE WREATH", Tgt: target{tgTerminal, ""}},
		{Dim: Human, Wait: true, Text: "WAIT FOR THE COURIER", Tgt: target{tgDoor, "celestine"}},
		{Dim: Ghost, Text: "PICK UP THE WREATH", Tgt: target{tgDoor, "celestine"}},
		{Dim: Ghost, Text: "GIVE " + upper(nameMadame) + " THE WREATH", Tgt: target{tgGuest, "celestine"}},
	},
	"O9": {
		{Dim: Ghost, Text: "ASK " + upper(nameTim) + " ABOUT THE KEY", Tgt: target{tgGuest, "tim"}},
		{Dim: Human, Text: "FIND THE OLDEST GUEST"},
		{Dim: Human, Text: "FIND THE OLDEST GUEST AT NIGHT"},
		{Dim: Ghost, Text: "GIVE " + upper(nameTim) + " THE KEY", Tgt: target{tgGuest, "tim"}},
	},
	"O10": {
		{Dim: Ghost, Text: "FIND " + upper(nameMadame) + "'S ROOM", Tgt: target{tgGuest, "celestine"}},
		{Dim: Human, Text: "MOVE " + upper(nameMagenta) + " TO ROOM {n}", Tgt: target{tgGuest, "magenta"}},
		{Dim: Human, Wait: true, Text: "LET THEM SLEEP UNTIL DAWN"},
	},
	"O11": {
		{Dim: Ghost, Text: "FIND THE GHOST WITH THE HAFT", Tgt: target{tgGuest, "ally"}},
		{Dim: Human, Text: "GIVE THE HAFT TO " + upper(nameMagenta), Tgt: target{tgGuest, "magenta"}},
	},
	"O12": {
		{Dim: Human, Text: "FIND THE ODD ENTRY", Tgt: target{tgBook, ""}},
		{Dim: Ghost, Text: "READ THE SAME PAGE", Tgt: target{tgBook, ""}},
	},
	"O13": {
		{Dim: Human, Any: true, Text: "BUY THE CRATE", Tgt: target{tgTerminal, ""}},
		{Dim: Human, Wait: true, Text: "WAIT IN " + upper(nameMadame) + "'S ROOM", Tgt: target{tgDoor, "celestine"}},
	},
}
