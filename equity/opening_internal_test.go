package equity

import (
	"testing"

	"github.com/matryer/is"

	"github.com/domino14/macondo/board"
)

// The squares next to a 2LS, for the boards that once had them hardcoded.
func TestExposes2LS(t *testing.T) {
	for _, tc := range []struct {
		name string
		desc []string
		want map[int]bool
	}{
		{"standard", board.CrosswordGameBoard, map[int]bool{2: true, 6: true, 8: true, 12: true}},
		{"super", board.SuperCrosswordGameBoard, map[int]bool{5: true, 9: true, 11: true, 15: true}},
		// The rows beside the center have only 3Ls.
		{"crossplay", board.CrossplayBoard, map[int]bool{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)
			b := board.MakeBoard(tc.desc)
			for j := range b.Dim() {
				is.Equal(exposes2LS(b, j, false), tc.want[j])
				is.Equal(exposes2LS(b, j, true), tc.want[j])
			}
		})
	}
}
