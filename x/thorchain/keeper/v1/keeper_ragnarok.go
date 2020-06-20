package keeperv1

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// RagnarokInProgress return true only when Ragnarok is happening, when Ragnarok block height is not 0
func (k KVStore) RagnarokInProgress(ctx cosmos.Context) bool {
	height, err := k.GetRagnarokBlockHeight(ctx)
	if err != nil {
		ctx.Logger().Error(err.Error())
		return true
	}
	return height > 0
}

// GetRagnarokBlockHeight get ragnarok block height from key value store
func (k KVStore) GetRagnarokBlockHeight(ctx cosmos.Context) (int64, error) {
	key := k.GetKey(ctx, prefixRagnarokHeight, "")
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return 0, nil
	}
	var ragnarok int64
	buf := store.Get([]byte(key))
	err := k.cdc.UnmarshalBinaryBare(buf, &ragnarok)
	if err != nil {
		return 0, dbError(ctx, "Unmarshal: ragnarok height", err)
	}
	return ragnarok, nil
}

// SetRagnarokBlockHeight save ragnarok block height to key value store, once it get set , it means ragnarok started
func (k KVStore) SetRagnarokBlockHeight(ctx cosmos.Context, height int64) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixRagnarokHeight, "")
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(height))
}

// GetRagnarokNth when ragnarok get triggered , THORNode will use a few rounds to refund all assets
// this method return which round it is in
func (k KVStore) GetRagnarokNth(ctx cosmos.Context) (int64, error) {
	key := k.GetKey(ctx, prefixRagnarokNth, "")
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return 0, nil
	}
	var ragnarok int64
	buf := store.Get([]byte(key))
	err := k.cdc.UnmarshalBinaryBare(buf, &ragnarok)
	if err != nil {
		return 0, dbError(ctx, "Unmarshal: ragnarok nth", err)
	}
	return ragnarok, nil
}

// SetRagnarokNth save the round number into key value store
func (k KVStore) SetRagnarokNth(ctx cosmos.Context, nth int64) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixRagnarokNth, "")
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(nth))
}

// GetRagnarokPending get ragnarok pending state from key value store
func (k KVStore) GetRagnarokPending(ctx cosmos.Context) (int64, error) {
	key := k.GetKey(ctx, prefixRagnarokPending, "")
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return 0, nil
	}
	var ragnarok int64
	buf := store.Get([]byte(key))
	err := k.cdc.UnmarshalBinaryBare(buf, &ragnarok)
	if err != nil {
		return 0, dbError(ctx, "Unmarshal: ragnarok pending", err)
	}
	return ragnarok, nil
}

// SetRagnarokPending save ragnarok pending to key value store
func (k KVStore) SetRagnarokPending(ctx cosmos.Context, pending int64) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixRagnarokPending, "")
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(pending))
}
