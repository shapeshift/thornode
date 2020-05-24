package thorchain

import (
	"fmt"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// KeeperVaultData func to access Vault in key value store
type KeeperVaultData interface {
	GetVaultData(ctx cosmos.Context) (VaultData, error)
	SetVaultData(ctx cosmos.Context, data VaultData) error
}

// GetVaultData retrieve vault data from key value store
func (k KVStoreV1) GetVaultData(ctx cosmos.Context) (VaultData, error) {
	data := NewVaultData()
	key := k.GetKey(ctx, prefixVaultData, "")
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return data, nil
	}
	buf := store.Get([]byte(key))
	if err := k.cdc.UnmarshalBinaryBare(buf, &data); err != nil {
		return data, dbError(ctx, "fail to unmarshal vault data", err)
	}

	return data, nil
}

// SetVaultData save the given vault data to key value store, it will overwrite existing vault
func (k KVStoreV1) SetVaultData(ctx cosmos.Context, data VaultData) error {
	key := k.GetKey(ctx, prefixVaultData, "")
	store := ctx.KVStore(k.storeKey)
	buf, err := k.cdc.MarshalBinaryBare(data)
	if err != nil {
		return fmt.Errorf("fail to marshal vault data: %w", err)
	}
	store.Set([]byte(key), buf)
	return nil
}
