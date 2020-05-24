package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type KeeperGas interface {
	GetGas(_ cosmos.Context, asset common.Asset) ([]cosmos.Uint, error)
	SetGas(_ cosmos.Context, asset common.Asset, units []cosmos.Uint)
	GetGasIterator(ctx cosmos.Context) cosmos.Iterator
}

func (k KVStoreV1) GetGas(ctx cosmos.Context, asset common.Asset) ([]cosmos.Uint, error) {
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

func (k KVStoreV1) SetGas(ctx cosmos.Context, asset common.Asset, units []cosmos.Uint) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixGas, asset.String())
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(units))
}

// GetGasIterator iterate gas units
func (k KVStoreV1) GetGasIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixGas))
}
