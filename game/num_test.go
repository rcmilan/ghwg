package game

import "testing"

// T174: fmtNum floors to 3 significant digits, so a number never shows more than it is.
func TestFmtNum(t *testing.T) {
	for _, c := range []struct {
		n    Num
		want string
	}{
		{0, "0"}, {1, "1"}, {999, "999"}, {999.9, "999"},
		{1000, "1.00K"}, {1001, "1.00K"}, {1150, "1.15K"}, {12345, "12.3K"}, {999999, "999K"},
		{1e6, "1.00M"}, {999e6, "999M"}, {999999999, "999M"},
		{1e9, "1.00B"}, {1.23e9, "1.23B"}, {1e12, "1.00T"}, {1e15, "1.00Qa"},
		{999.99e15, "999Qa"}, {1e18 - 1e6, "999Qa"},
		{1e18, "1.00e18"}, {1.234e18, "1.23e18"}, {1e300, "1.00e300"},
		{-1500, "-1.50K"},
	} {
		if got := fmtNum(c.n); got != c.want {
			t.Errorf("fmtNum(%v) = %q, want %q", float64(c.n), got, c.want)
		}
	}
}
