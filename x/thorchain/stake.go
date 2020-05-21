package thorchain

import (
	"errors"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// validateStakeMessage is to do some validation, and make sure it is legit
func validateStakeMessage(ctx cosmos.Context, keeper Keeper, asset common.Asset, requestTxHash common.TxID, runeAddr, assetAddr common.Address) error {
	if asset.IsEmpty() {
		return errors.New("asset is empty")
	}
	if requestTxHash.IsEmpty() {
		return errors.New("request tx hash is empty")
	}
	if asset.Chain.Equals(common.RuneAsset().Chain) {
		if runeAddr.IsEmpty() {
			return errors.New("rune address is empty")
		}
	} else {
		if assetAddr.IsEmpty() {
			return errors.New("asset address is empty")
		}
	}
	if !keeper.PoolExist(ctx, asset) {
		return fmt.Errorf("%s doesn't exist", asset)
	}
	return nil
}

func stake(ctx cosmos.Context, keeper Keeper,
	asset common.Asset,
	stakeRuneAmount, stakeAssetAmount cosmos.Uint,
	runeAddr, assetAddr common.Address,
	requestTxHash common.TxID, constAccessor constants.ConstantValues) (cosmos.Uint, error) {
	ctx.Logger().Info(fmt.Sprintf("%s staking %s %s", asset, stakeRuneAmount, stakeAssetAmount))
	if err := validateStakeMessage(ctx, keeper, asset, requestTxHash, runeAddr, assetAddr); err != nil {
		ctx.Logger().Error("stake message fail validation", "error", err)
		return cosmos.ZeroUint(), errStakeFailValidation
	}
	if stakeRuneAmount.IsZero() && stakeAssetAmount.IsZero() {
		ctx.Logger().Error("both rune and asset is zero")
		return cosmos.ZeroUint(), errStakeFailValidation
	}
	if runeAddr.IsEmpty() {
		ctx.Logger().Error("rune address cannot be empty")
		return cosmos.ZeroUint(), errStakeFailValidation
	}

	pool, err := keeper.GetPool(ctx, asset)
	if err != nil {
		return cosmos.ZeroUint(), ErrInternal(err, fmt.Sprintf("fail to get pool(%s)", asset))
	}

	// if THORNode have no balance, set the default pool status
	if pool.BalanceAsset.IsZero() && pool.BalanceRune.IsZero() {
		defaultPoolStatus := PoolEnabled.String()

		// if we have pools that are already enabled, use the default status
		iterator := keeper.GetPoolIterator(ctx)
		defer iterator.Close()
		for ; iterator.Valid(); iterator.Next() {
			var p Pool
			err := keeper.Cdc().UnmarshalBinaryBare(iterator.Value(), &p)
			if err != nil {
				continue
			}
			if p.Status == PoolEnabled {
				defaultPoolStatus = constAccessor.GetStringValue(constants.DefaultPoolStatus)
				break
			}
		}
		pool.Status = GetPoolStatus(defaultPoolStatus)
	}

	su, err := keeper.GetStaker(ctx, asset, runeAddr)
	if err != nil {
		ctx.Logger().Error("fail to get staker", "error", err)
		return cosmos.ZeroUint(), errFailGetStaker
	}

	su.LastStakeHeight = ctx.BlockHeight()
	if su.RuneAddress.IsEmpty() {
		su.RuneAddress = runeAddr
	}
	if su.AssetAddress.IsEmpty() {
		su.AssetAddress = assetAddr
	} else {
		if !su.AssetAddress.Equals(assetAddr) {
			// mismatch of asset addresses from what is known to the address
			// given. Refund it.
			return cosmos.ZeroUint(), errStakeMismatchAssetAddr
		}
	}

	if !asset.Chain.Equals(common.RuneAsset().Chain) {
		if stakeAssetAmount.IsZero() {
			su.PendingRune = su.PendingRune.Add(stakeRuneAmount)
			keeper.SetStaker(ctx, su)
			return cosmos.ZeroUint(), nil
		}
		stakeRuneAmount = su.PendingRune.Add(stakeRuneAmount)
		su.PendingRune = cosmos.ZeroUint()
	}

	fAssetAmt := stakeAssetAmount
	fRuneAmt := stakeRuneAmount

	ctx.Logger().Info(fmt.Sprintf("Pre-Pool: %sRUNE %sAsset", pool.BalanceRune, pool.BalanceAsset))
	ctx.Logger().Info(fmt.Sprintf("Staking: %sRUNE %sAsset", stakeRuneAmount, stakeAssetAmount))

	balanceRune := pool.BalanceRune
	balanceAsset := pool.BalanceAsset

	oldPoolUnits := pool.PoolUnits
	newPoolUnits, stakerUnits, err := calculatePoolUnits(oldPoolUnits, balanceRune, balanceAsset, fRuneAmt, fAssetAmt)
	if err != nil {
		ctx.Logger().Error("fail to calculate pool unit", "error", err)
		return cosmos.ZeroUint(), errStakeInvalidPoolAsset
	}

	ctx.Logger().Info(fmt.Sprintf("current pool units : %s ,staker units : %s", newPoolUnits, stakerUnits))
	poolRune := balanceRune.Add(fRuneAmt)
	poolAsset := balanceAsset.Add(fAssetAmt)
	pool.PoolUnits = newPoolUnits
	pool.BalanceRune = poolRune
	pool.BalanceAsset = poolAsset
	ctx.Logger().Info(fmt.Sprintf("Post-Pool: %sRUNE %sAsset", pool.BalanceRune, pool.BalanceAsset))
	if err := keeper.SetPool(ctx, pool); err != nil {
		return cosmos.ZeroUint(), ErrInternal(err, "fail to save pool")
	}
	// maintain staker structure

	fex := su.Units
	totalStakerUnits := fex.Add(stakerUnits)

	su.Units = totalStakerUnits
	keeper.SetStaker(ctx, su)
	return stakerUnits, nil
}

// calculatePoolUnits calculate the pool units and staker units
// returns newPoolUnit,stakerUnit, error
func calculatePoolUnits(oldPoolUnits, poolRune, poolAsset, stakeRune, stakeAsset cosmos.Uint) (cosmos.Uint, cosmos.Uint, error) {
	if stakeRune.Add(poolRune).IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), errors.New("total RUNE in the pool is zero")
	}
	if stakeAsset.Add(poolAsset).IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), errors.New("total asset in the pool is zero")
	}

	poolRuneAfter := poolRune.Add(stakeRune)
	poolAssetAfter := poolAsset.Add(stakeAsset)

	// ((R + A) * (r * A + R * a))/(4 * R * A)
	nominator1 := poolRuneAfter.Add(poolAssetAfter)
	nominator2 := stakeRune.Mul(poolAssetAfter).Add(poolRuneAfter.Mul(stakeAsset))
	denominator := cosmos.NewUint(4).Mul(poolRuneAfter).Mul(poolAssetAfter)
	stakeUnits := nominator1.Mul(nominator2).Quo(denominator)
	newPoolUnit := oldPoolUnits.Add(stakeUnits)
	return newPoolUnit, stakeUnits, nil
}
