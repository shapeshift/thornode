package thorchain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	keeper "gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type WithdrawV24Suite struct{}

var _ = Suite(&WithdrawV24Suite{})

func (s *WithdrawV24Suite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

func (s WithdrawV24Suite) TestCalculateUnsake(c *C) {
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
		withDrawRune, withDrawAsset, unitAfter, err := calculateWithdrawV1(item.poolUnit, item.poolRune, item.poolAsset, item.lpUnit, item.percentage, common.EmptyAsset)
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

func (WithdrawV24Suite) TestWithdraw(c *C) {
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
				WithdrawAddress: "",
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("empty withdraw address"),
		},
		{
			name: "empty-request-txhash",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{},
				Signer:          accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("request tx hash is empty"),
		},
		{
			name: "empty-asset",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.Asset{},
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("empty asset"),
		},
		{
			name: "invalid-basis-point",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10001),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("withdraw basis points 10001 is invalid"),
		},
		{
			name: "invalid-pool-notexist",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.Asset{Chain: common.BNBChain, Ticker: "NOTEXIST", Symbol: "NOTEXIST"},
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("pool-BNB.NOTEXIST doesn't exist"),
		},
		{
			name: "invalid-pool-liquidity-provider-notexist",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.Asset{Chain: common.BNBChain, Ticker: "NOTEXISTSTICKER", Symbol: "NOTEXISTSTICKER"},
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: errors.New("you asked for it"),
		},
		{
			name: "nothing-to-withdraw",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(0),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			ps:            ps,
			runeAmount:    cosmos.ZeroUint(),
			assetAmount:   cosmos.ZeroUint(),
			expectedError: nil,
		},
		{
			name: "all-good-half",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(5000),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
			},
			ps:            ps2,
			runeAmount:    cosmos.NewUint(50 * common.One),
			assetAmount:   cosmos.NewUint(50 * common.One),
			expectedError: nil,
		},
		{
			name: "all-good",
			msg: MsgWithdrawLiquidity{
				WithdrawAddress: runeAddress,
				BasisPoints:     cosmos.NewUint(10000),
				Asset:           common.BNBAsset,
				Tx:              common.Tx{ID: "28B40BF105A112389A339A64BD1A042E6140DC9082C679586C6CF493A9FDE3FE"},
				Signer:          accountAddr,
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
		r, asset, _, _, err := withdrawV1(ctx, version, tc.ps, tc.msg, mgr)
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

func (WithdrawV24Suite) TestWithdrawAsym(c *C) {
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
				WithdrawAddress: runeAddress,
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
				WithdrawAddress: runeAddress,
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
		r, asset, _, _, err := withdrawV1(ctx, version, ps, tc.msg, mgr)
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

func (WithdrawV24Suite) TestImpLossProtection100(c *C) {
	fmt.Println("TEST 100% IMP LOSS")
	version := semver.MustParse("0.24.0")
	signer := GetRandomBech32Addr()
	ctx, k := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(1728000) // 100% imp loss protection

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceRune = cosmos.NewUint(1111 * common.One)
	pool.BalanceAsset = cosmos.NewUint(34 * common.One)
	pool.PoolUnits = pool.BalanceRune
	c.Assert(k.SetPool(ctx, pool), IsNil)

	// mint some rune into the reserve to cover imp loss
	coin := common.NewCoin(common.RuneAsset(), pool.BalanceRune.MulUint64(100))
	c.Assert(k.MintToModule(ctx, ModuleName, coin), IsNil)
	c.Assert(k.SendFromModuleToModule(ctx, ModuleName, ReserveName, common.NewCoins(coin)), IsNil)

	lp1 := LiquidityProvider{
		Asset:              pool.Asset,
		RuneAddress:        GetRandomRUNEAddress(),
		AssetAddress:       GetRandomBNBAddress(),
		LastAddHeight:      0,
		LastWithdrawHeight: 0,
		Units:              cosmos.NewUint(100 * common.One),
		PendingRune:        cosmos.ZeroUint(),
		PendingAsset:       cosmos.ZeroUint(),
		RuneDepositValue:   cosmos.NewUint(100 * common.One).MulUint64(2), // inflating actual ownership by 2x
		AssetDepositValue:  cosmos.NewUint(306030603).MulUint64(2),        // inflating actual ownership by 2x
	}
	k.SetLiquidityProvider(ctx, lp1)

	mgr := NewManagers(k)
	mgr.BeginBlock(ctx)
	c.Assert(k.SaveNetworkFee(ctx, common.BNBChain, NetworkFee{
		Chain:              common.BNBChain,
		TransactionSize:    1,
		TransactionFeeRate: bnbSingleTxFee.Uint64(),
	}), IsNil)

	msg := MsgWithdrawLiquidity{
		WithdrawAddress: lp1.RuneAddress,
		WithdrawalAsset: common.EmptyAsset,
		BasisPoints:     cosmos.NewUint(10000),
		Asset:           pool.Asset,
		Tx:              common.Tx{ID: GetRandomTxHash()},
		Signer:          signer,
	}

	runeAmt, assetAmt, protection, units, _, err := withdrawV24(ctx, version, k, msg, mgr)
	c.Assert(err, IsNil)
	c.Check(runeAmt.Uint64(), Equals, uint64(20154225530), Commentf("%d", runeAmt.Uint64()))
	c.Check(assetAmt.Uint64(), Equals, uint64(523514456), Commentf("%d", assetAmt.Uint64()))
	c.Check(units.Uint64(), Equals, uint64(18399992160), Commentf("%d", units.Uint64()))
	c.Check(protection.Uint64(), Equals, uint64(19792979296), Commentf("%d", protection.Uint64()))
}

func (WithdrawV24Suite) TestImpLossProtection50(c *C) {
	fmt.Println("TEST 50% IMP LOSS")
	version := semver.MustParse("0.24.0")
	signer := GetRandomBech32Addr()
	ctx, k := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(1728000 / 2) // 50% imp loss protection

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceRune = cosmos.NewUint(1111 * common.One)
	pool.BalanceAsset = cosmos.NewUint(34 * common.One)
	pool.PoolUnits = pool.BalanceRune
	c.Assert(k.SetPool(ctx, pool), IsNil)

	// mint some rune into the reserve to cover imp loss
	coin := common.NewCoin(common.RuneAsset(), pool.BalanceRune.MulUint64(100))
	c.Assert(k.MintToModule(ctx, ModuleName, coin), IsNil)
	c.Assert(k.SendFromModuleToModule(ctx, ModuleName, ReserveName, common.NewCoins(coin)), IsNil)

	lp1 := LiquidityProvider{
		Asset:              pool.Asset,
		RuneAddress:        GetRandomRUNEAddress(),
		AssetAddress:       GetRandomBNBAddress(),
		LastAddHeight:      0,
		LastWithdrawHeight: 0,
		Units:              cosmos.NewUint(100 * common.One),
		PendingRune:        cosmos.ZeroUint(),
		PendingAsset:       cosmos.ZeroUint(),
		RuneDepositValue:   cosmos.NewUint(100 * common.One).MulUint64(2), // inflating actual ownership by 2x
		AssetDepositValue:  cosmos.NewUint(306030603).MulUint64(2),        // inflating actual ownership by 2x
	}
	k.SetLiquidityProvider(ctx, lp1)

	mgr := NewManagers(k)
	mgr.BeginBlock(ctx)
	c.Assert(k.SaveNetworkFee(ctx, common.BNBChain, NetworkFee{
		Chain:              common.BNBChain,
		TransactionSize:    1,
		TransactionFeeRate: bnbSingleTxFee.Uint64(),
	}), IsNil)

	msg := MsgWithdrawLiquidity{
		WithdrawAddress: lp1.RuneAddress,
		WithdrawalAsset: common.EmptyAsset,
		BasisPoints:     cosmos.NewUint(10000),
		Asset:           pool.Asset,
		Tx:              common.Tx{ID: GetRandomTxHash()},
		Signer:          signer,
	}

	runeAmt, assetAmt, protection, units, _, err := withdrawV24(ctx, version, k, msg, mgr)
	c.Assert(err, IsNil)
	c.Check(runeAmt.Uint64(), Equals, uint64(15216718522), Commentf("%d", runeAmt.Uint64()))
	c.Check(assetAmt.Uint64(), Equals, uint64(427589620), Commentf("%d", assetAmt.Uint64()))
	c.Check(units.Uint64(), Equals, uint64(14543520242), Commentf("%d", units.Uint64()))
	c.Check(protection.Uint64(), Equals, uint64(9896489648), Commentf("%d", protection.Uint64()))
}
