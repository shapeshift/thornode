package keeperv1

import (
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

func (k KVStore) RagnarokInProgress(ctx cosmos.Context) bool {
	height, err := k.GetRagnarokBlockHeight(ctx)
	if err != nil {
		ctx.Logger().Error(err.Error())
		return true
	}
	return height > 0
}

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

func (k KVStore) SetRagnarokBlockHeight(ctx cosmos.Context, height int64) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixRagnarokHeight, "")
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(height))
}

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

func (k KVStore) SetRagnarokNth(ctx cosmos.Context, nth int64) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixRagnarokNth, "")
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(nth))
}

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

func (k KVStore) SetRagnarokPending(ctx cosmos.Context, pending int64) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixRagnarokPending, "")
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(pending))
}
