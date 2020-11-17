package thorchain

import (
	"errors"
	"fmt"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	keeper "gitlab.com/thorchain/thornode/x/thorchain/keeper"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

type WithdrawSuite struct{}

var _ = Suite(&WithdrawSuite{})

func (s *WithdrawSuite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

type WithdrawTestKeeper struct {
	keeper.KVStoreDummy
	store       map[string]interface{}
	networkFees map[common.Chain]NetworkFee
	keeper      keeper.Keeper
}

func NewWithdrawTestKeeper(keeper keeper.Keeper) *WithdrawTestKeeper {
	return &WithdrawTestKeeper{
		keeper:      keeper,
		store:       make(map[string]interface{}),
		networkFees: make(map[common.Chain]NetworkFee),
	}
}

func (k *WithdrawTestKeeper) PoolExist(ctx cosmos.Context, asset common.Asset) bool {
	if asset.Equals(common.Asset{Chain: common.BNBChain, Symbol: "NOTEXIST", Ticker: "NOTEXIST"}) {
		return false
	}
	return true
}

func (k *WithdrawTestKeeper) GetPool(ctx cosmos.Context, asset common.Asset) (types.Pool, error) {
	if asset.Equals(common.Asset{Chain: common.BNBChain, Symbol: "NOTEXIST", Ticker: "NOTEXIST"}) {
		return types.Pool{}, nil
	} else {
		if val, ok := k.store[asset.String()]; ok {
			return val.(types.Pool), nil
		}
		return types.Pool{
			BalanceRune:  cosmos.NewUint(100).MulUint64(common.One),
			BalanceAsset: cosmos.NewUint(100).MulUint64(common.One),
			PoolUnits:    cosmos.NewUint(100).MulUint64(common.One),
			Status:       types.Enabled,
			Asset:        asset,
		}, nil
	}
}

func (k *WithdrawTestKeeper) SetPool(ctx cosmos.Context, ps Pool) error {
	k.store[ps.Asset.String()] = ps
	return nil
}

func (k *WithdrawTestKeeper) GetGas(ctx cosmos.Context, asset common.Asset) ([]cosmos.Uint, error) {
	return []cosmos.Uint{cosmos.NewUint(37500), cosmos.NewUint(30000)}, nil
}

func (k *WithdrawTestKeeper) AddOwnership(ctx cosmos.Context, coin common.Coin, addr cosmos.AccAddress) error {
	return k.keeper.AddOwnership(ctx, coin, addr)
}

func (k *WithdrawTestKeeper) RemoveOwnership(ctx cosmos.Context, coin common.Coin, addr cosmos.AccAddress) error {
	return k.keeper.RemoveOwnership(ctx, coin, addr)
}

func (k *WithdrawTestKeeper) GetLiquidityProvider(ctx cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	if asset.Equals(common.Asset{Chain: common.BNBChain, Symbol: "NOTEXISTSTICKER", Ticker: "NOTEXISTSTICKER"}) {
		return types.LiquidityProvider{}, errors.New("you asked for it")
	}
	if notExistLiquidityProviderAsset.Equals(asset) {
		return LiquidityProvider{}, errors.New("simulate error for test")
	}
	return k.keeper.GetLiquidityProvider(ctx, asset, addr)
}

func (k *WithdrawTestKeeper) GetNetworkFee(ctx cosmos.Context, chain common.Chain) (NetworkFee, error) {
	return k.networkFees[chain], nil
}

func (k *WithdrawTestKeeper) SaveNetworkFee(ctx cosmos.Context, chain common.Chain, networkFee NetworkFee) error {
	k.networkFees[chain] = networkFee
	return nil
}

func (k *WithdrawTestKeeper) SetLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider) {
	k.keeper.SetLiquidityProvider(ctx, lp)
}

func (s WithdrawSuite) TestCalculateUnsake(c *C) {
	inputs := []struct {
		name                  string
		poolUnit              cosmos.Uint
		poolRune              cosmos.Uint
		poolAsset             cosmos.Uint
		lpUnit                cosmos.Uint
		percentage            cosmos.Uint
		expectedWithdrawRune  cosmos.Uint
		expectedWithdrawAsset cosmos.Uint
		expectedUnitLeft      cosmos.Uint
		expectedErr           error
	}{
		{
			name:                  "zero-poolunit",
			poolUnit:              cosmos.ZeroUint(),
			poolRune:              cosmos.ZeroUint(),
			poolAsset:             cosmos.ZeroUint(),
			lpUnit:                cosmos.ZeroUint(),
			percentage:            cosmos.ZeroUint(),
			expectedWithdrawRune:  cosmos.ZeroUint(),
			expectedWithdrawAsset: cosmos.ZeroUint(),
			expectedUnitLeft:      cosmos.ZeroUint(),
			expectedErr:           errors.New("poolUnits can't be zero"),
		},

		{
			name:                  "zero-poolrune",
			poolUnit:              cosmos.NewUint(500 * common.One),
			poolRune:              cosmos.ZeroUint(),
			poolAsset:             cosmos.ZeroUint(),
			lpUnit:                cosmos.ZeroUint(),
			percentage:            cosmos.ZeroUint(),
			expectedWithdrawRune:  cosmos.ZeroUint(),
			expectedWithdrawAsset: cosmos.ZeroUint(),
			expectedUnitLeft:      cosmos.ZeroUint(),
			expectedErr:           errors.New("pool rune balance can't be zero"),
		},

		{
			name:                  "zero-poolasset",
			poolUnit:              cosmos.NewUint(500 * common.One),
			poolRune:              cosmos.NewUint(500 * common.One),
			poolAsset:             cosmos.ZeroUint(),
			lpUnit:                cosmos.ZeroUint(),
			percentage:            cosmos.ZeroUint(),
			expectedWithdrawRune:  cosmos.ZeroUint(),
			expectedWithdrawAsset: cosmos.ZeroUint(),
			expectedUnitLeft:      cosmos.ZeroUint(),
			expectedErr:           errors.New("pool asset balance can't be zero"),
		},
		{
			name:                  "negative-liquidity-provider-unit",
			poolUnit:              cosmos.NewUint(500 * common.One),
			poolRune:              cosmos.NewUint(500 * common.One),
			poolAsset:             cosmos.NewUint(5100 * common.One),
			lpUnit:                cosmos.ZeroUint(),
			percentage:            cosmos.ZeroUint(),
			expectedWithdrawRune:  cosmos.ZeroUint(),
			expectedWithdrawAsset: cosmos.ZeroUint(),
			expectedUnitLeft:      cosmos.ZeroUint(),
			expectedErr:           errors.New("liquidity provider unit can't be zero"),
		},

		{
			name:                  "percentage-larger-than-100",
			poolUnit:              cosmos.NewUint(500 * common.One),
			poolRune:              cosmos.NewUint(500 * common.One),
			poolAsset:             cosmos.NewUint(500 * common.One),
			lpUnit:                cosmos.NewUint(100 * common.One),
			percentage:            cosmos.NewUint(12000),
			expectedWithdrawRune:  cosmos.ZeroUint(),
			expectedWithdrawAsset: cosmos.ZeroUint(),
			expectedUnitLeft:      cosmos.ZeroUint(),
			expectedErr:           fmt.Errorf("withdraw basis point %s is not valid", cosmos.NewUint(12000)),
		},
		{
			name:                  "withdraw-1",
			poolUnit:              cosmos.NewUint(700 * common.One),
			poolRune:              cosmos.NewUint(700 * common.One),
			poolAsset:             cosmos.NewUint(700 * common.One),
			lpUnit:                cosmos.NewUint(200 * common.One),
			percentage:            cosmos.NewUint(10000),
			expectedUnitLeft:      cosmos.ZeroUint(),
			expectedWithdrawAsset: cosmos.NewUint(200 * common.One),
			expectedWithdrawRune:  cosmos.NewUint(200 * common.One),
			expectedErr:           nil,
		},
		{
			name:                  "withdraw-2",
			poolUnit:              cosmos.NewUint(100),
			poolRune:              cosmos.NewUint(15 * common.One),
			poolAsset:             cosmos.NewUint(155 * common.One),
			lpUnit:                cosmos.NewUint(100),
			percentage:            cosmos.NewUint(1000),
			expectedUnitLeft:      cosmos.NewUint(90),
			expectedWithdrawAsset: cosmos.NewUint(1550000000),
			expectedWithdrawRune:  cosmos.NewUint(150000000),
			expectedErr:           nil,
		},
	}

	for _, item := range inputs {
		c.Logf("name:%s", item.name)
		withDrawRune, withDrawAsset, unitAfter, err := calculateWithdraw(item.poolUnit, item.poolRune, item.poolAsset, item.lpUnit, item.percentage, common.EmptyAsset)
		if item.expectedErr == nil {
			c.Assert(err, IsNil)
		} else {
			c.Assert(err.Error(), Equals, item.expectedErr.Error())
		}
		c.Logf("expected rune:%s,rune:%s", item.expectedWithdrawRune, withDrawRune)
		c.Check(item.expectedWithdrawRune.Uint64(), Equals, withDrawRune.Uint64(), Commentf("Expected %d, got %d", item.expectedWithdrawRune.Uint64(), withDrawRune.Uint64()))
		c.Check(item.expectedWithdrawAsset.Uint64(), Equals, withDrawAsset.Uint64(), Commentf("Expected %d, got %d", item.expectedWithdrawAsset.Uint64(), withDrawAsset.Uint64()))
		c.Check(item.expectedUnitLeft.Uint64(), Equals, unitAfter.Uint64())
	}
}

// TestValidateWithdraw is to test validateWithdraw function
func (s WithdrawSuite) TestValidateWithdraw(c *C) {
	accountAddr := GetRandomNodeAccount(NodeWhiteListed).NodeAddress
	runeAddress, err := common.NewAddress("bnb1g0xakzh03tpa54khxyvheeu92hwzypkdce77rm")
	if err != nil {
		c.Error("fail to create new BNB Address")
	}
	inputs := []struct {
		name          string
		msg           MsgWithdrawLiquidity
		expectedError error
	}{
		{
			name: "empty-rune-address",
			msg: MsgWithdrawLiquidity{
				RuneAddress: "",
				BasisPoints: cosmos.NewUint(10000),
				Asset:       common.BNBAsset,
				Tx:          common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:      accountAddr,
			},
			expectedError: errors.New("empty rune address"),
		},
		{
			name: "empty-withdraw-basis-points",
			msg: MsgWithdrawLiquidity{
				RuneAddress: runeAddress,
				BasisPoints: cosmos.ZeroUint(),
				Asset:       common.BNBAsset,
				Tx:          common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:      accountAddr,
			},
			expectedError: nil,
		},
		{
			name: "empty-request-txhash",
			msg: MsgWithdrawLiquidity{
				RuneAddress: runeAddress,
				BasisPoints: cosmos.NewUint(10000),
				Asset:       common.BNBAsset,
				Tx:          common.Tx{},
				Signer:      accountAddr,
			},
			expectedError: errors.New("request tx hash is empty"),
		},
		{
			name: "empty-asset",
			msg: MsgWithdrawLiquidity{
				RuneAddress: runeAddress,
				BasisPoints: cosmos.NewUint(10000),
				Asset:       common.Asset{},
				Tx:          common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:      accountAddr,
			},
			expectedError: errors.New("empty asset"),
		},
		{
			name: "invalid-basis-point",
			msg: MsgWithdrawLiquidity{
				RuneAddress: runeAddress,
				BasisPoints: cosmos.NewUint(10001),
				Asset:       common.BNBAsset,
				Tx:          common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:      accountAddr,
			},
			expectedError: errors.New("withdraw basis points 10001 is invalid"),
		},
		{
			name: "invalid-pool-notexist",
			msg: MsgWithdrawLiquidity{
				RuneAddress: runeAddress,
				BasisPoints: cosmos.NewUint(10000),
				Asset:       common.Asset{Chain: common.BNBChain, Ticker: "NOTEXIST", Symbol: "NOTEXIST"},
				Tx:          common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:      accountAddr,
			},
			expectedError: errors.New("pool-BNB.NOTEXIST doesn't exist"),
		},
		{
			name: "all-good",
			msg: MsgWithdrawLiquidity{
				RuneAddress: runeAddress,
				BasisPoints: cosmos.NewUint(10000),
				Asset:       common.BNBAsset,
				Tx:          common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:      accountAddr,
			},
			expectedError: nil,
		},
	}

	for _, item := range inputs {
		ctx, _ := setupKeeperForTest(c)
		ps := &WithdrawTestKeeper{}
		c.Logf("name:%s", item.name)
		err := validateWithdraw(ctx, ps, item.msg)
		if item.expectedError != nil {
			c.Assert(err, NotNil)
			c.Assert(err.Error(), Equals, item.expectedError.Error())
			continue
		}
		c.Assert(err, IsNil)
	}
}

func (WithdrawSuite) TestWithdraw(c *C) {
	ctx, k := setupKeeperForTest(c)
	accountAddr := GetRandomNodeAccount(NodeWhiteListed).NodeAddress
	runeAddress := GetRandomRUNEAddress()
	ps := NewWithdrawTestKeeper(k)
	ps2 := getWithdrawTestKeeper(c, ctx, k, runeAddress)

	remainGas := uint64(37500)
	testCases := []struct {
		name          string
		msg           MsgWithdrawLiquidity
		ps            keeper.Keeper
		runeAmount    cosmos.Uint
		assetAmount   cosmos.Uint
		expectedError error
	}{
		{
			name: "empty-rune-address",
			msg: MsgWithdrawLiquidity{
				RuneAddress: "",
				BasisPoints: cosmos.NewUint(10000),
				Asset:       common.BNBAsset,
				Tx:          common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:      accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("empty rune address"),
		},
		{
			name: "empty-withdraw-basis-points",
			msg: MsgWithdrawLiquidity{
				RuneAddress: runeAddress,
				BasisPoints: cosmos.ZeroUint(),
				Asset:       common.BNBAsset,
				Tx:          common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:      accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("nothing to withdraw"),
		},
		{
			name: "empty-request-txhash",
			msg: MsgWithdrawLiquidity{
				RuneAddress: runeAddress,
				BasisPoints: cosmos.NewUint(10000),
				Asset:       common.BNBAsset,
				Tx:          common.Tx{},
				Signer:      accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("request tx hash is empty"),
		},
		{
			name: "empty-asset",
			msg: MsgWithdrawLiquidity{
				RuneAddress: runeAddress,
				BasisPoints: cosmos.NewUint(10000),
				Asset:       common.Asset{},
				Tx:          common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:      accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("empty asset"),
		},
		{
			name: "invalid-basis-point",
			msg: MsgWithdrawLiquidity{
				RuneAddress: runeAddress,
				BasisPoints: cosmos.NewUint(10001),
				Asset:       common.BNBAsset,
				Tx:          common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:      accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("withdraw basis points 10001 is invalid"),
		},
		{
			name: "invalid-pool-notexist",
			msg: MsgWithdrawLiquidity{
				RuneAddress: runeAddress,
				BasisPoints: cosmos.NewUint(10000),
				Asset:       common.Asset{Chain: common.BNBChain, Ticker: "NOTEXIST", Symbol: "NOTEXIST"},
				Tx:          common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:      accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("pool-BNB.NOTEXIST doesn't exist"),
		},
		{
			name: "invalid-pool-liquidity-provider-notexist",
			msg: MsgWithdrawLiquidity{
				RuneAddress: runeAddress,
				BasisPoints: cosmos.NewUint(10000),
				Asset:       common.Asset{Chain: common.BNBChain, Ticker: "NOTEXISTSTICKER", Symbol: "NOTEXISTSTICKER"},
				Tx:          common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:      accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("you asked for it"),
		},
		{
			name: "nothing-to-withdraw",
			msg: MsgWithdrawLiquidity{
				RuneAddress: runeAddress,
				BasisPoints: cosmos.NewUint(0),
				Asset:       common.BNBAsset,
				Tx:          common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:      accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("nothing to withdraw"),
		},
		{
			name: "all-good-half",
			msg: MsgWithdrawLiquidity{
				RuneAddress: runeAddress,
				BasisPoints: cosmos.NewUint(5000),
				Asset:       common.BNBAsset,
				Tx:          common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:      accountAddr,
			},
			ps:            ps2,
			runeAmount:    cosmos.NewUint(50 * common.One),
			assetAmount:   cosmos.NewUint(50 * common.One),
			expectedError: nil,
		},
		{
			name: "all-good",
			msg: MsgWithdrawLiquidity{
				RuneAddress: runeAddress,
				BasisPoints: cosmos.NewUint(10000),
				Asset:       common.BNBAsset,
				Tx:          common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:      accountAddr,
			},
			ps:            ps2,
			runeAmount:    cosmos.NewUint(50 * common.One),
			assetAmount:   cosmos.NewUint(50 * common.One).Sub(cosmos.NewUint(remainGas)),
			expectedError: nil,
		},
	}
	for _, tc := range testCases {
		c.Logf("name:%s", tc.name)
		version := constants.SWVersion
		mgr := NewManagers(tc.ps)
		mgr.BeginBlock(ctx)
		tc.ps.SaveNetworkFee(ctx, common.BNBChain, NetworkFee{
			Chain:              common.BNBChain,
			TransactionSize:    1,
			TransactionFeeRate: bnbSingleTxFee.Uint64(),
		})
		r, asset, _, _, err := withdraw(ctx, version, tc.ps, tc.msg, mgr)
		if tc.expectedError != nil {
			c.Assert(err, NotNil)
			c.Check(err.Error(), Equals, tc.expectedError.Error())
			c.Check(r.Uint64(), Equals, tc.runeAmount.Uint64())
			c.Check(asset.Uint64(), Equals, tc.assetAmount.Uint64())
			continue
		}
		c.Assert(err, IsNil)
		c.Assert(r.Uint64(), Equals, tc.runeAmount.Uint64(), Commentf("%d != %d", r.Uint64(), tc.runeAmount.Uint64()))
		c.Assert(asset.Equal(tc.assetAmount), Equals, true, Commentf("expect:%s, however got:%s", tc.assetAmount.String(), asset.String()))
	}
}

func (WithdrawSuite) TestWithdrawAsym(c *C) {
	accountAddr := GetRandomNodeAccount(NodeWhiteListed).NodeAddress
	runeAddress := GetRandomRUNEAddress()

	testCases := []struct {
		name          string
		msg           MsgWithdrawLiquidity
		runeAmount    cosmos.Uint
		assetAmount   cosmos.Uint
		expectedError error
	}{
		{
			name: "all-good-asymmetric-rune",
			msg: MsgWithdrawLiquidity{
				RuneAddress:     runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				WithdrawalAsset: common.RuneAsset(),
				Signer:          accountAddr,
			},
			runeAmount:    cosmos.NewUint(6250000000),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: nil,
		},
		{
			name: "all-good-asymmetric-asset",
			msg: MsgWithdrawLiquidity{
				RuneAddress:     runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				WithdrawalAsset: common.BNBAsset,
				Signer:          accountAddr,
			},
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.NewUint(6250000000),
			expectedError: nil,
		},
	}
	for _, tc := range testCases {
		c.Logf("name:%s", tc.name)
		version := constants.SWVersion
		ctx, k := setupKeeperForTest(c)
		ps := getWithdrawTestKeeper2(c, ctx, k, runeAddress)
		mgr := NewManagers(ps)
		mgr.BeginBlock(ctx)
		ps.SaveNetworkFee(ctx, common.BNBChain, NetworkFee{
			Chain:              common.BNBChain,
			TransactionSize:    1,
			TransactionFeeRate: bnbSingleTxFee.Uint64(),
		})
		r, asset, _, _, err := withdraw(ctx, version, ps, tc.msg, mgr)
		if tc.expectedError != nil {
			c.Assert(err, NotNil)
			c.Check(err.Error(), Equals, tc.expectedError.Error())
			c.Check(r.Uint64(), Equals, tc.runeAmount.Uint64())
			c.Check(asset.Uint64(), Equals, tc.assetAmount.Uint64())
			continue
		}
		c.Assert(err, IsNil)
		c.Assert(r.Uint64(), Equals, tc.runeAmount.Uint64(), Commentf("%d != %d", r.Uint64(), tc.runeAmount.Uint64()))
		c.Assert(asset.Equal(tc.assetAmount), Equals, true, Commentf("expect:%s, however got:%s", tc.assetAmount.String(), asset.String()))
	}
}

func getWithdrawTestKeeper(c *C, ctx cosmos.Context, k keeper.Keeper, runeAddress common.Address) keeper.Keeper {
	store := NewWithdrawTestKeeper(k)
	pool := Pool{
		BalanceRune:  cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BNBAsset,
		PoolUnits:    cosmos.NewUint(100 * common.One),
		Status:       PoolEnabled,
	}
	c.Assert(store.SetPool(ctx, pool), IsNil)
	lp := LiquidityProvider{
		Asset:        pool.Asset,
		RuneAddress:  runeAddress,
		AssetAddress: runeAddress,
		Units:        cosmos.NewUint(100 * common.One),
		PendingRune:  cosmos.ZeroUint(),
	}
	store.SetLiquidityProvider(ctx, lp)
	accAddr, err := lp.RuneAddress.AccAddress()
	c.Assert(err, IsNil)
	amt := store.GetLiquidityProviderBalance(ctx, pool.Asset.LiquidityAsset(), accAddr)
	if amt.IsZero() {
		err = store.AddOwnership(ctx, common.NewCoin(pool.Asset.LiquidityAsset(), lp.Units), accAddr)
		c.Assert(err, IsNil)
	}

	return store
}

// this one has an extra liquidity provider already set
func getWithdrawTestKeeper2(c *C, ctx cosmos.Context, k keeper.Keeper, runeAddress common.Address) keeper.Keeper {
	store := NewWithdrawTestKeeper(k)
	pool := Pool{
		BalanceRune:  cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BNBAsset,
		PoolUnits:    cosmos.NewUint(200 * common.One),
		Status:       PoolEnabled,
	}
	c.Assert(store.SetPool(ctx, pool), IsNil)
	lp := LiquidityProvider{
		Asset:        pool.Asset,
		RuneAddress:  runeAddress,
		AssetAddress: runeAddress,
		Units:        cosmos.NewUint(100 * common.One),
		PendingRune:  cosmos.ZeroUint(),
	}
	store.SetLiquidityProvider(ctx, lp)
	accAddr, err := lp.RuneAddress.AccAddress()
	c.Assert(err, IsNil)
	amt := store.GetLiquidityProviderBalance(ctx, pool.Asset.LiquidityAsset(), accAddr)
	if amt.IsZero() {
		err = store.AddOwnership(ctx, common.NewCoin(pool.Asset.LiquidityAsset(), lp.Units), accAddr)
		c.Assert(err, IsNil)
	}

	return store
}
