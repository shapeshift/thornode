package keeperv1

import "gitlab.com/thorchain/thornode/common/cosmos"

// SetTssVoter - save a tss voter object
func (k KVStore) SetTssVoter(ctx cosmos.Context, tss TssVoter) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixTss, tss.String())
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(tss))
}

// GetTssVoterIterator iterate tx in voters
func (k KVStore) GetTssVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixTss))
}

// GetTss - gets information of a tx hash
func (k KVStore) GetTssVoter(ctx cosmos.Context, id string) (TssVoter, error) {
	key := k.GetKey(ctx, prefixTss, id)

	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return TssVoter{ID: id}, nil
	}

	bz := store.Get([]byte(key))
	var record TssVoter
	if err := k.cdc.UnmarshalBinaryBare(bz, &record); err != nil {
		return TssVoter{}, dbError(ctx, "Unmarshal: tss voter", err)
	}
	return record, nil
}
