package keeperv1

import (
	"fmt"
	"strconv"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// AddToLiquidityFees - measure of fees collected in each block
func (k KVStore) AddToLiquidityFees(ctx cosmos.Context, asset common.Asset, fee cosmos.Uint) error {
	currentHeight := uint64(ctx.BlockHeight())

	totalFees, err := k.GetTotalLiquidityFees(ctx, currentHeight)
	if err != nil {
		return err
	}
	poolFees, err := k.GetPoolLiquidityFees(ctx, currentHeight, asset)
	if err != nil {
		return err
	}

	totalFees = totalFees.Add(fee)
	poolFees = poolFees.Add(fee)

	// update total liquidity
	k.setUint64(ctx, k.GetKey(ctx, prefixTotalLiquidityFee, strconv.FormatUint(currentHeight, 10)), totalFees.Uint64())

	// update pool liquidity
	k.setUint64(ctx, k.GetKey(ctx, prefixPoolLiquidityFee, fmt.Sprintf("%d-%s", currentHeight, asset.String())), poolFees.Uint64())
	return nil
}

func (k KVStore) getLiquidityFees(ctx cosmos.Context, key string) (cosmos.Uint, error) {
	var record uint64
	_, err := k.getUint64(ctx, key, &record)
	return cosmos.NewUint(record), err
}

// GetTotalLiquidityFees - total of all fees collected in each block
func (k KVStore) GetTotalLiquidityFees(ctx cosmos.Context, height uint64) (cosmos.Uint, error) {
	key := k.GetKey(ctx, prefixTotalLiquidityFee, strconv.FormatUint(height, 10))
	return k.getLiquidityFees(ctx, key)
}

// GetPoolLiquidityFees - total of fees collected in each block per pool
func (k KVStore) GetPoolLiquidityFees(ctx cosmos.Context, height uint64, asset common.Asset) (cosmos.Uint, error) {
	key := k.GetKey(ctx, prefixPoolLiquidityFee, fmt.Sprintf("%d-%s", height, asset.String()))
	return k.getLiquidityFees(ctx, key)
}
