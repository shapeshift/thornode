package keeperv1

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (k KVStore) setNetwork(ctx cosmos.Context, key string, record Network) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshal(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getNetwork(ctx cosmos.Context, key string, record *Network) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, record); err != nil {
		return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	return true, nil
}

// GetNetwork retrieve network data from key value store
func (k KVStore) GetNetwork(ctx cosmos.Context) (Network, error) {
	record := NewNetwork()
	_, err := k.getNetwork(ctx, k.GetKey(ctx, prefixNetwork, ""), &record)
	return record, err
}

// SetNetwork save the given network data to key value store, it will overwrite existing vault
func (k KVStore) SetNetwork(ctx cosmos.Context, data Network) error {
	k.setNetwork(ctx, k.GetKey(ctx, prefixNetwork, ""), data)
	return nil
}
