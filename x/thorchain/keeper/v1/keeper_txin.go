package keeperv1

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
)

// SetObservedTxInVoter - save a txin voter object
func (k KVStore) SetObservedTxInVoter(ctx cosmos.Context, tx ObservedTxVoter) {
	k.setObservedTxVoter(ctx, prefixObservedTxIn, tx)
}

// SetObservedTxOutVoter - save a txout voter object
func (k KVStore) SetObservedTxOutVoter(ctx cosmos.Context, tx ObservedTxVoter) {
	k.setObservedTxVoter(ctx, prefixObservedTxOut, tx)
}

func (k KVStore) setObservedTxVoter(ctx cosmos.Context, prefix types.DbPrefix, tx ObservedTxVoter) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefix, tx.String())
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(tx))
}

// GetObservedTxInVoterIterator iterate tx in voters
func (k KVStore) GetObservedTxInVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getObservedTxVoterIterator(ctx, prefixObservedTxIn)
}

// GetObservedTxOutVoterIterator iterate tx out voters
func (k KVStore) GetObservedTxOutVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getObservedTxVoterIterator(ctx, prefixObservedTxOut)
}

func (k KVStore) getObservedTxVoterIterator(ctx cosmos.Context, prefix types.DbPrefix) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefix))
}

// GetObservedTxInVoter - gets information of an observed inbound tx based on the txid
func (k KVStore) GetObservedTxInVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error) {
	return k.getObservedTxVoter(ctx, prefixObservedTxIn, hash)
}

// GetObservedTxOutVoter - gets information of an observed outbound tx based on the txid
func (k KVStore) GetObservedTxOutVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error) {
	return k.getObservedTxVoter(ctx, prefixObservedTxOut, hash)
}

func (k KVStore) getObservedTxVoter(ctx cosmos.Context, prefix types.DbPrefix, hash common.TxID) (ObservedTxVoter, error) {
	key := k.GetKey(ctx, prefix, hash.String())
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return ObservedTxVoter{TxID: hash}, nil
	}

	bz := store.Get([]byte(key))
	var record ObservedTxVoter
	if err := k.cdc.UnmarshalBinaryBare(bz, &record); err != nil {
		return ObservedTxVoter{}, dbError(ctx, "unmarshal: observed tx voter", err)
	}
	return record, nil
}
