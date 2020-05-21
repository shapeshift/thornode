package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// CommonOutboundTxHandler is the place where those common logic can be shared between multiple different kind of outbound tx handler
// at the moment, handler_refund, and handler_outbound_tx are largely the same , only some small difference
type CommonOutboundTxHandler struct {
	keeper Keeper
	mgr    Manager
}

// NewCommonOutboundTxHandler create a new instance of the CommonOutboundTxHandler
func NewCommonOutboundTxHandler(k Keeper, mgr Manager) CommonOutboundTxHandler {
	return CommonOutboundTxHandler{
		keeper: k,
		mgr:    mgr,
	}
}

func (h CommonOutboundTxHandler) slash(ctx cosmos.Context, version semver.Version, tx ObservedTx) error {
	var returnErr error
	slasher, err := NewSlasher(h.keeper, version, h.mgr)
	if err != nil {
		return fmt.Errorf("fail to create new slasher,error:%w", err)
	}
	for _, c := range tx.Tx.Coins {
		if err := slasher.SlashNodeAccount(ctx, tx.ObservedPubKey, c.Asset, c.Amount); err != nil {
			ctx.Logger().Error("fail to slash account", "error", err)
			returnErr = err
		}
	}
	return returnErr
}

func (h CommonOutboundTxHandler) handle(ctx cosmos.Context, version semver.Version, tx ObservedTx, inTxID common.TxID, status EventStatus) (*cosmos.Result, error) {
	voter, err := h.keeper.GetObservedTxVoter(ctx, inTxID)
	if err != nil {
		return nil, ErrInternal(err, "fail to get observed tx voter")
	}

	if voter.Height > 0 {
		voter.AddOutTx(tx.Tx)
		h.keeper.SetObservedTxVoter(ctx, voter)
	}

	// complete events
	if voter.IsDone() {
		err := completeEvents(ctx, h.keeper, inTxID, voter.OutTxs, status)
		if err != nil {
			return nil, ErrInternal(err, "unable to complete events")
		}
		for _, item := range voter.OutTxs {
			if err := h.mgr.EventMgr().EmitOutboundEvent(ctx, NewEventOutbound(inTxID, item)); err != nil {
				return nil, ErrInternal(err, "fail to emit outbound event")
			}
		}
	}

	// update txOut record with our TxID that sent funds out of the pool
	txOut, err := h.keeper.GetTxOut(ctx, voter.Height)
	if err != nil {
		ctx.Logger().Error("unable to get txOut record", "error", err)
		return nil, cosmos.ErrUnknownRequest(err.Error())
	}

	// Save TxOut back with the TxID only when the TxOut on the block height is
	// not empty
	shouldSlash := true
	for i, txOutItem := range txOut.TxArray {
		// withdraw , refund etc, one inbound tx might result two outbound
		// txes, THORNode have to correlate outbound tx back to the
		// inbound, and also txitem , thus THORNode could record both
		// outbound tx hash correctly given every tx item will only have
		// one coin in it , THORNode could use that to identify which tx it
		// is
		if txOutItem.InHash.Equals(inTxID) &&
			txOutItem.OutHash.IsEmpty() &&
			tx.Tx.Coins.Equals(common.Coins{txOutItem.Coin}) &&
			tx.Tx.Chain.Equals(txOutItem.Chain) &&
			tx.Tx.ToAddress.Equals(txOutItem.ToAddress) &&
			tx.ObservedPubKey.Equals(txOutItem.VaultPubKey) {

			txOut.TxArray[i].OutHash = tx.Tx.ID
			shouldSlash = false

			if err := h.keeper.SetTxOut(ctx, txOut); err != nil {
				ctx.Logger().Error("fail to save tx out", "error", err)
			}
			break
		}
	}

	if shouldSlash {
		if err := h.slash(ctx, version, tx); err != nil {
			return nil, ErrInternal(err, "fail to slash account")
		}
	}

	h.keeper.SetLastSignedHeight(ctx, voter.Height)

	return &cosmos.Result{}, nil
}
