package thorchain

import (
	"context"
	"fmt"

	"github.com/armon/go-metrics"
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/telemetry"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// SolvencyHandler is to process MsgSolvency message from bifrost
// Bifrost constantly monitor the account balance , and report to THORNode
// If it detect that wallet is short of fund , much less than vault, the network should automatically halt trading
type SolvencyHandler struct {
	mgr Manager
}

// NewSolvencyHandler create a new instance of solvency handler
func NewSolvencyHandler(mgr Manager) SolvencyHandler {
	return SolvencyHandler{
		mgr: mgr,
	}
}

// Run is the main entry point to process MsgSolvency
func (h SolvencyHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgSolvency)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg solvency failed validation", "error", err)
		return nil, err
	}
	return h.handle(ctx, *msg)
}

func (h SolvencyHandler) validate(ctx cosmos.Context, msg MsgSolvency) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.70.0")) {
		return h.validateV70(ctx, msg)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h SolvencyHandler) validateV70(ctx cosmos.Context, msg MsgSolvency) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	m, err := NewMsgSolvency(msg.Chain, msg.PubKey, msg.Coins, msg.Height, msg.Signer)
	if err != nil {
		ctx.Logger().Error("fail to reconstruct msg solvency", "error", err)
		return err
	}
	if !m.Id.Equals(msg.Id) {
		return cosmos.ErrUnknownRequest("invalid solvency message")
	}
	if !isSignedByActiveNodeAccounts(ctx, h.mgr, msg.GetSigners()) {
		return cosmos.ErrUnauthorized(fmt.Sprintf("%+v are not authorized", msg.GetSigners()))
	}
	return nil
}

func (h SolvencyHandler) handle(ctx cosmos.Context, msg MsgSolvency) (*cosmos.Result, error) {
	ctx.Logger().Debug("handle Solvency request", "id", msg.Id.String(), "signer", msg.Signer.String())
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.79.0")) {
		return h.handleV79(ctx, msg)
	} else if version.GTE(semver.MustParse("0.70.0")) {
		return h.handleV70(ctx, msg)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg)
	}
	ctx.Logger().Error(errInvalidVersion.Error())
	return nil, errBadVersion
}

// handleCurrent is the logic to process MsgSolvency, the feature works like this
// 1. Bifrost report MsgSolvency to thornode , which is the balance of asgard wallet on each individual chain
// 2. once MsgSolvency reach consensus , then the network compare the wallet balance against wallet
//    if wallet has less fund than asgard vault , and the gap is more than 1% , then the chain
//    that is insolvent will be halt
// 3. When chain is halt , bifrost will not observe inbound , and will not sign outbound txs until the issue has been investigated , and enabled it again using mimir
func (h SolvencyHandler) handleV79(ctx cosmos.Context, msg MsgSolvency) (*cosmos.Result, error) {
	voter, err := h.mgr.Keeper().GetSolvencyVoter(ctx, msg.Id, msg.Chain)
	if err != nil {
		return &cosmos.Result{}, fmt.Errorf("fail to get solvency voter, err: %w", err)
	}
	observeSlashPoints := h.mgr.GetConstants().GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := h.mgr.GetConstants().GetInt64Value(constants.ObservationDelayFlexibility)

	slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
		telemetry.NewLabel("reason", "failed_observe_solvency"),
		telemetry.NewLabel("chain", string(msg.Chain)),
	}))
	h.mgr.Slasher().IncSlashPoints(slashCtx, observeSlashPoints, msg.Signer)

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
			h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, msg.Signer)
		}
		// solvency tx already processed
		return &cosmos.Result{}, nil
	}
	voter.ConsensusBlockHeight = common.BlockHeight(ctx)
	h.mgr.Keeper().SetSolvencyVoter(ctx, voter)
	// decrease the slash points
	h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, voter.GetSigners()...)
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
	haltChainKey := fmt.Sprintf(`Halt%sChain`, voter.Chain)
	haltChain, err := h.mgr.Keeper().GetMimir(ctx, haltChainKey)
	if err != nil {
		ctx.Logger().Error("fail to get mimir", "error", err)
	}

	if !h.insolvencyCheckV79(ctx, vault, voter.Coins, voter.Chain) {
		// here doesn't override HaltChain when the vault is solvent
		// in some case even the vault is solvent , the network might need to halt by admin mimir
		// admin mimir halt chain usually set the value to 1
		if haltChain <= 1 {
			return &cosmos.Result{}, nil
		}
		// if the chain was halted by previous solvency checker, auto unhalt it
		ctx.Logger().Info("auto un-halt", "chain", voter.Chain, "previous halt height", haltChain, "current block height", common.BlockHeight(ctx))
		h.mgr.Keeper().SetMimir(ctx, haltChainKey, 0)
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
func (h SolvencyHandler) insolvencyCheckV79(ctx cosmos.Context, vault Vault, coins common.Coins, chain common.Chain) bool {
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
			} else if c.Amount.LTE(gas.Amount.MulUint64(10)) {
				// if the amount left in asgard vault is not enough for 10 * max gas, then skip it from solvency check
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

func (h SolvencyHandler) excludePendingOutboundFromVault(ctx cosmos.Context, vault Vault) (Vault, error) {
	// go back SigningTransactionPeriod blocks to see whether there are outstanding tx, the vault need to send out
	// if there is , deduct it from their balance
	signingPeriod := h.mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
	startHeight := common.BlockHeight(ctx) - signingPeriod
	if startHeight < 1 {
		startHeight = 1
	}
	for i := startHeight; i < common.BlockHeight(ctx); i++ {
		blockOut, err := h.mgr.Keeper().GetTxOut(ctx, i)
		if err != nil {
			ctx.Logger().Error("fail to get block tx out", "error", err)
			return vault, fmt.Errorf("fail to get block tx out, err: %w", err)
		}
		vault = h.deductVaultBlockPendingOutbound(vault, blockOut)
	}
	return vault, nil
}

func (h SolvencyHandler) deductVaultBlockPendingOutbound(vault Vault, block *TxOut) Vault {
	for _, txOutItem := range block.TxArray {
		if !txOutItem.VaultPubKey.Equals(vault.PubKey) {
			continue
		}
		// only still outstanding txout will be considered
		if !txOutItem.OutHash.IsEmpty() {
			continue
		}
		// deduct the gas asset from the vault as well
		var gasCoin common.Coin
		if !txOutItem.MaxGas.IsEmpty() {
			gasCoin = txOutItem.MaxGas.ToCoins().GetCoin(txOutItem.Chain.GetGasAsset())
		}
		for i, yggCoin := range vault.Coins {
			if yggCoin.Asset.Equals(txOutItem.Coin.Asset) {
				vault.Coins[i].Amount = common.SafeSub(vault.Coins[i].Amount, txOutItem.Coin.Amount)
			}
			if yggCoin.Asset.Equals(gasCoin.Asset) {
				vault.Coins[i].Amount = common.SafeSub(vault.Coins[i].Amount, gasCoin.Amount)
			}
		}
	}
	return vault
}
