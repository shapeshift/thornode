package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// SetObservedTxVoter - save a txin voter object
func (k KVStore) SetObservedTxVoter(ctx cosmos.Context, tx ObservedTxVoter) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixObservedTx, tx.String())
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(tx))
}

// GetObservedTxVoterIterator iterate tx in voters
func (k KVStore) GetObservedTxVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixObservedTx))
}

// GetObservedTx - gets information of a tx hash
func (k KVStore) GetObservedTxVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error) {
	key := k.GetKey(ctx, prefixObservedTx, hash.String())

	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return ObservedTxVoter{TxID: hash}, nil
	}

	bz := store.Get([]byte(key))
	var record ObservedTxVoter
	if err := k.cdc.UnmarshalBinaryBare(bz, &record); err != nil {
		return ObservedTxVoter{}, dbError(ctx, "Unmarshal: observed tx voter", err)
	}
	return record, nil
}
