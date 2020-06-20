package keeperv1

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// TODO these will not be needed once dynamic network fee get merged

// GetGas get gas information from key value store
func (k KVStore) GetGas(ctx cosmos.Context, asset common.Asset) ([]cosmos.Uint, error) {
	key := k.GetKey(ctx, prefixGas, asset.String())
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return nil, nil
	}
	var gas []cosmos.Uint
	buf := store.Get([]byte(key))
	err := k.cdc.UnmarshalBinaryBare(buf, &gas)
	if err != nil {
		return nil, dbError(ctx, "Unmarshal: gas", err)
	}
	return gas, nil
}

// SetGas save gas information to key value store
func (k KVStore) SetGas(ctx cosmos.Context, asset common.Asset, units []cosmos.Uint) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixGas, asset.String())
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(units))
}

// GetGasIterator iterate gas units
func (k KVStore) GetGasIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixGas))
}
