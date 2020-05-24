package thorchain

import (
	"strconv"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type KeeperKeygen interface {
	SetKeygenBlock(ctx cosmos.Context, keygenBlock KeygenBlock) error
	GetKeygenBlockIterator(ctx cosmos.Context) cosmos.Iterator
	GetKeygenBlock(ctx cosmos.Context, height int64) (KeygenBlock, error)
}

// SetKeygenBlock save the KeygenBlock to kv store
func (k KVStoreV1) SetKeygenBlock(ctx cosmos.Context, keygen KeygenBlock) error {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixKeygen, strconv.FormatInt(keygen.Height, 10))
	buf, err := k.cdc.MarshalBinaryBare(keygen)
	if err != nil {
		return dbError(ctx, "fail to marshal keygen block", err)
	}
	store.Set([]byte(key), buf)
	return nil
}

// GetKeygenBlockIterator return an iterator
func (k KVStoreV1) GetKeygenBlockIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixKeygen))
}

// GetKeygenBlock from a given height
func (k KVStoreV1) GetKeygenBlock(ctx cosmos.Context, height int64) (KeygenBlock, error) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixKeygen, strconv.FormatInt(height, 10))
	if !store.Has([]byte(key)) {
		return NewKeygenBlock(height), nil
	}
	buf := store.Get([]byte(key))
	var keygenBlock KeygenBlock
	if err := k.cdc.UnmarshalBinaryBare(buf, &keygenBlock); err != nil {
		return KeygenBlock{}, dbError(ctx, "fail to unmarshal keygen block", err)
	}
	return keygenBlock, nil
}
