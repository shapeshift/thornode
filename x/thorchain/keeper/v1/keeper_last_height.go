package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

func (k KVStore) SetLastSignedHeight(ctx cosmos.Context, height int64) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixLastSignedHeight, "")
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(height))
}

func (k KVStore) GetLastSignedHeight(ctx cosmos.Context) (int64, error) {
	var height int64
	key := k.GetKey(ctx, prefixLastSignedHeight, "")
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return 0, nil
	}
	buf := store.Get([]byte(key))
	if err := k.cdc.UnmarshalBinaryBare(buf, &height); err != nil {
		return 0, dbError(ctx, "Unmarshal: last heights", err)
	}
	return height, nil
}

func (k KVStore) SetLastChainHeight(ctx cosmos.Context, chain common.Chain, height int64) error {
	lastHeight, err := k.GetLastChainHeight(ctx, chain)
	if err != nil {
		return err
	}
	if lastHeight > height {
		err := fmt.Errorf("last block height :%d is larger than %d , block height can't go backward ", lastHeight, height)
		return dbError(ctx, "", err)
	}
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixLastChainHeight, chain.String())
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(height))
	return nil
}

func (k KVStore) GetLastChainHeight(ctx cosmos.Context, chain common.Chain) (int64, error) {
	var height int64
	key := k.GetKey(ctx, prefixLastChainHeight, chain.String())
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return 0, nil
	}
	buf := store.Get([]byte(key))
	if err := k.cdc.UnmarshalBinaryBare(buf, &height); err != nil {
		return height, dbError(ctx, "Unmarshal: last heights", err)
	}
	return height, nil
}
