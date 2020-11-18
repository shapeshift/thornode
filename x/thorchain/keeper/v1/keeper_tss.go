package keeperv1

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// SetTssVoter - save a tss voter object
func (k KVStore) SetTssVoter(ctx cosmos.Context, tss TssVoter) {
	k.set(ctx, k.GetKey(ctx, prefixTss, tss.String()), tss)
}

// GetTssVoterIterator iterate tx in voters
func (k KVStore) GetTssVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getIterator(ctx, prefixTss)
}

// GetTssVoter - gets information of a tx hash
func (k KVStore) GetTssVoter(ctx cosmos.Context, id string) (TssVoter, error) {
	record := TssVoter{ID: id}
	_, err := k.get(ctx, k.GetKey(ctx, prefixTss, id), &record)
	return record, err
}

// GetTssKeygenMetric get tss keygen metric from key value store
func (k KVStore) GetTssKeygenMetric(ctx cosmos.Context, pubkey common.PubKey) (*TssKeygenMetric, error) {
	record := TssKeygenMetric{PubKey: pubkey}
	_, err := k.get(ctx, k.GetKey(ctx, prefixTssKeygenMetric, pubkey.String()), &record)
	return &record, err
}

// SetTssKeygenMetric save TssKeygenMetric to key value store
func (k KVStore) SetTssKeygenMetric(ctx cosmos.Context, metric *TssKeygenMetric) {
	k.set(ctx, k.GetKey(ctx, prefixTssKeygenMetric, metric.PubKey.String()), metric)
}

// GetTssKeysignMetric get tss keygen metric from key value store
func (k KVStore) GetTssKeysignMetric(ctx cosmos.Context, txID common.TxID) (*TssKeysignMetric, error) {
	record := TssKeysignMetric{}
	_, err := k.get(ctx, k.GetKey(ctx, prefixTssKeysignMetric, txID.String()), &record)
	return &record, err
}

// SetTssKeysignMetric save TssKeygenMetric to key value store
func (k KVStore) SetTssKeysignMetric(ctx cosmos.Context, metric *TssKeysignMetric) {
	// save the tss keysign metric against tx id
	k.set(ctx, k.GetKey(ctx, prefixTssKeysignMetric, metric.TxID.String()), metric)
	// save the latest keysign metric , it override previous
	k.set(ctx, k.GetKey(ctx, prefixTssKeysignMetricLatest, "keysign"), metric)
}

// GetLatestTssKeysignMetric return the latest tss keysign metric
func (k KVStore) GetLatestTssKeysignMetric(ctx cosmos.Context) (*TssKeysignMetric, error) {
	record := TssKeysignMetric{}
	_, err := k.get(ctx, k.GetKey(ctx, prefixTssKeysignMetricLatest, "keysign"), &record)
	return &record, err
}
