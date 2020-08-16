package thorchain

import (
	"errors"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	keeper "gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// validate if pools exist
func validatePools(ctx cosmos.Context, keeper keeper.Keeper, assets ...common.Asset) error {
	for _, asset := range assets {
		if !asset.IsRune() {
			if !keeper.PoolExist(ctx, asset) {
				return fmt.Errorf("%s pool doesn't exist", asset)
			}
			pool, err := keeper.GetPool(ctx, asset)
			if err != nil {
				return ErrInternal(err, fmt.Sprintf("fail to get %s pool", asset))
			}

			if pool.Status != PoolEnabled {
				return errInvalidPoolStatus
			}
		}
	}
	return nil
}

// validateMessage is trying to validate the legitimacy of the incoming message and decide whether THORNode can handle it
func validateMessage(tx common.Tx, target common.Asset, destination common.Address) error {
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

func swap(ctx cosmos.Context,
	keeper keeper.Keeper, tx common.Tx,
	target common.Asset,
	destination common.Address,
	tradeTarget cosmos.Uint,
	transactionFee cosmos.Uint) (cosmos.Uint, []EventSwap, error) {
	var swapEvents []EventSwap

	if err := validateMessage(tx, target, destination); err != nil {
		return cosmos.ZeroUint(), swapEvents, err
	}
	source := tx.Coins[0].Asset

	if err := validatePools(ctx, keeper, source, target); err != nil {
		return cosmos.ZeroUint(), swapEvents, err
	}
	pools := make([]Pool, 0)
	isDoubleSwap := !source.IsRune() && !target.IsRune()
	if isDoubleSwap {
		var swapErr error
		var swapEvt EventSwap
		var amt cosmos.Uint
		// Here we use a tradeTarget of 0 because the target is for the next swap asset in a double swap
		amt, sourcePool, swapEvt, swapErr := swapOne(ctx, keeper, tx, common.RuneAsset(), destination, cosmos.ZeroUint(), transactionFee)
		if swapErr != nil {
			return cosmos.ZeroUint(), swapEvents, swapErr
		}
		pools = append(pools, sourcePool)
		tx.Coins = common.Coins{common.NewCoin(common.RuneAsset(), amt)}
		tx.Gas = nil
		swapEvt.OutTxs = common.NewTx(common.BlankTxID, tx.FromAddress, tx.ToAddress, tx.Coins, tx.Gas, tx.Memo)
		swapEvents = append(swapEvents, swapEvt)
	}

	assetAmount, pool, swapEvt, swapErr := swapOne(ctx, keeper, tx, target, destination, tradeTarget, transactionFee)
	if swapErr != nil {
		return cosmos.ZeroUint(), swapEvents, swapErr
	}
	swapEvents = append(swapEvents, swapEvt)
	pools = append(pools, pool)
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

	// Update pools
	for _, pool := range pools {
		if err := keeper.SetPool(ctx, pool); err != nil {
			return cosmos.ZeroUint(), swapEvents, errSwapFail
		}
	}

	return assetAmount, swapEvents, nil
}

func swapOne(ctx cosmos.Context,
	keeper keeper.Keeper, tx common.Tx,
	target common.Asset,
	destination common.Address,
	tradeTarget cosmos.Uint,
	transactionFee cosmos.Uint) (amt cosmos.Uint, poolResult Pool, evt EventSwap, swapErr error) {
	source := tx.Coins[0].Asset
	amount := tx.Coins[0].Amount

	ctx.Logger().Info(fmt.Sprintf("%s Swapping %s(%s) -> %s to %s (Fee %s)", tx.FromAddress, source, tx.Coins[0].Amount, target, destination, transactionFee))

	var X, x, Y, liquidityFee, emitAssets cosmos.Uint
	var tradeSlip cosmos.Uint

	// Set asset to our non-rune asset asset
	asset := source
	if source.IsRune() {
		asset = target
		if amount.LTE(transactionFee) {
			// stop swap , because the output will not enough to pay for transaction fee
			return cosmos.ZeroUint(), Pool{}, evt, errSwapFailNotEnoughFee
		}
	}

	swapEvt := NewEventSwap(
		asset,
		tradeTarget,
		cosmos.ZeroUint(),
		cosmos.ZeroUint(),
		cosmos.ZeroUint(),
		tx,
	)

	// Check if pool exists
	if !keeper.PoolExist(ctx, asset) {
		err := fmt.Errorf("pool %s doesn't exist", asset)
		return cosmos.ZeroUint(), Pool{}, evt, err
	}

	// Get our pool from the KVStore
	pool, poolErr := keeper.GetPool(ctx, asset)
	if poolErr != nil {
		return cosmos.ZeroUint(), Pool{}, evt, ErrInternal(poolErr, fmt.Sprintf("fail to get pool(%s)", asset))
	}
	if pool.Status != PoolEnabled {
		return cosmos.ZeroUint(), pool, evt, errInvalidPoolStatus
	}

	// Get our X, x, Y values
	if source.IsRune() {
		X = pool.BalanceRune
		Y = pool.BalanceAsset
	} else {
		Y = pool.BalanceRune
		X = pool.BalanceAsset
	}
	x = amount

	// check our X,x,Y values are valid
	if x.IsZero() {
		return cosmos.ZeroUint(), pool, evt, errSwapFailInvalidAmount
	}
	if X.IsZero() || Y.IsZero() {
		return cosmos.ZeroUint(), pool, evt, errSwapFailInvalidBalance
	}

	liquidityFee = calcLiquidityFee(X, x, Y)
	tradeSlip = calcTradeSlip(X, x)
	emitAssets = calcAssetEmission(X, x, Y)
	swapEvt.LiquidityFee = liquidityFee

	if source.IsRune() {
		swapEvt.LiquidityFeeInRune = pool.AssetValueInRune(liquidityFee)
	} else {
		// because the output asset is RUNE , so liqualidtyFee is already in RUNE
		swapEvt.LiquidityFeeInRune = liquidityFee
	}
	swapEvt.TradeSlip = tradeSlip

	// do THORNode have enough balance to swap?
	if emitAssets.GTE(Y) {
		return cosmos.ZeroUint(), pool, evt, errSwapFailNotEnoughBalance
	}

	ctx.Logger().Debug(fmt.Sprintf("Pre-Pool: %sRune %sAsset", pool.BalanceRune, pool.BalanceAsset))

	if source.IsRune() {
		pool.BalanceRune = X.Add(x)
		pool.BalanceAsset = common.SafeSub(Y, emitAssets)
	} else {
		pool.BalanceAsset = X.Add(x)
		pool.BalanceRune = common.SafeSub(Y, emitAssets)
	}
	ctx.Logger().Debug(fmt.Sprintf("Post-swap: %sRune %sAsset , user get:%s ", pool.BalanceRune, pool.BalanceAsset, emitAssets))

	return emitAssets, pool, swapEvt, nil
}

// calculate the number of assets sent to the address (includes liquidity fee)
func calcAssetEmission(X, x, Y cosmos.Uint) cosmos.Uint {
	// ( x * X * Y ) / ( x + X )^2
	numerator := x.Mul(X).Mul(Y)
	denominator := x.Add(X).Mul(x.Add(X))
	if denominator.IsZero() {
		return cosmos.ZeroUint()
	}
	return numerator.Quo(denominator)
}

// calculateFee the fee of the swap
func calcLiquidityFee(X, x, Y cosmos.Uint) cosmos.Uint {
	// ( x^2 *  Y ) / ( x + X )^2
	numerator := x.Mul(x).Mul(Y)
	denominator := x.Add(X).Mul(x.Add(X))
	if denominator.IsZero() {
		return cosmos.ZeroUint()
	}
	return numerator.Quo(denominator)
}

// calcTradeSlip - calculate the trade slip, expressed in basis points (10000)
func calcTradeSlip(Xi, xi cosmos.Uint) cosmos.Uint {
	// Cast to DECs
	xD := cosmos.NewDecFromBigInt(xi.BigInt())
	XD := cosmos.NewDecFromBigInt(Xi.BigInt())
	dec2 := cosmos.NewDec(2)
	dec10k := cosmos.NewDec(10000)

	// x * (2*X + x) / (X * X)
	numD := xD.Mul((dec2.Mul(XD)).Add(xD))
	denD := XD.Mul(XD)

	if denD.IsZero() {
		return cosmos.ZeroUint()
	}
	tradeSlipD := numD.Quo(denD) // Division with DECs

	tradeSlip := tradeSlipD.Mul(dec10k)                             // Adds 5 0's
	tradeSlipUint := cosmos.NewUint(uint64(tradeSlip.RoundInt64())) // Casts back to Uint as Basis Points
	return tradeSlipUint
}
