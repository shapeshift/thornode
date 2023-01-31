package thorchain

import (
	"errors"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (h SwapHandler) validateV98(ctx cosmos.Context, msg MsgSwap) error {
	if err := msg.ValidateBasicV63(); err != nil {
		return err
	}

	target := msg.TargetAsset
	if isTradingHalt(ctx, &msg, h.mgr) {
		return errors.New("trading is halted, can't process swap")
	}
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
				pool, err := h.mgr.Keeper().GetPool(ctx, coin.Asset.GetLayer1Asset())
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
		maxSynths, err := h.mgr.Keeper().GetMimir(ctx, constants.MaxSynthPerPoolDepth.String())
		if maxSynths < 0 || err != nil {
			maxSynths = h.mgr.GetConstants().GetInt64Value(constants.MaxSynthPerPoolDepth)
		}
		synthSupply := h.mgr.Keeper().GetTotalSupply(ctx, target.GetSyntheticAsset())
		pool, err := h.mgr.Keeper().GetPool(ctx, target.GetLayer1Asset())
		if err != nil {
			return ErrInternal(err, "fail to get pool")
		}
		if pool.BalanceAsset.IsZero() {
			return fmt.Errorf("pool(%s) has zero asset balance", pool.Asset.String())
		}
		coverage := int64(synthSupply.MulUint64(MaxWithdrawBasisPoints).Quo(pool.BalanceAsset.MulUint64(2)).Uint64())
		if coverage > maxSynths {
			return fmt.Errorf("synth quantity is too high relative to asset depth of related pool (%d/%d)", coverage, maxSynths)
		}

		ensureLiquidityNoLargerThanBond := h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
		if !ensureLiquidityNoLargerThanBond {
			return nil
		}
		securityBond, err := h.getEffectiveSecurityBond(ctx)
		if err != nil {
			return ErrInternal(err, "fail to get security bond RUNE")
		}
		if totalLiquidityRUNE.GT(securityBond) {
			ctx.Logger().Info("total liquidity RUNE is more than effective security bond", "liquidity rune", totalLiquidityRUNE, "effective security bond", securityBond)
			return errAddLiquidityRUNEMoreThanBond
		}
	}

	if len(msg.Aggregator) > 0 {
		swapOutDisabled := fetchConfigInt64(ctx, h.mgr, constants.SwapOutDexAggregationDisabled)
		if swapOutDisabled > 0 {
			return errors.New("swap out dex integration disabled")
		}
		if !msg.TargetAsset.Equals(msg.TargetAsset.Chain.GetGasAsset()) {
			return fmt.Errorf("target asset (%s) is not gas asset , can't use dex feature", msg.TargetAsset)
		}
		// validate that a referenced dex aggregator is legit
		addr, err := FetchDexAggregator(h.mgr.GetVersion(), target.Chain, msg.Aggregator)
		if err != nil {
			return err
		}
		if addr == "" {
			return fmt.Errorf("aggregator address is empty")
		}
		if len(msg.AggregatorTargetAddress) == 0 {
			return fmt.Errorf("aggregator target address is empty")
		}
	}

	return nil
}

func (h SwapHandler) handleV95(ctx cosmos.Context, msg MsgSwap) (*cosmos.Result, error) {
	// test that the network we are running matches the destination network
	if !common.CurrentChainNetwork.SoftEquals(msg.Destination.GetNetwork(h.mgr.GetVersion(), msg.Destination.GetChain())) {
		return nil, fmt.Errorf("address(%s) is not same network", msg.Destination)
	}
	transactionFee := h.mgr.GasMgr().GetFee(ctx, msg.Destination.GetChain(), common.RuneAsset())
	synthVirtualDepthMult, err := h.mgr.Keeper().GetMimir(ctx, constants.VirtualMultSynthsBasisPoints.String())
	if synthVirtualDepthMult < 1 || err != nil {
		synthVirtualDepthMult = h.mgr.GetConstants().GetInt64Value(constants.VirtualMultSynthsBasisPoints)
	}

	if msg.TargetAsset.IsRune() && !msg.TargetAsset.IsNativeRune() {
		return nil, fmt.Errorf("target asset can't be %s", msg.TargetAsset.String())
	}

	dexAgg := ""
	dexAggTargetAsset := ""
	if len(msg.Aggregator) > 0 {
		dexAgg, err = FetchDexAggregator(h.mgr.GetVersion(), msg.TargetAsset.Chain, msg.Aggregator)
		if err != nil {
			return nil, err
		}
	}
	dexAggTargetAsset = msg.AggregatorTargetAddress

	swapper, err := GetSwapper(h.mgr.Keeper().GetVersion())
	if err != nil {
		return nil, err
	}

	emit, _, swapErr := swapper.Swap(
		ctx,
		h.mgr.Keeper(),
		msg.Tx,
		msg.TargetAsset,
		msg.Destination,
		msg.TradeTarget,
		dexAgg,
		dexAggTargetAsset,
		msg.AggregatorTargetLimit,
		transactionFee,
		synthVirtualDepthMult,
		h.mgr)
	if swapErr != nil {
		return nil, swapErr
	}

	mem, err := ParseMemoWithTHORNames(ctx, h.mgr.Keeper(), msg.Tx.Memo)
	if err != nil {
		ctx.Logger().Error("swap handler failed to parse memo", "memo", msg.Tx.Memo, "error", err)
		return nil, err
	}
	if mem.IsType(TxAdd) {
		m, ok := mem.(AddLiquidityMemo)
		if !ok {
			return nil, fmt.Errorf("fail to cast add liquidity memo")
		}
		m.Asset = fuzzyAssetMatch(ctx, h.mgr.Keeper(), m.Asset)
		msg.Tx.Coins = common.NewCoins(common.NewCoin(m.Asset, emit))
		obTx := ObservedTx{Tx: msg.Tx}
		msg, err := getMsgAddLiquidityFromMemo(ctx, m, obTx, msg.Signer)
		if err != nil {
			return nil, err
		}
		handler := NewAddLiquidityHandler(h.mgr)
		_, err = handler.Run(ctx, msg)
		if err != nil {
			ctx.Logger().Error("swap handler failed to add liquidity", "error", err)
			return nil, err
		}
	}

	return &cosmos.Result{}, nil
}

func (h SwapHandler) validateV95(ctx cosmos.Context, msg MsgSwap) error {
	if err := msg.ValidateBasicV63(); err != nil {
		return err
	}

	target := msg.TargetAsset
	if isTradingHalt(ctx, &msg, h.mgr) {
		return errors.New("trading is halted, can't process swap")
	}
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
				pool, err := h.mgr.Keeper().GetPool(ctx, coin.Asset.GetLayer1Asset())
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
		pool, err := h.mgr.Keeper().GetPool(ctx, target.GetLayer1Asset())
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
		securityBond, err := h.getEffectiveSecurityBond(ctx)
		if err != nil {
			return ErrInternal(err, "fail to get security bond RUNE")
		}
		if totalLiquidityRUNE.GT(securityBond) {
			ctx.Logger().Info("total liquidity RUNE is more than effective security bond", "liquidity rune", totalLiquidityRUNE, "effective security bond", securityBond)
			return errAddLiquidityRUNEMoreThanBond
		}
	}

	if len(msg.Aggregator) > 0 {
		swapOutDisabled := fetchConfigInt64(ctx, h.mgr, constants.SwapOutDexAggregationDisabled)
		if swapOutDisabled > 0 {
			return errors.New("swap out dex integration disabled")
		}
		if !msg.TargetAsset.Equals(msg.TargetAsset.Chain.GetGasAsset()) {
			return fmt.Errorf("target asset (%s) is not gas asset , can't use dex feature", msg.TargetAsset)
		}
		// validate that a referenced dex aggregator is legit
		addr, err := FetchDexAggregator(h.mgr.GetVersion(), target.Chain, msg.Aggregator)
		if err != nil {
			return err
		}
		if addr == "" {
			return fmt.Errorf("aggregator address is empty")
		}
		if len(msg.AggregatorTargetAddress) == 0 {
			return fmt.Errorf("aggregator target address is empty")
		}
	}

	return nil
}

func (h SwapHandler) validateV92(ctx cosmos.Context, msg MsgSwap) error {
	if err := msg.ValidateBasicV63(); err != nil {
		return err
	}

	target := msg.TargetAsset
	if isTradingHalt(ctx, &msg, h.mgr) {
		return errors.New("trading is halted, can't process swap")
	}
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
				pool, err := h.mgr.Keeper().GetPool(ctx, coin.Asset.GetLayer1Asset())
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
		pool, err := h.mgr.Keeper().GetPool(ctx, target.GetLayer1Asset())
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

	if len(msg.Aggregator) > 0 {
		swapOutDisabled := fetchConfigInt64(ctx, h.mgr, constants.SwapOutDexAggregationDisabled)
		if swapOutDisabled > 0 {
			return errors.New("swap out dex integration disabled")
		}
		if !msg.TargetAsset.Equals(msg.TargetAsset.Chain.GetGasAsset()) {
			return fmt.Errorf("target asset (%s) is not gas asset , can't use dex feature", msg.TargetAsset)
		}
		// validate that a referenced dex aggregator is legit
		addr, err := FetchDexAggregator(h.mgr.GetVersion(), target.Chain, msg.Aggregator)
		if err != nil {
			return err
		}
		if addr == "" {
			return fmt.Errorf("aggregator address is empty")
		}
		if len(msg.AggregatorTargetAddress) == 0 {
			return fmt.Errorf("aggregator target address is empty")
		}
	}

	return nil
}

// getTotalActiveBond
func (h SwapHandler) getTotalActiveBond(ctx cosmos.Context) (cosmos.Uint, error) {
	nodeAccounts, err := h.mgr.Keeper().ListValidatorsWithBond(ctx)
	if err != nil {
		return cosmos.ZeroUint(), err
	}
	total := cosmos.ZeroUint()
	for _, na := range nodeAccounts {
		if na.Status != NodeActive {
			continue
		}
		total = total.Add(na.Bond)
	}
	return total, nil
}

func (h SwapHandler) handleV92(ctx cosmos.Context, msg MsgSwap) (*cosmos.Result, error) {
	// test that the network we are running matches the destination network
	if !common.CurrentChainNetwork.SoftEquals(msg.Destination.GetNetwork(h.mgr.GetVersion(), msg.Destination.GetChain())) {
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

	dexAgg := ""
	dexAggTargetAsset := ""
	if len(msg.Aggregator) > 0 {
		dexAgg, err = FetchDexAggregator(h.mgr.GetVersion(), msg.TargetAsset.Chain, msg.Aggregator)
		if err != nil {
			return nil, err
		}
	}
	dexAggTargetAsset = msg.AggregatorTargetAddress

	swapper, err := GetSwapper(h.mgr.Keeper().GetVersion())
	if err != nil {
		return nil, err
	}
	_, _, swapErr := swapper.Swap(
		ctx,
		h.mgr.Keeper(),
		msg.Tx,
		msg.TargetAsset,
		msg.Destination,
		msg.TradeTarget,
		dexAgg,
		dexAggTargetAsset,
		msg.AggregatorTargetLimit,
		transactionFee,
		synthVirtualDepthMult,
		h.mgr)
	if swapErr != nil {
		return nil, swapErr
	}
	return &cosmos.Result{}, nil
}

func (h SwapHandler) handleV93(ctx cosmos.Context, msg MsgSwap) (*cosmos.Result, error) {
	// test that the network we are running matches the destination network
	if !common.CurrentChainNetwork.SoftEquals(msg.Destination.GetNetwork(h.mgr.GetVersion(), msg.Destination.GetChain())) {
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

	dexAgg := ""
	dexAggTargetAsset := ""
	if len(msg.Aggregator) > 0 {
		dexAgg, err = FetchDexAggregator(h.mgr.GetVersion(), msg.TargetAsset.Chain, msg.Aggregator)
		if err != nil {
			return nil, err
		}
	}
	dexAggTargetAsset = msg.AggregatorTargetAddress

	swapper, err := GetSwapper(h.mgr.Keeper().GetVersion())
	if err != nil {
		return nil, err
	}

	emit, _, swapErr := swapper.Swap(
		ctx,
		h.mgr.Keeper(),
		msg.Tx,
		msg.TargetAsset,
		msg.Destination,
		msg.TradeTarget,
		dexAgg,
		dexAggTargetAsset,
		msg.AggregatorTargetLimit,
		transactionFee,
		synthVirtualDepthMult,
		h.mgr)
	if swapErr != nil {
		return nil, swapErr
	}

	mem, err := ParseMemoWithTHORNames(ctx, h.mgr.Keeper(), msg.Tx.Memo)
	if err != nil {
		ctx.Logger().Error("swap handler failed to parse memo", "memo", msg.Tx.Memo, "error", err)
		return nil, err
	}
	if mem.IsType(TxAdd) {
		m, ok := mem.(AddLiquidityMemo)
		if !ok {
			return nil, fmt.Errorf("fail to cast add liquidity memo")
		}
		m.Asset = fuzzyAssetMatch(ctx, h.mgr.Keeper(), m.Asset)
		msg.Tx.Coins = common.NewCoins(common.NewCoin(m.Asset, emit))
		obTx := ObservedTx{Tx: msg.Tx}
		msg, err := getMsgAddLiquidityFromMemo(ctx, m, obTx, msg.Signer)
		if err != nil {
			return nil, err
		}
		handler := NewAddLiquidityHandler(h.mgr)
		_, err = handler.Run(ctx, msg)
		if err != nil {
			ctx.Logger().Error("swap handler failed to add liquidity", "error", err)
			return nil, err
		}
	}

	return &cosmos.Result{}, nil
}

func (h SwapHandler) validateV88(ctx cosmos.Context, msg MsgSwap) error {
	if err := msg.ValidateBasicV63(); err != nil {
		return err
	}

	target := msg.TargetAsset
	if isTradingHalt(ctx, &msg, h.mgr) {
		return errors.New("trading is halted, can't process swap")
	}
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
				pool, err := h.mgr.Keeper().GetPool(ctx, coin.Asset.GetLayer1Asset())
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
		pool, err := h.mgr.Keeper().GetPool(ctx, target.GetLayer1Asset())
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
				pool, err := h.mgr.Keeper().GetPool(ctx, coin.Asset.GetLayer1Asset())
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
		pool, err := h.mgr.Keeper().GetPool(ctx, target.GetLayer1Asset())
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
	if !common.CurrentChainNetwork.SoftEquals(msg.Destination.GetNetwork(h.mgr.GetVersion(), msg.Destination.GetChain())) {
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

	swapper, err := GetSwapper(h.mgr.Keeper().GetVersion())
	if err != nil {
		return nil, err
	}

	_, _, swapErr := swapper.Swap(
		ctx,
		h.mgr.Keeper(),
		msg.Tx,
		msg.TargetAsset,
		msg.Destination,
		msg.TradeTarget,
		"",
		"",
		nil,
		transactionFee,
		synthVirtualDepthMult,
		h.mgr)
	if swapErr != nil {
		return nil, swapErr
	}
	return &cosmos.Result{}, nil
}

func (h SwapHandler) handleV98(ctx cosmos.Context, msg MsgSwap) (*cosmos.Result, error) {
	// test that the network we are running matches the destination network
	// Don't change msg.Destination here; this line was introduced to avoid people from swapping mainnet asset,
	// but using testnet address.
	if !common.CurrentChainNetwork.SoftEquals(msg.Destination.GetNetwork(h.mgr.GetVersion(), msg.Destination.GetChain())) {
		return nil, fmt.Errorf("address(%s) is not same network", msg.Destination)
	}
	transactionFee := h.mgr.GasMgr().GetFee(ctx, msg.TargetAsset.GetChain(), common.RuneAsset())
	synthVirtualDepthMult, err := h.mgr.Keeper().GetMimir(ctx, constants.VirtualMultSynthsBasisPoints.String())
	if synthVirtualDepthMult < 1 || err != nil {
		synthVirtualDepthMult = h.mgr.GetConstants().GetInt64Value(constants.VirtualMultSynthsBasisPoints)
	}

	if msg.TargetAsset.IsRune() && !msg.TargetAsset.IsNativeRune() {
		return nil, fmt.Errorf("target asset can't be %s", msg.TargetAsset.String())
	}

	dexAgg := ""
	dexAggTargetAsset := ""
	if len(msg.Aggregator) > 0 {
		dexAgg, err = FetchDexAggregator(h.mgr.GetVersion(), msg.TargetAsset.Chain, msg.Aggregator)
		if err != nil {
			return nil, err
		}
	}
	dexAggTargetAsset = msg.AggregatorTargetAddress

	swapper, err := GetSwapper(h.mgr.Keeper().GetVersion())
	if err != nil {
		return nil, err
	}

	emit, _, swapErr := swapper.Swap(
		ctx,
		h.mgr.Keeper(),
		msg.Tx,
		msg.TargetAsset,
		msg.Destination,
		msg.TradeTarget,
		dexAgg,
		dexAggTargetAsset,
		msg.AggregatorTargetLimit,
		transactionFee,
		synthVirtualDepthMult,
		h.mgr)
	if swapErr != nil {
		return nil, swapErr
	}

	mem, err := ParseMemoWithTHORNames(ctx, h.mgr.Keeper(), msg.Tx.Memo)
	if err != nil {
		ctx.Logger().Error("swap handler failed to parse memo", "memo", msg.Tx.Memo, "error", err)
		return nil, err
	}
	if mem.IsType(TxAdd) {
		m, ok := mem.(AddLiquidityMemo)
		if !ok {
			return nil, fmt.Errorf("fail to cast add liquidity memo")
		}
		m.Asset = fuzzyAssetMatch(ctx, h.mgr.Keeper(), m.Asset)
		msg.Tx.Coins = common.NewCoins(common.NewCoin(m.Asset, emit))
		obTx := ObservedTx{Tx: msg.Tx}
		msg, err := getMsgAddLiquidityFromMemo(ctx, m, obTx, msg.Signer)
		if err != nil {
			return nil, err
		}
		handler := NewAddLiquidityHandler(h.mgr)
		_, err = handler.Run(ctx, msg)
		if err != nil {
			ctx.Logger().Error("swap handler failed to add liquidity", "error", err)
			return nil, err
		}
	}

	return &cosmos.Result{}, nil
}
