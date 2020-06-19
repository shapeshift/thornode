package types

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type NetworkFeeSuite struct{}

var _ = Suite(&NetworkFeeSuite{})

func (NetworkFeeSuite) TestNetworkFee(c *C) {
	n := NewNetworkFee(common.BNBChain, 1, bnbSingleTxFee)
	c.Check(n.Validate(), IsNil)
	n1 := NewNetworkFee(common.EmptyChain, 1, bnbSingleTxFee)
	c.Check(n1.Validate(), NotNil)
	n2 := NewNetworkFee(common.BNBChain, -1, bnbSingleTxFee)
	c.Check(n2.Validate(), NotNil)

	n3 := NewNetworkFee(common.BNBChain, 1, cosmos.ZeroUint())
	c.Check(n3.Validate(), NotNil)
}
