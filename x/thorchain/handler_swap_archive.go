package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (h SwapHandler) validateV65(ctx cosmos.Context, msg MsgSwap) error {
	if err := msg.ValidateBasicV63(); err != nil {
		return err
	}

	target := msg.TargetAsset
	if target.IsSyntheticAsset() {

		// the following  only applicable for chaosnet
		totalLiquidityRUNE, err := h.getTotalLiquidityRUNE(ctx)
		if err != nil {
			return ErrInternal(err, "fail to get total liquidity RUNE")
		}

		// total liquidity RUNE after current add liquidity
		if len(msg.Tx.Coins) > 0 {
			// calculate rune value on incoming swap, and add to total liquidity.
			coin := msg.Tx.Coins[0]
			runeVal := coin.Amount
			if !coin.Asset.IsRune() {
				pool, err := h.mgr.Keeper().GetPool(ctx, coin.Asset)
				if err != nil {
					return ErrInternal(err, "fail to get pool")
				}
				runeVal = pool.AssetValueInRune(coin.Amount)
			}
			totalLiquidityRUNE = totalLiquidityRUNE.Add(runeVal)
		}
		maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityRune.String())
		if maximumLiquidityRune < 0 || err != nil {
			maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityRune)
		}
		if maximumLiquidityRune > 0 {
			if totalLiquidityRUNE.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
				return errAddLiquidityRUNEOverLimit
			}
		}

		// fail validation if synth supply is already too high, relative to pool depth
		maxSynths, err := h.mgr.Keeper().GetMimir(ctx, constants.MaxSynthPerAssetDepth.String())
		if maxSynths < 0 || err != nil {
			maxSynths = h.mgr.GetConstants().GetInt64Value(constants.MaxSynthPerAssetDepth)
		}
		synthSupply := h.mgr.Keeper().GetTotalSupply(ctx, target.GetSyntheticAsset())
		pool, err := h.mgr.Keeper().GetPool(ctx, target)
		if err != nil {
			return ErrInternal(err, "fail to get pool")
		}
		if pool.BalanceAsset.IsZero() {
			return fmt.Errorf("pool(%s) has zero asset balance", pool.Asset.String())
		}
		coverage := int64(synthSupply.MulUint64(MaxWithdrawBasisPoints).Quo(pool.BalanceAsset).Uint64())
		if coverage > maxSynths {
			return fmt.Errorf("synth quantity is too high relative to asset depth of related pool (%d/%d)", coverage, maxSynths)
		}

		ensureLiquidityNoLargerThanBond := h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
		if !ensureLiquidityNoLargerThanBond {
			return nil
		}
		totalBondRune, err := h.getTotalActiveBond(ctx)
		if err != nil {
			return ErrInternal(err, "fail to get total bond RUNE")
		}
		if totalLiquidityRUNE.GT(totalBondRune) {
			ctx.Logger().Info("total liquidity RUNE is more than total Bond", "liquidity rune", totalLiquidityRUNE, "bond rune", totalBondRune)
			return errAddLiquidityRUNEMoreThanBond
		}
	}
	return nil
}

func (h SwapHandler) handleV81(ctx cosmos.Context, msg MsgSwap) (*cosmos.Result, error) {
	// test that the network we are running matches the destination network
	if !common.GetCurrentChainNetwork().SoftEquals(msg.Destination.GetNetwork(msg.Destination.GetChain())) {
		return nil, fmt.Errorf("address(%s) is not same network", msg.Destination)
	}
	transactionFee := h.mgr.GasMgr().GetFee(ctx, msg.Destination.GetChain(), common.RuneAsset())
	synthVirtualDepthMult, err := h.mgr.Keeper().GetMimir(ctx, constants.VirtualMultSynths.String())
	if synthVirtualDepthMult < 1 || err != nil {
		synthVirtualDepthMult = h.mgr.GetConstants().GetInt64Value(constants.VirtualMultSynths)
	}

	if msg.TargetAsset.IsRune() && !msg.TargetAsset.IsNativeRune() {
		return nil, fmt.Errorf("target asset can't be %s", msg.TargetAsset.String())
	}

	swapper := newSwapperV90()
	_, _, swapErr := swapper.swap(
		ctx,
		h.mgr.Keeper(),
		msg.Tx,
		msg.TargetAsset,
		msg.Destination,
		msg.TradeTarget,
		transactionFee,
		synthVirtualDepthMult,
		h.mgr)
	if swapErr != nil {
		return nil, swapErr
	}
	return &cosmos.Result{}, nil
}
