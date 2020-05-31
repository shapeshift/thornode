package keeperv1

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
)

type KeeperObservedNetworkFeeVoterSuite struct{}

var _ = Suite(&KeeperObservedNetworkFeeVoterSuite{})

func (*KeeperObservedNetworkFeeVoterSuite) TestObservedNetworkFeeVoter(c *C) {
	ctx, k := setupKeeperForTest(c)
	voter := NewObservedNetworkFeeVoter(1024, common.BNBChain)
	k.SetObservedNetworkFeeVoter(ctx, voter)
	voter, err := k.GetObservedNetworkFeeVoter(ctx, 1024, voter.Chain)
	c.Assert(err, IsNil)
	c.Check(voter.BlockHeight, Equals, 1024)
	c.Check(voter.Chain.Equals(common.BNBChain), Equals, true)
}
