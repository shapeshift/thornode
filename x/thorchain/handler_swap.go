package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type SwapHandler struct {
	keeper                Keeper
	versionedTxOutStore   VersionedTxOutStore
	versionedEventManager VersionedEventManager
}

func NewSwapHandler(keeper Keeper, versionedTxOutStore VersionedTxOutStore, versionedEventManager VersionedEventManager) SwapHandler {
	return SwapHandler{
		keeper:                keeper,
		versionedTxOutStore:   versionedTxOutStore,
		versionedEventManager: versionedEventManager,
	}
}

func (h SwapHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) cosmos.Result {
	msg, ok := m.(MsgSwap)
	if !ok {
		return errInvalidMessage.Result()
	}
	if err := h.validate(ctx, msg, version); err != nil {
		return cosmos.ErrInternal(err.Error()).Result()
	}
	return h.handle(ctx, msg, version, constAccessor)
}

func (h SwapHandler) validate(ctx cosmos.Context, msg MsgSwap, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	} else {
		ctx.Logger().Error(errInvalidVersion.Error())
		return errInvalidVersion
	}
}

func (h SwapHandler) validateV1(ctx cosmos.Context, msg MsgSwap) error {
	if err := msg.ValidateBasic(); err != nil {
		ctx.Logger().Error(err.Error())
		return err
	}
	return nil
}

func (h SwapHandler) handle(ctx cosmos.Context, msg MsgSwap, version semver.Version, constAccessor constants.ConstantValues) cosmos.Result {
	ctx.Logger().Info("receive MsgSwap", "request tx hash", msg.Tx.ID, "source asset", msg.Tx.Coins[0].Asset, "target asset", msg.TargetAsset, "signer", msg.Signer.String())
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, version, constAccessor)
	} else {
		ctx.Logger().Error(errInvalidVersion.Error())
		return errBadVersion.Result()
	}
}

func (h SwapHandler) handleV1(ctx cosmos.Context, msg MsgSwap, version semver.Version, constAccessor constants.ConstantValues) cosmos.Result {
	transactionFee := constAccessor.GetInt64Value(constants.TransactionFee)
	amount, events, swapErr := swap(
		ctx,
		h.keeper,
		msg.Tx,
		msg.TargetAsset,
		msg.Destination,
		msg.TradeTarget,
		cosmos.NewUint(uint64(transactionFee)))
	if swapErr != nil {
		ctx.Logger().Error("fail to process swap message", "error", swapErr)
		return swapErr.Result()
	}
	eventMgr, err := h.versionedEventManager.GetEventManager(ctx, version)
	if err != nil {
		ctx.Logger().Error("fail to get event manager", "error", err)
		return errFailGetEventManager.Result()
	}
	for _, evt := range events {
		if err := eventMgr.EmitSwapEvent(ctx, h.keeper, evt); err != nil {
			ctx.Logger().Error("fail to emit swap event", "error", err)
		}
		if err := h.keeper.AddToLiquidityFees(ctx, evt.Pool, evt.LiquidityFeeInRune); err != nil {
			return cosmos.ErrInternal(err.Error()).Result()
		}
	}

	res, err := h.keeper.Cdc().MarshalBinaryLengthPrefixed(
		struct {
			Asset cosmos.Uint `json:"asset"`
		}{
			Asset: amount,
		})
	if err != nil {
		ctx.Logger().Error("fail to encode result to json", "error", err)
		return cosmos.ErrInternal("fail to encode result to json").Result()
	}
	txOutStore, err := h.versionedTxOutStore.GetTxOutStore(ctx, h.keeper, version)
	if err != nil {
		ctx.Logger().Error("fail to get txout store", "error", err)
		return errBadVersion.Result()
	}
	toi := &TxOutItem{
		Chain:     msg.TargetAsset.Chain,
		InHash:    msg.Tx.ID,
		ToAddress: msg.Destination,
		Coin:      common.NewCoin(msg.TargetAsset, amount),
	}
	ok, err := txOutStore.TryAddTxOutItem(ctx, toi)
	if err != nil {
		ctx.Logger().Error("fail to add outbound tx", "error", err)
		return cosmos.ErrInternal(fmt.Errorf("fail to add outbound tx: %w", err).Error()).Result()
	}
	if !ok {
		return cosmos.NewError(DefaultCodespace, CodeFailAddOutboundTx, "prepare outbound tx not successful").Result()
	}

	return cosmos.Result{
		Code:      cosmos.CodeOK,
		Data:      res,
		Codespace: DefaultCodespace,
	}
}
