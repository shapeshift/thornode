package thorchain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

var mimirValidKey = regexp.MustCompile(`^[a-zA-Z-]+$`).MatchString

// MimirHandler is to handle admin messages
type MimirHandler struct {
	mgr Manager
}

// NewMimirHandler create new instance of MimirHandler
func NewMimirHandler(mgr Manager) MimirHandler {
	return MimirHandler{
		mgr: mgr,
	}
}

// Run is the main entry point to execute mimir logic
func (h MimirHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgMimir)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg mimir failed validation", "error", err)
		return nil, err
	}
	if err := h.handle(ctx, *msg); err != nil {
		ctx.Logger().Error("fail to process msg set mimir", "error", err)
		return nil, err
	}

	return &cosmos.Result{}, nil
}

func (h MimirHandler) validate(ctx cosmos.Context, msg MsgMimir) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.78.0")) {
		return h.validateV78(ctx, msg)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h MimirHandler) isAdmin(acc cosmos.AccAddress) bool {
	for _, admin := range ADMINS {
		addr, err := cosmos.AccAddressFromBech32(admin)
		if acc.Equals(addr) && err == nil {
			return true
		}
	}
	return false
}

func (h MimirHandler) validateV78(ctx cosmos.Context, msg MsgMimir) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	if !mimirValidKey(msg.Key) || len(msg.Key) > 64 {
		return cosmos.ErrUnknownRequest("invalid mimir key")
	}
	if !h.isAdmin(msg.Signer) && !isSignedByActiveNodeAccounts(ctx, h.mgr, msg.GetSigners()) {
		return cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorizaed", msg.Signer))
	}
	return nil
}

func (h MimirHandler) handle(ctx cosmos.Context, msg MsgMimir) error {
	ctx.Logger().Info("handleMsgMimir request", "node", msg.Signer, "key", msg.Key, "value", msg.Value)
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.81.0")) {
		return h.handleV81(ctx, msg)
	} else if version.GTE(semver.MustParse("0.78.0")) {
		return h.handleV78(ctx, msg)
	} else if version.GTE(semver.MustParse("0.65.0")) {
		return h.handleV65(ctx, msg)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg)
	}
	ctx.Logger().Error(errInvalidVersion.Error())
	return errBadVersion
}

func (h MimirHandler) handleV81(ctx cosmos.Context, msg MsgMimir) error {
	if h.isAdmin(msg.Signer) {
		if msg.Value < 0 {
			_ = h.mgr.Keeper().DeleteMimir(ctx, msg.Key)
		} else {
			h.mgr.Keeper().SetMimir(ctx, msg.Key, msg.Value)
		}

		ctx.EventManager().EmitEvent(
			cosmos.NewEvent("set_mimir",
				cosmos.NewAttribute("key", msg.Key),
				cosmos.NewAttribute("value", strconv.FormatInt(msg.Value, 10))))
	} else {
		nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.Signer)
		if err != nil {
			ctx.Logger().Error("fail to get node account", "error", err, "address", msg.Signer.String())
			return cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorized", msg.Signer))
		}

		c, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
		if err != nil || c < 0 {
			c = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
		}
		cost := cosmos.NewUint(uint64(c))
		nodeAccount.Bond = common.SafeSub(nodeAccount.Bond, cost)
		if err := h.mgr.Keeper().SetNodeAccount(ctx, nodeAccount); err != nil {
			ctx.Logger().Error("fail to save node account", "error", err)
			return fmt.Errorf("fail to save node account: %w", err)
		}

		// add 10 bond to reserve
		coin := common.NewCoin(common.RuneNative, cost)
		if !cost.IsZero() {
			if err := h.mgr.Keeper().SendFromModuleToModule(ctx, BondName, ReserveName, common.NewCoins(coin)); err != nil {
				ctx.Logger().Error("fail to transfer funds from bond to reserve", "error", err)
				return err
			}
		}

		if err := h.mgr.Keeper().SetNodeMimir(ctx, msg.Key, msg.Value, msg.Signer); err != nil {
			ctx.Logger().Error("fail to save node mimir", "error", err)
			return err
		}

		ctx.EventManager().EmitEvent(
			cosmos.NewEvent("set_node_mimir",
				cosmos.NewAttribute("key", strings.ToUpper(msg.Key)),
				cosmos.NewAttribute("value", strconv.FormatInt(msg.Value, 10)),
				cosmos.NewAttribute("address", msg.Signer.String())))

		tx := common.Tx{}
		tx.ID = common.BlankTxID
		tx.ToAddress = common.Address(nodeAccount.String())
		bondEvent := NewEventBond(cost, BondCost, tx)
		if err := h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
			ctx.Logger().Error("fail to emit bond event", "error", err)
		}
	}

	return nil
}
