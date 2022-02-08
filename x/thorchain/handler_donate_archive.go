package thorchain

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (h DonateHandler) validateV1(ctx cosmos.Context, msg MsgDonate) error {
	return msg.ValidateBasic()
}
