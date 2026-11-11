package game

import (
	"math"
	"strconv"
	"strings"
)

// Num is every big number in the game (D178): Money, Ecto, taps. A named float64 reaches 1e308; swapping it
// for a mantissa/exponent pair later touches this file only.
type Num float64

var numSuffixes = []string{"K", "M", "B", "T", "Qa", "Qi"}

// fmtNum: an integer below 1,000, then 3 significant digits with a suffix (1.00K, 12.3K, 999M), then
// scientific from 1e18 (1.23e18). It floors, so 999,999 reads 999K, never 1000K.
func fmtNum(n Num) string {
	v := float64(n)
	switch {
	case v < 0:
		return "-" + fmtNum(-n)
	case v < 1e3:
		return strconv.Itoa(int(v))
	case v >= 1e18:
		return strings.Replace(strconv.FormatFloat(v, 'e', 2, 64), "e+", "e", 1)
	}
	s := -1
	for v >= 1e3 {
		v, s = v/1e3, s+1
	}
	d := 2 // decimals: 3 significant digits in all
	if v >= 100 {
		d = 0
	} else if v >= 10 {
		d = 1
	}
	p := math.Pow10(d)
	// the nudge undoes float error (1.15*100 = 114.99…) without rounding a true 999.999… up
	return strconv.FormatFloat(math.Floor(v*p*(1+1e-14))/p, 'f', d, 64) + numSuffixes[s]
}
