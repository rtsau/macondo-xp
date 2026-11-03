package turnplayer

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"

	wglconfig "github.com/domino14/word-golib/config"
	"github.com/domino14/word-golib/tilemapping"

	"github.com/domino14/macondo/board"
	"github.com/domino14/macondo/config"
	"github.com/domino14/macondo/game"
	pb "github.com/domino14/macondo/gen/api/proto/macondo"
	"github.com/domino14/macondo/lexicon"
)

type Lexicon struct {
	Name         string
	Distribution string
}

func (lex *Lexicon) ToDisplayString() string {
	return fmt.Sprintf("%s [%s]", lex.Name, lex.Distribution)
}

type GameOptions struct {
	Lexicon         *Lexicon
	ChallengeRule   pb.ChallengeRule
	BoardLayoutName string
	Variant         game.Variant
}

func (opts *GameOptions) SetDefaults(cfg *config.Config) {
	if opts.Lexicon == nil {
		opts.Lexicon = &Lexicon{cfg.GetString(config.ConfigDefaultLexicon), cfg.GetString(config.ConfigDefaultLetterDistribution)}
		log.Info().Msgf("using default lexicon %v", opts.Lexicon)
	}
	if opts.BoardLayoutName == "" {
		opts.BoardLayoutName = cfg.GetString(config.ConfigDefaultBoardLayout)
		log.Info().Msgf("using default board layout %v", opts.BoardLayoutName)
	}
	if opts.Variant == "" {
		opts.Variant = game.VarClassic
	}
	if game.IsCrossplay(opts.Variant) {
		opts.useCrossplaySettings()
	}
}

// useCrossplaySettings selects the board and tiles that Crossplay plays with.
func (opts *GameOptions) useCrossplaySettings() {
	opts.BoardLayoutName = board.CrossplayLayout
	if opts.Lexicon != nil {
		opts.Lexicon.Distribution = game.CrossplayLetterDistribution
	}
}

// EnsureKWG checks whether the KWG file for lexname is present and, if not,
// downloads it from the woogles-io/liwords repository. It is safe to call
// before any code that calls kwg.GetKWG so that load paths other than
// "set lexicon" also get the file-not-found download behaviour.
func EnsureKWG(lexname string, cfg *wglconfig.Config) error {
	return lexicon.EnsureLexiconFile(lexname, ".kwg", cfg)
}

func (opts *GameOptions) SetLexicon(fields []string, cfg *wglconfig.Config) error {
	lexname := ""
	letdist := ""
	var err error
	if len(fields) == 1 {
		lexname = fields[0]
		if game.IsCrossplay(opts.Variant) {
			letdist = game.CrossplayLetterDistribution
		} else {
			letdist, err = tilemapping.ProbableLetterDistributionName(lexname)
		}
		if err != nil {
			return fmt.Errorf("could not determine a letter distribution for lexicon %q; "+
				"please specify one explicitly, e.g. 'set lexicon %s english'", lexname, lexname)
		}
		log.Info().Msgf("assuming letter distribution %v", letdist)
	} else if len(fields) == 2 {
		lexname, letdist = fields[0], fields[1]
	} else {
		return errors.New("Valid formats are 'lexicon' and 'lexicon alphabet'")
	}
	lexname = strings.ToUpper(lexname)
	if err := EnsureKWG(lexname, cfg); err != nil {
		return err
	}
	opts.Lexicon = &Lexicon{Name: lexname, Distribution: letdist}
	return nil
}

func (opts *GameOptions) SetChallenge(rule string) error {
	val, err := ParseChallengeRule(rule)
	if err != nil {
		return err
	}
	opts.ChallengeRule = val
	return nil
}

func (opts *GameOptions) SetBoardLayoutName(name string) error {
	switch name {
	case board.CrosswordGameLayout, board.SuperCrosswordGameLayout, board.CrossplayLayout:
		opts.BoardLayoutName = name
	default:
		return fmt.Errorf("%v is not a supported board layout", name)
	}
	return nil
}

func (opts *GameOptions) SetVariant(name string) error {
	wasCrossplay := game.IsCrossplay(opts.Variant)
	switch name {
	case string(game.VarClassic):
		opts.Variant = game.VarClassic
	case string(game.VarClassicSuper):
		opts.Variant = game.VarClassicSuper
	case string(game.VarWordSmog):
		opts.Variant = game.VarWordSmog
	case string(game.VarWordSmogSuper):
		opts.Variant = game.VarWordSmogSuper
	case string(game.VarCrossplay):
		opts.Variant = game.VarCrossplay
		opts.useCrossplaySettings()
	default:
		return fmt.Errorf("%v is not a supported variant name", name)
	}
	if wasCrossplay && !game.IsCrossplay(opts.Variant) {
		// Don't leave Crossplay's board and tiles behind for a variant that
		// would reject them.
		opts.BoardLayoutName = board.CrosswordGameLayout
		if opts.Lexicon != nil {
			letdist, err := tilemapping.ProbableLetterDistributionName(opts.Lexicon.Name)
			if err != nil {
				return fmt.Errorf("could not determine a letter distribution for lexicon %q: %w",
					opts.Lexicon.Name, err)
			}
			opts.Lexicon.Distribution = letdist
		}
	}
	return nil
}
