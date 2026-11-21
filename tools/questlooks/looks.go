package main

import "image"

// inks: the accessories' colors. The outlines are the sheets' own.
var inks = map[rune]uint32{
	'k': 0x3a3a50, 'o': 0x46465e, // outline: dark below, lighter on top, as the sheets' heads
	'd': 0x4d4263, 'b': 0x5a4d75, 'h': 0x6c5981, // the witch hat's felt: shade, body, the top-left light
	'p': 0x9f74a8, 'q': 0x805e8e, // its band
	'y': 0xf6d868, 'g': 0xd09c3c, // gold: light, shade (the tiara, the cap's band, the lamp's glow)
	'c': 0x8ae6f0,                               // the tiara's gem
	'A': 0xe4e4ee, 'B': 0xc6c6d4, 'C': 0xacacbe, // Celestine's bun, her hair's silver
	'V': 0x9a6ee0, 'v': 0x8558cc, 'u': 0x6a44aa, // the bellhop cap's violet: light, body, shade
	'w': 0xf4f6fa, 's': 0xcdd6e2, 'n': 0x44608c, // the sailor's cap: white, its shade, the navy band
	'L': 0xfff4b0,                // the lamp's flame
	'S': 0xb8f0ff, 'T': 0x6cc0ea, // the phone's screen
}

// skins: each base sheet's skin colors, to find the hand.
var skins = map[string][]uint32{
	"Amelia": {0xbf8b78, 0xb57972, 0xa77a67, 0xaa5e56, 0xb2736d, 0xb58472},
	"Alex":   {0xffcbb0, 0xf6ae9f, 0xe19b9b, 0xffb893, 0xf69784},
	"Adam":   {0xffcbb0, 0xf6ae9f, 0xe19b9b, 0xffb893, 0xf69784, 0xbba386},
}

// legs: each base sheet's trousers and shoes, where a skirt goes.
var legs = map[string][]uint32{"Amelia": {0xc78c59, 0xb35e3f, 0xab4a36}}

// amelia: a swap of Amelia's ramps, each given light to dark as the sheet has them: hair ba8d5e 957350
// 8d7051 8a6552 674d49 (the last is also the eyes' lower pixel); skin bf8b78 b57972 a77a67 aa5e56 (the
// cheek) and the hands' b58472 b2736d; top b95d72 a85377; bottom c78c59 b35e3f and the shoes' ab4a36.
func amelia(hair [5]uint32, skin [4]uint32, top [2]uint32, bottom [3]uint32) swap {
	return swap{head: map[uint32]uint32{
		0xba8d5e: hair[0], 0x957350: hair[1], 0x8d7051: hair[2], 0x8a6552: hair[3], 0x674d49: hair[4],
		0xbf8b78: skin[0], 0xb57972: skin[1], 0xa77a67: skin[2], 0xaa5e56: skin[3], 0xb58472: skin[0], 0xb2736d: skin[1],
		0xb95d72: top[0], 0xa85377: top[1],
		0xc78c59: bottom[0], 0xb35e3f: bottom[1], 0xab4a36: bottom[2],
	}}
}

// alex: a swap of Alex's ramps: hair ba8d5e 957350 8d7051 8a6552 6f5446; skin ffcbb0 f6ae9f e19b9b and the
// hands' ffb893 f69784; jacket ae4a52 a2394b 6f494d 5a444a (5a444a is also the eyes' lower pixel, kept,
// and the shoes); shirt 9da3b7 8b8bab; trousers 8091a5 787d93.
func alex(hair [5]uint32, skin [3]uint32, jacket [4]uint32, shirt, trousers [2]uint32, shoes uint32) swap {
	return swap{
		head: map[uint32]uint32{
			0xba8d5e: hair[0], 0x957350: hair[1], 0x8d7051: hair[2], 0x8a6552: hair[3], 0x6f5446: hair[4],
			0xffcbb0: skin[0], 0xf6ae9f: skin[1], 0xe19b9b: skin[2], 0xffb893: skin[0], 0xf69784: skin[1],
			0xae4a52: jacket[0], 0xa2394b: jacket[1], 0x6f494d: jacket[2],
			0x9da3b7: shirt[0], 0x8b8bab: shirt[1],
			0x8091a5: trousers[0], 0x787d93: trousers[1],
		},
		body: map[uint32]uint32{0x5a444a: jacket[3]},
		legs: map[uint32]uint32{0x5a444a: shoes},
	}
}

var looks = []look{
	{name: "Magenta", base: "Amelia", // white skin, red hair, a green shawl over a purple robe, the pointed hat low on her head
		swap: amelia([5]uint32{0xe8643c, 0xcf4a30, 0xc2432f, 0xb03a2e, 0x842c32}, [4]uint32{0xf6ebe6, 0xe6d3d0, 0xd6bfc0, 0xe39a9c},
			[2]uint32{0x5aa66a, 0x43875a}, [3]uint32{0x8a62b0, 0x73509a, 0x573d7a}),
		skirt: &skirt{cloth: [2]uint32{0x8a62b0, 0x73509a}},
		accessories: []accessory{{over: 6, art: [4][]string{
			{ // right: the tip bends back
				"....ooo.........",
				"...ohhbo........",
				"...kkbbbo.......",
				".....obbdk......",
				"....ohbbbdk.....",
				"xxxxohbbbbdkxxxx",
				"xxxohbbbbbdkxxxx",
				"xxxoppppppqkxxxx",
				".oohbbbbbbbdkkk.",
				"ohhbbbbbbbbbbddk",
				".kkkkkkkkkkkkkk.",
			},
			{ // up
				"......oo........",
				".....ohbo.......",
				".....ohbdk......",
				"....ohbbdk......",
				"....ohbbbdk.....",
				"xxxohbbbbdkxxxxx",
				"xxxohbbbbbdkxxxx",
				"xxxoppppppqkxxxx",
				".oohbbbbbbbdkkk.",
				"ohhbbbbbbbbbbddk",
				".kkkkkkkkkkkkkk.",
			},
			{ // left
				".........ooo....",
				"........ohbbo...",
				".......ohbbkk...",
				"......ohbdk.....",
				".....ohbbbdk....",
				"xxxxohbbbbdkxxxx",
				"xxxxohbbbbbdkxxx",
				"xxxxoppppppqkxxx",
				".ooohbbbbbbbdkk.",
				"ohhbbbbbbbbbbddk",
				".kkkkkkkkkkkkkk.",
			},
			{ // down
				"........oo......",
				".......ohbo.....",
				"......ohbdk.....",
				"......ohbbdk....",
				".....ohbbbdk....",
				"xxxxxohbbbbdkxxx",
				"xxxxohbbbbbdkxxx",
				"xxxxoppppppqkxxx",
				".ooohbbbbbbbdkk.",
				"ohhbbbbbbbbbbddk",
				".kkkkkkkkkkkkkk.",
			},
		}}}},
	{name: "Celestine", base: "Amelia", // a rose opera gown to the floor, silver hair up in a bun, a small tiara
		swap: amelia([5]uint32{0xe4e4ee, 0xc6c6d4, 0xbcbccb, 0xacacbe, 0x8686a0}, [4]uint32{0xf0d6cc, 0xdfbdb8, 0xcfa9a8, 0xd98c98},
			[2]uint32{0xe87aa0, 0xd0608c}, [3]uint32{0xe87aa0, 0xd0608c, 0xb04a78}),
		skirt: &skirt{cloth: [2]uint32{0xe87aa0, 0xd0608c}},
		accessories: []accessory{{over: 3, art: [4][]string{
			{ // right
				".....oooo.......",
				"....oABBCo......",
				"....kBBCCk......",
				"................",
				"........ycy.....",
				"........gyyg....",
			},
			{ // up: the bun, the tiara's two ends
				"......oooo......",
				".....oABBCo.....",
				".....kBBCCk.....",
				"................",
				"....y......y....",
				"................",
			},
			{ // left
				".......oooo.....",
				"......oABBCo....",
				"......kBBCCk....",
				"................",
				".....ycy........",
				"....gyyg........",
			},
			{ // down
				"......oooo......",
				".....oABBCo.....",
				".....kBBCCk.....",
				"................",
				".....y.cc.y.....",
				".....gyyyyg.....",
			},
		}}}},
	{name: "Magda", base: "Amelia", // grey hair, a long navy coat, a small lamp in her hand
		swap: amelia([5]uint32{0xcfcac6, 0xafa9a8, 0xa59fa0, 0x979198, 0x787284}, [4]uint32{0xecc0a6, 0xdba892, 0xc99684, 0xcf8078},
			[2]uint32{0x5670b4, 0x465a98}, [3]uint32{0x5670b4, 0x465a98, 0x6a5048}),
		skirt: &skirt{cloth: [2]uint32{0x5670b4, 0x465a98}, hem: 2},
		accessories: []accessory{{hand: true,
			at: [4]image.Point{{1, -2}, {-1, 1}, {-4, -2}, {-4, -1}},
			art: [4][]string{
				{".oo.", "oLLo", "oLyo", "oyyo", "kkkk"},
				{".o.", "oLo", "oLo", "oyo", "kkk"}, // from behind: what shows past her side
				{".oo.", "oLLo", "oyLo", "oyyo", "kkkk"},
				{".oo.", "oLLo", "oLyo", "oyyo", "kkkk"},
			}}}},
	{name: "Tim", base: "Alex", // the old bellhop: white hair, a violet suit with gold buttons, a pillbox cap
		swap: alex([5]uint32{0xe2e0e6, 0xc8c5cf, 0xbebbc6, 0xafacb9, 0x908da0}, [3]uint32{0xc08660, 0xa87052, 0x8e5a48},
			[4]uint32{0x9468dc, 0x8558cc, 0x8558cc, 0x7648be}, [2]uint32{0xf6d868, 0xd09c3c}, [2]uint32{0x8558cc, 0x6a44aa}, 0x4a4458),
		accessories: []accessory{{over: 2, art: [4][]string{
			{ // right
				".....ooooo......",
				"....oVvvvuk.....",
				"....ovvvvuk.....",
				"....oyyyygk.....",
				"....kkkkkkk.....",
			},
			{ // up
				".....oooooo.....",
				"....oVvvvvuk....",
				"....ovvvvvuk....",
				"....oyyyyygk....",
				"....kkkkkkkk....",
			},
			{ // left
				"......ooooo.....",
				".....oVvvvuk....",
				".....ovvvvuk....",
				".....oyyyygk....",
				".....kkkkkkk....",
			},
			{ // down
				".....oooooo.....",
				"....oVvvvvuk....",
				"....ovvvvvuk....",
				"....oyyyyygk....",
				"....kkkkkkkk....",
			},
		}}}},
	{name: "Teo", base: "Alex", // a green hoodie, white sneakers, his phone held up
		swap: alex([5]uint32{0x76604f, 0x624d42, 0x5c473e, 0x55413b, 0x4a3838}, [3]uint32{0xf0b890, 0xde9e80, 0xcf8a7c},
			[4]uint32{0x5cac6c, 0x4f9c60, 0x4f9c60, 0x44895a}, [2]uint32{0xeef2ee, 0xcfd8d0}, [2]uint32{0x5a6890, 0x4c5a80}, 0xe6e6ee),
		accessories: []accessory{{hand: true,
			at: [4]image.Point{{1, -5}, {-1, -5}, {-3, -5}, {-4, -3}},
			art: [4][]string{
				{"ooo", "oSk", "oSk", "oTk", "kkk"},
				{"ooo", "oSk", "oSk", "oTk", "kkk"},
				{"ooo", "oSk", "oSk", "oTk", "kkk"},
				{"ooo", "oSk", "oSk", "oTk", "kkk"},
			}}}},
	{name: "Osvaldo", base: "Adam", // pale blue from his white hair to his shoes, a sailor's cap
		// Adam: hair 959d58 687253 5f694a 5d6043 (5f694a is also the eyes' lower pixel; 687253 and 5d6043 the
		// trousers); skin ffcbb0 f6ae9f e19b9b, bba386 under the hair, the hands' ffb893 f69784; shirt 9f74a8
		// 805e8e (805e8e also the shoes).
		swap: swap{
			head: map[uint32]uint32{
				0x959d58: 0xc6d8ea, 0x687253: 0xaac0d8, 0x5f694a: 0x8fa8c4, 0x5d6043: 0x7c96b4,
				0xffcbb0: 0xe6f0f8, 0xf6ae9f: 0xcbdbea, 0xe19b9b: 0xb4c6da, 0xbba386: 0xc2d2e2, 0xffb893: 0xe6f0f8, 0xf69784: 0xcbdbea,
				0x9f74a8: 0x8fb8e0, 0x805e8e: 0x6f9acb,
			},
			legs: map[uint32]uint32{0x687253: 0x5c7fae, 0x5d6043: 0x4f709e, 0x805e8e: 0x44608c},
		},
		accessories: []accessory{{over: 1, art: [4][]string{
			{ // right
				".....oooooooo...",
				"....owwwwwwwwo..",
				"....kswwwwwwsk..",
				".....knnnnnnk...",
			},
			{ // up
				"...oooooooo.....",
				"..owwwwwwwwo....",
				"..kswwwwwwsk....",
				"...knnnnnnk.....",
			},
			{ // left
				"...oooooooo.....",
				"..owwwwwwwwo....",
				"..kswwwwwwsk....",
				"...knnnnnnk.....",
			},
			{ // down
				"...oooooooo.....",
				"..owwwwwwwwo....",
				"..kswwwwwwsk....",
				"...knnnnnnk.....",
			},
		}}}},
}
