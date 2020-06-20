package keeperv1

import "gitlab.com/thorchain/thornode/common/cosmos"

// SetBanVoter - save a ban voter object
func (k KVStore) SetBanVoter(ctx cosmos.Context, ban BanVoter) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixBanVoter, ban.String())
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(ban))
}

// GetBanVoter - gets information of ban voter
func (k KVStore) GetBanVoter(ctx cosmos.Context, addr cosmos.AccAddress) (BanVoter, error) {
	ban := NewBanVoter(addr)
	key := k.GetKey(ctx, prefixBanVoter, ban.String())

	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return ban, nil
	}

	bz := store.Get([]byte(key))
	var record BanVoter
	if err := k.cdc.UnmarshalBinaryBare(bz, &record); err != nil {
		return ban, dbError(ctx, "Unmarshal: ban voter", err)
	}
	return record, nil
}

// GetBanVoterIterator - get an iterator for ban voter
func (k KVStore) GetBanVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixBanVoter))
}
