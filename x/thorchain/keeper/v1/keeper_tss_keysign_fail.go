package keeperv1

import cosmos "gitlab.com/thorchain/thornode/common/cosmos"

// SetTssKeysignFailVoter - save a txin voter object
func (k KVStore) SetTssKeysignFailVoter(ctx cosmos.Context, tss TssKeysignFailVoter) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixTss, tss.String())
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(tss))
}

// GetTssKeysignFailVoterIterator iterate tx in voters
func (k KVStore) GetTssKeysignFailVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixTss))
}

// GetTss - gets information of a tx hash
func (k KVStore) GetTssKeysignFailVoter(ctx cosmos.Context, id string) (TssKeysignFailVoter, error) {
	key := k.GetKey(ctx, prefixTss, id)

	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return TssKeysignFailVoter{ID: id}, nil
	}

	bz := store.Get([]byte(key))
	var record TssKeysignFailVoter
	if err := k.cdc.UnmarshalBinaryBare(bz, &record); err != nil {
		return TssKeysignFailVoter{}, dbError(ctx, "Unmarshal: tss voter", err)
	}
	return record, nil
}
