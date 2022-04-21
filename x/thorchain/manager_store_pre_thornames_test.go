package thorchain

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
	. "gopkg.in/check.v1"
)

type PreTHORNameTestSuite struct{}

var _ = Suite(&PreTHORNameTestSuite{})

func (s *PreTHORNameTestSuite) SetUpSuite(c *C) {
	config := cosmos.GetConfig()
	config.SetBech32PrefixForAccount("thor", "thorpub")
}

func (s *PreTHORNameTestSuite) TestLoadingJson(c *C) {
	names, err := getPreRegisterTHORNames(100)
	c.Assert(err, IsNil)
	c.Check(names, HasLen, 9167, Commentf("%d", len(names)))
}
