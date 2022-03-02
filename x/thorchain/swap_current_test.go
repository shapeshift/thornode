package thorchain

import (
	"errors"
	"os"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type SwapV81Suite struct{}

var _ = Suite(&SwapV81Suite{})

func (s *SwapV81Suite) SetUpSuite(c *C) {
	err := os.Setenv("NET", "other")
	c.Assert(err, IsNil)
	SetupConfigForTest()
}

func (s *SwapV81Suite) TestSwap(c *C) {
	poolStorage := &TestSwapKeeper{}
	inputs := []struct {
		name          string
		requestTxHash common.TxID
		source        common.Asset
		target        common.Asset
		amount        cosmos.Uint
		requester     common.Address
		destination   common.Address
		returnAmount  cosmos.Uint
		tradeTarget   cosmos.Uint
		expectedErr   error
		events        int
	}{
		{
			name:          "empty-source",
			requestTxHash: "hash",
			source:        common.Asset{},
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(100 * common.One),
			requester:     "tester",
			destination:   "whatever",
			returnAmount:  cosmos.ZeroUint(),
			expectedErr:   errors.New("Denom cannot be empty"),
		},
		{
			name:          "empty-target",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.Asset{},
			amount:        cosmos.NewUint(100 * common.One),
			requester:     "tester",
			destination:   "whatever",
			returnAmount:  cosmos.ZeroUint(),
			expectedErr:   errors.New("target is empty"),
		},
		{
			name:          "empty-requestTxHash",
			requestTxHash: "",
			source:        common.RuneAsset(),
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(100 * common.One),
			requester:     "tester",
			destination:   "whatever",
			returnAmount:  cosmos.ZeroUint(),
			expectedErr:   errors.New("Tx ID cannot be empty"),
		},
		{
			name:          "empty-amount",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.BNBAsset,
			amount:        cosmos.ZeroUint(),
			requester:     "tester",
			destination:   "whatever",
			returnAmount:  cosmos.ZeroUint(),
			expectedErr:   errors.New("Amount cannot be zero"),
		},
		{
			name:          "empty-requester",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(100 * common.One),
			requester:     "",
			destination:   "whatever",
			returnAmount:  cosmos.ZeroUint(),
			expectedErr:   errors.New("from address cannot be empty"),
		},
		{
			name:          "empty-destination",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(100 * common.One),
			requester:     GetRandomBNBAddress(),
			destination:   "",
			returnAmount:  cosmos.ZeroUint(),
			expectedErr:   errors.New("to address cannot be empty"),
		},
		{
			name:          "pool-not-exist",
			requestTxHash: "hash",
			source:        common.Asset{Chain: common.BNBChain, Ticker: "NOTEXIST", Symbol: "NOTEXIST"},
			target:        common.RuneAsset(),
			amount:        cosmos.NewUint(100 * common.One),
			requester:     GetRandomBNBAddress(),
			destination:   GetRandomBNBAddress(),
			tradeTarget:   cosmos.NewUint(110000000),
			returnAmount:  cosmos.ZeroUint(),
			expectedErr:   errors.New("BNB.NOTEXIST pool doesn't exist"),
		},
		{
			name:          "pool-not-exist-1",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.Asset{Chain: common.BNBChain, Ticker: "NOTEXIST", Symbol: "NOTEXIST"},
			amount:        cosmos.NewUint(100 * common.One),
			requester:     "tester",
			destination:   "don'tknow",
			tradeTarget:   cosmos.NewUint(120000000),
			returnAmount:  cosmos.ZeroUint(),
			expectedErr:   errors.New("BNB.NOTEXIST pool doesn't exist"),
		},
		{
			name:          "swap-cross-chain-different-address",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.BTCAsset,
			amount:        cosmos.NewUint(50 * common.One),
			requester:     "tester",
			destination:   GetRandomBNBAddress(),
			returnAmount:  cosmos.ZeroUint(),
			tradeTarget:   cosmos.ZeroUint(),
			expectedErr:   errors.New("destination address is not a valid BTC address"),
			events:        1,
		},
		{
			name:          "swap-no-global-sliplimit",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(50 * common.One),
			requester:     "tester",
			destination:   GetRandomBNBAddress(),
			returnAmount:  cosmos.NewUint(2222222222),
			tradeTarget:   cosmos.ZeroUint(),
			expectedErr:   nil,
			events:        1,
		},
		{
			name:          "swap-over-trade-sliplimit",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(9 * common.One),
			requester:     "tester",
			destination:   GetRandomBNBAddress(),
			returnAmount:  cosmos.ZeroUint(),
			tradeTarget:   cosmos.NewUint(9 * common.One),
			expectedErr:   errors.New("emit asset 757511993 less than price limit 900000000"),
		},
		{
			name:          "swap-no-target-price-no-protection",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(8 * common.One),
			requester:     "tester",
			destination:   GetRandomBNBAddress(),
			returnAmount:  cosmos.NewUint(685871056),
			tradeTarget:   cosmos.ZeroUint(),
			expectedErr:   nil,
			events:        1,
		},
		{
			name:          "swap",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(5 * common.One),
			requester:     "tester",
			destination:   GetRandomBNBAddress(),
			returnAmount:  cosmos.NewUint(453514739),
			tradeTarget:   cosmos.NewUint(453514738),
			expectedErr:   nil,
			events:        1,
		},
		{
			name:          "double-swap",
			requestTxHash: "hash",
			source:        common.Asset{Chain: common.BTCChain, Ticker: "BTC", Symbol: "BTC"},
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(5 * common.One),
			requester:     "tester",
			destination:   GetRandomBNBAddress(),
			returnAmount:  cosmos.NewUint(415017809),
			tradeTarget:   cosmos.NewUint(415017809),
			expectedErr:   nil,
			events:        2,
		},
		{
			name:          "swap-synth-to-rune-when-pool-is-not-available",
			requestTxHash: "hash",
			source:        common.BCHAsset.GetSyntheticAsset(),
			target:        common.RuneAsset(),
			amount:        cosmos.NewUint(5 * common.One),
			requester:     "tester",
			destination:   GetRandomTHORAddress(),
			returnAmount:  cosmos.NewUint(475907198),
			tradeTarget:   cosmos.NewUint(453514738),
			expectedErr:   nil,
			events:        1,
		},
	}

	for _, item := range inputs {
		c.Logf("test name:%s", item.name)
		tx := common.NewTx(
			item.requestTxHash,
			item.requester,
			item.destination,
			common.Coins{
				common.NewCoin(item.source, item.amount),
			},
			BNBGasFeeSingleton,
			"",
		)
		tx.Chain = common.BNBChain
		ctx, mgr := setupManagerForTest(c)
		mgr.K = poolStorage
		mgr.txOutStore = NewTxStoreDummy()

		amount, evts, err := newSwapperV81().swap(ctx, poolStorage, tx, item.target, item.destination, item.tradeTarget, cosmos.NewUint(1000_000), 2, mgr)
		if item.expectedErr == nil {
			c.Assert(err, IsNil)
			c.Assert(evts, HasLen, item.events)
		} else {
			c.Assert(err, NotNil, Commentf("Expected: %s, got nil", item.expectedErr.Error()))
			c.Assert(err.Error(), Equals, item.expectedErr.Error())
		}

		c.Logf("expected amount:%s, actual amount:%s", item.returnAmount, amount)
		c.Check(item.returnAmount.Uint64(), Equals, amount.Uint64())

	}
}

func (s *SwapV81Suite) TestSynthSwap(c *C) {
	ctx, mgr := setupManagerForTest(c)
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceRune = cosmos.NewUint(1111 * common.One)
	pool.BalanceAsset = cosmos.NewUint(34 * common.One)
	pool.LPUnits = pool.BalanceRune
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	asgardVault := GetRandomVault()
	c.Assert(mgr.Keeper().SetVault(ctx, asgardVault), IsNil)

	addr := GetRandomTHORAddress()
	tx := common.NewTx(
		GetRandomTxHash(),
		addr,
		addr,
		common.NewCoins(
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(50*common.One)),
		),
		BNBGasFeeSingleton,
		"",
	)
	tx.Chain = common.BNBChain

	// swap rune --> synth
	amount, _, err := newSwapperV81().swap(ctx, mgr.Keeper(), tx, common.BNBAsset.GetSyntheticAsset(), addr, cosmos.ZeroUint(), cosmos.NewUint(1000_000), 2, mgr)
	c.Assert(err, IsNil)
	c.Check(amount.Uint64(), Equals, uint64(146354579))
	pool, err = mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	pool.CalcUnits(mgr.GetVersion(), mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset()))
	c.Check(pool.BalanceAsset.Uint64(), Equals, uint64(34*common.One))
	c.Check(pool.BalanceRune.Uint64(), Equals, uint64(116098000000), Commentf("%d", pool.BalanceRune.Uint64()))
	c.Check(pool.LPUnits.Uint64(), Equals, uint64(111100000000), Commentf("%d", pool.LPUnits.Uint64()))
	c.Check(pool.SynthUnits.Uint64(), Equals, uint64(2442772720), Commentf("%d", pool.SynthUnits.Uint64()))
	coin := common.NewCoin(common.BNBAsset.GetSyntheticAsset(), amount)
	c.Assert(mgr.Keeper().MintToModule(ctx, ModuleName, coin), IsNil)
	c.Assert(mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, AsgardName, common.NewCoins(coin)), IsNil)

	// do another rune --> synth
	amount, _, err = newSwapperV81().swap(ctx, mgr.Keeper(), tx, common.BNBAsset.GetSyntheticAsset(), addr, cosmos.ZeroUint(), cosmos.NewUint(1000_000), 2, mgr)
	c.Assert(err, IsNil)
	c.Check(amount.Uint64(), Equals, uint64(140319790), Commentf("%d", amount.Uint64()))
	pool, err = mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	c.Check(pool.BalanceAsset.Uint64(), Equals, uint64(34*common.One))
	c.Check(pool.BalanceRune.Uint64(), Equals, uint64(121096000000), Commentf("%d", pool.BalanceRune.Uint64()))
	c.Check(pool.LPUnits.Uint64(), Equals, uint64(111100000000), Commentf("%d", pool.LPUnits.Uint64()))
	c.Check(pool.SynthUnits.Uint64(), Equals, uint64(4996424545), Commentf("%d", pool.SynthUnits.Uint64()))
	coin = common.NewCoin(common.BNBAsset.GetSyntheticAsset(), amount)
	c.Assert(mgr.Keeper().MintToModule(ctx, ModuleName, coin), IsNil)
	c.Assert(mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, AsgardName, common.NewCoins(coin)), IsNil)

	// swap synth --> rune
	tx.Coins = common.NewCoins(common.NewCoin(common.BNBAsset.GetSyntheticAsset(), cosmos.NewUint(146354579)))
	amount, _, err = newSwapperV81().swap(ctx, mgr.Keeper(), tx, common.RuneAsset(), addr, cosmos.ZeroUint(), cosmos.NewUint(1000_000), 2, mgr)
	c.Assert(err, IsNil)
	c.Check(amount.Uint64(), Equals, uint64(4995294812), Commentf("%d", amount.Uint64()))
	pool, err = mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	c.Check(pool.BalanceAsset.Uint64(), Equals, uint64(34*common.One))
	c.Check(pool.BalanceRune.Uint64(), Equals, uint64(116100705188), Commentf("%d", pool.BalanceRune.Uint64()))
	c.Check(pool.LPUnits.Uint64(), Equals, uint64(111100000000), Commentf("%d", pool.LPUnits.Uint64()))
	c.Check(pool.SynthUnits.Uint64(), Equals, uint64(10227829216), Commentf("%d", pool.SynthUnits.Uint64()))

	// swap synth --> rune again
	totalSupply := mgr.Keeper().GetTotalSupply(ctx, common.BNBAsset.GetSyntheticAsset())
	tx.Coins = common.NewCoins(common.NewCoin(common.BNBAsset.GetSyntheticAsset(), totalSupply))
	amount, _, err = newSwapperV81().swap(ctx, mgr.Keeper(), tx, common.RuneAsset(), addr, cosmos.ZeroUint(), cosmos.NewUint(1000_000), 2, mgr)
	c.Assert(err, IsNil)
	c.Check(amount.Uint64(), Equals, uint64(12905575651), Commentf("%d", amount.Uint64()))
	pool, err = mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	c.Check(pool.BalanceAsset.Uint64(), Equals, uint64(34*common.One))
	c.Check(pool.BalanceRune.Uint64(), Equals, uint64(103195129537), Commentf("%d", pool.BalanceRune.Uint64()))
	c.Check(pool.LPUnits.Uint64(), Equals, uint64(111100000000), Commentf("%d", pool.LPUnits.Uint64()))
	c.Check(pool.SynthUnits.Uint64(), Equals, uint64(7441614333), Commentf("%d", pool.SynthUnits.Uint64()))

	// swap BNB.BNB -> BNB/BNB (external asset directly to synth)
	tx1 := common.NewTx(
		GetRandomTxHash(),
		addr,
		addr,
		common.NewCoins(
			common.NewCoin(common.BNBAsset, cosmos.NewUint(50*common.One)),
		),
		BNBGasFeeSingleton,
		"",
	)
	tx.Chain = common.BNBChain
	amount, _, err = newSwapperV81().swap(ctx, mgr.Keeper(), tx1, common.BNBAsset.GetSyntheticAsset(), addr, cosmos.ZeroUint(), cosmos.NewUint(1000_000), 2, mgr)
	c.Assert(err, IsNil)
	c.Check(amount.Uint64(), Equals, uint64(1985844476), Commentf("%d", amount.Uint64()))
	pool, err = mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	c.Check(pool.BalanceAsset.Uint64(), Equals, uint64(84*common.One))
	c.Check(pool.BalanceRune.Uint64(), Equals, uint64(103193129537), Commentf("%d", pool.BalanceRune.Uint64()))
	c.Check(pool.LPUnits.Uint64(), Equals, uint64(111100000000), Commentf("%d", pool.LPUnits.Uint64()))
	c.Check(pool.SynthUnits.Uint64(), Equals, uint64(7441614333), Commentf("%d", pool.SynthUnits.Uint64()))

	// emit asset is not enough to pay for fee , then pool balance should be restored
	tx2 := common.NewTx(
		GetRandomTxHash(),
		addr,
		addr,
		common.NewCoins(
			common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One/2)),
		),
		BNBGasFeeSingleton,
		"",
	)
	tx.Chain = common.BNBChain
	btcPool := NewPool()
	btcPool.Asset = common.BTCAsset
	btcPool.BalanceAsset = cosmos.NewUint(common.One)
	btcPool.BalanceRune = cosmos.NewUint(common.One * 10)
	btcPool.LPUnits = cosmos.NewUint(100)
	btcPool.SynthUnits = cosmos.ZeroUint()
	c.Assert(mgr.Keeper().SetPool(ctx, btcPool), IsNil)

	amount, _, err = newSwapperV81().swap(ctx, mgr.Keeper(), tx2, common.BTCAsset, addr, cosmos.ZeroUint(), cosmos.NewUint(1000_000_000_000), 2, mgr)
	c.Assert(err, NotNil)
	c.Check(amount.IsZero(), Equals, true)
	pool, err = mgr.Keeper().GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	c.Check(pool.BalanceAsset.Uint64(), Equals, uint64(common.One))
	c.Check(pool.BalanceRune.Uint64(), Equals, uint64(10*common.One), Commentf("%d", pool.BalanceRune.Uint64()))
	c.Check(pool.LPUnits.Uint64(), Equals, uint64(100), Commentf("%d", pool.LPUnits.Uint64()))
	c.Check(pool.SynthUnits.Uint64(), Equals, uint64(0), Commentf("%d", pool.SynthUnits.Uint64()))
}
