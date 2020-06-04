package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type DummyGasManager struct {
}

func NewDummyGasManager() *DummyGasManager {
	return &DummyGasManager{}
}

func (m *DummyGasManager) BeginBlock() {}
func (m *DummyGasManager) EndBlock(ctx cosmos.Context, keeper keeper.Keeper, eventManager EventManager) {
}
func (m *DummyGasManager) AddGasAsset(gas common.Gas)                          {}
func (m *DummyGasManager) GetGas() common.Gas                                  { return nil }
func (m *DummyGasManager) ProcessGas(ctx cosmos.Context, keeper keeper.Keeper) {}
func (m *DummyGasManager) GetFee(ctx cosmos.Context, chain common.Chain) int64 {
	return 0
}

func (m *DummyGasManager) GetMaxGas(ctx cosmos.Context, chain common.Chain) (common.Coin, error) {
	if chain.Equals(common.BNBChain) {
		return common.NewCoin(common.BNBAsset, bnbSingleTxFee), nil
	}
	if chain.Equals(common.BTCChain) {
		return common.NewCoin(common.BTCAsset, cosmos.NewUint(1000)), nil
	}
	return common.NoCoin, kaboom
}
