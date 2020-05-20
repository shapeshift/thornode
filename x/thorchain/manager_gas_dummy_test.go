package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type DummyGasManager struct {
}

func NewDummyGasManager() *DummyGasManager {
	return &DummyGasManager{}
}

func (m *DummyGasManager) BeginBlock()                                                           {}
func (m *DummyGasManager) EndBlock(ctx cosmos.Context, keeper Keeper, eventManager EventManager) {}
func (m *DummyGasManager) AddGasAsset(gas common.Gas)                                            {}
func (m *DummyGasManager) GetGas() common.Gas                                                    { return nil }
func (m *DummyGasManager) ProcessGas(ctx cosmos.Context, keeper Keeper)                          {}
