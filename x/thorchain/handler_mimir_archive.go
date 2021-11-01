package thorchain

import (
	"strconv"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (h MimirHandler) handleV1(ctx cosmos.Context, msg MsgMimir) error {
	h.mgr.Keeper().SetMimir(ctx, msg.Key, msg.Value)

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("set_mimir",
			cosmos.NewAttribute("key", msg.Key),
			cosmos.NewAttribute("value", strconv.FormatInt(msg.Value, 10))))

	return nil

}
