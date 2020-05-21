package thorchain

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type RagnarokHandler struct {
	keeper Keeper
	mgr    Manager
}

func NewRagnarokHandler(keeper Keeper, mgr Manager) RagnarokHandler {
	return RagnarokHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

func (h RagnarokHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, _ constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(MsgRagnarok)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, msg, version); err != nil {
		return nil, err
	}
	return h.handle(ctx, version, msg)
}

func (h RagnarokHandler) validate(ctx cosmos.Context, msg MsgRagnarok, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	ctx.Logger().Error(errInvalidVersion.Error())
	return errInvalidVersion
}

func (h RagnarokHandler) validateV1(ctx cosmos.Context, msg MsgRagnarok) error {
	if err := msg.ValidateBasic(); nil != err {
		ctx.Logger().Error(err.Error())
		return err
	}
	return nil
}

func (h RagnarokHandler) handle(ctx cosmos.Context, version semver.Version, msg MsgRagnarok) (*cosmos.Result, error) {
	ctx.Logger().Info("receive MsgRagnarok", "request tx hash", msg.Tx.Tx.ID)
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, version, msg)
	}
	ctx.Logger().Error(errInvalidVersion.Error())
	return nil, errBadVersion
}

func (h RagnarokHandler) slash(ctx cosmos.Context, version semver.Version, tx ObservedTx) error {
	var returnErr error
	for _, c := range tx.Tx.Coins {
		if err := h.mgr.Slasher().SlashNodeAccount(ctx, tx.ObservedPubKey, c.Asset, c.Amount, h.mgr); err != nil {
			ctx.Logger().Error("fail to slash account", "error", err)
			returnErr = err
		}
	}
	return returnErr
}

func (h RagnarokHandler) handleV1(ctx cosmos.Context, version semver.Version, msg MsgRagnarok) (*cosmos.Result, error) {
	// update txOut record with our TxID that sent funds out of the pool
	txOut, err := h.keeper.GetTxOut(ctx, msg.BlockHeight)
	if err != nil {
		ctx.Logger().Error("unable to get txOut record", "error", err)
		return nil, cosmos.ErrUnknownRequest(err.Error())
	}

	shouldSlash := true
	for i, tx := range txOut.TxArray {
		// ragnarok is the memo used by thorchain to identify fund returns to
		// bonders, LPs, and reserve contributors.
		// it use ragnarok:{block height} to mark a tx out caused by the ragnarok protocol
		// this type of tx out is special, because it doesn't have relevant tx
		// in to trigger it, it is trigger by thorchain itself.
		fromAddress, _ := tx.VaultPubKey.GetAddress(tx.Chain)
		if tx.InHash.Equals(common.BlankTxID) &&
			tx.OutHash.IsEmpty() &&
			msg.Tx.Tx.Coins.Contains(tx.Coin) &&
			tx.ToAddress.Equals(msg.Tx.Tx.ToAddress) &&
			fromAddress.Equals(msg.Tx.Tx.FromAddress) {

			txOut.TxArray[i].OutHash = msg.Tx.Tx.ID
			shouldSlash = false
			if err := h.keeper.SetTxOut(ctx, txOut); nil != err {
				return nil, ErrInternal(err, "fail to save tx out")
			}

			break
		}
	}

	if shouldSlash {
		if err := h.slash(ctx, version, msg.Tx); err != nil {
			return nil, ErrInternal(err, "fail to slash account")
		}
	}

	h.keeper.SetLastSignedHeight(ctx, msg.BlockHeight)

	return &cosmos.Result{}, nil
}
