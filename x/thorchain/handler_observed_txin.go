package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// ObservedTxInHandler to handle MsgObservedTxIn
type ObservedTxInHandler struct {
	keeper Keeper
	mgr    Manager
}

// NewObservedTxInHandler create a new instance of ObservedTxInHandler
func NewObservedTxInHandler(keeper Keeper, mgr Manager) ObservedTxInHandler {
	return ObservedTxInHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

func (h ObservedTxInHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, _ constants.ConstantValues) cosmos.Result {
	msg, ok := m.(MsgObservedTxIn)
	if !ok {
		return errInvalidMessage.Result()
	}
	isNewSigner, err := h.validate(ctx, msg, version)
	if err != nil {
		return cosmos.ErrInternal(err.Error()).Result()
	}
	if isNewSigner {
		return cosmos.Result{
			Code:      cosmos.CodeOK,
			Codespace: DefaultCodespace,
		}
	}
	return h.handle(ctx, msg, version)
}

func (h ObservedTxInHandler) validate(ctx cosmos.Context, msg MsgObservedTxIn, version semver.Version) (bool, error) {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	} else {
		ctx.Logger().Error(errInvalidVersion.Error())
		return false, errInvalidVersion
	}
}

func (h ObservedTxInHandler) validateV1(ctx cosmos.Context, msg MsgObservedTxIn) (bool, error) {
	if err := msg.ValidateBasic(); err != nil {
		ctx.Logger().Error(err.Error())
		return false, err
	}

	if !isSignedByActiveNodeAccounts(ctx, h.keeper, msg.GetSigners()) {
		ctx.Logger().Error(notAuthorized.Error())
		return false, notAuthorized
	}

	return false, nil
}

func (h ObservedTxInHandler) handle(ctx cosmos.Context, msg MsgObservedTxIn, version semver.Version) cosmos.Result {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, version, msg)
	} else {
		ctx.Logger().Error(errInvalidVersion.Error())
		return errBadVersion.Result()
	}
}

func (h ObservedTxInHandler) preflight(ctx cosmos.Context, voter ObservedTxVoter, nas NodeAccounts, tx ObservedTx, signer cosmos.AccAddress, slasher *Slasher, version semver.Version) (ObservedTxVoter, bool) {
	constAccessor := constants.GetConstantValues(version)
	observeSlashPoints := constAccessor.GetInt64Value(constants.ObserveSlashPoints)
	slasher.IncSlashPoints(ctx, observeSlashPoints, signer)
	ok := false
	if !voter.Add(tx, signer) {
		return voter, ok
	}
	if voter.HasConsensus(nas) {
		if voter.Height == 0 {
			ok = true
			voter.Height = ctx.BlockHeight()
			// this is the tx that has consensus
			voter.Tx = voter.GetTx(nas)

			// tx has consensus now, so decrease the slashing points for all the signers whom had voted for it
			slasher.DecSlashPoints(ctx, observeSlashPoints, voter.Tx.Signers...)
		} else {
			// event the tx had been processed , given the signer just a bit late , so still take away their slash points
			// but only when the tx signer are voting is the tx that already reached consensus
			if ctx.BlockHeight() == voter.Height && voter.Tx.Equals(tx) {
				slasher.DecSlashPoints(ctx, observeSlashPoints, signer)
			}
		}
	}
	h.keeper.SetObservedTxVoter(ctx, voter)

	// Check to see if we have enough identical observations to process the transaction
	return voter, ok
}

// Handle a message to observe inbound tx
func (h ObservedTxInHandler) handleV1(ctx cosmos.Context, version semver.Version, msg MsgObservedTxIn) cosmos.Result {
	constAccessor := constants.GetConstantValues(version)
	activeNodeAccounts, err := h.keeper.ListActiveNodeAccounts(ctx)
	if err != nil {
		err = wrapError(ctx, err, "fail to get list of active node accounts")
		return cosmos.ErrInternal(err.Error()).Result()
	}
	slasher, err := NewSlasher(h.keeper, version, h.mgr)
	if err != nil {
		ctx.Logger().Error("fail to create slasher", "error", err)
		return cosmos.ErrInternal("fail to create slasher").Result()
	}
	handler := NewInternalHandler(h.keeper, h.mgr)
	for _, tx := range msg.Txs {

		// check we are sending to a valid vault
		if !h.keeper.VaultExists(ctx, tx.ObservedPubKey) {
			ctx.Logger().Info("Not valid Observed Pubkey", "observed pub key", tx.ObservedPubKey)
			continue
		}

		voter, err := h.keeper.GetObservedTxVoter(ctx, tx.Tx.ID)
		if err != nil {
			return cosmos.ErrInternal(err.Error()).Result()
		}

		voter, ok := h.preflight(ctx, voter, activeNodeAccounts, tx, msg.Signer, slasher, version)
		if !ok {
			if voter.Height == ctx.BlockHeight() {
				// we've already process the transaction, but we should still
				// update the observing addresses
				h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, msg.GetSigners())
			}
			continue
		}

		tx.Tx.Memo = fetchMemo(ctx, constAccessor, h.keeper, tx.Tx)
		if len(tx.Tx.Memo) == 0 {
			// we didn't find our memo, it might be yggdrasil return. These are
			// tx markers without coin amounts because we allow yggdrasil to
			// figure out the coin amounts
			txYgg := tx.Tx
			txYgg.Coins = common.Coins{
				common.NewCoin(common.RuneAsset(), cosmos.ZeroUint()),
			}
			tx.Tx.Memo = fetchMemo(ctx, constAccessor, h.keeper, txYgg)
		}

		ctx.Logger().Info("handleMsgObservedTxIn request", "Tx:", tx.String())

		txIn := voter.GetTx(activeNodeAccounts)
		txIn.Tx.Memo = tx.Tx.Memo
		vault, err := h.keeper.GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get vault", "error", err)
			return cosmos.ErrInternal(err.Error()).Result()
		}

		vault.AddFunds(tx.Tx.Coins)
		vault.InboundTxCount += 1
		memo, _ := ParseMemo(tx.Tx.Memo) // ignore err
		if vault.IsYggdrasil() && memo.IsType(TxYggdrasilFund) {
			vault.RemovePendingTxBlockHeights(memo.GetBlockHeight())
		}
		if err := h.keeper.SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
			return cosmos.ErrInternal(err.Error()).Result()
		}

		if !vault.IsAsgard() {
			ctx.Logger().Error("Vault is not an Asgard vault, transaction ignored.")
			continue
		}
		if vault.Status == InactiveVault {
			ctx.Logger().Error("Vault is inactive, transaction ignored.")
			continue
		}

		// tx is not observed at current vault - refund
		// yggdrasil pool is ok
		if ok := isCurrentVaultPubKey(ctx, h.keeper, tx); !ok {
			reason := fmt.Sprintf("vault %s is not current vault", tx.ObservedPubKey)
			ctx.Logger().Info("refund reason", reason)
			if err := refundTx(ctx, tx, h.mgr, h.keeper, constAccessor, CodeInvalidVault, reason); err != nil {
				return cosmos.ErrInternal(err.Error()).Result()
			}
			continue
		}
		// chain is empty
		if tx.Tx.Chain.IsEmpty() {
			if err := refundTx(ctx, tx, h.mgr, h.keeper, constAccessor, CodeEmptyChain, "chain is empty"); err != nil {
				return cosmos.ErrInternal(err.Error()).Result()
			}
			continue
		}

		// construct msg from memo
		m, txErr := processOneTxIn(ctx, h.keeper, txIn, msg.Signer)
		if txErr != nil {
			ctx.Logger().Error("fail to process inbound tx", "error", txErr.Error(), "tx hash", tx.Tx.ID.String())
			if newErr := refundTx(ctx, tx, h.mgr, h.keeper, constAccessor, txErr.Code(), fmt.Sprint(txErr.Data())); nil != newErr {
				return cosmos.ErrInternal(newErr.Error()).Result()
			}
			continue
		}

		if memo.IsOutbound() {
			// no one should send an outbound tx to vault
			continue
		}

		if err := h.keeper.SetLastChainHeight(ctx, tx.Tx.Chain, tx.BlockHeight); err != nil {
			return cosmos.ErrInternal(err.Error()).Result()
		}

		// add addresses to observing addresses. This is used to detect
		// active/inactive observing node accounts
		h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, txIn.Signers)

		// check if we've halted trading
		_, isSwap := m.(MsgSwap)
		_, isStake := m.(MsgSetStakeData)
		haltTrading, err := h.keeper.GetMimir(ctx, "HaltTrading")
		if isSwap || isStake {
			if (haltTrading > 0 && haltTrading < ctx.BlockHeight() && err == nil) || h.keeper.RagnarokInProgress(ctx) {
				ctx.Logger().Info("trading is halted!!")
				if newErr := refundTx(ctx, tx, h.mgr, h.keeper, constAccessor, cosmos.CodeUnauthorized, "trading halted"); nil != newErr {
					return cosmos.ErrInternal(newErr.Error()).Result()
				}
				continue
			}
		}

		// if its a swap, send it to our queue for processing later
		if isSwap {
			if err := h.keeper.SetSwapQueueItem(ctx, m.(MsgSwap)); err != nil {
				return cosmos.ErrInternal(err.Error()).Result()
			}
			return cosmos.Result{
				Code:      cosmos.CodeOK,
				Codespace: DefaultCodespace,
			}
		}

		result := handler(ctx, m)
		if !result.IsOK() {
			refundMsg, err := getErrMessageFromABCILog(result.Log)
			if err != nil {
				ctx.Logger().Error(err.Error())
			}
			if err := refundTx(ctx, tx, h.mgr, h.keeper, constAccessor, result.Code, refundMsg); err != nil {
				return cosmos.ErrInternal(err.Error()).Result()
			}
		}
	}
	return cosmos.Result{
		Code:      cosmos.CodeOK,
		Codespace: DefaultCodespace,
	}
}
