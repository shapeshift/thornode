package thorchain

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// DummyEventMgr used for test purpose , and it implement EventManager interface
type DummyEventMgr struct {
}

func NewDummyEventMgr() *DummyEventMgr {
	return &DummyEventMgr{}
}

func (m *DummyEventMgr) CompleteEvents(ctx cosmos.Context, keeper Keeper, height int64, txID common.TxID, txs common.Txs, eventStatus EventStatus) {
}

func (m *DummyEventMgr) EmitPoolEvent(ctx cosmos.Context, keeper Keeper, txIn common.TxID, status EventStatus, poolEvt EventPool) error {
	return nil
}

func (m *DummyEventMgr) EmitErrataEvent(ctx cosmos.Context, keeper Keeper, txIn common.TxID, errataEvent EventErrata) error {
	return nil
}

func (m *DummyEventMgr) EmitGasEvent(ctx cosmos.Context, keeper Keeper, gasEvent *EventGas) error {
	return nil
}

func (m *DummyEventMgr) EmitStakeEvent(ctx cosmos.Context, keeper Keeper, inTx common.Tx, stakeEvent EventStake) error {
	return nil
}

func (m *DummyEventMgr) EmitRewardEvent(ctx cosmos.Context, keeper Keeper, rewardEvt EventRewards) error {
	return nil
}

func (m *DummyEventMgr) EmitReserveEvent(ctx cosmos.Context, keeper Keeper, reserveEvent EventReserve) error {
	return nil
}

func (m *DummyEventMgr) EmitUnstakeEvent(ctx cosmos.Context, keeper Keeper, unstakeEvt EventUnstake) error {
	return nil
}

func (m *DummyEventMgr) EmitSwapEvent(ctx cosmos.Context, keeper Keeper, swap EventSwap) error {
	return nil
}

func (m *DummyEventMgr) EmitAddEvent(ctx cosmos.Context, keeper Keeper, addEvt EventAdd) error {
	return nil
}

func (m *DummyEventMgr) EmitRefundEvent(ctx cosmos.Context, keeper Keeper, refundEvt EventRefund, status EventStatus) error {
	return nil
}

func (m *DummyEventMgr) EmitBondEvent(ctx cosmos.Context, keeper Keeper, bondEvent EventBond) error {
	return nil
}

func (m *DummyEventMgr) EmitFeeEvent(ctx cosmos.Context, keeper Keeper, feeEvent EventFee) error {
	return nil
}

func (m *DummyEventMgr) EmitSlashEvent(ctx cosmos.Context, keeper Keeper, slashEvt EventSlash) error {
	return nil
}

func (m *DummyEventMgr) EmitOutboundEvent(ctx cosmos.Context, outbound EventOutbound) error {
	return nil
}

type DummyVersionedEventMgr struct{}

func NewDummyVersionedEventMgr() *DummyVersionedEventMgr {
	return &DummyVersionedEventMgr{}
}

func (m *DummyVersionedEventMgr) GetEventManager(ctx cosmos.Context, version semver.Version) (EventManager, error) {
	return NewDummyEventMgr(), nil
}
