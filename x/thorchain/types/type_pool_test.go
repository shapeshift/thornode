package types

import (
	"encoding/json"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type PoolTestSuite struct{}

var _ = Suite(&PoolTestSuite{})

func (PoolTestSuite) TestPool(c *C) {
	p := NewPool()
	c.Check(p.IsEmpty(), Equals, true)
	p.Asset = common.BNBAsset
	c.Check(p.IsEmpty(), Equals, false)
	p.BalanceRune = cosmos.NewUint(100 * common.One)
	p.BalanceAsset = cosmos.NewUint(50 * common.One)
	c.Check(p.AssetValueInRune(cosmos.NewUint(25*common.One)).Equal(cosmos.NewUint(50*common.One)), Equals, true)
	c.Check(p.AssetValueInRuneWithSlip(cosmos.NewUint(25*common.One)).Equal(cosmos.NewUint(10000000000)), Equals, true, Commentf("%d", p.AssetValueInRuneWithSlip(cosmos.NewUint(25*common.One)).Uint64()))
	c.Check(p.RuneValueInAsset(cosmos.NewUint(50*common.One)).Equal(cosmos.NewUint(25*common.One)), Equals, true)
	c.Check(p.RuneValueInAssetWithSlip(cosmos.NewUint(50*common.One)).Equal(cosmos.NewUint(50*common.One)), Equals, true)

	signer := GetRandomBech32Addr()
	bnbAddress := GetRandomBNBAddress()
	txID := GetRandomTxHash()

	tx := common.NewTx(
		txID,
		GetRandomBNBAddress(),
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(1)),
		},
		BNBGasFeeSingleton,
		"",
	)
	m := NewMsgSwap(tx, common.BNBAsset, bnbAddress, cosmos.NewUint(2), common.NoAddress, cosmos.ZeroUint(), signer)

	c.Check(p.EnsureValidPoolStatus(m), IsNil)
	msgNoop := NewMsgNoOp(GetRandomObservedTx(), signer, "")
	c.Check(p.EnsureValidPoolStatus(msgNoop), IsNil)
	p.Status = PoolStatus_Available
	c.Check(p.EnsureValidPoolStatus(m), IsNil)
	p.Status = PoolStatus(100)
	c.Check(p.EnsureValidPoolStatus(msgNoop), NotNil)

	p.Status = PoolStatus_Suspended
	c.Check(p.EnsureValidPoolStatus(msgNoop), NotNil)
	p1 := NewPool()
	c.Check(p1.Valid(), NotNil)
	p1.Asset = common.BNBAsset
	c.Check(p1.AssetValueInRune(cosmos.NewUint(100)).Uint64(), Equals, cosmos.ZeroUint().Uint64())
	c.Check(p1.AssetValueInRuneWithSlip(cosmos.NewUint(100)).Uint64(), Equals, cosmos.ZeroUint().Uint64())
	c.Check(p1.RuneValueInAsset(cosmos.NewUint(100)).Uint64(), Equals, cosmos.ZeroUint().Uint64())
	c.Check(p1.RuneValueInAssetWithSlip(cosmos.NewUint(100)).Uint64(), Equals, cosmos.ZeroUint().Uint64())
	p1.BalanceRune = cosmos.NewUint(100 * common.One)
	p1.BalanceAsset = cosmos.NewUint(50 * common.One)
	c.Check(p1.Valid(), IsNil)

	c.Check(p1.IsAvailable(), Equals, true)

	// When Pool is in staged status, it can't swap
	p2 := NewPool()
	p2.Status = PoolStatus_Staged
	msgSwap := NewMsgSwap(GetRandomTx(), common.BNBAsset, GetRandomBNBAddress(), cosmos.NewUint(1000), common.NoAddress, cosmos.ZeroUint(), GetRandomBech32Addr())
	c.Check(p2.EnsureValidPoolStatus(msgSwap), NotNil)
	c.Check(p2.EnsureValidPoolStatus(msgNoop), IsNil)
}

func (PoolTestSuite) TestPoolStatus(c *C) {
	inputs := []string{
		"Available", "Staged", "Suspended", "whatever",
	}
	for _, item := range inputs {
		ps := GetPoolStatus(item)
		c.Assert(ps.Valid(), IsNil)
	}
	var ps PoolStatus
	err := json.Unmarshal([]byte(`"Available"`), &ps)
	c.Assert(err, IsNil)
	c.Check(ps == PoolStatus_Available, Equals, true)
	err = json.Unmarshal([]byte(`{asdf}`), &ps)
	c.Assert(err, NotNil)

	buf, err := json.Marshal(ps)
	c.Check(err, IsNil)
	c.Check(buf, NotNil)
}

func (PoolTestSuite) TestPools(c *C) {
	pools := make(Pools, 0)
	bnb := NewPool()
	bnb.Asset = common.BNBAsset
	btc := NewPool()
	btc.Asset = common.BTCAsset
	btc.BalanceRune = cosmos.NewUint(10)

	pools = pools.Set(bnb)
	pools = pools.Set(btc)
	c.Assert(pools, HasLen, 2)

	pool, ok := pools.Get(common.BNBAsset)
	c.Check(ok, Equals, true)
	c.Check(pool.Asset.Equals(common.BNBAsset), Equals, true)

	pool, ok = pools.Get(common.BTCAsset)
	c.Check(ok, Equals, true)
	c.Check(pool.Asset.Equals(common.BTCAsset), Equals, true)
	c.Check(pool.BalanceRune.Uint64(), Equals, uint64(10))

	pool.BalanceRune = cosmos.NewUint(20)
	pools = pools.Set(pool)
	pool, ok = pools.Get(common.BTCAsset)
	c.Check(ok, Equals, true)
	c.Check(pool.Asset.Equals(common.BTCAsset), Equals, true)
	c.Check(pool.BalanceRune.Uint64(), Equals, uint64(20))
}
