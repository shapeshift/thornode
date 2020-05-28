package thorchain

import (
	"fmt"

	"github.com/blang/semver"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	keeper "gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

var _ = Suite(&HandlerErrataTxSuite{})

type HandlerErrataTxSuite struct{}

type TestErrataTxKeeper struct {
	keeper.KVStoreDummy
	observedTx ObservedTxVoter
	pool       Pool
	na         NodeAccount
	stakers    []Staker
	err        error
}

func (k *TestErrataTxKeeper) ListActiveNodeAccounts(_ cosmos.Context) (NodeAccounts, error) {
	return NodeAccounts{k.na}, k.err
}

func (k *TestErrataTxKeeper) GetNodeAccount(_ cosmos.Context, _ cosmos.AccAddress) (NodeAccount, error) {
	return k.na, k.err
}

func (k *TestErrataTxKeeper) GetObservedTxVoter(_ cosmos.Context, txID common.TxID) (ObservedTxVoter, error) {
	return k.observedTx, k.err
}

func (k *TestErrataTxKeeper) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
	return k.pool, k.err
}

func (k *TestErrataTxKeeper) SetPool(_ cosmos.Context, pool Pool) error {
	k.pool = pool
	return k.err
}

func (k *TestErrataTxKeeper) GetStaker(_ cosmos.Context, asset common.Asset, addr common.Address) (Staker, error) {
	for _, staker := range k.stakers {
		if staker.RuneAddress.Equals(addr) {
			return staker, k.err
		}
	}
	return Staker{}, k.err
}

func (k *TestErrataTxKeeper) SetStaker(_ cosmos.Context, staker Staker) {
	for i, skr := range k.stakers {
		if skr.RuneAddress.Equals(staker.RuneAddress) {
			k.stakers[i] = staker
		}
	}
}

func (k *TestErrataTxKeeper) GetErrataTxVoter(_ cosmos.Context, txID common.TxID, chain common.Chain) (ErrataTxVoter, error) {
	return NewErrataTxVoter(txID, chain), k.err
}

func (s *HandlerErrataTxSuite) TestValidate(c *C) {
	ctx, _ := setupKeeperForTest(c)

	keeper := &TestErrataTxKeeper{
		na: GetRandomNodeAccount(NodeActive),
	}

	handler := NewErrataTxHandler(keeper, NewDummyMgr())
	// happy path
	ver := constants.SWVersion
	msg := NewMsgErrataTx(GetRandomTxHash(), common.BNBChain, keeper.na.NodeAddress)
	err := handler.validate(ctx, msg, ver)
	c.Assert(err, IsNil)

	// invalid version
	err = handler.validate(ctx, msg, semver.Version{})
	c.Assert(err, Equals, errBadVersion)

	// invalid msg
	msg = MsgErrataTx{}
	err = handler.validate(ctx, msg, ver)
	c.Assert(err, NotNil)
}

func (s *HandlerErrataTxSuite) TestHandle(c *C) {
	ctx, _ := setupKeeperForTest(c)
	ver := constants.SWVersion

	txID := GetRandomTxHash()
	na := GetRandomNodeAccount(NodeActive)
	addr := GetRandomBNBAddress()
	totalUnits := cosmos.NewUint(1600)

	keeper := &TestErrataTxKeeper{
		na: na,
		observedTx: ObservedTxVoter{
			Tx: ObservedTx{
				Tx: common.Tx{
					ID:          txID,
					Chain:       common.BNBChain,
					FromAddress: addr,
					Coins: common.Coins{
						common.NewCoin(common.RuneAsset(), cosmos.NewUint(30*common.One)),
					},
					Memo: fmt.Sprintf("STAKE:BNB.BNB:%s", GetRandomRUNEAddress()),
				},
			},
		},
		pool: Pool{
			Asset:        common.BNBAsset,
			PoolUnits:    totalUnits,
			BalanceRune:  cosmos.NewUint(100 * common.One),
			BalanceAsset: cosmos.NewUint(100 * common.One),
		},
		stakers: []Staker{
			Staker{
				RuneAddress:     addr,
				LastStakeHeight: 5,
				Units:           totalUnits.QuoUint64(2),
				PendingRune:     cosmos.ZeroUint(),
			},
			Staker{
				RuneAddress:     GetRandomBNBAddress(),
				LastStakeHeight: 10,
				Units:           totalUnits.QuoUint64(2),
				PendingRune:     cosmos.ZeroUint(),
			},
		},
	}

	mgr := NewManagers(keeper)
	c.Assert(mgr.BeginBlock(ctx), IsNil)
	handler := NewErrataTxHandler(keeper, mgr)
	msg := NewMsgErrataTx(txID, common.BNBChain, na.NodeAddress)
	_, err := handler.handle(ctx, msg, ver)
	c.Assert(err, IsNil)
	c.Check(keeper.pool.BalanceRune.Equal(cosmos.NewUint(70*common.One)), Equals, true)
	c.Check(keeper.pool.BalanceAsset.Equal(cosmos.NewUint(100*common.One)), Equals, true)
	c.Check(keeper.stakers[0].Units.IsZero(), Equals, true, Commentf("%d", keeper.stakers[0].Units.Uint64()))
	c.Check(keeper.stakers[0].LastStakeHeight, Equals, int64(18))
}
