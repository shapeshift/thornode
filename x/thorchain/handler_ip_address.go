package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// IPAddressHandler is to handle ip address message
type IPAddressHandler struct {
	keeper Keeper
	mgr    Manager
}

// NewIPAddressHandler create new instance of IPAddressHandler
func NewIPAddressHandler(keeper Keeper, mgr Manager) IPAddressHandler {
	return IPAddressHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

// Run it the main entry point to execute ip address logic
func (h IPAddressHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, _ constants.ConstantValues) cosmos.Result {
	msg, ok := m.(MsgSetIPAddress)
	if !ok {
		return errInvalidMessage.Result()
	}
	ctx.Logger().Info("receive ip address", "address", msg.IPAddress)
	if err := h.validate(ctx, msg, version); err != nil {
		ctx.Logger().Error("msg set version failed validation", "error", err)
		return err.Result()
	}
	if err := h.handle(ctx, msg, version); err != nil {
		ctx.Logger().Error("fail to process msg set version", "error", err)
		return err.Result()
	}

	return cosmos.Result{
		Code:      cosmos.CodeOK,
		Codespace: DefaultCodespace,
	}
}

func (h IPAddressHandler) validate(ctx cosmos.Context, msg MsgSetIPAddress, version semver.Version) cosmos.Error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	} else {
		return errBadVersion
	}
}

func (h IPAddressHandler) validateV1(ctx cosmos.Context, msg MsgSetIPAddress) cosmos.Error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	nodeAccount, err := h.keeper.GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		ctx.Logger().Error("fail to get node account", "error", err, "address", msg.Signer.String())
		return cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorizaed", msg.Signer))
	}
	if nodeAccount.IsEmpty() {
		ctx.Logger().Error("unauthorized account", "address", msg.Signer.String())
		return cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorizaed", msg.Signer))
	}

	return nil
}

func (h IPAddressHandler) handle(ctx cosmos.Context, msg MsgSetIPAddress, version semver.Version) cosmos.Error {
	ctx.Logger().Info("handleMsgSetIPAddress request", "ip address", msg.IPAddress)
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg)
	} else {
		ctx.Logger().Error(errInvalidVersion.Error())
		return errBadVersion
	}
}

func (h IPAddressHandler) handleV1(ctx cosmos.Context, msg MsgSetIPAddress) cosmos.Error {
	nodeAccount, err := h.keeper.GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		ctx.Logger().Error("fail to get node account", "error", err, "address", msg.Signer.String())
		return cosmos.ErrUnauthorized(fmt.Sprintf("unable to find account: %s", msg.Signer))
	}

	nodeAccount.IPAddress = msg.IPAddress

	if err := h.keeper.SetNodeAccount(ctx, nodeAccount); err != nil {
		ctx.Logger().Error("fail to save node account", "error", err)
		return cosmos.ErrInternal("fail to save node account")
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("set_ip_address",
			cosmos.NewAttribute("thor_address", msg.Signer.String()),
			cosmos.NewAttribute("address", msg.IPAddress)))

	return nil
}
