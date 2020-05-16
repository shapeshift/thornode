package thorchain

import (
	"github.com/blang/semver"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// VersionedVaultManager
type VersionedVaultManager interface {
	GetVaultManager(ctx cosmos.Context, keeper Keeper, version semver.Version) (VaultManager, error)
}

// VaultManager interface define the contract of Vault Manager
type VaultManager interface {
	TriggerKeygen(ctx cosmos.Context, nas NodeAccounts) error
	RotateVault(ctx cosmos.Context, vault Vault) error
	EndBlock(ctx cosmos.Context, version semver.Version, constAccessor constants.ConstantValues) error
}

// VersionedVaultMgr is an implementation of versioned Vault Manager
type VersionedVaultMgr struct {
	vaultMgrV1            *VaultMgr
	versionedTxOutStore   VersionedTxOutStore
	versionedEventManager VersionedEventManager
}

// NewVersionedVaultMgr create a new instance of VersionedVaultMgr
func NewVersionedVaultMgr(versionedTxOutStore VersionedTxOutStore, versionedEventManager VersionedEventManager) *VersionedVaultMgr {
	return &VersionedVaultMgr{
		versionedTxOutStore: versionedTxOutStore,
	}
}

// GetVaultManager retrieve a VaultManager that is compatible with the given version
func (v *VersionedVaultMgr) GetVaultManager(ctx cosmos.Context, keeper Keeper, version semver.Version) (VaultManager, error) {
	if version.GTE(semver.MustParse("0.1.0")) {
		if v.vaultMgrV1 == nil {
			v.vaultMgrV1 = NewVaultMgr(keeper, v.versionedTxOutStore, v.versionedEventManager)
		}
		return v.vaultMgrV1, nil
	}
	return nil, errInvalidVersion
}
