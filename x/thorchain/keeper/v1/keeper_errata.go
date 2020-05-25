package keeperv1

import (
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// SetErrataTxVoter - save a txin voter object
func (k KVStore) SetErrataTxVoter(ctx cosmos.Context, errata ErrataTxVoter) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixErrataTx, errata.String())
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(errata))
}

// GetErrataTxVoterIterator iterate tx in voters
func (k KVStore) GetErrataTxVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixErrataTx))
}

// GetErrataTx - gets information of a tx hash
func (k KVStore) GetErrataTxVoter(ctx cosmos.Context, txID common.TxID, chain common.Chain) (ErrataTxVoter, error) {
	errata := NewErrataTxVoter(txID, chain)
	key := k.GetKey(ctx, prefixErrataTx, errata.String())

	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return errata, nil
	}

	bz := store.Get([]byte(key))
	var record ErrataTxVoter
	if err := k.cdc.UnmarshalBinaryBare(bz, &record); err != nil {
		return errata, dbError(ctx, "Unmarshal: errata tx voter", err)
	}
	return record, nil
}
