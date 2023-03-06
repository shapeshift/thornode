package thorchain

import (
	"errors"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

func MsgSendValidateV1(ctx cosmos.Context, mgr Manager, msg *MsgSend) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// check if we're sending to asgard, bond modules. If we are, forward to the native tx handler
	if msg.ToAddress.Equals(mgr.Keeper().GetModuleAccAddress(AsgardName)) || msg.ToAddress.Equals(mgr.Keeper().GetModuleAccAddress(BondName)) {
		return errors.New("cannot use MsgSend for Asgard or Bond transactions, use MsgDeposit instead")
	}

	return nil
}
