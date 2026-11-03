package turnplayer

import (
	"testing"

	"github.com/matryer/is"

	"github.com/domino14/macondo/board"
	"github.com/domino14/macondo/config"
	"github.com/domino14/macondo/game"
)

func TestCrossplayOptions(t *testing.T) {
	is := is.New(t)
	cfg := config.DefaultConfig()

	// The shell can choose the variant before or after the lexicon, and each
	// choice has to leave a combination that can start a game.
	assertStarts := func(opts *GameOptions) {
		t.Helper()
		_, err := game.NewBasicGameRules(cfg, opts.Lexicon.Name, opts.BoardLayoutName,
			opts.Lexicon.Distribution, game.CrossScoreAndSet, opts.Variant)
		is.NoErr(err)
	}

	opts := &GameOptions{}
	opts.SetDefaults(cfg)
	is.Equal(opts.Lexicon.Distribution, "English")
	is.NoErr(opts.SetVariant("crossplay"))
	is.Equal(opts.Variant, game.VarCrossplay)
	is.Equal(opts.BoardLayoutName, board.CrossplayLayout)
	is.Equal(opts.Lexicon.Distribution, game.CrossplayLetterDistribution)
	assertStarts(opts)

	is.NoErr(opts.SetLexicon([]string{"NWL23"}, cfg.WGLConfig()))
	is.Equal(opts.Lexicon.Distribution, game.CrossplayLetterDistribution)
	assertStarts(opts)

	// Going back must not strand Crossplay's board and tiles on a variant that
	// would reject them.
	is.NoErr(opts.SetVariant("classic"))
	is.Equal(opts.BoardLayoutName, board.CrosswordGameLayout)
	is.Equal(opts.Lexicon.Distribution, "english")
	assertStarts(opts)

	// Variant first, with defaults filled in afterward.
	opts = &GameOptions{}
	is.NoErr(opts.SetVariant("crossplay"))
	opts.SetDefaults(cfg)
	is.Equal(opts.Lexicon.Distribution, game.CrossplayLetterDistribution)
	is.Equal(opts.BoardLayoutName, board.CrossplayLayout)
	assertStarts(opts)
}
