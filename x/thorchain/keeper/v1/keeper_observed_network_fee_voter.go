package keeperv1

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// SetObservedNetworkFeeVoter - save a observed network fee voter object
func (k KVStore) SetObservedNetworkFeeVoter(ctx cosmos.Context, networkFeeVoter ObservedNetworkFeeVoter) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixNetworkFeeVoter, networkFeeVoter.String())
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(networkFeeVoter))
}

// GetObservedNetworkFeeVoterIterator iterate tx in voters
func (k KVStore) GetObservedNetworkFeeVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixNetworkFeeVoter))
}

// GetObservedNetworkFeeVoter - gets information of an observed network fee voter
func (k KVStore) GetObservedNetworkFeeVoter(ctx cosmos.Context, height int64, chain common.Chain) (ObservedNetworkFeeVoter, error) {
	networkFeeVoter := NewObservedNetworkFeeVoter(height, chain)
	key := k.GetKey(ctx, prefixNetworkFeeVoter, networkFeeVoter.String())

	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return networkFeeVoter, nil
	}

	bz := store.Get([]byte(key))
	var record ObservedNetworkFeeVoter
	if err := k.cdc.UnmarshalBinaryBare(bz, &record); err != nil {
		return networkFeeVoter, dbError(ctx, "unmarshal: observed network fee voter", err)
	}
	return record, nil
}
