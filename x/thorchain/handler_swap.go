package thorchain

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// SwapHandler is the handler to process swap request
type SwapHandler struct {
	keeper keeper.Keeper
	mgr    Manager
}

// NewSwapHandler create a new instance of swap handler
func NewSwapHandler(keeper keeper.Keeper, mgr Manager) SwapHandler {
	return SwapHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

// Run is the main entry point of swap message
func (h SwapHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(*MsgSwap)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg, version); err != nil {
		ctx.Logger().Error("MsgSwap failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg, version, constAccessor)
	if err != nil {
		ctx.Logger().Error("fail to handle MsgSwap", "error", err)
		return nil, err
	}
	return result, err
}

func (h SwapHandler) validate(ctx cosmos.Context, msg MsgSwap, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errInvalidVersion
}

func (h SwapHandler) validateV1(ctx cosmos.Context, msg MsgSwap) error {
	return h.validateCurrent(ctx, msg)
}

func (h SwapHandler) validateCurrent(ctx cosmos.Context, msg MsgSwap) error {
	return msg.ValidateBasic()
}

func (h SwapHandler) handle(ctx cosmos.Context, msg MsgSwap, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	ctx.Logger().Info("receive MsgSwap", "request tx hash", msg.Tx.ID, "source asset", msg.Tx.Coins[0].Asset, "target asset", msg.TargetAsset, "signer", msg.Signer.String())
	// multichain testnet has been upgraded to 0.39.0 version already , but unfortunately there is a bug in it , `handleCurrent` should use Swapperv39 , but instead it was using SwapperV1
	// This will cause issues on chaosnet , thus need to use a new version `0.40.0` to make it right.
	// There will have no `0.39.0` on chaosnet , chaosnet will go from 0.38.0 -> 0.40.0
	// Once both testnet & chaosnet get upgrade to 0.40.0 the logic should be consistent
	if version.GTE(semver.MustParse("0.40.0")) {
		return h.handleCurrent(ctx, msg, version, constAccessor)
	} else if version.GTE(semver.MustParse("0.39.0")) {
		return h.handleV39(ctx, msg, version, constAccessor)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, version, constAccessor)
	}
	return nil, errBadVersion
}
func (h SwapHandler) handleV1(ctx cosmos.Context, msg MsgSwap, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	transactionFee := h.mgr.GasMgr().GetFee(ctx, msg.Destination.GetChain(), common.RuneAsset())
	synthVirtualDepthMult, err := h.keeper.GetMimir(ctx, constants.VirtualMultSynths.String())
	if synthVirtualDepthMult < 1 || err != nil {
		synthVirtualDepthMult = constAccessor.GetInt64Value(constants.VirtualMultSynths)
	}
	swapper := NewSwapperV1()
	_, _, swapErr := swapper.swap(
		ctx,
		h.keeper,
		msg.Tx,
		msg.TargetAsset,
		msg.Destination,
		msg.TradeTarget,
		transactionFee,
		synthVirtualDepthMult,
		h.mgr)
	if swapErr != nil {
		return nil, swapErr
	}
	return &cosmos.Result{}, nil
}
func (h SwapHandler) handleV39(ctx cosmos.Context, msg MsgSwap, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	transactionFee := h.mgr.GasMgr().GetFee(ctx, msg.Destination.GetChain(), common.RuneAsset())
	synthVirtualDepthMult, err := h.keeper.GetMimir(ctx, constants.VirtualMultSynths.String())
	if synthVirtualDepthMult < 1 || err != nil {
		synthVirtualDepthMult = constAccessor.GetInt64Value(constants.VirtualMultSynths)
	}
	swapper := NewSwapperV1()
	_, _, swapErr := swapper.swap(
		ctx,
		h.keeper,
		msg.Tx,
		msg.TargetAsset,
		msg.Destination,
		msg.TradeTarget,
		transactionFee,
		synthVirtualDepthMult,
		h.mgr)
	if swapErr != nil {
		return nil, swapErr
	}
	return &cosmos.Result{}, nil
}

func (h SwapHandler) handleCurrent(ctx cosmos.Context, msg MsgSwap, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	transactionFee := h.mgr.GasMgr().GetFee(ctx, msg.Destination.GetChain(), common.RuneAsset())
	synthVirtualDepthMult, err := h.keeper.GetMimir(ctx, constants.VirtualMultSynths.String())
	if synthVirtualDepthMult < 1 || err != nil {
		synthVirtualDepthMult = constAccessor.GetInt64Value(constants.VirtualMultSynths)
	}
	swapper := NewSwapperV39()
	_, _, swapErr := swapper.swap(
		ctx,
		h.keeper,
		msg.Tx,
		msg.TargetAsset,
		msg.Destination,
		msg.TradeTarget,
		transactionFee,
		synthVirtualDepthMult,
		h.mgr)
	if swapErr != nil {
		return nil, swapErr
	}
	return &cosmos.Result{}, nil
}
