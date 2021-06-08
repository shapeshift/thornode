package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	. "gopkg.in/check.v1"
)

type StoreManagerTestSuite struct{}

var _ = Suite(&StoreManagerTestSuite{})

func (s *StoreManagerTestSuite) TestMigrateStoreV55(c *C) {
	ctx, keeper := setupKeeperForTest(c)
	storeMgr := NewStoreMgr(keeper)
	version := GetCurrentVersion()
	constantAccessor := constants.GetConstantValues(version)
	assetToAdjust, err := common.NewAsset("BNB.USDT-6D8")
	c.Assert(err, IsNil)
	pool := NewPool()
	pool.Asset = assetToAdjust
	c.Assert(keeper.SetPool(ctx, pool), IsNil)
	storeMgr.migrateStoreV55(ctx, version, constantAccessor)
	newPool, err := keeper.GetPool(ctx, assetToAdjust)
	c.Assert(err, IsNil)
	c.Assert(newPool.BalanceAsset.Equal(cosmos.NewUint(900000000)), Equals, true)
}
