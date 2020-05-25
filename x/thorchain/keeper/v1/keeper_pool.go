package keeperv1

import (
	"errors"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// GetPoolIterator iterate pools
func (k KVStore) GetPoolIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixPool))
}

func (k KVStore) GetPools(ctx cosmos.Context) (Pools, error) {
	var pools Pools
	iterator := k.GetPoolIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		err := k.Cdc().UnmarshalBinaryBare(iterator.Value(), &pool)
		if err != nil {
			return nil, dbError(ctx, "Unmarsahl: pool", err)
		}
		pools = append(pools, pool)
	}
	return pools, nil
}

// GetPool get the entire Pool metadata struct for a pool ID
func (k KVStore) GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error) {
	key := k.GetKey(ctx, prefixPool, asset.String())
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return NewPool(), nil
	}
	buf := store.Get([]byte(key))
	var pool Pool
	if err := k.cdc.UnmarshalBinaryBare(buf, &pool); err != nil {
		return NewPool(), dbError(ctx, "Unmarshal: pool", err)
	}
	return pool, nil
}

// Sets the entire Pool metadata struct for a pool ID
func (k KVStore) SetPool(ctx cosmos.Context, pool Pool) error {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixPool, pool.Asset.String())

	if pool.Asset.IsEmpty() {
		return errors.New("cannot save a pool with an empty asset")
	}

	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(pool))
	return nil
}

// PoolExist check whether the given pool exist in the datastore
func (k KVStore) PoolExist(ctx cosmos.Context, asset common.Asset) bool {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixPool, asset.String())
	return store.Has([]byte(key))
}
