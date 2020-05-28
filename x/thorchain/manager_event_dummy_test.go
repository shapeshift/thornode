package thorchain

import (
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// DummyEventMgr used for test purpose , and it implement EventManager interface
type DummyEventMgr struct {
}

func NewDummyEventMgr() *DummyEventMgr {
	return &DummyEventMgr{}
}

func (m *DummyEventMgr) EmitPoolEvent(ctx cosmos.Context, poolEvt EventPool) error {
	return nil
}

func (m *DummyEventMgr) EmitErrataEvent(ctx cosmos.Context, errataEvent EventErrata) error {
	return nil
}

func (m *DummyEventMgr) EmitGasEvent(ctx cosmos.Context, gasEvent *EventGas) error {
	return nil
}

func (m *DummyEventMgr) EmitStakeEvent(ctx cosmos.Context, stakeEvent EventStake) error {
	return nil
}

func (m *DummyEventMgr) EmitRewardEvent(ctx cosmos.Context, rewardEvt EventRewards) error {
	return nil
}

func (m *DummyEventMgr) EmitReserveEvent(ctx cosmos.Context, reserveEvent EventReserve) error {
	return nil
}

func (m *DummyEventMgr) EmitUnstakeEvent(ctx cosmos.Context, unstakeEvt EventUnstake) error {
	return nil
}

func (m *DummyEventMgr) EmitSwapEvent(ctx cosmos.Context, swap EventSwap) error {
	return nil
}

func (m *DummyEventMgr) EmitAddEvent(ctx cosmos.Context, addEvt EventAdd) error {
	return nil
}

func (m *DummyEventMgr) EmitRefundEvent(ctx cosmos.Context, refundEvt EventRefund) error {
	return nil
}

func (m *DummyEventMgr) EmitBondEvent(ctx cosmos.Context, bondEvent EventBond) error {
	return nil
}

func (m *DummyEventMgr) EmitFeeEvent(ctx cosmos.Context, feeEvent EventFee) error {
	return nil
}

func (m *DummyEventMgr) EmitSlashEvent(ctx cosmos.Context, slashEvt EventSlash) error {
	return nil
}

func (m *DummyEventMgr) EmitOutboundEvent(ctx cosmos.Context, outbound EventOutbound) error {
	return nil
}
