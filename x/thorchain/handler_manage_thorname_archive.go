package thorchain

import (
	"errors"
	"fmt"

	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (h ManageTHORNameHandler) validateV1(ctx cosmos.Context, msg MsgManageTHORName) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	exists := h.mgr.Keeper().THORNameExists(ctx, msg.Name)

	if !exists {
		// thorname doesn't appear to exist, let's validate the name
		if err := h.validateNameV1(msg.Name); err != nil {
			return err
		}
		registrationFee := h.mgr.GetConstants().GetInt64Value(constants.TNSRegisterFee)
		if msg.Coin.Amount.LTE(cosmos.NewUint(uint64(registrationFee))) {
			return fmt.Errorf("not enough funds")
		}
	} else {
		name, err := h.mgr.Keeper().GetTHORName(ctx, msg.Name)
		if err != nil {
			return err
		}

		// if this thorname is already owned, check signer has ownership. If
		// expiration is past, allow different user to take ownership
		if !name.Owner.Equals(msg.Signer) && ctx.BlockHeight() <= name.ExpireBlockHeight {
			ctx.Logger().Error("no authorization", "owner", name.Owner)
			return fmt.Errorf("no authorization: owned by %s", name.Owner)
		}

		// ensure user isn't inflating their expire block height artificaially
		if name.ExpireBlockHeight < msg.ExpireBlockHeight {
			return errors.New("cannot artificially inflate expire block height")
		}
	}

	return nil
}
