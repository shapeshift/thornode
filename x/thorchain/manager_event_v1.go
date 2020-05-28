package thorchain

import (
	"fmt"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// EventMgrV1 implement EventManager interface
type EventMgrV1 struct {
}

// NewEventMgrV1 create a new instance of EventMgrV1
func NewEventMgrV1() *EventMgrV1 {
	return &EventMgrV1{}
}

// EmitPoolEvent is going to save a pool event to storage
func (m *EventMgrV1) EmitPoolEvent(ctx cosmos.Context, poolEvt EventPool) error {
	events, err := poolEvt.Events()
	if err != nil {
		return fmt.Errorf("fail to get pool events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)

	return nil
}

// EmitErrataEvent generate an errata event
func (m *EventMgrV1) EmitErrataEvent(ctx cosmos.Context, errataEvent EventErrata) error {
	events, err := errataEvent.Events()
	if err != nil {
		return fmt.Errorf("fail to emit standard event: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

func (m *EventMgrV1) EmitGasEvent(ctx cosmos.Context, gasEvent *EventGas) error {
	if gasEvent == nil {
		return nil
	}
	events, err := gasEvent.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)

	return nil
}

// EmitStakeEvent add the stake event to block
func (m *EventMgrV1) EmitStakeEvent(ctx cosmos.Context, stakeEvent EventStake) error {
	events, err := stakeEvent.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

// EmitRewardEvent save the reward event to keyvalue store and also use event manager
func (m *EventMgrV1) EmitRewardEvent(ctx cosmos.Context, rewardEvt EventRewards) error {
	events, err := rewardEvt.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

func (m *EventMgrV1) EmitSwapEvent(ctx cosmos.Context, swap EventSwap) error {
	// OutTxs is a temporary field that we used, as for now we need to keep backward compatibility so the
	// events change doesn't break midgard and smoke test, for double swap , we first swap the source asset to RUNE ,
	// and then from RUNE to target asset, so the first will be marked as success
	if !swap.OutTxs.IsEmpty() {
		outboundEvt := NewEventOutbound(swap.InTx.ID, swap.OutTxs)
		if err := m.EmitOutboundEvent(ctx, outboundEvt); err != nil {
			return fmt.Errorf("fail to emit an outbound event for double swap: %w", err)
		}
	}
	events, err := swap.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

// EmitReserveEvent emit reserve event both save it to local key value store , and also event manager
func (m *EventMgrV1) EmitReserveEvent(ctx cosmos.Context, reserveEvent EventReserve) error {
	events, err := reserveEvent.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

// EmitRefundEvent emit refund event , save it to local key value store and also emit through event manager
func (m *EventMgrV1) EmitRefundEvent(ctx cosmos.Context, refundEvt EventRefund) error {
	events, err := refundEvt.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

func (m *EventMgrV1) EmitBondEvent(ctx cosmos.Context, bondEvent EventBond) error {
	events, err := bondEvent.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

// EmitUnstakeEvent save unstake event to local key value store , and also add it to event manager
func (m *EventMgrV1) EmitUnstakeEvent(ctx cosmos.Context, unstakeEvt EventUnstake) error {
	events, err := unstakeEvt.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

// EmitAddEvent save add event to local key value store , and add it to event manager
func (m *EventMgrV1) EmitAddEvent(ctx cosmos.Context, addEvt EventAdd) error {
	events, err := addEvt.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

// EmitSlashEvent
func (m *EventMgrV1) EmitSlashEvent(ctx cosmos.Context, slashEvt EventSlash) error {
	events, err := slashEvt.Events()
	if err != nil {
		return fmt.Errorf("fail to get events: %w", err)
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}

// EmitFeeEvent emit a fee event through event manager
func (m *EventMgrV1) EmitFeeEvent(ctx cosmos.Context, feeEvent EventFee) error {
	if feeEvent.Fee.Coins.IsEmpty() && feeEvent.Fee.PoolDeduct.IsZero() {
		return nil
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
