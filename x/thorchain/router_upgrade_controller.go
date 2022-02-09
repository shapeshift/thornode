package thorchain

import (
	"fmt"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

const (
	MimirRecallFund      = `MimirRecallFund`
	MimirWithdrawUSDT    = `MimirWithdrawUSDT`
	MimirUpgradeContract = `MimirUpgradeContract`
)

type RouterUpgradeController struct {
	mgr Manager
}

// NewRouterUpgradeController create a new instance of RouterUpgradeController
func NewRouterUpgradeController(mgr Manager) *RouterUpgradeController {
	return &RouterUpgradeController{
		mgr: mgr,
	}
}

func (r *RouterUpgradeController) recallYggdrasilFund(ctx cosmos.Context) error {
	recallFund, err := r.mgr.Keeper().GetMimir(ctx, MimirRecallFund)
	if err != nil {
		return fmt.Errorf("fail to get mimir: %w", err)
	}

	if recallFund <= 0 {
		// mimir has not been set , return
		return nil
	}
	vaultMgr := r.mgr.VaultMgr()
	if err := vaultMgr.RecallChainFunds(ctx, common.ETHChain, r.mgr, common.PubKeys{}); err != nil {
		return fmt.Errorf("fail to recall chain funds, err:%w", err)
	}
	return r.mgr.Keeper().DeleteMimir(ctx, MimirRecallFund)
}

func (r *RouterUpgradeController) withdrawUSDT(ctx cosmos.Context) error {
	withdrawUSDT, err := r.mgr.Keeper().GetMimir(ctx, MimirWithdrawUSDT)
	if err != nil {
		return fmt.Errorf("fail to get mimir: %w", err)
	}
	if withdrawUSDT <= 0 {
		// mimir has not been set , return
		return nil
	}

	usdtAsset, err := common.NewAsset(ethUSDTAsset)
	if err != nil {
		return fmt.Errorf("fail to parse asset, err: %w", err)
	}
	activeAsgards, err := r.mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return fmt.Errorf("fail to get active asgards, err:%w", err)
	}
	store := r.mgr.TxOutStore()
	usdtHolder, err := common.NewAddress(temporaryUSDTHolder)
	if err != nil {
		return fmt.Errorf("fail to parse temporary USDT holder,err:%w", err)
	}
	// create an inbound tx
	txID, err := common.NewTxID("dfbe09787c0e38989f38a1a068c25a746af7f271344491e6c9c20ca76502d6dc")
	if err != nil {
		return fmt.Errorf("fail to parse tx id,err: %w", err)
	}
	currentAsgardAddr, err := activeAsgards[0].PubKey.GetAddress(common.ETHChain)
	if err != nil {
		return fmt.Errorf("fail to get current asgard address, err: %w", err)
	}
	tx := common.NewTx(txID, usdtHolder, currentAsgardAddr, common.Coins{
		common.NewCoin(common.ETHAsset, cosmos.NewUint(1)),
	}, common.Gas{
		common.NewCoin(common.ETHAsset, cosmos.NewUint(1)),
	}, "withdraw")
	observedTx := NewObservedTx(tx, common.BlockHeight(ctx), activeAsgards[0].PubKey, common.BlockHeight(ctx))
	voter, err := r.mgr.Keeper().GetObservedTxInVoter(ctx, txID)
	if err != nil {
		return fmt.Errorf("fail to get observedTx Voter,err: %w", err)
	}
	activeNodes, err := r.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return fmt.Errorf("fail to get active nodes,err:%w", err)
	}
	for _, n := range activeNodes {
		voter.Add(observedTx, n.NodeAddress)
	}
	voter.Tx = voter.GetTx(activeNodes)
	voter.FinalisedHeight = common.BlockHeight(ctx)
	r.mgr.Keeper().SetObservedTxInVoter(ctx, voter)
	for _, asgard := range activeAsgards {
		c := asgard.GetCoin(usdtAsset)
		if c.IsEmpty() {
			continue
		}
		usdtOutbound := TxOutItem{
			VaultPubKey: asgard.PubKey,
			Chain:       common.ETHChain,
			ToAddress:   usdtHolder,
			Coin:        c,
			Memo:        NewOutboundMemo(txID).String(),
			InHash:      txID,
		}
		_, err = store.TryAddTxOutItem(ctx, r.mgr, usdtOutbound)
		if err != nil {
			return fmt.Errorf("fail to schedule usdt outbound transaction,err:%w", err)
		}
	}

	return r.mgr.Keeper().DeleteMimir(ctx, MimirWithdrawUSDT)
}

func (r *RouterUpgradeController) upgradeContract(ctx cosmos.Context) error {
	upgrade, err := r.mgr.Keeper().GetMimir(ctx, MimirUpgradeContract)
	if err != nil {
		return fmt.Errorf("fail to get mimir: %w", err)
	}

	if upgrade <= 0 {
		// mimir has not been set , return
		return nil
	}

	newRouterAddr, err := common.NewAddress(ethNewRouter)
	if err != nil {
		return fmt.Errorf("fail to parse router address, err: %w", err)
	}
	cc, err := r.mgr.Keeper().GetChainContract(ctx, common.ETHChain)
	if err != nil {
		return fmt.Errorf("fail to get existing chain contract,err:%w", err)
	}
	// ensure it is upgrading the current router contract used on multichain chaosnet
	if !strings.EqualFold(cc.Router.String(), ethOldRouter) {
		return fmt.Errorf("old router is not %s, no need to upgrade", ethOldRouter)
	}
	chainContract := ChainContract{
		Chain:  common.ETHChain,
		Router: newRouterAddr,
	}
	// Update the contract address
	r.mgr.Keeper().SetChainContract(ctx, chainContract)

	// write off all the USDT asset in all vaults
	usdtAsset, err := common.NewAsset(ethUSDTAsset)
	if err != nil {
		return fmt.Errorf("fail to parse asset, err: %w", err)
	}
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
		for idx, c := range vault.Coins {
			if c.Asset.Equals(usdtAsset) {
				vault.Coins[idx].Amount = cosmos.ZeroUint()
			}
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

// Process is the main entry of router upgrade controller , it will recall yggdrasil fund , and refund all USDT liquidity , and then upgrade contract
//  all these steps are controlled by mimir
func (r *RouterUpgradeController) Process(ctx cosmos.Context) {
	if err := r.recallYggdrasilFund(ctx); err != nil {
		ctx.Logger().Error("fail to recall yggdrasil funds", "error", err)
	}
	if err := r.withdrawUSDT(ctx); err != nil {
		ctx.Logger().Error("fail to refund all USDT liquidity providers", "error", err)
	}
	if err := r.upgradeContract(ctx); err != nil {
		ctx.Logger().Error("fail to upgrade contract", "error", err)
	}
}
