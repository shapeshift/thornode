package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
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
func (s *StoreManagerTestSuite) TestMigrateStoreV58(c *C) {
	SetupConfigForTest()
	ctx, keeper := setupKeeperForTest(c)
	storeMgr := NewStoreMgr(keeper)
	version := GetCurrentVersion()
	constantAccessor := constants.GetConstantValues(version)
	pubKey, err := common.NewPubKey("tthorpub1addwnpepqg65km6vfflrlymsjhrnmn4w58d2d36h977pcu3aqp6dxee2yf88yg0z3v4")
	c.Assert(err, IsNil)
	retiredVault := NewVault(1024, types.VaultStatus_InactiveVault, AsgardVault, pubKey, []string{
		common.BTCChain.String(),
		common.BNBChain.String(),
		common.ETHChain.String(),
		common.LTCChain.String(),
		common.BCHChain.String(),
	}, nil)
	c.Assert(keeper.SetVault(ctx, retiredVault), IsNil)

	retiringPubKey, err := common.NewPubKey("tthorpub1addwnpepqfz98sx54jpv3f95qfg39zkx500avc6tr0d8ww0lv283yu3ucgq3g9y9njj")
	c.Assert(err, IsNil)
	retiringVault := NewVault(1024, types.VaultStatus_RetiringVault, AsgardVault, retiringPubKey, []string{
		common.BTCChain.String(),
		common.BNBChain.String(),
		common.ETHChain.String(),
		common.LTCChain.String(),
		common.BCHChain.String(),
	}, nil)
	inputs := []struct {
		assetName string
		amount    cosmos.Uint
	}{
		{"ETH.ETH", cosmos.NewUint(2357498870)},
		{"ETH.DAI-0XAD6D458402F60FD3BD25163575031ACDCE07538D", cosmos.NewUint(380250359282)},
		{"ETH.MARS-0X9465DC5A988957CB56BE398D1F05A66F65170361", cosmos.NewUint(5747652585)},
		{"ETH.REP-0X6FD34013CDD2905D8D27B0ADAD5B97B2345CF2B8", cosmos.NewUint(1556246434)},
		{"ETH.UNI-0X71D82EB6A5051CFF99582F4CDF2AE9CD402A4882", cosmos.NewUint(548635445548)},
		{"ETH.USDT-0XA3910454BF2CB59B8B3A401589A3BACC5CA42306", cosmos.NewUint(10987963700)},
		{"ETH.XEENUS-0X7E0480CA9FD50EB7A3855CF53C347A1B4D6A2FF5", cosmos.NewUint(25626085749)},
		{"ETH.XRUNE-0X0FE3ECD525D16FA09AA1FF177014DE5304C835E2", cosmos.NewUint(3550865535196787)},
		{"ETH.XRUNE-0X8626DB1A4F9F3E1002EEB9A4F3C6D391436FFC23", cosmos.NewUint(16098175953548)},
		{"ETH.ZRX-0XE4C6182EA459E63B8F1BE7C428381994CCC2D49C", cosmos.NewUint(6732474974)},
		{"ETH.XRUNE-0XD9B37D046C543EB0E5E3EC27B86609E31DA205D7", cosmos.NewUint(10000000000)},
	}
	var coinsToSubtract common.Coins
	for _, item := range inputs {
		asset, err := common.NewAsset(item.assetName)
		if err != nil {
			ctx.Logger().Error("fail to parse asset", "error", err, "asset name", item.assetName)
			continue
		}
		coinsToSubtract = append(coinsToSubtract, common.NewCoin(asset, item.amount))
	}
	retiringVault.AddFunds(coinsToSubtract)
	c.Assert(keeper.SetVault(ctx, retiringVault), IsNil)
	c.Assert(retiredVault.HasFunds(), Equals, false)
	storeMgr.migrateStoreV58(ctx, version, constantAccessor)
	vaultAfter, err := keeper.GetVault(ctx, pubKey)
	c.Assert(err, IsNil)
	c.Assert(vaultAfter.Status.String(), Equals, RetiringVault.String())
	c.Assert(vaultAfter.HasFunds(), Equals, true)
	vaultAfter1, err := keeper.GetVault(ctx, retiringPubKey)
	c.Assert(err, IsNil)
	c.Assert(vaultAfter1.HasFunds(), Equals, false)
}
