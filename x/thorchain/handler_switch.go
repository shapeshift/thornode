package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// SwitchHandler is to handle Switch message
// MsgSwitch is used to switch from bep2 RUNE to native RUNE
type SwitchHandler struct {
	mgr Manager
}

// NewSwitchHandler create new instance of SwitchHandler
func NewSwitchHandler(mgr Manager) SwitchHandler {
	return SwitchHandler{
		mgr: mgr,
	}
}

// Run it the main entry point to execute Switch logic
func (h SwitchHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgSwitch)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg switch failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("failed to process msg switch", "error", err)
		return nil, err
	}
	return result, err
}

func (h SwitchHandler) validate(ctx cosmos.Context, msg MsgSwitch) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h SwitchHandler) validateV1(ctx cosmos.Context, msg MsgSwitch) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// if we are getting a non-native asset, ensure its signed by an active
	// node account
	if !msg.Tx.Coins[0].IsNative() {
		if !isSignedByActiveNodeAccounts(ctx, h.mgr, msg.GetSigners()) {
			return cosmos.ErrUnauthorized(notAuthorized.Error())
		}
	}

	return nil
}

func (h SwitchHandler) handle(ctx cosmos.Context, msg MsgSwitch) (*cosmos.Result, error) {
	ctx.Logger().Info("handleMsgSwitch request", "destination address", msg.Destination.String())
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.56.0")) {
		return h.handleV56(ctx, msg)
	}
	return nil, errBadVersion
}

func (h SwitchHandler) handleV56(ctx cosmos.Context, msg MsgSwitch) (*cosmos.Result, error) {
	haltHeight, err := h.mgr.Keeper().GetMimir(ctx, "HaltTHORChain")
	if err != nil {
		return nil, fmt.Errorf("failed to get mimir setting: %w", err)
	}
	if haltHeight > 0 && common.BlockHeight(ctx) > haltHeight {
		return nil, fmt.Errorf("mimir has halted THORChain transactions")
	}

	if !msg.Tx.Coins[0].IsNative() && msg.Tx.Coins[0].Asset.IsRune() {
		return h.toNativeV56(ctx, msg)
	}

	return nil, fmt.Errorf("only non-native rune can be 'switched' to native rune")
}

func (h SwitchHandler) toNativeV56(ctx cosmos.Context, msg MsgSwitch) (*cosmos.Result, error) {
	coin := common.NewCoin(common.RuneNative, msg.Tx.Coins[0].Amount)

	addr, err := cosmos.AccAddressFromBech32(msg.Destination.String())
	if err != nil {
		return nil, ErrInternal(err, "fail to parse thor address")
	}
	if err := h.mgr.Keeper().MintAndSendToAccount(ctx, addr, coin); err != nil {
		return nil, ErrInternal(err, "fail to mint native rune coins")
	}

	// update network data
	network, err := h.mgr.Keeper().GetNetwork(ctx)
	if err != nil {
		// do not cause the transaction to fail
		ctx.Logger().Error("failed to get network", "error", err)
	}

	switch msg.Tx.Chain {
	case common.BNBChain:
		network.BurnedBep2Rune = network.BurnedBep2Rune.Add(coin.Amount)
	case common.ETHChain:
		network.BurnedErc20Rune = network.BurnedErc20Rune.Add(coin.Amount)
	}
	if err := h.mgr.Keeper().SetNetwork(ctx, network); err != nil {
		ctx.Logger().Error("failed to set network", "error", err)
	}

	switchEvent := NewEventSwitchV56(msg.Tx.FromAddress, addr, msg.Tx.Coins[0], msg.Tx.ID)
	if err := h.mgr.EventMgr().EmitEvent(ctx, switchEvent); err != nil {
		ctx.Logger().Error("fail to emit switch event", "error", err)
	}

	return &cosmos.Result{}, nil
}
