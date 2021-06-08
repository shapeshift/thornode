package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// ObservedTxOutHandler process MsgObservedTxOut messages
type ObservedTxOutHandler struct {
	mgr Manager
}

// NewObservedTxOutHandler create a new instance of ObservedTxOutHandler
func NewObservedTxOutHandler(mgr Manager) ObservedTxOutHandler {
	return ObservedTxOutHandler{
		mgr: mgr,
	}
}

// Run is the main entry point for ObservedTxOutHandler
func (h ObservedTxOutHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(*MsgObservedTxOut)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg, version); err != nil {
		ctx.Logger().Error("MsgObserveTxOut failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg, version, constAccessor)
	if err != nil {
		ctx.Logger().Error("fail to handle MsgObserveTxOut", "error", err)
	}
	return result, err
}

func (h ObservedTxOutHandler) validate(ctx cosmos.Context, msg MsgObservedTxOut, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errInvalidVersion
}

func (h ObservedTxOutHandler) validateV1(ctx cosmos.Context, msg MsgObservedTxOut) error {
	return h.validateCurrent(ctx, msg)
}

func (h ObservedTxOutHandler) validateCurrent(ctx cosmos.Context, msg MsgObservedTxOut) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	if !isSignedByActiveNodeAccounts(ctx, h.mgr, msg.GetSigners()) {
		return cosmos.ErrUnauthorized(fmt.Sprintf("%+v are not authorized", msg.GetSigners()))
	}

	return nil
}

func (h ObservedTxOutHandler) handle(ctx cosmos.Context, msg MsgObservedTxOut, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	if version.GTE(semver.MustParse("0.46.0")) {
		return h.handleV46(ctx, version, msg, constAccessor)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, version, msg, constAccessor)
	}
	return nil, errBadVersion
}

func (h ObservedTxOutHandler) preflightV1(ctx cosmos.Context, voter ObservedTxVoter, nas NodeAccounts, tx ObservedTx, signer cosmos.AccAddress, version semver.Version, constAccessor constants.ConstantValues) (ObservedTxVoter, bool) {
	observeSlashPoints := constAccessor.GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := constAccessor.GetInt64Value(constants.ObservationDelayFlexibility)
	ok := false
	h.mgr.Slasher().IncSlashPoints(ctx, observeSlashPoints, signer)
	if err := h.mgr.Keeper().SetLastObserveHeight(ctx, tx.Tx.Chain, signer, tx.BlockHeight); err != nil {
		ctx.Logger().Error("fail to save last observe height", "error", err, "signer", signer, "chain", tx.Tx.Chain)
	}
	if !voter.Add(tx, signer) {
		// when the signer already sign it
		return voter, ok
	}
	if voter.HasFinalised(nas) {
		if voter.FinalisedHeight == 0 {
			ok = true
			voter.FinalisedHeight = common.BlockHeight(ctx)
			voter.Tx = voter.GetTx(nas)
			// tx has consensus now, so decrease the slashing point for all the signers whom voted for it
			h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, voter.Tx.GetSigners()...)

		} else {
			// event the tx had been processed , given the signer just a bit late , so we still take away their slash points
			if common.BlockHeight(ctx) <= (voter.FinalisedHeight+observeFlex) && voter.Tx.Equals(tx) {
				h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, signer)
			}
		}
	}
	h.mgr.Keeper().SetObservedTxOutVoter(ctx, voter)

	// Check to see if we have enough identical observations to process the transaction
	return voter, ok
}

// Handle a message to observe outbound tx
func (h ObservedTxOutHandler) handleV1(ctx cosmos.Context, version semver.Version, msg MsgObservedTxOut, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	activeNodeAccounts, err := h.mgr.Keeper().ListActiveNodeAccounts(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}

	handler := NewInternalHandler(h.mgr)

	for _, tx := range msg.Txs {
		// check we are sending from a valid vault
		if !h.mgr.Keeper().VaultExists(ctx, tx.ObservedPubKey) {
			ctx.Logger().Info("Not valid Observed Pubkey", tx.ObservedPubKey)
			continue
		}
		if tx.KeysignMs > 0 {
			keysignMetric, err := h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get keysing metric", "error", err)
			} else {
				keysignMetric.AddNodeTssTime(msg.Signer, tx.KeysignMs)
				h.mgr.Keeper().SetTssKeysignMetric(ctx, keysignMetric)
			}
		}
		voter, err := h.mgr.Keeper().GetObservedTxOutVoter(ctx, tx.Tx.ID)
		if err != nil {
			ctx.Logger().Error("fail to get tx out voter", "error", err)
			continue
		}

		// check whether the tx has consensus
		voter, ok := h.preflightV1(ctx, voter, activeNodeAccounts, tx, msg.Signer, version, constAccessor)
		if !ok {
			if voter.FinalisedHeight == common.BlockHeight(ctx) {
				// we've already process the transaction, but we should still
				// update the observing addresses
				h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, msg.GetSigners())
			}
			continue
		}
		ctx.Logger().Info("handleMsgObservedTxOut request", "Tx:", tx.String())

		// if memo isn't valid or its an inbound memo, and its funds moving
		// from a yggdrasil vault, slash the node
		memo, _ := ParseMemo(tx.Tx.Memo)
		if memo.IsEmpty() || memo.IsInbound() {
			vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
			if err != nil {
				ctx.Logger().Error("fail to get vault", "error", err)
				continue
			}
			toSlash := tx.Tx.Coins.Adds(tx.Tx.Gas.ToCoins())
			if err := h.mgr.Slasher().SlashVault(ctx, tx.ObservedPubKey, toSlash, h.mgr); err != nil {
				ctx.Logger().Error("fail to slash account for sending extra fund", "error", err)
			}
			vault.SubFunds(toSlash)
			if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				ctx.Logger().Error("fail to save vault", "error", err)
			}

			continue
		}

		txOut := voter.GetTx(activeNodeAccounts) // get consensus tx, in case our for loop is incorrect
		txOut.Tx.Memo = tx.Tx.Memo
		m, err := processOneTxIn(ctx, h.mgr.Keeper(), txOut, msg.Signer)
		if err != nil || tx.Tx.Chain.IsEmpty() {
			ctx.Logger().Error("fail to process txOut",
				"error", err,
				"tx", tx.Tx.String())
			continue
		}

		// Apply Gas fees
		if err := addGasFees(ctx, h.mgr, tx); err != nil {
			ctx.Logger().Error("fail to add gas fee", "error", err)
			continue
		}

		// If sending from one of our vaults, decrement coins
		vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get vault", "error", err)
			continue
		}
		vault.SubFunds(tx.Tx.Coins)
		vault.OutboundTxCount++
		if vault.IsAsgard() && memo.IsType(TxMigrate) {
			// only remove the block height that had been specified in the memo
			vault.RemovePendingTxBlockHeights(memo.GetBlockHeight())
		}

		if !vault.HasFunds() && vault.Status == RetiringVault {
			// we have successfully removed all funds from a retiring vault,
			// mark it as inactive
			vault.Status = InactiveVault
		}
		if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
			continue
		}

		// add addresses to observing addresses. This is used to detect
		// active/inactive observing node accounts
		h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, txOut.GetSigners())

		// emit tss keysign metrics
		if tx.KeysignMs > 0 {
			keysignMetric, err := h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get tss keysign metric", "error", err, "hash", tx.Tx.ID)
			} else {
				evt := NewEventTssKeysignMetric(keysignMetric.TxID, keysignMetric.GetMedianTime())
				if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
					ctx.Logger().Error("fail to emit tss metric event", "error", err)
				}
			}
		}
		_, err = handler(ctx, m)
		if err != nil {
			ctx.Logger().Error("handler failed:", "error", err)
			continue
		}
		voter.SetDone()
		h.mgr.Keeper().SetObservedTxOutVoter(ctx, voter)
	}
	return &cosmos.Result{}, nil

}

// Handle a message to observe outbound tx
func (h ObservedTxOutHandler) handleV46(ctx cosmos.Context, version semver.Version, msg MsgObservedTxOut, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	return h.handleCurrent(ctx, version, msg, constAccessor)
}

// Handle a message to observe outbound tx
func (h ObservedTxOutHandler) handleCurrent(ctx cosmos.Context, version semver.Version, msg MsgObservedTxOut, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	activeNodeAccounts, err := h.mgr.Keeper().ListActiveNodeAccounts(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}

	handler := NewInternalHandler(h.mgr)

	for _, tx := range msg.Txs {
		// check we are sending from a valid vault
		if !h.mgr.Keeper().VaultExists(ctx, tx.ObservedPubKey) {
			ctx.Logger().Info("Not valid Observed Pubkey", tx.ObservedPubKey)
			continue
		}
		if tx.KeysignMs > 0 {
			keysignMetric, err := h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get keysing metric", "error", err)
			} else {
				keysignMetric.AddNodeTssTime(msg.Signer, tx.KeysignMs)
				h.mgr.Keeper().SetTssKeysignMetric(ctx, keysignMetric)
			}
		}
		voter, err := h.mgr.Keeper().GetObservedTxOutVoter(ctx, tx.Tx.ID)
		if err != nil {
			ctx.Logger().Error("fail to get tx out voter", "error", err)
			continue
		}

		// check whether the tx has consensus
		voter, ok := h.preflightV1(ctx, voter, activeNodeAccounts, tx, msg.Signer, version, constAccessor)
		if !ok {
			if voter.FinalisedHeight == common.BlockHeight(ctx) {
				// we've already process the transaction, but we should still
				// update the observing addresses
				h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, msg.GetSigners())
			}
			continue
		}
		ctx.Logger().Info("handleMsgObservedTxOut request", "Tx:", tx.String())

		// if memo isn't valid or its an inbound memo, and its funds moving
		// from a yggdrasil vault, slash the node
		memo, _ := ParseMemo(tx.Tx.Memo)
		if memo.IsEmpty() || memo.IsInbound() {
			vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
			if err != nil {
				ctx.Logger().Error("fail to get vault", "error", err)
				continue
			}
			toSlash := tx.Tx.Coins.Adds(tx.Tx.Gas.ToCoins())
			if err := h.mgr.Slasher().SlashVault(ctx, tx.ObservedPubKey, toSlash, h.mgr); err != nil {
				ctx.Logger().Error("fail to slash account for sending extra fund", "error", err)
			}
			vault.SubFunds(toSlash)
			if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				ctx.Logger().Error("fail to save vault", "error", err)
			}

			continue
		}

		txOut := voter.GetTx(activeNodeAccounts) // get consensus tx, in case our for loop is incorrect
		txOut.Tx.Memo = tx.Tx.Memo
		m, err := processOneTxInV46(ctx, h.mgr.Keeper(), txOut, msg.Signer)
		if err != nil || tx.Tx.Chain.IsEmpty() {
			ctx.Logger().Error("fail to process txOut",
				"error", err,
				"tx", tx.Tx.String())
			continue
		}

		// Apply Gas fees
		if err := addGasFees(ctx, h.mgr, tx); err != nil {
			ctx.Logger().Error("fail to add gas fee", "error", err)
			continue
		}

		// If sending from one of our vaults, decrement coins
		vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get vault", "error", err)
			continue
		}
		vault.SubFunds(tx.Tx.Coins)
		vault.OutboundTxCount++
		if vault.IsAsgard() && memo.IsType(TxMigrate) {
			// only remove the block height that had been specified in the memo
			vault.RemovePendingTxBlockHeights(memo.GetBlockHeight())
		}

		if !vault.HasFunds() && vault.Status == RetiringVault {
			// we have successfully removed all funds from a retiring vault,
			// mark it as inactive
			vault.Status = InactiveVault
		}
		if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
			continue
		}

		// add addresses to observing addresses. This is used to detect
		// active/inactive observing node accounts
		h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, txOut.GetSigners())

		// emit tss keysign metrics
		if tx.KeysignMs > 0 {
			keysignMetric, err := h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get tss keysign metric", "error", err, "hash", tx.Tx.ID)
			} else {
				evt := NewEventTssKeysignMetric(keysignMetric.TxID, keysignMetric.GetMedianTime())
				if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
					ctx.Logger().Error("fail to emit tss metric event", "error", err)
				}
			}
		}
		_, err = handler(ctx, m)
		if err != nil {
			ctx.Logger().Error("handler failed:", "error", err)
			continue
		}
		voter.SetDone()
		h.mgr.Keeper().SetObservedTxOutVoter(ctx, voter)
	}
	return &cosmos.Result{}, nil
}
