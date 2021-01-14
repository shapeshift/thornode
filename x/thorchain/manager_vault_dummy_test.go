package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type VaultMgrDummy struct {
	nas   NodeAccounts
	vault Vault
}

func NewVaultMgrDummy() *VaultMgrDummy {
	return &VaultMgrDummy{}
}

func (vm *VaultMgrDummy) EndBlock(ctx cosmos.Context, mgr Manager, constAccessor constants.ConstantValues) error {
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

func (vm *VaultMgrDummy) UpdateNetwork(ctx cosmos.Context, constAccessor constants.ConstantValues, gasManager GasManager, eventMgr EventManager) error {
	return nil
}

func (vm *VaultMgrDummy) RecallChainFunds(ctx cosmos.Context, chain common.Chain, mgr Manager, excludeNodeKeys common.PubKeys) error {
	return nil
}
