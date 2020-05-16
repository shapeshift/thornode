package common

import cosmos "gitlab.com/thorchain/thornode/common/cosmos"

type Fee struct {
	Coins      Coins       `json:"coins"`
	PoolDeduct cosmos.Uint `json:"pool_deduct"`
}

// NewFee return a new instance of Fee
func NewFee(coins Coins, poolDeduct cosmos.Uint) Fee {
	return Fee{
		Coins:      coins,
		PoolDeduct: poolDeduct,
	}
}

// Asset retun asset name of fee coins
func (fee *Fee) Asset() Asset {
	for _, coin := range fee.Coins {
		if !coin.Asset.IsRune() {
			return coin.Asset
		}
	}
	return Asset{}
}
