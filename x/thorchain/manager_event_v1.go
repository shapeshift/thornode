package thorchain

import (
	"encoding/json"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	keeper "gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// EventMgrV1 implement EventManager interface
type EventMgrV1 struct {
}

// NewEventMgrV1 create a new instance of EventMgrV1
func NewEventMgrV1() *EventMgrV1 {
	return &EventMgrV1{}
}

// CompleteEvents Mark an event in the given block height to the given status
func (m *EventMgrV1) CompleteEvents(ctx cosmos.Context, keeper keeper.Keeper, height int64, txID common.TxID, txs common.Txs, eventStatus EventStatus) {
}

// EmitPoolEvent is going to save a pool event to storage
func (m *EventMgrV1) EmitPoolEvent(ctx cosmos.Context, keeper keeper.Keeper, txIn common.TxID, status EventStatus, poolEvt EventPool) error {
	bytes, err := json.Marshal(poolEvt)
	if err != nil {
		return fmt.Errorf("fail to marshal pool event: %w", err)
	}

	tx := common.Tx{
		ID: txIn,
	}
	evt := NewEvent(poolEvt.Type(), ctx.BlockHeight(), tx, bytes, status)
	if err := keeper.UpsertEvent(ctx, evt); err != nil {
		return fmt.Errorf("fail to save pool status change event: %w", err)
	}
	events, err := poolEvt.Events()
	if err != nil {
		return fmt.Errorf("fail to get pool events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)

	return nil
}

// EmitErrataEvent generate an errata event
func (m *EventMgrV1) EmitErrataEvent(ctx cosmos.Context, keeper keeper.Keeper, txIn common.TxID, errataEvent EventErrata) error {
	errataBuf, err := json.Marshal(errataEvent)
	if err != nil {
		ctx.Logger().Error("fail to marshal errata event to buf", "error", err)
		return fmt.Errorf("fail to marshal errata event to json: %w", err)
	}
	evt := NewEvent(
		errataEvent.Type(),
		ctx.BlockHeight(),
		common.Tx{ID: txIn},
		errataBuf,
		EventSuccess,
	)
	if err := keeper.UpsertEvent(ctx, evt); err != nil {
		ctx.Logger().Error("fail to save errata event", "error", err)
		return fmt.Errorf("fail to save errata event: %w", err)
	}
	events, err := errataEvent.Events()
	if err != nil {
		return fmt.Errorf("fail to emit standard event: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

func (m *EventMgrV1) EmitGasEvent(ctx cosmos.Context, keeper keeper.Keeper, gasEvent *EventGas) error {
	if gasEvent == nil {
		return nil
	}
	buf, err := json.Marshal(gasEvent)
	if err != nil {
		ctx.Logger().Error("fail to marshal gas event", "error", err)
		return fmt.Errorf("fail to marshal gas event to json: %w", err)
	}
	evt := NewEvent(gasEvent.Type(), ctx.BlockHeight(), common.Tx{ID: common.BlankTxID}, buf, EventSuccess)
	if err := keeper.UpsertEvent(ctx, evt); err != nil {
		ctx.Logger().Error("fail to upsert event", "error", err)
		return fmt.Errorf("fail to save gas event: %w", err)
	}
	events, err := gasEvent.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)

	return nil
}

// EmitStakeEvent add the stake event to block
func (m *EventMgrV1) EmitStakeEvent(ctx cosmos.Context, keeper keeper.Keeper, stakeEvent EventStake) error {
	events, err := stakeEvent.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

// EmitRewardEvent save the reward event to keyvalue store and also use event manager
func (m *EventMgrV1) EmitRewardEvent(ctx cosmos.Context, keeper keeper.Keeper, rewardEvt EventRewards) error {
	evtBytes, err := json.Marshal(rewardEvt)
	if err != nil {
		return fmt.Errorf("fail to marshal reward event to json: %w", err)
	}
	evt := NewEvent(
		rewardEvt.Type(),
		ctx.BlockHeight(),
		common.Tx{ID: common.BlankTxID},
		evtBytes,
		EventSuccess,
	)
	if err := keeper.UpsertEvent(ctx, evt); err != nil {
		return fmt.Errorf("fail to save event: %w", err)
	}
	events, err := rewardEvt.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

func (m *EventMgrV1) EmitSwapEvent(ctx cosmos.Context, keeper keeper.Keeper, swap EventSwap) error {
	buf, err := json.Marshal(swap)
	if err != nil {
		return fmt.Errorf("fail to marshal swap event to json: %w", err)
	}
	evt := NewEvent(swap.Type(), ctx.BlockHeight(), swap.InTx, buf, EventPending)
	// OutTxs is a temporary field that we used, as for now we need to keep backward compatibility so the
	// events change doesn't break midgard and smoke test, for double swap , we first swap the source asset to RUNE ,
	// and then from RUNE to target asset, so the first will be marked as success
	if !swap.OutTxs.IsEmpty() {
		evt.Status = EventSuccess
		evt.OutTxs = common.Txs{swap.OutTxs}
		outboundEvt := NewEventOutbound(swap.InTx.ID, swap.OutTxs)
		if err := m.EmitOutboundEvent(ctx, outboundEvt); err != nil {
			return fmt.Errorf("fail to emit an outbound event for double swap: %w", err)
		}
	}
	if err := keeper.UpsertEvent(ctx, evt); err != nil {
		return fmt.Errorf("fail to save swap event: %w", err)
	}
	events, err := swap.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

// EmitReserveEvent emit reserve event both save it to local key value store , and also event manager
func (m *EventMgrV1) EmitReserveEvent(ctx cosmos.Context, keeper keeper.Keeper, reserveEvent EventReserve) error {
	buf, err := json.Marshal(reserveEvent)
	if nil != err {
		return err
	}
	e := NewEvent(reserveEvent.Type(), ctx.BlockHeight(), reserveEvent.InTx, buf, EventSuccess)
	if err := keeper.UpsertEvent(ctx, e); err != nil {
		return fmt.Errorf("fail to save reserve event: %w", err)
	}
	events, err := reserveEvent.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

// EmitRefundEvent emit refund event , save it to local key value store and also emit through event manager
func (m *EventMgrV1) EmitRefundEvent(ctx cosmos.Context, keeper keeper.Keeper, refundEvt EventRefund, status EventStatus) error {
	buf, err := json.Marshal(refundEvt)
	if err != nil {
		return fmt.Errorf("fail to marshal refund event: %w", err)
	}
	event := NewEvent(refundEvt.Type(), ctx.BlockHeight(), refundEvt.InTx, buf, status)
	event.Fee = refundEvt.Fee
	if err := keeper.UpsertEvent(ctx, event); err != nil {
		return fmt.Errorf("fail to save refund event: %w", err)
	}
	events, err := refundEvt.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

func (m *EventMgrV1) EmitBondEvent(ctx cosmos.Context, keeper keeper.Keeper, bondEvent EventBond) error {
	buf, err := json.Marshal(bondEvent)
	if err != nil {
		return fmt.Errorf("fail to marshal bond event: %w", err)
	}

	e := NewEvent(bondEvent.Type(), ctx.BlockHeight(), bondEvent.TxIn, buf, EventSuccess)
	if err := keeper.UpsertEvent(ctx, e); err != nil {
		return fmt.Errorf("fail to save bond event: %w", err)
	}
	events, err := bondEvent.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

// EmitUnstakeEvent save unstake event to local key value store , and also add it to event manager
func (m *EventMgrV1) EmitUnstakeEvent(ctx cosmos.Context, keeper keeper.Keeper, unstakeEvt EventUnstake) error {
	unstakeBytes, err := json.Marshal(unstakeEvt)
	if err != nil {
		return fmt.Errorf("fail to marshal unstake event: %w", err)
	}

	// unstake event is pending , once signer send the fund to customer successfully, then this should be marked as success
	evt := NewEvent(
		unstakeEvt.Type(),
		ctx.BlockHeight(),
		unstakeEvt.InTx,
		unstakeBytes,
		EventPending,
	)

	if err := keeper.UpsertEvent(ctx, evt); err != nil {
		return fmt.Errorf("fail to save unstake event: %w", err)
	}
	events, err := unstakeEvt.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

// EmitAddEvent save add event to local key value store , and add it to event manager
func (m *EventMgrV1) EmitAddEvent(ctx cosmos.Context, keeper keeper.Keeper, addEvt EventAdd) error {
	buf, err := json.Marshal(addEvt)
	if err != nil {
		return fmt.Errorf("fail to marshal add event: %w", err)
	}
	evt := NewEvent(
		addEvt.Type(),
		ctx.BlockHeight(),
		addEvt.InTx,
		buf,
		EventSuccess,
	)
	if err := keeper.UpsertEvent(ctx, evt); err != nil {
		return fmt.Errorf("fail to save event: %w", err)
	}
	events, err := addEvt.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

// EmitSlashEvent
func (m *EventMgrV1) EmitSlashEvent(ctx cosmos.Context, keeper keeper.Keeper, slashEvt EventSlash) error {
	slashBuf, err := json.Marshal(slashEvt)
	if err != nil {
		return fmt.Errorf("fail to marshal slash event to buf: %w", err)
	}
	event := NewEvent(
		slashEvt.Type(),
		ctx.BlockHeight(),
		common.Tx{ID: common.BlankTxID},
		slashBuf,
		EventSuccess,
	)
	if err := keeper.UpsertEvent(ctx, event); err != nil {
		return fmt.Errorf("fail to save event: %w", err)
	}
	events, err := slashEvt.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

// EmitFeeEvent emit a fee event through event manager
func (m *EventMgrV1) EmitFeeEvent(ctx cosmos.Context, keeper keeper.Keeper, feeEvent EventFee) error {
	if err := updateEventFee(ctx, keeper, feeEvent.TxID, feeEvent.Fee); err != nil {
		return fmt.Errorf("fail to update event fee: %w", err)
	}

	if feeEvent.Fee.Coins.IsEmpty() && feeEvent.Fee.PoolDeduct.IsZero() {
		return nil
	}
	events, err := feeEvent.Events()
	if err != nil {
		return fmt.Errorf("fail to emit fee event: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

// EmitOutboundEvent emit an outbound event
func (m *EventMgrV1) EmitOutboundEvent(ctx cosmos.Context, outbound EventOutbound) error {
	events, err := outbound.Events()
	if err != nil {
		return fmt.Errorf("fail to emit outbound event: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}
