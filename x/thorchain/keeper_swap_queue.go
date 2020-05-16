package thorchain

import (
	"errors"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type KeeperSwapQueue interface {
	SetSwapQueueItem(ctx cosmos.Context, msg MsgSwap) error
	GetSwapQueueIterator(ctx cosmos.Context) cosmos.Iterator
	GetSwapQueueItem(ctx cosmos.Context, txID common.TxID) (MsgSwap, error)
	RemoveSwapQueueItem(ctx cosmos.Context, txID common.TxID)
}

// SetSwapQueueItem - writes a swap item to the kvstore
func (k KVStore) SetSwapQueueItem(ctx cosmos.Context, msg MsgSwap) error {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixSwapQueueItem, msg.Tx.ID.String())
	buf, err := k.cdc.MarshalBinaryBare(msg)
	if err != nil {
		return dbError(ctx, "fail to marshal swap item to binary", err)
	}
	store.Set([]byte(key), buf)
	return nil
}

// GetSwapQueueIterator iterate tx out
func (k KVStore) GetSwapQueueIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixSwapQueueItem))
}

// GetSwapQueueItem - write the given swap queue item information to key values tore
func (k KVStore) GetSwapQueueItem(ctx cosmos.Context, txID common.TxID) (MsgSwap, error) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixSwapQueueItem, txID.String())
	if !store.Has([]byte(key)) {
		return MsgSwap{}, errors.New("not found")
	}
	var msg MsgSwap
	buf := store.Get([]byte(key))
	if err := k.cdc.UnmarshalBinaryBare(buf, &msg); err != nil {
		return msg, dbError(ctx, "fail to unmarshal swap queue item", err)
	}
	return msg, nil
}

// RemoveSwapQueueItem - removes a swap item to the kvstore
func (k KVStore) RemoveSwapQueueItem(ctx cosmos.Context, txID common.TxID) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixSwapQueueItem, txID.String())
	store.Delete([]byte(key))
}
