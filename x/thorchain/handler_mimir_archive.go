package thorchain

import (
	"fmt"
	"strconv"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (h MimirHandler) validateV1(ctx cosmos.Context, msg MsgMimir) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	for _, admin := range ADMINS {
		addr, err := cosmos.AccAddressFromBech32(admin)
		if msg.Signer.Equals(addr) && err == nil {
			return nil
		}
	}
	return cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorizaed", msg.Signer))
}

func (h MimirHandler) handleV1(ctx cosmos.Context, msg MsgMimir) error {
	h.mgr.Keeper().SetMimir(ctx, msg.Key, msg.Value)

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("set_mimir",
			cosmos.NewAttribute("key", msg.Key),
			cosmos.NewAttribute("value", strconv.FormatInt(msg.Value, 10))))

	return nil
}

func (h MimirHandler) handleV65(ctx cosmos.Context, msg MsgMimir) error {
	if msg.Value < 0 {
		_ = h.mgr.Keeper().DeleteMimir(ctx, msg.Key)
	} else {
		h.mgr.Keeper().SetMimir(ctx, msg.Key, msg.Value)
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("set_mimir",
			cosmos.NewAttribute("key", msg.Key),
			cosmos.NewAttribute("value", strconv.FormatInt(msg.Value, 10))))

	return nil
}
