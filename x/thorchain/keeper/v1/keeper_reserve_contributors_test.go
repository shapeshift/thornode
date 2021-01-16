package keeperv1

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type KeeperReserveContributorsSuite struct{}

var _ = Suite(&KeeperReserveContributorsSuite{})

func (KeeperReserveContributorsSuite) TestReserveContributors(c *C) {
	ctx, k := setupKeeperForTest(c)
	FundModule(c, ctx, k, AsgardName, 100000000)
	c.Assert(k.AddFeeToReserve(ctx, cosmos.NewUint(common.One*100)), IsNil)
}
