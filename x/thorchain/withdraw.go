package thorchain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

func validateWithdraw(ctx cosmos.Context, keeper keeper.Keeper, msg MsgWithdrawLiquidity) error {
	if msg.RuneAddress.IsEmpty() {
		return errors.New("empty rune address")
	}
	if msg.Tx.ID.IsEmpty() {
		return errors.New("request tx hash is empty")
	}
	if msg.Asset.IsEmpty() {
		return errors.New("empty asset")
	}
	withdrawBasisPoints := msg.BasisPoints
	if !withdrawBasisPoints.GTE(cosmos.ZeroUint()) || withdrawBasisPoints.GT(cosmos.NewUint(MaxWithdrawBasisPoints)) {
		return fmt.Errorf("withdraw basis points %s is invalid", msg.BasisPoints)
	}
	if !keeper.PoolExist(ctx, msg.Asset) {
		// pool doesn't exist
		return fmt.Errorf("pool-%s doesn't exist", msg.Asset)
	}
	return nil
}

// withdraw all the asset
// it returns runeAmt,assetAmount,units, lastWithdraw,err
func withdraw(ctx cosmos.Context, version semver.Version, keeper keeper.Keeper, msg MsgWithdrawLiquidity, manager Manager) (cosmos.Uint, cosmos.Uint, cosmos.Uint, cosmos.Uint, error) {
	if err := validateWithdraw(ctx, keeper, msg); err != nil {
		ctx.Logger().Error("msg withdraw fail validation", "error", err)
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), err
	}

	pool, err := keeper.GetPool(ctx, msg.Asset)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), err
	}

	lp, err := keeper.GetLiquidityProvider(ctx, msg.Asset, msg.RuneAddress)
	if err != nil {
		ctx.Logger().Error("can't find liquidity provider", "error", err)
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), err

	}

	poolUnits := pool.PoolUnits
	poolRune := pool.BalanceRune
	poolAsset := pool.BalanceAsset
	fLiquidityProviderUnit := lp.Units
	if lp.Units.IsZero() || msg.BasisPoints.IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errNoLiquidityUnitLeft
	}

	cv := constants.GetConstantValues(version)
	height := common.BlockHeight(ctx)
	if height < (lp.LastAddHeight + cv.GetInt64Value(constants.LiquidityLockUpBlocks)) {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errWithdrawWithin24Hours
	}

	ctx.Logger().Info("pool before withdraw", "pool unit", poolUnits, "balance RUNE", poolRune, "balance asset", poolAsset)
	ctx.Logger().Info("liquidity provider before withdraw", "liquidity provider unit", fLiquidityProviderUnit)
	withdrawRune, withDrawAsset, unitAfter, err := calculateWithdraw(poolUnits, poolRune, poolAsset, fLiquidityProviderUnit, msg.BasisPoints, msg.WithdrawalAsset)
	if err != nil {
		ctx.Logger().Error("fail to withdraw", "error", err)
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errWithdrawFail
	}
	if (withdrawRune.Equal(poolRune) && !withDrawAsset.Equal(poolAsset)) || (!withdrawRune.Equal(poolRune) && withDrawAsset.Equal(poolAsset)) {
		ctx.Logger().Error("fail to withdraw: cannot withdraw 100% of only one side of the pool")
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errWithdrawFail
	}
	gasAsset := cosmos.ZeroUint()
	// If the pool is empty, and there is a gas asset, subtract required gas
	if common.SafeSub(poolUnits, fLiquidityProviderUnit).Add(unitAfter).IsZero() {
		maxGas, err := manager.GasMgr().GetMaxGas(ctx, pool.Asset.Chain)
		if err != nil {
			ctx.Logger().Error("fail to get gas for asset", "asset", pool.Asset, "error", err)
			return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errWithdrawFail
		}
		// minus gas costs for our transactions
		// TODO: chain specific logic should be in a single location
		if pool.Asset.IsBNB() && !common.RuneAsset().Chain.Equals(common.THORChain) {
			originalAsset := withDrawAsset
			withDrawAsset = common.SafeSub(
				withDrawAsset,
				maxGas.Amount.MulUint64(2), // RUNE asset is on binance chain
			)
			gasAsset = originalAsset.Sub(withDrawAsset)
		} else if pool.Asset.Chain.GetGasAsset().Equals(pool.Asset) {
			gasAsset = maxGas.Amount
			if gasAsset.GT(withDrawAsset) {
				gasAsset = withDrawAsset
			}
			withDrawAsset = common.SafeSub(withDrawAsset, gasAsset)
		}
	}

	withdrawRune = withdrawRune.Add(lp.PendingRune) // extract pending rune
	lp.PendingRune = cosmos.ZeroUint()              // reset pending to zero

	ctx.Logger().Info("client withdraw", "RUNE", withdrawRune, "asset", withDrawAsset, "units left", unitAfter)
	// update pool
	pool.PoolUnits = common.SafeSub(poolUnits, fLiquidityProviderUnit).Add(unitAfter)
	pool.BalanceRune = common.SafeSub(poolRune, withdrawRune)
	pool.BalanceAsset = common.SafeSub(poolAsset, withDrawAsset)

	ctx.Logger().Info("pool after withdraw", "pool unit", pool.PoolUnits, "balance RUNE", pool.BalanceRune, "balance asset", pool.BalanceAsset)
	// update liquidity provider
	acc, err := lp.RuneAddress.AccAddress()
	if err != nil {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), ErrInternal(err, "fail to convert rune address")
	}
	err = keeper.RemoveOwnership(ctx, common.NewCoin(pool.Asset.LiquidityAsset(), lp.Units.Sub(unitAfter)), acc)
	if err != nil {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), ErrInternal(err, "fail to withdraw liquidity")
	}
	lp.LastWithdrawHeight = common.BlockHeight(ctx)

	// Create a pool event if THORNode have no rune or assets
	if pool.BalanceAsset.IsZero() || pool.BalanceRune.IsZero() {
		poolEvt := NewEventPool(pool.Asset, PoolStaged)
		if err := manager.EventMgr().EmitEvent(ctx, poolEvt); nil != err {
			ctx.Logger().Error("fail to emit pool event", "error", err)
		}
		pool.Status = PoolStaged
	}

	if err := keeper.SetPool(ctx, pool); err != nil {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), ErrInternal(err, "fail to save pool")
	}
	if keeper.RagnarokInProgress(ctx) {
		keeper.SetLiquidityProvider(ctx, lp)
	} else {
		if !lp.Units.IsZero() {
			keeper.SetLiquidityProvider(ctx, lp)
		} else {
			keeper.RemoveLiquidityProvider(ctx, lp)
		}
	}
	return withdrawRune, withDrawAsset, common.SafeSub(fLiquidityProviderUnit, unitAfter), gasAsset, nil
}

func calculateWithdraw(poolUnits, poolRune, poolAsset, lpUnits, withdrawBasisPoints cosmos.Uint, withdrawalAsset common.Asset) (cosmos.Uint, cosmos.Uint, cosmos.Uint, error) {
	if poolUnits.IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errors.New("poolUnits can't be zero")
	}
	if poolRune.IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errors.New("pool rune balance can't be zero")
	}
	if poolAsset.IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errors.New("pool asset balance can't be zero")
	}
	if lpUnits.IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errors.New("liquidity provider unit can't be zero")
	}
	if withdrawBasisPoints.GT(cosmos.NewUint(MaxWithdrawBasisPoints)) {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), fmt.Errorf("withdraw basis point %s is not valid", withdrawBasisPoints.String())
	}

	unitsToClaim := common.GetShare(withdrawBasisPoints, cosmos.NewUint(10000), lpUnits)
	unitAfter := common.SafeSub(lpUnits, unitsToClaim)
	if withdrawalAsset.IsEmpty() {
		withdrawRune := common.GetShare(unitsToClaim, poolUnits, poolRune)
		withdrawAsset := common.GetShare(unitsToClaim, poolUnits, poolAsset)
		return withdrawRune, withdrawAsset, unitAfter, nil
	}
	if withdrawalAsset.IsRune() {
		return calcAsymWithdrawal(unitsToClaim, poolUnits, poolRune), cosmos.ZeroUint(), unitAfter, nil
	}
	return cosmos.ZeroUint(), calcAsymWithdrawal(unitsToClaim, poolUnits, poolAsset), unitAfter, nil
}

func calcAsymWithdrawal(s, T, A cosmos.Uint) cosmos.Uint {
	// share = (s * A * (2 * T^2 - 2 * T * s + s^2))/T^3
	// s = liquidity provider units for member (after factoring in withdrawBasisPoints)
	// T = totalPoolUnits for pool
	// A = assetDepth to be withdrawn
	// (part1 * (part2 - part3 + part4)) / part5
	part1 := s.Mul(A)
	part2 := T.Mul(T).MulUint64(2)
	part3 := T.Mul(s).MulUint64(2)
	part4 := s.Mul(s)
	numerator := part1.Mul(common.SafeSub(part2, part3).Add(part4))
	part5 := T.Mul(T).Mul(T)
	return numerator.Quo(part5)
}
