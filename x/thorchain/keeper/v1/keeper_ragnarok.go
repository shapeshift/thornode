package thorchain

import cosmos "gitlab.com/thorchain/thornode/common/cosmos"

func (k KVStore) RagnarokInProgress(ctx cosmos.Context) bool {
	height, err := k.GetRagnarokBlockHeight(ctx)
	if err != nil {
		ctx.Logger().Error(err.Error())
		return true
	}
	return height > 0
}

func (k KVStore) GetRagnarokBlockHeight(ctx cosmos.Context) (int64, error) {
	key := k.GetKey(ctx, prefixRagnarok, "")
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return 0, nil
	}
	var ragnarok int64
	buf := store.Get([]byte(key))
	err := k.cdc.UnmarshalBinaryBare(buf, &ragnarok)
	if err != nil {
		return 0, dbError(ctx, "Unmarshal: ragnarok", err)
	}
	return ragnarok, nil
}

func (k KVStore) SetRagnarokBlockHeight(ctx cosmos.Context, height int64) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixRagnarok, "")
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(height))
}
