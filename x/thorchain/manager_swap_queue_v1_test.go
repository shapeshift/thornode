package thorchain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	keeper "gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type SwapQueueSuite struct{}

var _ = Suite(&SwapQueueSuite{})

func (s SwapQueueSuite) TestGetTodoNum(c *C) {
	queue := NewSwapQv1(keeper.KVStoreDummy{})

	c.Check(queue.getTodoNum(50), Equals, 25)     // halves it
	c.Check(queue.getTodoNum(11), Equals, 5)      // halves it
	c.Check(queue.getTodoNum(10), Equals, 10)     // does all of them
	c.Check(queue.getTodoNum(1), Equals, 1)       // does all of them
	c.Check(queue.getTodoNum(0), Equals, 0)       // does none
	c.Check(queue.getTodoNum(10000), Equals, 100) // does max 100
	c.Check(queue.getTodoNum(200), Equals, 100)   // does max 100
}

func (s SwapQueueSuite) TestScoreMsgs(c *C) {
	ctx, k := setupKeeperForTest(c)

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceRune = cosmos.NewUint(143166 * common.One)
	pool.BalanceAsset = cosmos.NewUint(1000 * common.One)
	c.Assert(k.SetPool(ctx, pool), IsNil)
	pool = NewPool()
	pool.Asset = common.BTCAsset
	pool.BalanceRune = cosmos.NewUint(73708333 * common.One)
	pool.BalanceAsset = cosmos.NewUint(1000 * common.One)
	c.Assert(k.SetPool(ctx, pool), IsNil)

	queue := NewSwapQv1(k)

	// check that we sort by liquidity ok
	msgs := []MsgSwap{
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(2*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(50*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(1*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(10*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), GetRandomBech32Addr()),
	}

	swaps, err := queue.ScoreMsgs(ctx, msgs)
	c.Assert(err, IsNil)
	swaps = swaps.Sort()
	c.Check(swaps, HasLen, 5)
	c.Check(swaps[0].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(100*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[1].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(50*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[2].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(10*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[3].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(2*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[4].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(1*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))

	// check that slip is taken into account
	msgs = []MsgSwap{
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(2*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(50*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(1*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(10*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(2*common.One))},
		}, common.BTCAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(50*common.One))},
		}, common.BTCAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(1*common.One))},
		}, common.BTCAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(100*common.One))},
		}, common.BTCAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    GetRandomTxHash(),
			Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(10*common.One))},
		}, common.BTCAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), GetRandomBech32Addr()),
	}

	swaps, err = queue.ScoreMsgs(ctx, msgs)
	c.Assert(err, IsNil)
	swaps = swaps.Sort()
	c.Check(swaps, HasLen, 10)
	c.Check(swaps[0].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(100*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[0].msg.Tx.Coins[0].Asset.Equals(common.BTCAsset), Equals, true)
	c.Check(swaps[1].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(50*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[1].msg.Tx.Coins[0].Asset.Equals(common.BTCAsset), Equals, true)
	c.Check(swaps[2].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(10*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[2].msg.Tx.Coins[0].Asset.Equals(common.BTCAsset), Equals, true)
	c.Check(swaps[3].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(100*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[3].msg.Tx.Coins[0].Asset.Equals(common.BNBAsset), Equals, true)
	c.Check(swaps[4].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(50*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[4].msg.Tx.Coins[0].Asset.Equals(common.BNBAsset), Equals, true)
	c.Check(swaps[5].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(2*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[5].msg.Tx.Coins[0].Asset.Equals(common.BTCAsset), Equals, true)
	c.Check(swaps[6].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(1*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[6].msg.Tx.Coins[0].Asset.Equals(common.BTCAsset), Equals, true)
	c.Check(swaps[7].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(10*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[7].msg.Tx.Coins[0].Asset.Equals(common.BNBAsset), Equals, true)
	c.Check(swaps[8].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(2*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[8].msg.Tx.Coins[0].Asset.Equals(common.BNBAsset), Equals, true)
	c.Check(swaps[9].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(1*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[9].msg.Tx.Coins[0].Asset.Equals(common.BNBAsset), Equals, true)
}
