package thorchain

import (
	"github.com/blang/semver"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
	. "gopkg.in/check.v1"
)

type StoreManagerTestSuite struct{}

var _ = Suite(&StoreManagerTestSuite{})

func (StoreManagerTestSuite) TestStoreMgr_migrateStoreV36(c *C) {
	constants.SWVersion = semver.MustParse("0.36.0")
	ctx, keeper := setupKeeperForTest(c)
	node := GetRandomNodeAccount(NodeActive)
	node.Version = "0.36.0"
	txID, err := common.NewTxID("fce2585aedeaec263bada44fe1a68124b4dd33110758915ad46fdda31ed797ca")
	if err != nil {
		ctx.Logger().Error("fail to parse tx id", "error", err)
		return
	}
	vault := GetRandomVault()

	addr, err := common.NewAddress("qz7pmntvnlujmtpz9n5j5yc5m0tta0k3hy4nk5eg8g")
	c.Assert(err, IsNil)
	bchAddr, err := vault.PubKey.GetAddress(common.BCHChain)
	c.Assert(err, IsNil)

	observeTx := NewObservedTx(
		common.NewTx(txID, addr, bchAddr, common.Coins{
			common.NewCoin(common.BCHAsset, cosmos.NewUint(2000000000)),
		}, common.Gas{
			common.NewCoin(common.BCHAsset, cosmos.NewUint(291)),
		}, "+:BCH.BCH:tthor1qkd5f9xh2g87wmjc620uf5w08ygdx4etu0u9fs"),
		ctx.BlockHeight(),
		vault.PubKey,
		ctx.BlockHeight(),
	)
	voter := NewObservedTxVoter(txID, []ObservedTx{
		observeTx,
	})
	c.Assert(voter.Add(observeTx, node.NodeAddress), Equals, true)
	voter.GetTx(types.NodeAccounts{
		node,
	})
	keeper.SetObservedTxInVoter(ctx, voter)
	c.Assert(keeper.SetNodeAccount(ctx, node), IsNil)
	storeMgr := NewStoreMgr(keeper)
	c.Assert(storeMgr.Iterator(ctx), IsNil)
}
