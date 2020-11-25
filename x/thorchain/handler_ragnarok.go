package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// RagnarokHandler process MsgRagnarok
type RagnarokHandler struct {
	keeper keeper.Keeper
	mgr    Manager
}

// NewRagnarokHandler create a new instance of RagnarokHandler
func NewRagnarokHandler(keeper keeper.Keeper, mgr Manager) RagnarokHandler {
	return RagnarokHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

// Run is the main entry point of ragnarok handler
func (h RagnarokHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(MsgRagnarok)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, msg, version); err != nil {
		ctx.Logger().Error("MsgRagnarok failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, version, msg, constAccessor)
	if err != nil {
		ctx.Logger().Error("fail to process MsgRagnarok", "error", err)
	}
	return result, err
}

func (h RagnarokHandler) validate(ctx cosmos.Context, msg MsgRagnarok, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errInvalidVersion
}

func (h RagnarokHandler) validateV1(ctx cosmos.Context, msg MsgRagnarok) error {
	return msg.ValidateBasic()
}

func (h RagnarokHandler) handle(ctx cosmos.Context, version semver.Version, msg MsgRagnarok, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	ctx.Logger().Info("receive MsgRagnarok", "request tx hash", msg.Tx.Tx.ID)
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, version, msg, constAccessor)
	}
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

func (h RagnarokHandler) handleV1(ctx cosmos.Context, version semver.Version, msg MsgRagnarok, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	shouldSlash := true
	signingTransPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	for height := msg.BlockHeight; height <= common.BlockHeight(ctx); height += signingTransPeriod {
		// update txOut record with our TxID that sent funds out of the pool
		txOut, err := h.keeper.GetTxOut(ctx, height)
		if err != nil {
			return nil, ErrInternal(err, "unable to get txOut record")
		}
		for i, tx := range txOut.TxArray {
			// ragnarok is the memo used by thorchain to identify fund returns to
			// bonders, LPs, and reserve contributors.
			// it use ragnarok:{block height} to mark a tx out caused by the ragnarok protocol
			// this type of tx out is special, because it doesn't have relevant tx
			// in to trigger it, it is trigger by thorchain itself.

			fromAddress, _ := tx.VaultPubKey.GetAddress(tx.Chain)
			matchCoin := msg.Tx.Tx.Coins.Equals(common.Coins{tx.Coin})
			// when outbound is gas asset
			if !matchCoin && tx.Coin.Asset.Equals(tx.Chain.GetGasAsset()) {
				asset := tx.Chain.GetGasAsset()
				intendToSpend := tx.Coin.Amount.Add(tx.MaxGas.ToCoins().GetCoin(asset).Amount)
				actualSpend := msg.Tx.Tx.Coins.GetCoin(asset).Amount.Add(msg.Tx.Tx.Gas.ToCoins().GetCoin(asset).Amount)
				if intendToSpend.Equal(actualSpend) {
					diffGas := common.SafeSub(tx.MaxGas.ToCoins().GetCoin(asset).Amount, msg.Tx.Tx.Gas.ToCoins().GetCoin(asset).Amount)
					ctx.Logger().Info(fmt.Sprintf("intend to spend: %s, actual spend: %s are the same , override match coin, gas difference: %s ", intendToSpend, actualSpend, diffGas))
					matchCoin = true

					// when the actual gas is less than max gas , the gap had been paid to customer
					if diffGas.GT(cosmos.ZeroUint()) {
						pool, err := h.keeper.GetPool(ctx, asset)
						if err != nil {
							ctx.Logger().Error("fail to get pool", "error", err, "asset", asset)
						} else {
							runeValue := pool.AssetValueInRune(diffGas)
							pool.BalanceRune = pool.BalanceRune.Add(runeValue)
							pool.BalanceAsset = common.SafeSub(pool.BalanceAsset, diffGas)
							//	send RUNE to asgard
							if err := h.keeper.SendFromModuleToModule(ctx, ReserveName, AsgardName, common.NewCoin(common.RuneAsset(), runeValue)); err != nil {
								ctx.Logger().Error("fail to send fund from Reserve to Asgard", "error", err)
							}
							if err := h.keeper.SetPool(ctx, pool); err != nil {
								ctx.Logger().Error("fail to save pool", "error", err, "asset", asset)
							}
						}
					}
				}
			}

			if tx.InHash.Equals(common.BlankTxID) &&
				tx.OutHash.IsEmpty() &&
				matchCoin &&
				tx.ToAddress.Equals(msg.Tx.Tx.ToAddress) &&
				fromAddress.Equals(msg.Tx.Tx.FromAddress) {

				txOut.TxArray[i].OutHash = msg.Tx.Tx.ID
				shouldSlash = false
				if err := h.keeper.SetTxOut(ctx, txOut); nil != err {
					return nil, ErrInternal(err, "fail to save tx out")
				}

				pending, err := h.keeper.GetRagnarokPending(ctx)
				if err != nil {
					ctx.Logger().Error("fail to get ragnarok pending", "error", err)
				} else {
					h.keeper.SetRagnarokPending(ctx, pending-1)
					ctx.Logger().Info("remaining ragnarok transaction", "count", pending-1)
				}

				break
			}
		}
	}

	if shouldSlash {
		if err := h.slash(ctx, version, msg.Tx); err != nil {
			return nil, ErrInternal(err, "fail to slash account")
		}
	}

	if err := h.keeper.SetLastSignedHeight(ctx, msg.BlockHeight); err != nil {
		ctx.Logger().Info("fail to update last signed height", "error", err)
	}

	return &cosmos.Result{}, nil
}
