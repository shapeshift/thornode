package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type MigrateHandler struct {
	keeper                Keeper
	versionedEventManager VersionedEventManager
}

func NewMigrateHandler(keeper Keeper, versionedEventManager VersionedEventManager) MigrateHandler {
	return MigrateHandler{
		keeper:                keeper,
		versionedEventManager: versionedEventManager,
	}
}

func (h MigrateHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, _ constants.ConstantValues) cosmos.Result {
	msg, ok := m.(MsgMigrate)
	if !ok {
		return errInvalidMessage.Result()
	}
	if err := h.validate(ctx, msg, version); err != nil {
		return cosmos.ErrInternal(err.Error()).Result()
	}
	return h.handle(ctx, msg, version)
}

func (h MigrateHandler) validate(ctx cosmos.Context, msg MsgMigrate, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	ctx.Logger().Error(errInvalidVersion.Error())
	return errInvalidVersion
}

func (h MigrateHandler) validateV1(ctx cosmos.Context, msg MsgMigrate) error {
	if err := msg.ValidateBasic(); nil != err {
		ctx.Logger().Error(err.Error())
		return err
	}
	return nil
}

func (h MigrateHandler) handle(ctx cosmos.Context, msg MsgMigrate, version semver.Version) cosmos.Result {
	ctx.Logger().Info("receive MsgMigrate", "request tx hash", msg.Tx.Tx.ID)
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, version, msg)
	}
	ctx.Logger().Error(errInvalidVersion.Error())
	return errBadVersion.Result()
}

func (h MigrateHandler) slash(ctx cosmos.Context, version semver.Version, tx ObservedTx) error {
	var returnErr error
	slasher, err := NewSlasher(h.keeper, version, h.versionedEventManager)
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

func (h MigrateHandler) handleV1(ctx cosmos.Context, version semver.Version, msg MsgMigrate) cosmos.Result {
	// update txOut record with our TxID that sent funds out of the pool
	txOut, err := h.keeper.GetTxOut(ctx, msg.BlockHeight)
	if err != nil {
		ctx.Logger().Error("unable to get txOut record", "error", err)
		return cosmos.ErrUnknownRequest(err.Error()).Result()
	}

	shouldSlash := true
	for i, tx := range txOut.TxArray {
		// migrate is the memo used by thorchain to identify fund migration between asgard vault.
		// it use migrate:{block height} to mark a tx out caused by vault rotation
		// this type of tx out is special , because it doesn't have relevant tx in to trigger it, it is trigger by thorchain itself.
		fromAddress, _ := tx.VaultPubKey.GetAddress(tx.Chain)
		if tx.InHash.Equals(common.BlankTxID) &&
			tx.OutHash.IsEmpty() &&
			msg.Tx.Tx.Coins.Contains(tx.Coin) &&
			tx.ToAddress.Equals(msg.Tx.Tx.ToAddress) &&
			fromAddress.Equals(msg.Tx.Tx.FromAddress) {

			txOut.TxArray[i].OutHash = msg.Tx.Tx.ID
			shouldSlash = false

			if err := h.keeper.SetTxOut(ctx, txOut); nil != err {
				ctx.Logger().Error("fail to save tx out", "error", err)
				return cosmos.ErrInternal("fail to save tx out").Result()
			}

			break
		}
	}

	if shouldSlash {
		if err := h.slash(ctx, version, msg.Tx); err != nil {
			return cosmos.ErrInternal("fail to slash account").Result()
		}
	}

	h.keeper.SetLastSignedHeight(ctx, msg.BlockHeight)

	return cosmos.Result{
		Code:      cosmos.CodeOK,
		Codespace: DefaultCodespace,
	}
}
