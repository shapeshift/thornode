package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (h TssKeysignHandler) validateV1(ctx cosmos.Context, msg MsgTssKeysignFail) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	if !isSignedByActiveNodeAccounts(ctx, h.mgr, msg.GetSigners()) {
		shouldAccept := false
		vaults, err := h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, RetiringVault)
		if err != nil {
			return ErrInternal(err, "fail to get retiring vaults")
		}
		if len(vaults) > 0 {
			for _, signer := range msg.GetSigners() {
				nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, signer)
				if err != nil {
					return ErrInternal(err, "fail to get node account")
				}

				for _, v := range vaults {
					if v.GetMembership().Contains(nodeAccount.PubKeySet.Secp256k1) {
						shouldAccept = true
						break
					}
				}
				if shouldAccept {
					break
				}
			}
		}
		if !shouldAccept {
			return cosmos.ErrUnauthorized("not authorized")
		}
		ctx.Logger().Info("keysign failure message from retiring vault member, should accept")
	}

	active, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return wrapError(ctx, err, "fail to get list of active node accounts")
	}

	if !HasSimpleMajority(len(active)-len(msg.Blame.BlameNodes), len(active)) {
		ctx.Logger().Error("blame cast too wide", "blame", len(msg.Blame.BlameNodes))
		return fmt.Errorf("blame cast too wide: %d/%d", len(msg.Blame.BlameNodes), len(active))
	}

	return nil
}
