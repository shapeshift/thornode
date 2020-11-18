package keeperv1

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// GetNetwork retrieve network data from key value store
func (k KVStore) GetNetwork(ctx cosmos.Context) (Network, error) {
	record := NewNetwork()
	_, err := k.get(ctx, k.GetKey(ctx, prefixNetwork, ""), &record)
	return record, err
}

// SetNetwork save the given network data to key value store, it will overwrite existing vault
func (k KVStore) SetNetwork(ctx cosmos.Context, data Network) error {
	k.set(ctx, k.GetKey(ctx, prefixNetwork, ""), data)
	return nil
}
