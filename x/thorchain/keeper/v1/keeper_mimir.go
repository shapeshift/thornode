package keeperv1

import "gitlab.com/thorchain/thornode/common/cosmos"

const KRAKEN string = "ReleaseTheKraken"

// GetMimir get a mimir value from key value store
func (k KVStore) GetMimir(ctx cosmos.Context, key string) (int64, error) {
	// if we have the kraken, mimir is no more, ignore him
	if k.haveKraken(ctx) {
		return -1, nil
	}

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

// haveKraken - check to see if we have "released the kraken"
func (k KVStore) haveKraken(ctx cosmos.Context) bool {
	key := k.GetKey(ctx, prefixMimir, KRAKEN)
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false
	}
	var value int64
	buf := store.Get([]byte(key))
	k.cdc.MustUnmarshalBinaryBare(buf, &value)
	return value >= 0
}

// SetMimir save a mimir value to key value store
func (k KVStore) SetMimir(ctx cosmos.Context, key string, value int64) {
	// if we have the kraken, mimir is no more, ignore him
	if k.haveKraken(ctx) {
		return
	}
	store := ctx.KVStore(k.storeKey)
	key = k.GetKey(ctx, prefixMimir, key)
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(value))
}

// GetMimirIterator iterate gas units
func (k KVStore) GetMimirIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixMimir))
}
