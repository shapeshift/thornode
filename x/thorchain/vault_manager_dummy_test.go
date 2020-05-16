package thorchain

import (
	"github.com/blang/semver"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// VersionedVaultMgrDummy used for test purpose
type VersionedVaultMgrDummy struct {
	versionedTxOutStore VersionedTxOutStore
	vaultMgrDummy       *VaultMgrDummy
}

func NewVersionedVaultMgrDummy(versionedTxOutStore VersionedTxOutStore) *VersionedVaultMgrDummy {
	return &VersionedVaultMgrDummy{
		versionedTxOutStore: versionedTxOutStore,
	}
}

func (v *VersionedVaultMgrDummy) GetVaultManager(ctx cosmos.Context, keeper Keeper, version semver.Version) (VaultManager, error) {
	if v.vaultMgrDummy == nil {
		v.vaultMgrDummy = NewVaultMgrDummy()
	}
	return v.vaultMgrDummy, nil
}

type VaultMgrDummy struct {
	nas   NodeAccounts
	vault Vault
}

func NewVaultMgrDummy() *VaultMgrDummy {
	return &VaultMgrDummy{}
}

func (vm *VaultMgrDummy) EndBlock(ctx cosmos.Context, version semver.Version, constAccessor constants.ConstantValues) error {
	return nil
}

func (vm *VaultMgrDummy) TriggerKeygen(_ cosmos.Context, nas NodeAccounts) error {
	vm.nas = nas
	return nil
}

func (vm *VaultMgrDummy) RotateVault(ctx cosmos.Context, vault Vault) error {
	vm.vault = vault
	return nil
}
