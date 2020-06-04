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
	defaultTxFee := constAccessor.GetInt64Value(constants.TransactionFee)
	// when there is no network fee available, it should just get from the constants
	c.Assert(fee, Equals, defaultTxFee)
	networkFee := NewNetworkFee(common.BNBChain, 1, bnbSingleTxFee.Uint64())
	c.Assert(k.SaveNetworkFee(ctx, common.BNBChain, networkFee), IsNil)
	fee = gasMgr.GetFee(ctx, common.BNBChain)
	c.Assert(fee, Equals, defaultTxFee)
	c.Assert(k.SetPool(ctx, Pool{
		BalanceRune:  cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BNBAsset,
		Status:       PoolEnabled,
	}), IsNil)
	fee = gasMgr.GetFee(ctx, common.BNBChain)
	c.Assert(fee, Equals, int64(bnbSingleTxFee.Uint64()*3))

	// BTC chain
	networkFee = NewNetworkFee(common.BTCChain, 70, 50)
	c.Assert(k.SaveNetworkFee(ctx, common.BTCChain, networkFee), IsNil)
	fee = gasMgr.GetFee(ctx, common.BTCChain)
	c.Assert(fee, Equals, defaultTxFee)
	c.Assert(k.SetPool(ctx, Pool{
		BalanceRune:  cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BTCAsset,
		Status:       PoolEnabled,
	}), IsNil)
	fee = gasMgr.GetFee(ctx, common.BTCChain)
	c.Assert(fee, Equals, int64(70*50*3))
}
