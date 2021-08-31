package thorchain

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// withdrawV65 all the asset
// it returns runeAmt,assetAmount,protectionRuneAmt,units, lastWithdraw,err
func withdrawV65(ctx cosmos.Context, version semver.Version, msg MsgWithdrawLiquidity, manager Manager) (cosmos.Uint, cosmos.Uint, cosmos.Uint, cosmos.Uint, cosmos.Uint, error) {
	if err := validateWithdrawV1(ctx, manager.Keeper(), msg); err != nil {
		ctx.Logger().Error("msg withdraw fail validation", "error", err)
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), err
	}

	pool, err := manager.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), err
	}
	synthSupply := manager.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	pool.CalcUnits(version, synthSupply)

	lp, err := manager.Keeper().GetLiquidityProvider(ctx, msg.Asset, msg.WithdrawAddress)
	if err != nil {
		ctx.Logger().Error("can't find liquidity provider", "error", err)
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), err

	}

	poolRune := pool.BalanceRune
	poolAsset := pool.BalanceAsset
	originalLiquidityProviderUnits := lp.Units
	fLiquidityProviderUnit := lp.Units
	if lp.Units.IsZero() {
		if !lp.PendingRune.IsZero() || !lp.PendingAsset.IsZero() {
			manager.Keeper().RemoveLiquidityProvider(ctx, lp)
			pool.PendingInboundRune = common.SafeSub(pool.PendingInboundRune, lp.PendingRune)
			pool.PendingInboundAsset = common.SafeSub(pool.PendingInboundAsset, lp.PendingAsset)
			if err := manager.Keeper().SetPool(ctx, pool); err != nil {
				ctx.Logger().Error("fail to save pool pending inbound funds", "error", err)
			}
			// remove lp
			return lp.PendingRune, lp.PendingAsset, cosmos.ZeroUint(), lp.Units, cosmos.ZeroUint(), nil
		}
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errNoLiquidityUnitLeft
	}

	cv := constants.GetConstantValues(version)
	height := common.BlockHeight(ctx)
	if height < (lp.LastAddHeight + cv.GetInt64Value(constants.LiquidityLockUpBlocks)) {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errWithdrawWithin24Hours
	}

	ctx.Logger().Info("pool before withdraw", "pool units", pool.GetPoolUnits(), "balance RUNE", poolRune, "balance asset", poolAsset)
	ctx.Logger().Info("liquidity provider before withdraw", "liquidity provider unit", fLiquidityProviderUnit)

	assetToWithdraw := msg.WithdrawalAsset
	if assetToWithdraw.IsEmpty() {
		// for asymmetric staked lps, need to override the asset
		if lp.RuneAddress.IsEmpty() {
			assetToWithdraw = pool.Asset
		}
		if lp.AssetAddress.IsEmpty() {
			assetToWithdraw = common.RuneAsset()
		}
	}

	// calculate any impermament loss protection or not
	protectionRuneAmount := cosmos.ZeroUint()
	fullProtectionLine, err := manager.Keeper().GetMimir(ctx, constants.FullImpLossProtectionBlocks.String())
	if fullProtectionLine < 0 || err != nil {
		fullProtectionLine = cv.GetInt64Value(constants.FullImpLossProtectionBlocks)
	}
	// only when Pool is in Available status will apply impermanent loss protection
	if fullProtectionLine > 0 && pool.Status == PoolAvailable { // if protection line is zero, no imp loss protection is given
		protectionBasisPoints := calcImpLossProtectionAmtV1(ctx, lp.LastAddHeight, fullProtectionLine)
		implProtectionRuneAmount, depositValue, redeemValue := calcImpLossV63(lp, msg.BasisPoints, protectionBasisPoints, pool)
		ctx.Logger().Info("imp loss calculation", "deposit value", depositValue, "redeem value", redeemValue, "protection", implProtectionRuneAmount)
		if !implProtectionRuneAmount.IsZero() {
			protectionRuneAmount = implProtectionRuneAmount
			_, extraUnits, err := calculatePoolUnitsV1(pool.GetPoolUnits(), poolRune, poolAsset, implProtectionRuneAmount, cosmos.ZeroUint())
			if err != nil {
				return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), err
			}
			ctx.Logger().Info("liquidity provider granted imp loss protection", "extra provider units", extraUnits, "extra rune", implProtectionRuneAmount)
			poolRune = poolRune.Add(implProtectionRuneAmount)
			fLiquidityProviderUnit = fLiquidityProviderUnit.Add(extraUnits)
			pool.LPUnits = pool.LPUnits.Add(extraUnits)
		}
	}

	withdrawRune, withDrawAsset, unitAfter, err := calculateWithdrawV1(pool.GetPoolUnits(), poolRune, poolAsset, fLiquidityProviderUnit, msg.BasisPoints, assetToWithdraw)
	if err != nil {
		ctx.Logger().Error("fail to withdraw", "error", err)
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errWithdrawFail
	}
	if (withdrawRune.Equal(poolRune) && !withDrawAsset.Equal(poolAsset)) || (!withdrawRune.Equal(poolRune) && withDrawAsset.Equal(poolAsset)) {
		ctx.Logger().Error("fail to withdraw: cannot withdraw 100% of only one side of the pool")
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errWithdrawFail
	}
	withDrawAsset = cosmos.RoundToDecimal(withDrawAsset, pool.Decimals)
	gasAsset := cosmos.ZeroUint()
	// If the pool is empty, and there is a gas asset, subtract required gas
	if common.SafeSub(pool.GetPoolUnits(), fLiquidityProviderUnit).Add(unitAfter).IsZero() {
		maxGas, err := manager.GasMgr().GetMaxGas(ctx, pool.Asset.GetChain())
		if err != nil {
			ctx.Logger().Error("fail to get gas for asset", "asset", pool.Asset, "error", err)
			return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errWithdrawFail
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
		} else if pool.Asset.GetChain().GetGasAsset().Equals(pool.Asset) {
			gasAsset = maxGas.Amount
			if gasAsset.GT(withDrawAsset) {
				gasAsset = withDrawAsset
			}
			withDrawAsset = common.SafeSub(withDrawAsset, gasAsset)
		}
	}

	ctx.Logger().Info("client withdraw", "RUNE", withdrawRune, "asset", withDrawAsset, "units left", unitAfter)
	// update pool
	pool.LPUnits = common.SafeSub(pool.LPUnits, common.SafeSub(fLiquidityProviderUnit, unitAfter))
	pool.BalanceRune = common.SafeSub(poolRune, withdrawRune)
	pool.BalanceAsset = common.SafeSub(poolAsset, withDrawAsset)

	ctx.Logger().Info("pool after withdraw", "pool unit", pool.GetPoolUnits(), "balance RUNE", pool.BalanceRune, "balance asset", pool.BalanceAsset)

	lp.LastWithdrawHeight = common.BlockHeight(ctx)
	lp.RuneDepositValue = common.SafeSub(lp.RuneDepositValue, common.GetShare(common.SafeSub(fLiquidityProviderUnit, unitAfter), pool.GetPoolUnits(), pool.BalanceRune))
	lp.AssetDepositValue = common.SafeSub(lp.AssetDepositValue, common.GetShare(common.SafeSub(fLiquidityProviderUnit, unitAfter), pool.GetPoolUnits(), pool.BalanceAsset))
	lp.Units = unitAfter

	// Create a pool event if THORNode have no rune or assets
	if pool.BalanceAsset.IsZero() || pool.BalanceRune.IsZero() {
		poolEvt := NewEventPool(pool.Asset, PoolStaged)
		if err := manager.EventMgr().EmitEvent(ctx, poolEvt); nil != err {
			ctx.Logger().Error("fail to emit pool event", "error", err)
		}
		pool.Status = PoolStaged
	}

	if err := manager.Keeper().SetPool(ctx, pool); err != nil {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), ErrInternal(err, "fail to save pool")
	}
	if manager.Keeper().RagnarokInProgress(ctx) {
		manager.Keeper().SetLiquidityProvider(ctx, lp)
	} else {
		if !lp.Units.Add(lp.PendingAsset).Add(lp.PendingRune).IsZero() {
			manager.Keeper().SetLiquidityProvider(ctx, lp)
		} else {
			manager.Keeper().RemoveLiquidityProvider(ctx, lp)
		}
	}
	// add rune from the reserve to the asgard module, to cover imp loss protection
	if !protectionRuneAmount.IsZero() {
		err := manager.Keeper().SendFromModuleToModule(ctx, ReserveName, AsgardName, common.NewCoins(common.NewCoin(common.RuneAsset(), protectionRuneAmount)))
		if err != nil {
			return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), ErrInternal(err, "fail to move imp loss protection rune from the reserve to asgard")
		}
	}
	return withdrawRune, withDrawAsset, protectionRuneAmount, common.SafeSub(originalLiquidityProviderUnits, unitAfter), gasAsset, nil
}
