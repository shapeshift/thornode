package thorchain

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type SwapHandler struct {
	keeper Keeper
	mgr    Manager
}

func NewSwapHandler(keeper Keeper, mgr Manager) SwapHandler {
	return SwapHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

func (h SwapHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(MsgSwap)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, msg, version); err != nil {
		return nil, err
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
	return msg.ValidateBasic()
}

func (h SwapHandler) handle(ctx cosmos.Context, msg MsgSwap, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	ctx.Logger().Info("receive MsgSwap", "request tx hash", msg.Tx.ID, "source asset", msg.Tx.Coins[0].Asset, "target asset", msg.TargetAsset, "signer", msg.Signer.String())
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, version, constAccessor)
	}
	return nil, errBadVersion
}

func (h SwapHandler) handleV1(ctx cosmos.Context, msg MsgSwap, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
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
		return nil, swapErr
	}
	for _, evt := range events {
		if err := h.mgr.EventMgr().EmitSwapEvent(ctx, h.keeper, evt); err != nil {
			ctx.Logger().Error("fail to emit swap event", "error", err)
		}
		if err := h.keeper.AddToLiquidityFees(ctx, evt.Pool, evt.LiquidityFeeInRune); err != nil {
			return nil, err
		}
	}

	_, err := h.keeper.Cdc().MarshalBinaryLengthPrefixed(
		struct {
			Asset cosmos.Uint `json:"asset"`
		}{
			Asset: amount,
		})
	if err != nil {
		return nil, ErrInternal(err, "fail to encode result to json")
	}
	toi := &TxOutItem{
		Chain:     msg.TargetAsset.Chain,
		InHash:    msg.Tx.ID,
		ToAddress: msg.Destination,
		Coin:      common.NewCoin(msg.TargetAsset, amount),
	}
	ok, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi)
	if err != nil {
		return nil, ErrInternal(err, "fail to add outbound tx")
	}
	if !ok {
		return nil, errFailAddOutboundTx
	}

	return &cosmos.Result{}, nil
}
