package thorchain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type GasManagerTestSuite struct{}

var _ = Suite(&GasManagerTestSuite{})

func (GasManagerTestSuite) TestGasManagerV1(c *C) {
	ctx, k := setupKeeperForTest(c)
	constAccessor := constants.NewConstantValue010()
	gasMgr := NewGasMgrV1(constAccessor, k)
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

func (GasManagerTestSuite) TestGetFee(c *C) {
	ctx, k := setupKeeperForTest(c)
	constAccessor := constants.NewConstantValue010()
	gasMgr := NewGasMgrV1(constAccessor, k)
	fee := gasMgr.GetFee(ctx, common.BNBChain)
	defaultTxFee := cosmos.NewUint(uint64(constAccessor.GetInt64Value(constants.TransactionFee)))
	// when there is no network fee available, it should just get from the constants
	c.Assert(fee.Asset.IsRune(), Equals, true)
	c.Assert(fee.Amount.Equal(defaultTxFee), Equals, true)
	networkFee := NewNetworkFee(common.BNBChain, 1, bnbSingleTxFee)
	c.Assert(k.SaveNetworkFee(ctx, common.BNBChain, networkFee), IsNil)
	fee = gasMgr.GetFee(ctx, common.BNBChain)
	c.Assert(fee.Asset.Equals(common.BNBChain.GetGasAsset()), Equals, true)
	c.Assert(fee.Amount.Equal(cosmos.NewUint(bnbSingleTxFee.Uint64()*3)), Equals, true)

	// BTC chain
	networkFee = NewNetworkFee(common.BTCChain, 70, cosmos.NewUint(50))
	c.Assert(k.SaveNetworkFee(ctx, common.BTCChain, networkFee), IsNil)
	fee = gasMgr.GetFee(ctx, common.BTCChain)
	c.Assert(fee.Asset.Equals(common.BTCChain.GetGasAsset()), Equals, true)
	c.Assert(fee.Amount.Equal(cosmos.NewUint(70*50*3)), Equals, true)
}
