package thorchain

import (
	"fmt"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type RouterUpgradeControllerV3 struct {
	mgr Manager
}

// NewRouterUpgradeControllerV3 create a new instance of RouterUpgradeControllerV3
func NewRouterUpgradeControllerV3(mgr Manager) *RouterUpgradeControllerV3 {
	return &RouterUpgradeControllerV3{
		mgr: mgr,
	}
}

func (r *RouterUpgradeControllerV3) recallYggdrasilFund(ctx cosmos.Context) error {
	recallFund, err := r.mgr.Keeper().GetMimir(ctx, MimirRecallFund)
	if err != nil {
		return fmt.Errorf("fail to get mimir: %w", err)
	}

	if recallFund <= 0 {
		// mimir has not been set , return
		return nil
	}
	networkMgr := r.mgr.NetworkMgr()
	if err := networkMgr.RecallChainFunds(ctx, common.ETHChain, r.mgr, common.PubKeys{}); err != nil {
		return fmt.Errorf("fail to recall chain funds, err:%w", err)
	}
	return r.mgr.Keeper().DeleteMimir(ctx, MimirRecallFund)
}

func (r *RouterUpgradeControllerV3) upgradeContract(ctx cosmos.Context) error {
	upgrade, err := r.mgr.Keeper().GetMimir(ctx, MimirUpgradeContract)
	if err != nil {
		return fmt.Errorf("fail to get mimir: %w", err)
	}

	if upgrade <= 0 {
		// mimir has not been set , return
		return nil
	}

	ethRouterV3Addr, err := common.NewAddress(ethRouterV3)
	if err != nil {
		return fmt.Errorf("fail to parse router address, err: %w", err)
	}
	cc, err := r.mgr.Keeper().GetChainContract(ctx, common.ETHChain)
	if err != nil {
		return fmt.Errorf("fail to get existing chain contract,err:%w", err)
	}
	// ensure it is upgrading the current router contract used on multichain chaosnet
	if !strings.EqualFold(cc.Router.String(), ethNewRouter) {
		return fmt.Errorf("old router is not %s, no need to upgrade", ethNewRouter)
	}
	chainContract := ChainContract{
		Chain:  common.ETHChain,
		Router: ethRouterV3Addr,
	}
	// Update the contract address
	r.mgr.Keeper().SetChainContract(ctx, chainContract)

	vaultIter := r.mgr.Keeper().GetVaultIterator(ctx)
	defer vaultIter.Close()
	for ; vaultIter.Valid(); vaultIter.Next() {
		var vault Vault
		if err := r.mgr.Keeper().Cdc().Unmarshal(vaultIter.Value(), &vault); err != nil {
			ctx.Logger().Error("fail to unmarshal vault", "error", err)
			continue
		}
		// vault is empty , ignore
		if vault.IsEmpty() {
			continue
		}
		if vault.IsType(YggdrasilVault) {
			// update yggdrasil vault to use new router contract
			vault.UpdateContract(chainContract)
		}
		if err := r.mgr.Keeper().SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
		}

	}

	return r.mgr.Keeper().DeleteMimir(ctx, MimirUpgradeContract)
}

// Process is the main entry of router upgrade controller , it will recall yggdrasil fund , and then upgrade contract
//  all these steps are controlled by mimir
func (r *RouterUpgradeControllerV3) Process(ctx cosmos.Context) {
	if err := r.recallYggdrasilFund(ctx); err != nil {
		ctx.Logger().Error("fail to recall yggdrasil funds", "error", err)
	}
	if err := r.upgradeContract(ctx); err != nil {
		ctx.Logger().Error("fail to upgrade contract", "error", err)
	}
}
