package thorchain

import (
	"github.com/blang/semver"

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

type DummyVersionedGasMgr struct {
}

func NewDummyVersionedGasMgr() *DummyVersionedGasMgr {
	return &DummyVersionedGasMgr{}
}

func (m *DummyVersionedGasMgr) GetGasManager(ctx cosmos.Context, version semver.Version) (GasManager, error) {
	return NewDummyGasManager(), nil
}
