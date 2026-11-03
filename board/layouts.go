package board

var (
	// CrosswordGameBoard is a board for a fun Crossword Game, featuring lots
	// of wingos and blonks.
	CrosswordGameBoard []string
	// SuperCrosswordGameBoard is a board for a bigger Crossword game, featuring
	// even more wingos and blonks.
	SuperCrosswordGameBoard []string
	// CrossplayBoard is the board for the New York Times's Crossplay. Its
	// center square has no bonus.
	CrossplayBoard []string
)

const (
	CrosswordGameLayout      = "CrosswordGame"
	SuperCrosswordGameLayout = "SuperCrosswordGame"
	CrossplayLayout          = "CrossplayGame"
)

func init() {
	CrosswordGameBoard = []string{
		`=  '   =   '  =`,
		` -   "   "   - `,
		`  -   ' '   -  `,
		`'  -   '   -  '`,
		`    -     -    `,
		` "   "   "   " `,
		`  '   ' '   '  `,
		`=  '   -   '  =`,
		`  '   ' '   '  `,
		` "   "   "   " `,
		`    -     -    `,
		`'  -   '   -  '`,
		`  -   ' '   -  `,
		` -   "   "   - `,
		`=  '   =   '  =`,
	}
	CrossplayBoard = []string{
		`"  =   '   =  "`,
		` -    " "    - `,
		`    '     '    `,
		`=  '   -   '  =`,
		`  '  "   "  '  `,
		`    "  '  "    `,
		` "           " `,
		`'  - '   ' -  '`,
		` "           " `,
		`    "  '  "    `,
		`  '  "   "  '  `,
		`=  '   -   '  =`,
		`    '     '    `,
		` -    " "    - `,
		`"  =   '   =  "`,
	}
	SuperCrosswordGameBoard = []string{
		`~  '   =  '  =   '  ~`,
		` -  "   -   -   "  - `,
		`  -  ^   - -   ^  -  `,
		`'  =  '   =   '  =  '`,
		` "  -   "   "   -  " `,
		`  ^  -   ' '   -  ^  `,
		`   '  -   '   -  '   `,
		`=      -     -      =`,
		` -  "   "   "   "  - `,
		`  -  '   ' '   '  -  `,
		`'  =  '   -   '  =  '`,
		`  -  '   ' '   '  -  `,
		` -  "   "   "   "  - `,
		`=      -     -      =`,
		`   '  -   '   -  '   `,
		`  ^  -   ' '   -  ^  `,
		` "  -   "   "   -  " `,
		`'  =  '   =   '  =  '`,
		`  -  ^   - -   ^  -  `,
		` -  "   -   -   "  - `,
		`~  '   =  '  =   '  ~`,
	}
}
