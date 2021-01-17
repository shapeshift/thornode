package keeperv1

import (
	"fmt"
	"strconv"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (k KVStore) setTxOut(ctx cosmos.Context, key string, record TxOut) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshalBinaryBare(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getTxOut(ctx cosmos.Context, key string, record *TxOut) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	bz := store.Get([]byte(key))
	if err := k.cdc.UnmarshalBinaryBare(bz, record); err != nil {
		return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	return true, nil
}

// AppendTxOut - append the given item to txOut
func (k KVStore) AppendTxOut(ctx cosmos.Context, height int64, item TxOutItem) error {
	block, err := k.GetTxOut(ctx, height)
	if err != nil {
		return err
	}
	block.TxArray = append(block.TxArray, item)
	return k.SetTxOut(ctx, block)
}

// ClearTxOut - remove the txout of the given height from key value  store
func (k KVStore) ClearTxOut(ctx cosmos.Context, height int64) error {
	k.del(ctx, k.GetKey(ctx, prefixTxOut, strconv.FormatInt(height, 10)))
	return nil
}

// SetTxOut - write the given txout information to key value store
func (k KVStore) SetTxOut(ctx cosmos.Context, blockOut *TxOut) error {
	if blockOut == nil || blockOut.IsEmpty() {
		return nil
	}
	k.setTxOut(ctx, k.GetKey(ctx, prefixTxOut, strconv.FormatInt(blockOut.Height, 10)), *blockOut)
	return nil
}

// GetTxOutIterator iterate tx out
func (k KVStore) GetTxOutIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getIterator(ctx, prefixTxOut)
}

// GetTxOut - write the given txout information to key values tore
func (k KVStore) GetTxOut(ctx cosmos.Context, height int64) (*TxOut, error) {
	record := NewTxOut(height)
	_, err := k.getTxOut(ctx, k.GetKey(ctx, prefixTxOut, strconv.FormatInt(height, 10)), record)
	return record, err
}
