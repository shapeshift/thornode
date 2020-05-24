package thorchain

import cosmos "gitlab.com/thorchain/thornode/common/cosmos"

type KeeperTss interface {
	SetTssVoter(_ cosmos.Context, tss TssVoter)
	GetTssVoterIterator(_ cosmos.Context) cosmos.Iterator
	GetTssVoter(_ cosmos.Context, _ string) (TssVoter, error)
}

// SetTssVoter - save a txin voter object
func (k KVStoreV1) SetTssVoter(ctx cosmos.Context, tss TssVoter) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixTss, tss.String())
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(tss))
}

// GetTssVoterIterator iterate tx in voters
func (k KVStoreV1) GetTssVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixTss))
}

// GetTss - gets information of a tx hash
func (k KVStoreV1) GetTssVoter(ctx cosmos.Context, id string) (TssVoter, error) {
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
