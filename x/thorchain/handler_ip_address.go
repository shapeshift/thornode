package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// IPAddressHandler is to handle ip address message
type IPAddressHandler struct {
	mgr Manager
}

// NewIPAddressHandler create new instance of IPAddressHandler
func NewIPAddressHandler(mgr Manager) IPAddressHandler {
	return IPAddressHandler{
		mgr: mgr,
	}
}

// Run it the main entry point to execute ip address logic
func (h IPAddressHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(*MsgSetIPAddress)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive ip address", "address", msg.IPAddress)
	if err := h.validate(ctx, *msg, version, constAccessor); err != nil {
		ctx.Logger().Error("msg set version failed validation", "error", err)
		return nil, err
	}
	if err := h.handle(ctx, *msg, version, constAccessor); err != nil {
		ctx.Logger().Error("fail to process msg set version", "error", err)
		return nil, err
	}

	return &cosmos.Result{}, nil
}

func (h IPAddressHandler) validate(ctx cosmos.Context, msg MsgSetIPAddress, version semver.Version, constAccessor constants.ConstantValues) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg, constAccessor)
	}
	return errBadVersion
}

func (h IPAddressHandler) validateV1(ctx cosmos.Context, msg MsgSetIPAddress, constAccessor constants.ConstantValues) error {
	return h.validateCurrent(ctx, msg, constAccessor)
}

func (h IPAddressHandler) validateCurrent(ctx cosmos.Context, msg MsgSetIPAddress, constAccessor constants.ConstantValues) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		ctx.Logger().Error("fail to get node account", "error", err, "address", msg.Signer.String())
		return cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorizaed", msg.Signer))
	}
	if nodeAccount.IsEmpty() {
		ctx.Logger().Error("unauthorized account", "address", msg.Signer.String())
		return cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorizaed", msg.Signer))
	}

	cost, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || cost < 0 {
		cost = constAccessor.GetInt64Value(constants.NativeTransactionFee)
	}
	if nodeAccount.Bond.LT(cosmos.NewUint(uint64(cost))) {
		return cosmos.ErrUnauthorized("not enough bond")
	}

	return nil
}

func (h IPAddressHandler) handle(ctx cosmos.Context, msg MsgSetIPAddress, version semver.Version, constAccessor constants.ConstantValues) error {
	ctx.Logger().Info("handleMsgSetIPAddress request", "ip address", msg.IPAddress)
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, constAccessor)
	}
	ctx.Logger().Error(errInvalidVersion.Error())
	return errBadVersion
}

func (h IPAddressHandler) handleV1(ctx cosmos.Context, msg MsgSetIPAddress, constAccessor constants.ConstantValues) error {
	return h.handleCurrent(ctx, msg, constAccessor)
}

func (h IPAddressHandler) handleCurrent(ctx cosmos.Context, msg MsgSetIPAddress, constAccessor constants.ConstantValues) error {
	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		ctx.Logger().Error("fail to get node account", "error", err, "address", msg.Signer.String())
		return cosmos.ErrUnauthorized(fmt.Sprintf("unable to find account: %s", msg.Signer))
	}

	c, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || c < 0 {
		c = constAccessor.GetInt64Value(constants.NativeTransactionFee)
	}
	cost := cosmos.NewUint(uint64(c))
	if cost.GT(nodeAccount.Bond) {
		cost = nodeAccount.Bond
	}
	nodeAccount.IPAddress = msg.IPAddress
	nodeAccount.Bond = common.SafeSub(nodeAccount.Bond, cost) // take bond
	if err := h.mgr.Keeper().SetNodeAccount(ctx, nodeAccount); err != nil {
		return fmt.Errorf("fail to save node account: %w", err)
	}

	// add cost to reserve
	coin := common.NewCoin(common.RuneNative, cost)
	if !cost.IsZero() {
		if err := h.mgr.Keeper().SendFromAccountToModule(ctx, msg.Signer, ReserveName, common.NewCoins(coin)); err != nil {
			ctx.Logger().Error("fail to transfer funds from bond to reserve", "error", err)
			return err
		}
	}

	tx := common.Tx{}
	tx.ID = common.BlankTxID
	tx.FromAddress = nodeAccount.BondAddress
	bondEvent := NewEventBond(cost, BondCost, tx)
	if err := h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
		return fmt.Errorf("fail to emit bond event: %w", err)
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("set_ip_address",
			cosmos.NewAttribute("thor_address", msg.Signer.String()),
			cosmos.NewAttribute("address", msg.IPAddress)))

	return nil
}
