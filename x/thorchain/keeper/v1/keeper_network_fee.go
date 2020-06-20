package keeperv1

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// GetNetworkFee get the network fee of the given chain from kv store , if it doesn't exist , it will create an empty one
func (k KVStore) GetNetworkFee(ctx cosmos.Context, chain common.Chain) (NetworkFee, error) {
	key := k.GetKey(ctx, prefixNetworkFee, chain.String())
	store := ctx.KVStore(k.storeKey)
	emptyNetworkFee := NetworkFee{
		Chain:              chain,
		TransactionSize:    0,
		TransactionFeeRate: sdk.ZeroUint(),
	}
	if !store.Has([]byte(key)) {
		return emptyNetworkFee, nil
	}
	buf := store.Get([]byte(key))
	var networkFee NetworkFee
	if err := k.cdc.UnmarshalBinaryBare(buf, &networkFee); err != nil {
		return emptyNetworkFee, fmt.Errorf("fail to unmarshal network fee: %w", err)
	}
	return networkFee, nil
}

// SaveNetworkFee save the network fee to kv store
func (k KVStore) SaveNetworkFee(ctx cosmos.Context, chain common.Chain, networkFee NetworkFee) error {
	if err := networkFee.Validate(); err != nil {
		return err
	}
	key := k.GetKey(ctx, prefixNetworkFee, chain.String())
	store := ctx.KVStore(k.storeKey)
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(networkFee))
	return nil
}

// GetNetworkFeeIterator
func (k KVStore) GetNetworkFeeIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixNetworkFee))
}
