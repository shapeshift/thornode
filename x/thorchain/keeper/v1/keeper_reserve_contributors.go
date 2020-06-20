package keeperv1

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// AddFeeToReserve add fee to reserve, the fee is always in RUNE
func (k KVStore) AddFeeToReserve(ctx cosmos.Context, fee cosmos.Uint) error {
	vault, err := k.GetVaultData(ctx)
	if err != nil {
		return fmt.Errorf("fail to get vault: %w", err)
	}
	if common.RuneAsset().Chain.Equals(common.THORChain) {
		coin := common.NewCoin(common.RuneNative, fee)
		sdkErr := k.SendFromModuleToModule(ctx, AsgardName, ReserveName, coin)
		if sdkErr != nil {
			return dbError(ctx, "fail to send fee to reserve", sdkErr)
		}
	} else {
		vault.TotalReserve = vault.TotalReserve.Add(fee)
	}
	return k.SetVaultData(ctx, vault)
}

// GetReservesContributors return those address who contributed to the reserve
func (k KVStore) GetReservesContributors(ctx cosmos.Context) (ReserveContributors, error) {
	contributors := make(ReserveContributors, 0)
	key := k.GetKey(ctx, prefixReserves, "")
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return make(ReserveContributors, 0), nil
	}
	buf := store.Get([]byte(key))
	if err := k.cdc.UnmarshalBinaryBare(buf, &contributors); nil != err {
		return nil, dbError(ctx, "fail to unmarshal reserve contributors", err)
	}

	return contributors, nil
}

// SetReserveContributors save reserve contributors to key value store
func (k KVStore) SetReserveContributors(ctx cosmos.Context, contributors ReserveContributors) error {
	key := k.GetKey(ctx, prefixReserves, "")
	store := ctx.KVStore(k.storeKey)
	if contributors == nil {
		contributors = make(ReserveContributors, 0)
	}

	buf, err := k.cdc.MarshalBinaryBare(contributors)
	if err != nil {
		return dbError(ctx, "fail to marshal reserve contributors to binary", err)
	}
	// there is nothing to be set
	if buf == nil {
		return nil
	}
	store.Set([]byte(key), buf)
	return nil
}
