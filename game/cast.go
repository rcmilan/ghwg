package game

import (
	"math/rand/v2"

	"ghosthotel/entities"
)

// 08 W4 (T373 seam): context/08-quests/cast.md as data. Lines stay editable: change them there and here.
// Every guest carries an entities.Identity (guest.Who), rolled at booking from g.ident, its own rng, so the
// sim's g.rng never shifts. The BOOK slice adds the named arrivals (Who.Key); until then every guest is generated.

// Quirk and its constants are entities', named here so the game's code reads Skeptic, not entities.Skeptic.
type Quirk = entities.Quirk

const (
	NoQuirk   = entities.NoQuirk
	Medium    = entities.Medium
	Skeptic   = entities.Skeptic
	BigTipper = entities.BigTipper
	NightOwl  = entities.NightOwl
	NeatFreak = entities.NeatFreak
	ShowOff   = entities.ShowOff
	Shy       = entities.Shy
	Jealous   = entities.Jealous
	Gentle    = entities.Gentle
	Homebody  = entities.Homebody
)

// quirkNames: each quirk's display name, by Quirk.
var quirkNames = [...]string{"None", "Medium", "Skeptic", "Big tipper", "Night owl", "Neat freak", "Show-off", "Shy", "Jealous", "Gentle", "Homebody"}

func quirkName(q Quirk) string { return quirkNames[q] }

// humanQuirks and ghostQuirks: what a generated guest of each realm rolls from.
var (
	humanQuirks = []Quirk{Medium, Skeptic, BigTipper, NightOwl, NeatFreak}
	ghostQuirks = []Quirk{ShowOff, Shy, Jealous, Gentle, Homebody}
)

const (
	quirkOdds = 3 // a generated guest has a quirk 1 time in quirkOdds
	vegOdds   = 5 // the draw a human once spent on Vegetarian, kept so the identity stream does not move
)

// member is one person of a named human entry: a Party of two has two, one card each (A/D flips).
type member struct {
	Name    string
	Age     int
	AgeNote string // Age in words, for Age 0
	Sprite  string // the base character: Adam, Amelia or Alex
	Look    string // its recolour
	Lines   []string
}

// namedHuman: one entry of cast.md's named humans (7 people, 6 entries).
type namedHuman struct {
	Key     string
	Members []member
	Job     string
	Quirk   Quirk
	Note    string    // the entry's setup, not shown
	Visit   [3]string // visit 1..3; from visit 4 they rotate
	Dread   string
	Fled    string // empty for a Skeptic: he never flees
}

var namedHumans = []namedHuman{
	{Key: "magda", Job: "retired lighthouse keeper", Quirk: Medium,
		Members: []member{{"Magda Feltz", 67, "", "Amelia", "grey hair, navy coat", []string{"My Osvaldo always loved a hotel.", "He whistled off-key. I'd know it anywhere.", "Forty years of lighthouse. I sleep with a lamp on."}}},
		Note:    "Comes back looking for her late husband, who haunts the hotel (see Osvaldo).",
		Visit: [3]string{"Lovely place. The walls hum. Osvaldo would have liked it.",
			"I heard whistling at 3 a.m. He always whistled off-key.",
			"He was there by the window. He still owes me a dance. 5 stars."},
		Dread: "Something is angry here, and it isn't my husband.",
		Fled:  "Not this time, love. Not this room."},
	{Key: "teo", Job: "paranormal streamer", Quirk: Skeptic,
		Members: []member{{"Teo Barros", 29, "", "Alex", "green hoodie", []string{"Smash that subscribe if you see anything.", "Is this the haunted wing? Asking for my chat.", "Battery at 4%. Ghosts, now would be great."}}},
		Note:    "He hunts ghosts on camera and never sees one.",
		Visit: [3]string{"Four hours of footage. Zero ghosts. Bed was comfy though.",
			"My thermal camera melted. Still no ghosts. 3 stars.",
			"Chat says something waved behind me the whole stream. Chat lies."},
		Dread: "Vibes were off. Also no ghosts. Refund?"},
	{Key: "gus", Job: "vacuum cleaner salesman", Quirk: BigTipper,
		Members: []member{{"Gus Pimentel", 52, "", "Adam", "brown suit", []string{"Have you considered the Turbo 9000?", "Your rugs are crying out for suction.", "Twelve attachments. Lifetime warranty."}}},
		Visit: [3]string{"Sold two vacuums in the lobby. Keep the change!",
			"Your carpet deserves the Turbo 9000. Left a brochure. And a tip.",
			"The maid bought one! Best hotel in the business."},
		Dread: "Even the Turbo 9000 couldn't clean that feeling.",
		Fled:  "The vacuum turned itself on. I'm not selling that model anymore."},
	{Key: "noor", Job: "night-shift nurse", Quirk: NightOwl,
		Members: []member{{"Noor Haddad", 23, "", "Amelia", "teal scrubs", []string{"I'll sleep when I'm on vacation. This is not vacation.", "Wake me if anyone codes. Nobody will.", "Is there coffee? Real coffee?"}}},
		Visit: [3]string{"Room service at 4 a.m.! You people get it.",
			"Someone knocked at 2, 3 and 4. I thought it was my pager.",
			"Napped for six minutes. Personal best. Thank you."},
		Dread: "I've worked ERs that felt cozier.",
		Fled:  "Twelve-hour shifts I can do. Whatever that was, no."},
	{Key: "bealou", Job: "on their 50th honeymoon", Quirk: NeatFreak,
		Members: []member{
			{"Bea Marchetti", 74, "", "Amelia", "matching red sweater", []string{"Fifty years and he still can't fold a towel.", "Lou, stop touching the thermostat.", "We had our first fight in a hotel like this."}},
			{"Lou Marchetti", 76, "", "Adam", "matching red sweater", []string{"She says that every year.", "I'm not touching the thermostat.", "Our first fight? She won. She always does."}}},
		Note: "A named Party of two; one card, A/D flips between them.",
		Visit: [3]string{"Towels folded right. Lou cried a little. — B & L",
			"A crumb under the bed. We noticed. We forgive you. — B & L",
			"Spotless. We'll be back for the 51st. — B & L"},
		Dread: "Lou says it's the plumbing. Lou is wrong. — B",
		Fled:  "We survived 1979. We are not surviving this. — B & L"},
	{Key: "wendell", Job: "insurance auditor", Quirk: Skeptic,
		Members: []member{{"Wendell Crane", 38, "", "Alex", "grey suit, glasses", []string{"Just here to assess the building.", "Is that crack in the plaster recent?", "Everything is a liability. Including you."}}},
		Note:    "He is here for the hotel's insurance and keeps finding nothing in writing.",
		Visit: [3]string{"Claims of 'ghost damage': unsubstantiated. Breakfast: adequate.",
			"Filed form 12-B. The form refiled itself. Will investigate.",
			"Recommending the policy be renewed. Do not ask me why."},
		Dread: "Risk level: unquantifiable."},
	{Key: "morwenna", Job: "herbalist", Quirk: NightOwl,
		Members: []member{{"Morwenna Vale", 0, "ask the moon", "Amelia", "black dress, green shawl", []string{"Lovely energy. Very... crowded.", "The walls remember. Do you?", "Don't move the salt by the door."}}},
		Note:    "The Witch's setup (W3): for now an ordinary named guest whose comments hint at what she is. When W3 is built she becomes the Witch.",
		Visit: [3]string{"Left some sage in the drawer. You'll thank me.",
			"The darker rooms have such character. Book me one next time.",
			"Your guests on the other side say hello. Some of them, anyway."},
		Dread: "Now this is a room. Finally.",
		Fled:  "Too bright, too cheerful. I'll come back when it's worse."},
}

// namedGhost: one of cast.md's 3 named ghosts. Its notebook lines show only in ghost; it has no dread or fled.
type namedGhost struct {
	Key    string
	Name   string
	Died   int
	Kind   string // its Phantasm kind: a ghostKinds Name
	Job    string // what it was, empty if the card says none
	Sprite string
	Look   string
	Quirk  Quirk
	Lines  []string
	Fact   string    // shown by REVEAL
	Visit  [3]string // visit 1..3; from visit 4 they rotate
}

var namedGhosts = []namedGhost{
	{Key: "osvaldo", Name: "Osvaldo Feltz", Died: 1987, Kind: "Wisp", Sprite: "Adam", Look: "pale blue, sailor's cap", Quirk: Gentle,
		Lines: []string{"Has anyone seen my Magda?", "She hums when she can't sleep. Have you heard her?", "The sea took me. The hotel kept me."},
		Fact:  "Drowned saving the lighthouse cat. The cat is fine.",
		Visit: [3]string{"Nobody heard me whistle. Again.",
			"A lady said my name tonight. I think it was her.",
			"She danced with me. I can go now. Thank you, innkeeper."}},
	{Key: "ned", Name: "Ned Okafor", Died: 1959, Kind: "Specter", Job: "the old bellhop", Sprite: "Alex", Look: "violet, bellhop hat, a suitcase", Quirk: Homebody,
		Lines: []string{"Bags, sir? Bags? Anybody?", "Room 4? Up the stairs, I'll take it.", "Sixty years and not one tip."},
		Fact:  "He never finished his last delivery. The suitcase is still locked.",
		Visit: [3]string{"Carried a bag up four floors. Nobody tipped.",
			"The new man at the counter works too hard. I like him.",
			"Found the key to the suitcase. Not opening it yet."}},
	{Key: "celestine", Name: "Madame Celestine Arbogast", Died: 1923, Kind: "Poltergeist", Job: "the hotel's first owner", Sprite: "Amelia", Look: "rose, opera gown", Quirk: ShowOff,
		Lines: []string{"This is MY hotel, darling.", "Mind the chandelier, darling.", "Who chose these curtains? Out with them."},
		Fact:  "She sang one note so high on opening night that the chandelier fell. She was under it.",
		Visit: [3]string{"The wallpaper is a crime. I rattled it in protest.",
			"You've kept my hotel alive. Grudgingly impressed.",
			"Fine. You may run it. I'll still rattle the chandelier on Tuesdays."}},
}

// Generated humans.
var (
	humanFirst = []string{"Ana", "Ben", "Clara", "Davi", "Elif", "Finn", "Greta", "Hugo", "Iris", "Jonas", "Kira", "Leo", "Mara", "Nils", "Olga", "Paulo", "Rosa", "Sami", "Tilde", "Vera"}
	humanLast  = []string{"Abreu", "Berg", "Costa", "Dahl", "Ek", "Fontaine", "Gray", "Holm", "Ito", "Jansen", "Kahn", "Lund", "Moreau", "Novak", "Ortiz", "Pike", "Quinn", "Rossi", "Silva", "Thorne"}
	humanJobs  = []string{"baker", "bus driver", "chess coach", "clown", "dentist", "florist", "librarian", "locksmith", "magician", "mail carrier", "opera singer", "plumber", "retired pirate (self-described)", "sommelier", "taxidermist", "teacher", "tour guide", "translator", "watchmaker", "yoga instructor"}
	humanLines = []string{"Just passing through.", "Is breakfast included?", "I read the reviews. Mixed.", "My aunt stayed here once. She never said much about it.", "Quiet room, please.", "Is the minibar free?",
		"Is the wifi password 'boo'?", "The lift made a noise. A sad noise.", "I asked for a sea view. This is a wall.", "Do you get many complaints? About the cold spots?",
		"Lovely wallpaper. Was it always moving?", "Wake-up call at seven. A human one, please.", "Who left the bath running? I'm alone.", "First time in town. Last, maybe.",
		"The bed's comfy. The corner isn't.", "Is it normal for the mirror to fog by itself?", "I'm a light sleeper. Very light.", "Late checkout? Very late?"}
	// The notebook's lines for a generated human (BOOK reads them). The last fled one, a scribble, has no words.
	notebookCozy  = []string{"Slept like a baby.", "Would haunt again.", "Best pillows in town.", "The concierge has a lovely walk.", "10/10, the walls only whispered once."}
	notebookDread = []string{"Never felt so watched.", "The mirror blinked first.", "Please check room's vents. And soul.", "Don't take the dread room. Too late for me."}
	notebookFled  = []string{"Left at 3 a.m. Keep the deposit.", "NOPE.", "My hair is white now. It was brown.", ""}
)

const (
	humanAgeMin  = 18
	humanAgeMax  = 80
	ghostDiedMin = 1820
	ghostDiedMax = 1990
)

// Generated ghosts.
var (
	ghostFirst = []string{"Agatha", "Barnaby", "Cornelius", "Dorothea", "Ebenezer", "Florence", "Gideon", "Hortense", "Ignatius", "Josephine", "Lavinia", "Mortimer", "Octavia", "Percival", "Prudence", "Rupert", "Silas", "Theodora", "Ursula", "Wilhelmina"}
	ghostLast  = []string{"Ashgrove", "Blackwood", "Clatter", "Dimsdale", "Fogg", "Gravesend", "Hollow", "Knell", "Marrow", "Murk", "Nettle", "Pallid", "Quill", "Rattle", "Shroud", "Thistle", "Vane", "Wick", "Whisper", "Yew"}
	ghostCause = []string{"fell down the stairs", "choked on a wedding cake", "struck by a falling chandelier", "lost a duel over a teaspoon", "laughed too hard at a joke (it wasn't that funny)", "sleepwalked off the roof", "ate the mushrooms", "locked in the wine cellar", "startled by a mime", "waited too long for room service"}
	ghostLines = []string{"Who moved my staircase?", "I was here first.", "Is it still 1912?", "Don't mind me. Nobody does.", "Room service never came.", "Boo. Sorry. Habit.",
		"The pipes sing better than I did.", "Have you seen my head? Kidding. Mostly.", "Tell the new guests I'm friendly.", "I only rattle on weekends.",
		"Is it Tuesday? It's always Tuesday.", "This used to be my bedroom. And my grave.", "The living walk so loudly.", "I'd tip you, but my coins went through.",
		"Keep the lights low. It's flattering.", "I was a guest once. I never left.", "Don't move the furniture. I'm attached.", "One more night. Then I'll go. Probably."}
	// The notebook's lines for a generated ghost: it left on its own, or Father M sent it over.
	notebookPassed   = []string{"Finally checked out. Lovely stay.", "Off to the light. The pillows were better here.", "Ninety years. Worth it."}
	notebookBanished = []string{"Rude.", "I wasn't finished.", "Tell the priest I'll be back. I won't, but tell him."}
)

func choose[T any](r *rand.Rand, s []T) T { return s[r.IntN(len(s))] }

// lineSet: a generated guest's card lines, 3 of pool from one draw so the identity stream stays the one it was.
func lineSet(r *rand.Rand, pool []string) []string {
	i, n := r.IntN(len(pool)), len(pool)
	return []string{pool[i], pool[(i+5)%n], pool[(i+11)%n]}
}

// seedIdent (re)seeds the identity rolls from the Run's seed, apart from rng, chat and words.
func (g *Game) seedIdent(seed uint64) { g.ident = rand.New(rand.NewPCG(seed, seed+3)) }

// rollQuirk: no quirk, or about 1 time in quirkOdds one of the realm's, once the hotel has grown: before, the player
// can't read a quirk yet and the early Ecto must not stall (T376, TestPacing). It draws either way, so the rest of
// the identity keeps its rolls.
func (g *Game) rollQuirk(from []Quirk) Quirk {
	if g.ident.IntN(quirkOdds) == 0 {
		if q := choose(g.ident, from); g.grown() {
			return q
		}
	}
	return NoQuirk
}

// rollHuman gives a human guest a generated Identity, when its Party books (arrive).
func (g *Game) rollHuman(guest *entities.Guest) {
	guest.Who = entities.Identity{
		Name:  choose(g.ident, humanFirst) + " " + choose(g.ident, humanLast),
		Age:   humanAgeMin + g.ident.IntN(humanAgeMax-humanAgeMin+1),
		Job:   choose(g.ident, humanJobs),
		Lines: lineSet(g.ident, humanLines),
		Quirk: g.rollQuirk(humanQuirks),
	}
	g.ident.IntN(vegOdds)
}

// rollGhost gives a ghost guest a generated Identity, when it is created (newGhost).
func (g *Game) rollGhost(guest *entities.Guest) {
	guest.Who = entities.Identity{
		Name:  choose(g.ident, ghostFirst) + " " + choose(g.ident, ghostLast),
		Died:  ghostDiedMin + g.ident.IntN(ghostDiedMax-ghostDiedMin+1),
		Cause: choose(g.ident, ghostCause),
		Lines: lineSet(g.ident, ghostLines),
		Quirk: g.rollQuirk(ghostQuirks),
	}
}
