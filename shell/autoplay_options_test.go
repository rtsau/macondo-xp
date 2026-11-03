package shell

import (
	"testing"

	"github.com/matryer/is"

	pb "github.com/domino14/macondo/gen/api/proto/macondo"
)

// Each bot's files have to reach its config. These flags were once accepted and
// quietly ignored, so that both bots played with the default leaves.
func TestAutoplayFileFlagsReachTheirPlayer(t *testing.T) {
	is := is.New(t)
	options := CmdOptions{
		"leavefile1": {"one.klv2"},
		"leavefile2": {"two.klv2"},
		"pegfile2":   {"two.json"},
	}
	p1, p2 := &pb.AutoplayPlayerConfig{}, &pb.AutoplayPlayerConfig{}
	applyPlayerOverrides(p1, options, "1")
	applyPlayerOverrides(p2, options, "2")
	is.Equal(p1.LeaveFile, "one.klv2")
	is.Equal(p2.LeaveFile, "two.klv2")
	is.Equal(p1.PegFile, "")
	is.Equal(p2.PegFile, "two.json")
}
