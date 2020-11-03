package keeperv1

import (
	"errors"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// SetSwapQueueItem - writes a swap item to the kv store
func (k KVStore) SetSwapQueueItem(ctx cosmos.Context, msg MsgSwap, i int) error {
	k.set(ctx, k.GetKey(ctx, prefixSwapQueueItem, fmt.Sprintf("%s-%d", msg.Tx.ID.String(), i)), msg)
	return nil
}

// GetSwapQueueIterator iterate swap queue
func (k KVStore) GetSwapQueueIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getIterator(ctx, prefixSwapQueueItem)
}

// GetSwapQueueItem - write the given swap queue item information to key values tore
func (k KVStore) GetSwapQueueItem(ctx cosmos.Context, txID common.TxID, i int) (MsgSwap, error) {
	record := MsgSwap{}
	ok, err := k.get(ctx, k.GetKey(ctx, prefixSwapQueueItem, fmt.Sprintf("%s-%d", txID.String(), i)), &record)
	if !ok {
		return record, errors.New("not found")
	}
	return record, err
}

// RemoveSwapQueueItem - removes a swap item from the kv store
func (k KVStore) RemoveSwapQueueItem(ctx cosmos.Context, txID common.TxID, i int) {
	k.del(ctx, k.GetKey(ctx, prefixSwapQueueItem, fmt.Sprintf("%s-%d", txID.String(), i)))
}
