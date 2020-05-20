package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type DummyObserverManager struct {
}

func NewDummyObserverManager() *DummyObserverManager {
	return &DummyObserverManager{}
}

func (m *DummyObserverManager) BeginBlock()                                                  {}
func (m *DummyObserverManager) EndBlock(ctx cosmos.Context, keeper Keeper)                   {}
func (m *DummyObserverManager) AppendObserver(chain common.Chain, addrs []cosmos.AccAddress) {}
func (m *DummyObserverManager) List() []cosmos.AccAddress                                    { return nil }
