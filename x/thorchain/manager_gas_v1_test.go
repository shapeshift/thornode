package thorchain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type GasManagerTestSuite struct{}

var _ = Suite(&GasManagerTestSuite{})

func (GasManagerTestSuite) TestGasManagerV1(c *C) {
	ctx, k := setupKeeperForTest(c)
	gasMgr := NewGasMgrV1()
	gasEvent := gasMgr.gasEvent
	c.Assert(gasMgr, NotNil)
	gasMgr.BeginBlock()
	c.Assert(gasEvent != gasMgr.gasEvent, Equals, true)

	pool := NewPool()
	pool.Asset = common.BNBAsset
	c.Assert(k.SetPool(ctx, pool), IsNil)
	pool.Asset = common.BTCAsset
	c.Assert(k.SetPool(ctx, pool), IsNil)

	gasMgr.AddGasAsset(common.Gas{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(37500)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(1000)),
	})
	c.Assert(gasMgr.GetGas(), HasLen, 2)
	gasMgr.AddGasAsset(common.Gas{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(38500)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(2000)),
	})
	c.Assert(gasMgr.GetGas(), HasLen, 2)
	gasMgr.AddGasAsset(common.Gas{
		common.NewCoin(common.ETHAsset, cosmos.NewUint(38500)),
	})
	c.Assert(gasMgr.GetGas(), HasLen, 3)
	eventMgr := NewEventMgrV1()
	gasMgr.EndBlock(ctx, k, eventMgr)
}
