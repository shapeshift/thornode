package keeperv1

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// AddFeeToReserve add fee to reserve, the fee is always in RUNE
func (k KVStore) AddFeeToReserve(ctx cosmos.Context, fee cosmos.Uint) error {
	coin := common.NewCoin(common.RuneNative, fee)
	sdkErr := k.SendFromModuleToModule(ctx, AsgardName, ReserveName, common.NewCoins(coin))
	if sdkErr != nil {
		return dbError(ctx, "fail to send fee to reserve", sdkErr)
	}
	return nil
}

// GetReservesContributors return those address who contributed to the reserve
func (k KVStore) GetReservesContributors(ctx cosmos.Context) (ReserveContributors, error) {
	record := make(ReserveContributors, 0)
	// TODO remove reserve contributors
	// _, err := k.get(ctx, k.GetKey(ctx, prefixReserves, ""), &record)
	return record, fmt.Errorf("not implemented")
}

// SetReserveContributors save reserve contributors to key value store
func (k KVStore) SetReserveContributors(ctx cosmos.Context, contributors ReserveContributors) error {
	// TODO remove reserve contributors
	// k.set(ctx, k.GetKey(ctx, prefixReserves, ""), contributors)
	return fmt.Errorf("not implemented")
}
