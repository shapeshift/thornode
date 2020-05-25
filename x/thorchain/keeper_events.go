package thorchain

import (
	"errors"
	"fmt"
	"strconv"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type KeeperEvents interface {
	GetEvent(ctx cosmos.Context, eventID int64) (Event, error)
	GetEventsIterator(ctx cosmos.Context) cosmos.Iterator
	UpsertEvent(ctx cosmos.Context, event Event) error
	GetPendingEventID(ctx cosmos.Context, txID common.TxID) ([]int64, error)
	GetCurrentEventID(ctx cosmos.Context) (int64, error)
	SetCurrentEventID(ctx cosmos.Context, eventID int64)
	GetAllPendingEvents(ctx cosmos.Context) (Events, error)
	GetEventsIDByTxHash(ctx cosmos.Context, txID common.TxID) ([]int64, error)
}

var ErrEventNotFound = errors.New("event not found")

// GetEvent will retrieve event with the given id from data store
func (k KVStoreV1) GetEvent(ctx cosmos.Context, eventID int64) (Event, error) {
	key := k.GetKey(ctx, prefixEvents, strconv.FormatInt(eventID, 10))
	store := ctx.KVStore(k.storeKey)
	buf := store.Get([]byte(key))
	var e Event
	if err := k.Cdc().UnmarshalBinaryBare(buf, &e); err != nil {
		return Event{}, fmt.Errorf("fail to unmarshal event: %w", err)
	}
	return e, nil
}

// UpsertEvent add one event to data store
func (k KVStoreV1) UpsertEvent(ctx cosmos.Context, event Event) error {
	if event.InTx.ID.IsEmpty() {
		return fmt.Errorf("cant save event with empty TxIn ID")
	}
	if event.Height == 0 {
		return fmt.Errorf("cant save event with height equal to zero")
	}
	if event.ID == 0 {
		nextEventID, err := k.getNextEventID(ctx)
		if err != nil {
			return fmt.Errorf("fail to get next event id: %w", err)
		}
		event.ID = nextEventID
		// keep a map between tx hash and event id
		if err := k.upsertEventTxHash(ctx, event); err != nil {
			return err
		}
	}

	key := k.GetKey(ctx, prefixEvents, strconv.FormatInt(event.ID, 10))
	store := ctx.KVStore(k.storeKey)
	buf, err := k.cdc.MarshalBinaryBare(&event)
	if err != nil {
		return fmt.Errorf("fail to marshal event: %w", err)
	}
	store.Set([]byte(key), buf)
	if event.Status == EventPending {
		return k.setEventPending(ctx, event)
	}
	k.removeEventPending(ctx, event)
	return nil
}

func (k KVStoreV1) removeEventPending(ctx cosmos.Context, event Event) {
	key := k.GetKey(ctx, prefixPendingEvents, event.InTx.ID.String())
	store := ctx.KVStore(k.storeKey)
	store.Delete([]byte(key))
}

// setEventPending store the pending event use InTx hash as the key
func (k KVStoreV1) setEventPending(ctx cosmos.Context, event Event) error {
	if event.Status != EventPending {
		return nil
	}
	ctx.Logger().Info(fmt.Sprintf("event id(%d): %s", event.ID, event.InTx.ID))
	key := k.GetKey(ctx, prefixPendingEvents, event.InTx.ID.String())
	store := ctx.KVStore(k.storeKey)
	var eventIDs []int64
	var err error
	if store.Has([]byte(key)) {
		eventIDs, err = k.GetPendingEventID(ctx, event.InTx.ID)
		if err != nil {
			return fmt.Errorf("fail to get pending event ids: %w", err)
		}
	}
	eventIDs = append(eventIDs, event.ID)
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(eventIDs))
	return nil
}

// GetPendingEventID we store the event in pending status using it's in tx hash
func (k KVStoreV1) GetPendingEventID(ctx cosmos.Context, txID common.TxID) ([]int64, error) {
	key := k.GetKey(ctx, prefixPendingEvents, txID.String())
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return nil, ErrEventNotFound
	}
	buf := store.Get([]byte(key))
	var eventIDs []int64
	if err := k.Cdc().UnmarshalBinaryBare(buf, &eventIDs); err != nil {
		return nil, fmt.Errorf("fail to unmarshal event id: %w", err)
	}
	return eventIDs, nil
}

// GetCompleteEventIterator iterate complete events
func (k KVStoreV1) GetEventsIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixEvents))
}

// GetNextEventID will increase the event id in key value store
func (k KVStoreV1) getNextEventID(ctx cosmos.Context) (int64, error) {
	var currentEventID, nextEventID int64
	currentEventID, err := k.GetCurrentEventID(ctx)
	if err != nil {
		return currentEventID, err
	}
	nextEventID = currentEventID + 1
	k.SetCurrentEventID(ctx, nextEventID)
	return currentEventID, nil
}

// GetCurrentEventID get the current event id in data store without increasing it
func (k KVStoreV1) GetCurrentEventID(ctx cosmos.Context) (int64, error) {
	var currentEventID int64
	key := k.GetKey(ctx, prefixCurrentEventID, "")
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		// the event id start from 1
		return 1, nil
	}
	buf := store.Get([]byte(key))
	if err := k.cdc.UnmarshalBinaryBare(buf, &currentEventID); err != nil {
		return 1, dbError(ctx, "Unmarshal: current event id", err)
	}
	if currentEventID == 0 {
		currentEventID = 1
	}
	return currentEventID, nil
}

// SetCurrentEventID set the current event id in kv store
func (k KVStoreV1) SetCurrentEventID(ctx cosmos.Context, eventID int64) {
	key := k.GetKey(ctx, prefixCurrentEventID, "")
	store := ctx.KVStore(k.storeKey)
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(&eventID))
}

// GetAllPendingEvents all events in pending status
func (k KVStoreV1) GetAllPendingEvents(ctx cosmos.Context) (Events, error) {
	key := k.GetKey(ctx, prefixPendingEvents, "")
	store := ctx.KVStore(k.storeKey)
	var events Events
	iter := cosmos.KVStorePrefixIterator(store, []byte(key))
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var eventIDs []int64
		if err := k.Cdc().UnmarshalBinaryBare(iter.Value(), &eventIDs); err != nil {
			return nil, fmt.Errorf("fail to unmarshal event id: %w", err)
		}
		for _, eventID := range eventIDs {
			evt, err := k.GetEvent(ctx, eventID)
			if err != nil {
				return nil, fmt.Errorf("fail to get event: %w", err)
			}
			events = append(events, evt)
		}
	}
	return events, nil
}

// GetEventsIDByTxHash given a tx id, return a slice of events id that is related to the tx hash
func (k KVStoreV1) GetEventsIDByTxHash(ctx cosmos.Context, txID common.TxID) ([]int64, error) {
	key := k.GetKey(ctx, prefixTxHashEvents, txID.String())
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return nil, ErrEventNotFound
	}
	buf := store.Get([]byte(key))
	var eventIDs []int64
	if err := k.Cdc().UnmarshalBinaryBare(buf, &eventIDs); err != nil {
		return nil, fmt.Errorf("fail to unmarshal event id: %w", err)
	}
	return eventIDs, nil
}

func (k KVStoreV1) upsertEventTxHash(ctx cosmos.Context, event Event) error {
	key := k.GetKey(ctx, prefixTxHashEvents, event.InTx.ID.String())
	store := ctx.KVStore(k.storeKey)
	var eventIDs []int64
	var err error
	if store.Has([]byte(key)) {
		eventIDs, err = k.GetEventsIDByTxHash(ctx, event.InTx.ID)
		if err != nil {
			return fmt.Errorf("fail to get events id by tx hash id: %w", err)
		}
	}
	eventIDs = append(eventIDs, event.ID)
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(eventIDs))
	return nil
}
