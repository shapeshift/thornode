package keeperv1

import (
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
