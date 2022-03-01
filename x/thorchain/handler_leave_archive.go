package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (h LeaveHandler) handleV1(ctx cosmos.Context, msg MsgLeave) error {
	nodeAcc, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, "fail to get node account by bond address")
	}
	if nodeAcc.IsEmpty() {
		return cosmos.ErrUnknownRequest("node account doesn't exist")
	}
	if !nodeAcc.BondAddress.Equals(msg.Tx.FromAddress) {
		return cosmos.ErrUnauthorized(fmt.Sprintf("%s are not authorized to manage %s", msg.Tx.FromAddress, msg.NodeAddress))
	}
	// THORNode add the node to leave queue

	coin := msg.Tx.Coins.GetCoin(common.RuneAsset())
	if !coin.IsEmpty() {
		nodeAcc.Bond = nodeAcc.Bond.Add(coin.Amount)
	}

	if nodeAcc.Status == NodeActive {
		if nodeAcc.LeaveScore == 0 {
			// get to the 8th decimal point, but keep numbers integers for safer math
			age := cosmos.NewUint(uint64((common.BlockHeight(ctx) - nodeAcc.StatusSince) * common.One))
			slashPts, err := h.mgr.Keeper().GetNodeAccountSlashPoints(ctx, nodeAcc.NodeAddress)
			if err != nil || slashPts == 0 {
				ctx.Logger().Error("fail to get node account slash points", "error", err)
				nodeAcc.LeaveScore = age.Uint64()
			} else {
				nodeAcc.LeaveScore = age.QuoUint64(uint64(slashPts)).Uint64()
			}
		}
	} else {
		bondLockPeriod, err := h.mgr.Keeper().GetMimir(ctx, constants.BondLockupPeriod.String())
		if err != nil || bondLockPeriod < 0 {
			bondLockPeriod = h.mgr.GetConstants().GetInt64Value(constants.BondLockupPeriod)
		}
		if common.BlockHeight(ctx)-nodeAcc.StatusSince < bondLockPeriod {
			return fmt.Errorf("node can not unbond before %d", nodeAcc.StatusSince+bondLockPeriod)
		}
		vaults, err := h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, RetiringVault)
		if err != nil {
			return ErrInternal(err, "fail to get retiring vault")
		}
		isMemberOfRetiringVault := false
		for _, v := range vaults {
			if v.GetMembership().Contains(nodeAcc.PubKeySet.Secp256k1) {
				isMemberOfRetiringVault = true
				ctx.Logger().Info("node account is still part of the retiring vault,can't return bond yet")
				break
			}
		}
		if !isMemberOfRetiringVault {
			// NOTE: there is an edge case, where the first node doesn't have a
			// vault (it was destroyed when we successfully migrated funds from
			// their address to a new TSS vault
			if !h.mgr.Keeper().VaultExists(ctx, nodeAcc.PubKeySet.Secp256k1) {
				if err := refundBond(ctx, msg.Tx, nodeAcc.NodeAddress, cosmos.ZeroUint(), &nodeAcc, h.mgr); err != nil {
					return ErrInternal(err, "fail to refund bond")
				}
				nodeAcc.UpdateStatus(NodeDisabled, common.BlockHeight(ctx))
			} else {
				// given the node is not active, they should not have Yggdrasil pool either
				// but let's check it anyway just in case
				vault, err := h.mgr.Keeper().GetVault(ctx, nodeAcc.PubKeySet.Secp256k1)
				if err != nil {
					return ErrInternal(err, "fail to get vault pool")
				}
				if vault.IsYggdrasil() {
					if !vault.HasFunds() {
						// node is not active , they are free to leave , refund them
						if err := refundBond(ctx, msg.Tx, nodeAcc.NodeAddress, cosmos.ZeroUint(), &nodeAcc, h.mgr); err != nil {
							return ErrInternal(err, "fail to refund bond")
						}
						nodeAcc.UpdateStatus(NodeDisabled, common.BlockHeight(ctx))
					} else {
						if err := h.mgr.ValidatorMgr().RequestYggReturn(ctx, nodeAcc, h.mgr, h.mgr.GetConstants()); err != nil {
							return ErrInternal(err, "fail to request yggdrasil return fund")
						}
					}
				}
			}
		}
	}
	nodeAcc.RequestedToLeave = true
	if err := h.mgr.Keeper().SetNodeAccount(ctx, nodeAcc); err != nil {
		return ErrInternal(err, "fail to save node account to key value store")
	}
	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("validator_request_leave",
			cosmos.NewAttribute("signer bnb address", msg.Tx.FromAddress.String()),
			cosmos.NewAttribute("destination", nodeAcc.BondAddress.String()),
			cosmos.NewAttribute("tx", msg.Tx.ID.String())))

	return nil
}

func (h LeaveHandler) handleV46(ctx cosmos.Context, msg MsgLeave) error {
	nodeAcc, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, "fail to get node account by bond address")
	}
	if nodeAcc.IsEmpty() {
		return cosmos.ErrUnknownRequest("node account doesn't exist")
	}
	if !nodeAcc.BondAddress.Equals(msg.Tx.FromAddress) {
		return cosmos.ErrUnauthorized(fmt.Sprintf("%s are not authorized to manage %s", msg.Tx.FromAddress, msg.NodeAddress))
	}
	// THORNode add the node to leave queue

	coin := msg.Tx.Coins.GetCoin(common.RuneAsset())
	if !coin.IsEmpty() {
		nodeAcc.Bond = nodeAcc.Bond.Add(coin.Amount)
	}

	if nodeAcc.Status == NodeActive {
		if nodeAcc.LeaveScore == 0 {
			// get to the 8th decimal point, but keep numbers integers for safer math
			age := cosmos.NewUint(uint64((common.BlockHeight(ctx) - nodeAcc.StatusSince) * common.One))
			slashPts, err := h.mgr.Keeper().GetNodeAccountSlashPoints(ctx, nodeAcc.NodeAddress)
			if err != nil || slashPts == 0 {
				ctx.Logger().Error("fail to get node account slash points", "error", err)
				nodeAcc.LeaveScore = age.Uint64()
			} else {
				nodeAcc.LeaveScore = age.QuoUint64(uint64(slashPts)).Uint64()
			}
		}
	} else {
		bondLockPeriod, err := h.mgr.Keeper().GetMimir(ctx, constants.BondLockupPeriod.String())
		if err != nil || bondLockPeriod < 0 {
			bondLockPeriod = h.mgr.GetConstants().GetInt64Value(constants.BondLockupPeriod)
		}
		if common.BlockHeight(ctx)-nodeAcc.StatusSince < bondLockPeriod {
			return fmt.Errorf("node can not unbond before %d", nodeAcc.StatusSince+bondLockPeriod)
		}
		vaults, err := h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, RetiringVault)
		if err != nil {
			return ErrInternal(err, "fail to get retiring vault")
		}
		isMemberOfRetiringVault := false
		for _, v := range vaults {
			if v.GetMembership().Contains(nodeAcc.PubKeySet.Secp256k1) {
				isMemberOfRetiringVault = true
				ctx.Logger().Info("node account is still part of the retiring vault,can't return bond yet")
				break
			}
		}
		if !isMemberOfRetiringVault {
			// NOTE: there is an edge case, where the first node doesn't have a
			// vault (it was destroyed when we successfully migrated funds from
			// their address to a new TSS vault
			if !h.mgr.Keeper().VaultExists(ctx, nodeAcc.PubKeySet.Secp256k1) {
				if err := refundBond(ctx, msg.Tx, nodeAcc.NodeAddress, cosmos.ZeroUint(), &nodeAcc, h.mgr); err != nil {
					return ErrInternal(err, "fail to refund bond")
				}
				nodeAcc.UpdateStatus(NodeDisabled, common.BlockHeight(ctx))
			} else {
				// given the node is not active, they should not have Yggdrasil pool either
				// but let's check it anyway just in case
				vault, err := h.mgr.Keeper().GetVault(ctx, nodeAcc.PubKeySet.Secp256k1)
				if err != nil {
					return ErrInternal(err, "fail to get vault pool")
				}
				if vault.IsYggdrasil() {
					if !vault.HasFunds() {
						// node is not active , they are free to leave , refund them
						if err := refundBond(ctx, msg.Tx, nodeAcc.NodeAddress, cosmos.ZeroUint(), &nodeAcc, h.mgr); err != nil {
							return ErrInternal(err, "fail to refund bond")
						}
						nodeAcc.UpdateStatus(NodeDisabled, common.BlockHeight(ctx))
					} else {
						if err := h.mgr.ValidatorMgr().RequestYggReturn(ctx, nodeAcc, h.mgr, h.mgr.GetConstants()); err != nil {
							return ErrInternal(err, "fail to request yggdrasil return fund")
						}
					}
				}
			}
		}
	}
	nodeAcc.RequestedToLeave = true
	if err := h.mgr.Keeper().SetNodeAccount(ctx, nodeAcc); err != nil {
		return ErrInternal(err, "fail to save node account to key value store")
	}
	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("validator_request_leave",
			cosmos.NewAttribute("signer bnb address", msg.Tx.FromAddress.String()),
			cosmos.NewAttribute("destination", nodeAcc.BondAddress.String()),
			cosmos.NewAttribute("tx", msg.Tx.ID.String())))

	return nil
}
