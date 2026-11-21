package textpos

import (
	"testing"

	"github.com/whoisnighter/KataFighter/formats/diag"
)

func TestToRange(t *testing.T) {
	// "é" is 2 bytes / 1 UTF-16 unit, "😀" is 4 bytes / 2 units.
	ls := Lines("  é😀 abc  \r\nx")
	cases := []struct {
		r    diag.Range
		want Range
	}{
		// Byte col 9 is the space after the emoji: 2 + 1 + 2 = 5 units.
		{diag.At(1, 9), Range{Position{0, 5}, Position{0, 6}}},
		// Byte col 10 starts "abc"; col 6 is inside the emoji and snaps to its start.
		{diag.At(1, 10), Range{Position{0, 6}, Position{0, 9}}},
		{diag.At(1, 6), Range{Position{0, 3}, Position{0, 5}}},
		// Whole line: without indentation, trailing blanks or '\r'.
		{diag.At(1, 0), Range{Position{0, 2}, Position{0, 9}}},
		{diag.Range{Start: diag.Pos{Line: 1, Col: 3}, End: diag.Pos{Line: 2, Col: 2}}, Range{Position{0, 2}, Position{1, 1}}},
		// Whole file, and a line past the end.
		{diag.Range{}, Range{}},
		{diag.At(9, 1), Range{Position{1, 0}, Position{1, 0}}},
	}
	for _, c := range cases {
		if got := ToRange(ls, c.r); got != c.want {
			t.Errorf("ToRange(%v) = %v, want %v", c.r, got, c.want)
		}
	}
}
