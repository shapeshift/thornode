package thorchain

import (
	"errors"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// validate if pools exist
func validatePoolsV1(ctx cosmos.Context, keeper keeper.Keeper, assets ...common.Asset) error {
	for _, asset := range assets {
		if !asset.IsRune() {
			if !keeper.PoolExist(ctx, asset) {
				return fmt.Errorf("%s pool doesn't exist", asset)
			}
			pool, err := keeper.GetPool(ctx, asset)
			if err != nil {
				return ErrInternal(err, fmt.Sprintf("fail to get %s pool", asset))
			}

			if pool.Status != PoolAvailable {
				return errInvalidPoolStatus
			}
		}
	}
	return nil
}

// validateMessage is trying to validate the legitimacy of the incoming message and decide whether THORNode can handle it
func validateMessageV1(tx common.Tx, target common.Asset, destination common.Address) error {
	if err := tx.Valid(); err != nil {
		return err
	}
	if target.IsEmpty() {
		return errors.New("target is empty")
	}
	if destination.IsEmpty() {
		return errors.New("destination is empty")
	}

	return nil
}

func swapV1(ctx cosmos.Context,
	keeper keeper.Keeper,
	tx common.Tx,
	target common.Asset,
	destination common.Address,
	tradeTarget cosmos.Uint,
	transactionFee cosmos.Uint, synthVirtualDepthMult int64, mgr Manager) (cosmos.Uint, []*EventSwap, error) {
	var swapEvents []*EventSwap

	// determine if target is layer1 vs synthetic asset
	if !target.IsRune() && !destination.IsChain(target.Chain) {
		if destination.IsChain(common.THORChain) {
			target = target.GetSyntheticAsset()
		} else {
			target = target.GetLayer1Asset()
		}
	}

	if err := validateMessageV1(tx, target, destination); err != nil {
		return cosmos.ZeroUint(), swapEvents, err
	}
	source := tx.Coins[0].Asset

	if err := validatePoolsV1(ctx, keeper, source, target); err != nil {
		if err == errInvalidPoolStatus && source.IsSyntheticAsset() {
			// the pool is not available, but we can allow synthetic assets to still swap back to rune/asset ok
		} else {
			return cosmos.ZeroUint(), swapEvents, err
		}
	}
	if !destination.IsChain(target.GetChain()) {
		return cosmos.ZeroUint(), swapEvents, fmt.Errorf("destination address is not a valid %s address", target.GetChain())
	}
	if source.Equals(target) {
		return cosmos.ZeroUint(), swapEvents, fmt.Errorf("cannot swap from %s --> %s, assets match", source, target)
	}

	poolsBeforeSwap := make([]Pool, 0)
	pools := make([]Pool, 0)
	isDoubleSwap := !source.IsRune() && !target.IsRune()
	if isDoubleSwap {
		var swapErr error
		var swapEvt *EventSwap
		var amt cosmos.Uint
		// Here we use a tradeTarget of 0 because the target is for the next swap asset in a double swap
		amt, poolBefore, sourcePool, swapEvt, swapErr := swapOneV1(ctx, keeper, tx, common.RuneAsset(), destination, cosmos.ZeroUint(), transactionFee, synthVirtualDepthMult)
		if swapErr != nil {
			return cosmos.ZeroUint(), swapEvents, swapErr
		}
		pools = append(pools, sourcePool)
		poolsBeforeSwap = append(poolsBeforeSwap, poolBefore)
		tx.Coins = common.Coins{common.NewCoin(common.RuneAsset(), amt)}
		tx.Gas = nil
		swapEvt.OutTxs = common.NewTx(common.BlankTxID, tx.FromAddress, tx.ToAddress, tx.Coins, tx.Gas, tx.Memo)
		swapEvents = append(swapEvents, swapEvt)
	}
	assetAmount, poolBefore, pool, swapEvt, swapErr := swapOneV1(ctx, keeper, tx, target, destination, tradeTarget, transactionFee, synthVirtualDepthMult)
	if swapErr != nil {
		return cosmos.ZeroUint(), swapEvents, swapErr
	}
	swapEvents = append(swapEvents, swapEvt)
	pools = append(pools, pool)
	poolsBeforeSwap = append(poolsBeforeSwap, poolBefore)
	if !tradeTarget.IsZero() && assetAmount.LT(tradeTarget) {
		return cosmos.ZeroUint(), swapEvents, fmt.Errorf("emit asset %s less than price limit %s", assetAmount, tradeTarget)
	}
	if target.IsRune() {
		if assetAmount.LTE(transactionFee) {
			return cosmos.ZeroUint(), swapEvents, fmt.Errorf("output RUNE (%s) is not enough to pay transaction fee", assetAmount)
		}
	}
	// emit asset is zero
	if assetAmount.IsZero() {
		return cosmos.ZeroUint(), swapEvents, errors.New("zero emit asset")
	}

	// persistent pools to the key value store as the next step will be trying to add TxOutItem
	// during AddTxOutItem , it will try to take some asset from the emitted asset, and add it back to pool
	// thus it put some asset back to compensate gas
	for _, pool := range pools {
		if err := keeper.SetPool(ctx, pool); err != nil {
			return cosmos.ZeroUint(), swapEvents, errSwapFail
		}
	}

	toi := TxOutItem{
		Chain:     target.GetChain(),
		InHash:    tx.ID,
		ToAddress: destination,
		Coin:      common.NewCoin(target, assetAmount),
	}
	// let the txout manager mint our outbound asset if it is a synthetic asset
	if toi.Chain.IsTHORChain() && (toi.Coin.Asset.IsSyntheticAsset() || source.IsSyntheticAsset()) {
		toi.ModuleName = ModuleName
	}

	ok, err := mgr.TxOutStore().TryAddTxOutItem(ctx, mgr, toi)
	if err != nil {
		// when the emit asset is not enough to pay for tx fee, consider it as a success
		if !errors.Is(err, ErrNotEnoughToPayFee) {
			// when it fail to send out the txout item , thus let's restore the pool balance here , thus nothing happen to the pool
			// given the previous pool status is already in memory, so here just apply it again
			for _, pool := range poolsBeforeSwap {
				if err := keeper.SetPool(ctx, pool); err != nil {
					return cosmos.ZeroUint(), swapEvents, errSwapFail
				}
			}

			return assetAmount, swapEvents, ErrInternal(err, "fail to add outbound tx")
		}
		ok = true
	}
	if !ok {
		return assetAmount, swapEvents, errFailAddOutboundTx
	}

	// emit the swap events , by this stage , it is guarantee that swap already happened
	for _, evt := range swapEvents {
		if err := mgr.EventMgr().EmitSwapEvent(ctx, evt); err != nil {
			ctx.Logger().Error("fail to emit swap event", "error", err)
		}
		if err := keeper.AddToLiquidityFees(ctx, evt.Pool, evt.LiquidityFeeInRune); err != nil {
			return assetAmount, swapEvents, fmt.Errorf("fail to add to liquidity fees: %w", err)
		}
	}

	return assetAmount, swapEvents, nil
}

func swapOneV1(ctx cosmos.Context,
	keeper keeper.Keeper, tx common.Tx,
	target common.Asset,
	destination common.Address,
	tradeTarget cosmos.Uint,
	transactionFee cosmos.Uint,
	synthVirtualDepthMult int64) (amt cosmos.Uint, poolBefore, poolResult Pool, evt *EventSwap, swapErr error) {
	source := tx.Coins[0].Asset
	amount := tx.Coins[0].Amount

	ctx.Logger().Info(fmt.Sprintf("%s Swapping %s(%s) -> %s to %s (Fee %s)", tx.FromAddress, source, tx.Coins[0].Amount, target, destination, transactionFee))

	var X, x, Y, liquidityFee, emitAssets cosmos.Uint
	var tradeSlip cosmos.Uint

	// Set asset to our non-rune asset
	asset := source
	if source.IsRune() {
		asset = target
		if amount.LTE(transactionFee) {
			// stop swap , because the output will not enough to pay for transaction fee
			return cosmos.ZeroUint(), Pool{}, Pool{}, evt, errSwapFailNotEnoughFee
		}
	}
	if asset.IsSyntheticAsset() {
		asset = asset.GetLayer1Asset()
	}

	swapEvt := NewEventSwap(
		asset,
		tradeTarget,
		cosmos.ZeroUint(),
		cosmos.ZeroUint(),
		cosmos.ZeroUint(),
		tx,
		common.NoCoin,
	)

	// Check if pool exists
	if !keeper.PoolExist(ctx, asset) {
		err := fmt.Errorf("pool %s doesn't exist", asset)
		return cosmos.ZeroUint(), Pool{}, Pool{}, evt, err
	}

	// Get our pool from the KVStore
	pool, poolErr := keeper.GetPool(ctx, asset)
	if poolErr != nil {
		return cosmos.ZeroUint(), Pool{}, Pool{}, evt, ErrInternal(poolErr, fmt.Sprintf("fail to get pool(%s)", asset))
	}
	poolBefore = pool

	// Get our X, x, Y values
	if source.IsRune() {
		X = pool.BalanceRune
		Y = pool.BalanceAsset
	} else {
		Y = pool.BalanceRune
		X = pool.BalanceAsset
	}
	x = amount

	// give 2x virtual pool depth if we're swapping with a synthetic asset
	if source.IsSyntheticAsset() || target.IsSyntheticAsset() {
		X = X.MulUint64(uint64(synthVirtualDepthMult))
		Y = Y.MulUint64(uint64(synthVirtualDepthMult))
	}

	// check our X,x,Y values are valid
	if x.IsZero() {
		return cosmos.ZeroUint(), poolBefore, pool, evt, errSwapFailInvalidAmount
	}
	if X.IsZero() || Y.IsZero() {
		return cosmos.ZeroUint(), poolBefore, pool, evt, errSwapFailInvalidBalance
	}

	liquidityFee = calcLiquidityFeeV1(X, x, Y)
	tradeSlip = calcSwapSlipV1(X, x)
	emitAssets = calcAssetEmissionV1(X, x, Y)
	emitAssets = cosmos.RoundToDecimal(emitAssets, pool.Decimals)
	swapEvt.LiquidityFee = liquidityFee

	if source.IsRune() {
		swapEvt.LiquidityFeeInRune = pool.AssetValueInRune(liquidityFee)
	} else {
		// because the output asset is RUNE , so liqualidtyFee is already in RUNE
		swapEvt.LiquidityFeeInRune = liquidityFee
	}
	swapEvt.TradeSlip = tradeSlip
	swapEvt.EmitAsset = common.NewCoin(target, emitAssets)

	// do THORNode have enough balance to swap?
	if emitAssets.GTE(Y) {
		return cosmos.ZeroUint(), poolBefore, pool, evt, errSwapFailNotEnoughBalance
	}

	ctx.Logger().Info(fmt.Sprintf("Pre-Pool: %sRune %sAsset", pool.BalanceRune, pool.BalanceAsset))

	if source.IsSyntheticAsset() || target.IsSyntheticAsset() {
		// we're doing a synth swap
		if source.IsSyntheticAsset() {
			// our source is a pegged asset, burn it all
			totalSynthSupply := keeper.GetTotalSupply(ctx, source)
			minusSynthUnits := common.GetShare(x, totalSynthSupply, pool.SynthUnits)
			pool.BalanceRune = common.SafeSub(pool.BalanceRune, emitAssets)
			pool.SynthUnits = common.SafeSub(pool.SynthUnits, minusSynthUnits)
			pool.PoolUnits = common.SafeSub(pool.PoolUnits, minusSynthUnits)
			if err := keeper.SendFromModuleToModule(ctx, AsgardName, ModuleName, tx.Coins); err != nil {
				return cosmos.ZeroUint(), poolBefore, pool, evt, fmt.Errorf("fail to move coins during swap: %w", err)
			}
			if err := keeper.BurnFromModule(ctx, ModuleName, tx.Coins[0]); err != nil {
				return cosmos.ZeroUint(), poolBefore, pool, evt, fmt.Errorf("fail to burn coins during swap: %w", err)
			}
		} else {
			// our source is assumed to be rune
			// mint synth units so that LPs don't get access to the extra rune in the pool.
			// the minting of the synths themselves is handled in the txout manager
			newPoolUnits, synthUnits, err := calculatePoolUnitsV1(pool.PoolUnits, pool.BalanceRune, pool.BalanceAsset, x, cosmos.ZeroUint())
			if err != nil {
				return cosmos.ZeroUint(), poolBefore, pool, evt, ErrInternal(err, "fail to calculate synth unit")
			}
			pool.PoolUnits = newPoolUnits
			pool.SynthUnits = pool.SynthUnits.Add(synthUnits)
			pool.BalanceRune = pool.BalanceRune.Add(x)
		}
	} else {
		if source.IsRune() {
			pool.BalanceRune = X.Add(x)
			pool.BalanceAsset = common.SafeSub(Y, emitAssets)
		} else {
			pool.BalanceAsset = X.Add(x)
			pool.BalanceRune = common.SafeSub(Y, emitAssets)
		}
	}
	ctx.Logger().Info(fmt.Sprintf("Post-swap: %sRune %sAsset , user get:%s ", pool.BalanceRune, pool.BalanceAsset, emitAssets))

	return emitAssets, poolBefore, pool, swapEvt, nil
}

// calculate the number of assets sent to the address (includes liquidity fee)
func calcAssetEmissionV1(X, x, Y cosmos.Uint) cosmos.Uint {
	// ( x * X * Y ) / ( x + X )^2
	numerator := x.Mul(X).Mul(Y)
	denominator := x.Add(X).Mul(x.Add(X))
	if denominator.IsZero() {
		return cosmos.ZeroUint()
	}
	return numerator.Quo(denominator)
}

// calculateFee the fee of the swap
func calcLiquidityFeeV1(X, x, Y cosmos.Uint) cosmos.Uint {
	// ( x^2 *  Y ) / ( x + X )^2
	numerator := x.Mul(x).Mul(Y)
	denominator := x.Add(X).Mul(x.Add(X))
	if denominator.IsZero() {
		return cosmos.ZeroUint()
	}
	return numerator.Quo(denominator)
}

// calcSwapSlip - calculate the swap slip, expressed in basis points (10000)
func calcSwapSlipV1(Xi, xi cosmos.Uint) cosmos.Uint {
	// Cast to DECs
	xD := cosmos.NewDecFromBigInt(xi.BigInt())
	XD := cosmos.NewDecFromBigInt(Xi.BigInt())
	dec10k := cosmos.NewDec(10000)
	// x / (x + X)
	denD := xD.Add(XD)
	if denD.IsZero() {
		return cosmos.ZeroUint()
	}
	swapSlipD := xD.Quo(denD)                                     // Division with DECs
	swapSlip := swapSlipD.Mul(dec10k)                             // Adds 5 0's
	swapSlipUint := cosmos.NewUint(uint64(swapSlip.RoundInt64())) // Casts back to Uint as Basis Points
	return swapSlipUint
}
