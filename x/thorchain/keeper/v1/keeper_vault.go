package thorchain

import (
	"errors"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	kvTypes "gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
)

// GetVaultIterator only iterate vault pools
func (k KVStoreV1) GetVaultIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixVaultPool))
}

// SetVault save the Vault object to store
func (k KVStoreV1) SetVault(ctx cosmos.Context, vault Vault) error {
	key := k.GetKey(ctx, prefixVaultPool, vault.PubKey.String())
	store := ctx.KVStore(k.storeKey)
	buf, err := k.cdc.MarshalBinaryBare(vault)
	if err != nil {
		return dbError(ctx, "fail to marshal vault to binary", err)
	}
	if vault.IsAsgard() {
		if err := k.addAsgardIndex(ctx, vault.PubKey); err != nil {
			return err
		}
	}
	store.Set([]byte(key), buf)
	return nil
}

// VaultExists check whether the given pubkey is associated with a vault vault
func (k KVStoreV1) VaultExists(ctx cosmos.Context, pk common.PubKey) bool {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixVaultPool, pk.String())
	return store.Has([]byte(key))
}

// GetVault get Vault with the given pubkey from data store
func (k KVStoreV1) GetVault(ctx cosmos.Context, pk common.PubKey) (Vault, error) {
	var vault Vault
	key := k.GetKey(ctx, prefixVaultPool, pk.String())
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		vault.BlockHeight = ctx.BlockHeight()
		vault.PubKey = pk
		return vault, fmt.Errorf("vault with pubkey(%s) doesn't exist: %w", pk, kvTypes.ErrVaultNotFound)
	}
	buf := store.Get([]byte(key))
	if err := k.cdc.UnmarshalBinaryBare(buf, &vault); err != nil {
		return vault, dbError(ctx, "fail to unmarshal vault", err)
	}
	if vault.PubKey.IsEmpty() {
		vault.PubKey = pk
	}
	return vault, nil
}

// HasValidVaultPools check the datastore to see whether we have a valid vault pool
func (k KVStoreV1) HasValidVaultPools(ctx cosmos.Context) (bool, error) {
	iterator := k.GetVaultIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var vault Vault
		if err := k.cdc.UnmarshalBinaryBare(iterator.Value(), &vault); err != nil {
			return false, dbError(ctx, "fail to unmarshal vault", err)
		}
		if vault.HasFunds() {
			return true, nil
		}
	}
	return false, nil
}

func (k KVStoreV1) getAsgardIndex(ctx cosmos.Context) (common.PubKeys, error) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixVaultAsgardIndex, "")
	if !store.Has([]byte(key)) {
		return nil, nil
	}
	buf := store.Get([]byte(key))
	var pks common.PubKeys
	if err := k.cdc.UnmarshalBinaryBare(buf, &pks); err != nil {
		return nil, dbError(ctx, "fail to unmarshal asgard index", err)
	}
	return pks, nil
}

func (k KVStoreV1) addAsgardIndex(ctx cosmos.Context, pubkey common.PubKey) error {
	pks, err := k.getAsgardIndex(ctx)
	if err != nil {
		return err
	}
	for _, pk := range pks {
		if pk.Equals(pubkey) {
			return nil
		}
	}
	pks = append(pks, pubkey)
	key := k.GetKey(ctx, prefixVaultAsgardIndex, "")
	store := ctx.KVStore(k.storeKey)
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(pks))
	return nil
}

func (k KVStoreV1) GetAsgardVaults(ctx cosmos.Context) (Vaults, error) {
	pks, err := k.getAsgardIndex(ctx)
	if err != nil {
		return nil, err
	}

	var asgards Vaults
	for _, pk := range pks {
		vault, err := k.GetVault(ctx, pk)
		if err != nil {
			return nil, err
		}
		if vault.IsAsgard() {
			asgards = append(asgards, vault)
		}
	}

	return asgards, nil
}

func (k KVStoreV1) GetAsgardVaultsByStatus(ctx cosmos.Context, status VaultStatus) (Vaults, error) {
	all, err := k.GetAsgardVaults(ctx)
	if err != nil {
		return nil, err
	}

	var asgards Vaults
	for _, vault := range all {
		if vault.Status == status {
			asgards = append(asgards, vault)
		}
	}

	return asgards, nil
}

func (k KVStoreV1) DeleteVault(ctx cosmos.Context, pubkey common.PubKey) error {
	vault, err := k.GetVault(ctx, pubkey)
	if err != nil {
		if errors.Is(err, kvTypes.ErrVaultNotFound) {
			return nil
		}
		return err
	}

	if vault.HasFunds() {
		return errors.New("unable to delete vault: it still contains funds")
	}

	if vault.IsAsgard() {
		pks, err := k.getAsgardIndex(ctx)
		if err != nil {
			return err
		}

		newPks := common.PubKeys{}
		for _, pk := range pks {
			if !pk.Equals(pubkey) {
				newPks = append(newPks, pk)
			}
		}

		key := k.GetKey(ctx, prefixVaultAsgardIndex, "")
		store := ctx.KVStore(k.storeKey)
		if len(newPks) > 0 {
			store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(newPks))
		} else {
			store.Delete([]byte(key))
		}
	}
	// delete the actual vault
	key := k.GetKey(ctx, prefixVaultPool, vault.PubKey.String())
	store := ctx.KVStore(k.storeKey)
	store.Delete([]byte(key))
	return nil
}
