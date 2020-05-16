package thorchain

import cosmos "gitlab.com/thorchain/thornode/common/cosmos"

type KeeperMimir interface {
	GetMimir(_ cosmos.Context, key string) (int64, error)
	SetMimir(_ cosmos.Context, key string, value int64)
	GetMimirIterator(ctx cosmos.Context) cosmos.Iterator
}

func (k KVStore) GetMimir(ctx cosmos.Context, key string) (int64, error) {
	key = k.GetKey(ctx, prefixMimir, key)
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return -1, nil
	}
	var value int64
	buf := store.Get([]byte(key))
	err := k.cdc.UnmarshalBinaryBare(buf, &value)
	if err != nil {
		return -1, dbError(ctx, "Unmarshal: mimir attr", err)
	}
	return value, nil
}

func (k KVStore) SetMimir(ctx cosmos.Context, key string, value int64) {
	store := ctx.KVStore(k.storeKey)
	key = k.GetKey(ctx, prefixMimir, key)
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(value))
}

// GetMimirIterator iterate gas units
func (k KVStore) GetMimirIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixMimir))
}
