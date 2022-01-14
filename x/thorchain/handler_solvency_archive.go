package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (h SolvencyHandler) validateV1(ctx cosmos.Context, msg MsgSolvency) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	if !isSignedByActiveNodeAccounts(ctx, h.mgr, msg.GetSigners()) {
		return cosmos.ErrUnauthorized(fmt.Sprintf("%+v are not authorized", msg.GetSigners()))
	}
	return nil
}

// handle V1 is the first implementation of MsgSolvency, the feature works like this
// 1. Bifrost report MsgSolvency to thornode , which is the balance of asgard wallet on each individual chain
// 2. once MsgSolvency reach consensus , then the network compare the wallet balance against wallet
//    if wallet has less fund than asgard vault , and the gap is more than 1% , then the chain
//    that is insolvent will be halt
// 3. When chain is halt , bifrost will not observe inbound , and will not sign outbound txs until the issue has been investigated , and enabled it again using mimir
func (h SolvencyHandler) handleV1(ctx cosmos.Context, msg MsgSolvency) (*cosmos.Result, error) {
	voter, err := h.mgr.Keeper().GetSolvencyVoter(ctx, msg.Id, msg.Chain)
	if err != nil {
		return &cosmos.Result{}, fmt.Errorf("fail to get solvency voter, err: %w", err)
	}
	observeSlashPoints := h.mgr.GetConstants().GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := h.mgr.GetConstants().GetInt64Value(constants.ObservationDelayFlexibility)
	h.mgr.Slasher().IncSlashPoints(ctx, observeSlashPoints, msg.Signer)
	if voter.Empty() {
		voter = NewSolvencyVoter(msg.Id, msg.Chain, msg.PubKey, msg.Coins, msg.Height, msg.Signer)
	} else {
		if !voter.Sign(msg.Signer) {
			ctx.Logger().Info("signer already signed MsgSolvency", "signer", msg.Signer.String(), "id", msg.Id)
			return &cosmos.Result{}, nil
		}
	}
	h.mgr.Keeper().SetSolvencyVoter(ctx, voter)
	active, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}
	if !voter.HasConsensus(active) {
		return &cosmos.Result{}, nil
	}

	// from this point , solvency reach consensus
	if voter.ConsensusBlockHeight > 0 {
		if (voter.ConsensusBlockHeight + observeFlex) >= common.BlockHeight(ctx) {
			h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, msg.Signer)
		}
		// solvency tx already processed
		return &cosmos.Result{}, nil
	}
	voter.ConsensusBlockHeight = common.BlockHeight(ctx)
	h.mgr.Keeper().SetSolvencyVoter(ctx, voter)
	// decrease the slash points
	h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, voter.GetSigners()...)
	vault, err := h.mgr.Keeper().GetVault(ctx, voter.PubKey)
	if err != nil {
		ctx.Logger().Error("fail to get vault", "error", err)
		return &cosmos.Result{}, fmt.Errorf("fail to get vault: %w", err)
	}
	const StopSolvencyCheckKey = `StopSolvencyCheck`
	stopSolvencyCheck, err := h.mgr.Keeper().GetMimir(ctx, StopSolvencyCheckKey)
	if err != nil {
		ctx.Logger().Error("fail to get mimir", "key", StopSolvencyCheckKey, "error", err)
	}
	if stopSolvencyCheck > 0 && stopSolvencyCheck < common.BlockHeight(ctx) {
		return &cosmos.Result{}, nil
	}

	if !h.insolvencyCheck(ctx, vault, voter.Coins, voter.Chain) {
		// here doesn't override HaltChain when the vault is solvent
		// in some case even the vault is solvent , the network might need to halt
		// Use mimir to enable it again
		return &cosmos.Result{}, nil
	}
	haltChainKey := fmt.Sprintf(`Halt%sChain`, voter.Chain)
	haltChain, err := h.mgr.Keeper().GetMimir(ctx, haltChainKey)
	if err != nil {
		ctx.Logger().Error("fail to get mimir", "error", err)
	}
	if haltChain > 0 && haltChain < common.BlockHeight(ctx) {
		// Trading already halt
		return &cosmos.Result{}, nil
	}
	h.mgr.Keeper().SetMimir(ctx, haltChainKey, common.BlockHeight(ctx))
	ctx.Logger().Info("chain is insolvent, halt until it is resolved", "chain", voter.Chain)
	return &cosmos.Result{}, nil
}

// handleCurrent is the logic to process MsgSolvency, the feature works like this
// 1. Bifrost report MsgSolvency to thornode , which is the balance of asgard wallet on each individual chain
// 2. once MsgSolvency reach consensus , then the network compare the wallet balance against wallet
//    if wallet has less fund than asgard vault , and the gap is more than 1% , then the chain
//    that is insolvent will be halt
// 3. When chain is halt , bifrost will not observe inbound , and will not sign outbound txs until the issue has been investigated , and enabled it again using mimir
func (h SolvencyHandler) handleV70(ctx cosmos.Context, msg MsgSolvency) (*cosmos.Result, error) {
	voter, err := h.mgr.Keeper().GetSolvencyVoter(ctx, msg.Id, msg.Chain)
	if err != nil {
		return &cosmos.Result{}, fmt.Errorf("fail to get solvency voter, err: %w", err)
	}
	observeSlashPoints := h.mgr.GetConstants().GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := h.mgr.GetConstants().GetInt64Value(constants.ObservationDelayFlexibility)
	h.mgr.Slasher().IncSlashPoints(ctx, observeSlashPoints, msg.Signer)
	if voter.Empty() {
		voter = NewSolvencyVoter(msg.Id, msg.Chain, msg.PubKey, msg.Coins, msg.Height, msg.Signer)
	} else {
		if !voter.Sign(msg.Signer) {
			ctx.Logger().Info("signer already signed MsgSolvency", "signer", msg.Signer.String(), "id", msg.Id)
			return &cosmos.Result{}, nil
		}
	}
	h.mgr.Keeper().SetSolvencyVoter(ctx, voter)
	active, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}
	if !voter.HasConsensus(active) {
		return &cosmos.Result{}, nil
	}

	// from this point , solvency reach consensus
	if voter.ConsensusBlockHeight > 0 {
		if (voter.ConsensusBlockHeight + observeFlex) >= common.BlockHeight(ctx) {
			h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, msg.Signer)
		}
		// solvency tx already processed
		return &cosmos.Result{}, nil
	}
	voter.ConsensusBlockHeight = common.BlockHeight(ctx)
	h.mgr.Keeper().SetSolvencyVoter(ctx, voter)
	// decrease the slash points
	h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, voter.GetSigners()...)
	vault, err := h.mgr.Keeper().GetVault(ctx, voter.PubKey)
	if err != nil {
		ctx.Logger().Error("fail to get vault", "error", err)
		return &cosmos.Result{}, fmt.Errorf("fail to get vault: %w", err)
	}
	const StopSolvencyCheckKey = `StopSolvencyCheck`
	stopSolvencyCheck, err := h.mgr.Keeper().GetMimir(ctx, StopSolvencyCheckKey)
	if err != nil {
		ctx.Logger().Error("fail to get mimir", "key", StopSolvencyCheckKey, "error", err)
	}
	if stopSolvencyCheck > 0 && stopSolvencyCheck < common.BlockHeight(ctx) {
		return &cosmos.Result{}, nil
	}
	// stop solvency checker per chain
	// this allows the network to stop solvency checker for ETH chain for example , while other chains like BNB/BTC chains
	// their solvency checker are still active
	stopSolvencyCheckChain, err := h.mgr.Keeper().GetMimir(ctx, fmt.Sprintf(StopSolvencyCheckKey+voter.Chain.String()))
	if err != nil {
		ctx.Logger().Error("fail to get mimir", "key", StopSolvencyCheckKey+voter.Chain.String(), "error", err)
	}
	if stopSolvencyCheckChain > 0 && stopSolvencyCheckChain < common.BlockHeight(ctx) {
		return &cosmos.Result{}, nil
	}
	if !h.insolvencyCheck(ctx, vault, voter.Coins, voter.Chain) {
		// here doesn't override HaltChain when the vault is solvent
		// in some case even the vault is solvent , the network might need to halt
		// Use mimir to enable it again
		return &cosmos.Result{}, nil
	}
	haltChainKey := fmt.Sprintf(`Halt%sChain`, voter.Chain)
	haltChain, err := h.mgr.Keeper().GetMimir(ctx, haltChainKey)
	if err != nil {
		ctx.Logger().Error("fail to get mimir", "error", err)
	}
	if haltChain > 0 && haltChain < common.BlockHeight(ctx) {
		// Trading already halt
		return &cosmos.Result{}, nil
	}
	h.mgr.Keeper().SetMimir(ctx, haltChainKey, common.BlockHeight(ctx))
	ctx.Logger().Info("chain is insolvent, halt until it is resolved", "chain", voter.Chain)
	return &cosmos.Result{}, nil
}

// insolvencyCheck compare the coins in vault against the coins report by solvency message
// insolvent usually means vault has more coins than wallet
// return true means the vault is insolvent , the network should halt , otherwise false
func (h SolvencyHandler) insolvencyCheck(ctx cosmos.Context, vault Vault, coins common.Coins, chain common.Chain) bool {
	adjustVault, err := h.excludePendingOutboundFromVault(ctx, vault)
	if err != nil {
		return false
	}
	permittedSolvencyGap, err := h.mgr.Keeper().GetMimir(ctx, constants.PermittedSolvencyGap.String())
	if err != nil || permittedSolvencyGap <= 0 {
		permittedSolvencyGap = h.mgr.GetConstants().GetInt64Value(constants.PermittedSolvencyGap)
	}
	// Use the coin in vault as baseline , wallet can have more coins than vault
	for _, c := range adjustVault.Coins {
		if !c.Asset.Chain.Equals(chain) {
			continue
		}
		// ETH.RUNE will be burned on the way in , so the wallet will not have any, thus exclude it from solvency check
		if c.Asset.IsRune() {
			continue
		}
		if c.IsEmpty() {
			continue
		}
		walletCoin := coins.GetCoin(c.Asset)
		if walletCoin.IsEmpty() {
			ctx.Logger().Info("asset exist in vault , but not in wallet, insolvent", "asset", c.Asset.String(), "amount", c.Amount.String())
			return true
		}
		if c.Asset.IsGasAsset() {
			gas, err := h.mgr.GasMgr().GetMaxGas(ctx, c.Asset.GetChain())
			if err != nil {
				ctx.Logger().Error("fail to get max gas", "error", err)
			} else if c.Amount.LTE(gas.Amount.MulUint64(2)) {
				// if the amount left in asgard vault is not enough for 2 * max gas, then skip it from solvency check
				continue
			}
		}

		if c.Amount.GT(walletCoin.Amount) {
			gap := c.Amount.Sub(walletCoin.Amount)
			permittedGap := walletCoin.Amount.MulUint64(uint64(permittedSolvencyGap)).QuoUint64(10000)
			if gap.GT(permittedGap) {
				ctx.Logger().Info("vault has more asset than wallet, insolvent", "asset", c.Asset.String(), "vault amount", c.Amount.String(), "wallet amount", walletCoin.Amount.String(), "gap", gap.String())
				return true
			}
		}
	}
	return false
}
