package thorchain

import (
	"errors"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (h SendHandler) validateV1(ctx cosmos.Context, msg MsgSend) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// check if we're sending to asgard, bond modules. If we are, forward to the native tx handler
	if msg.ToAddress.Equals(h.mgr.Keeper().GetModuleAccAddress(AsgardName)) || msg.ToAddress.Equals(h.mgr.Keeper().GetModuleAccAddress(BondName)) {
		return errors.New("cannot use MsgSend for Asgard or Bond transactions, use MsgDeposit instead")
	}

	return nil
}
