package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (h BondHandler) validateV1(ctx cosmos.Context, msg MsgBond) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	// When RUNE is on thorchain , pay bond doesn't need to be active node
	// in fact , usually the node will not be active at the time it bond

	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node account(%s)", msg.NodeAddress))
	}

	bond := msg.Bond.Add(nodeAccount.Bond)

	maxBond, err := h.mgr.Keeper().GetMimir(ctx, "MaximumBondInRune")
	if maxBond > 0 && err == nil {
		maxValidatorBond := cosmos.NewUint(uint64(maxBond))
		if bond.GT(maxValidatorBond) {
			return cosmos.ErrUnknownRequest(fmt.Sprintf("too much bond, max validator bond (%s), bond(%s)", maxValidatorBond.String(), bond))
		}
	}

	return nil
}

func (h BondHandler) validateV78(ctx cosmos.Context, msg MsgBond) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	// When RUNE is on thorchain , pay bond doesn't need to be active node
	// in fact , usually the node will not be active at the time it bond

	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node account(%s)", msg.NodeAddress))
	}

	if nodeAccount.Status == NodeActive || nodeAccount.Status == NodeReady {
		return ErrInternal(err, "cannot add bond while node is active or ready status")
	}

	bond := msg.Bond.Add(nodeAccount.Bond)

	maxBond, err := h.mgr.Keeper().GetMimir(ctx, "MaximumBondInRune")
	if maxBond > 0 && err == nil {
		maxValidatorBond := cosmos.NewUint(uint64(maxBond))
		if bond.GT(maxValidatorBond) {
			return cosmos.ErrUnknownRequest(fmt.Sprintf("too much bond, max validator bond (%s), bond(%s)", maxValidatorBond.String(), bond))
		}
	}

	return nil
}

func (h BondHandler) validateV80(ctx cosmos.Context, msg MsgBond) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	// When RUNE is on thorchain , pay bond doesn't need to be active node
	// in fact , usually the node will not be active at the time it bond

	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node account(%s)", msg.NodeAddress))
	}

	if nodeAccount.Status == NodeReady {
		return ErrInternal(err, "cannot add bond while node is ready status")
	}

	if nodeAccount.Status == NodeActive {
		validatorMaxRewardRatio, err := h.mgr.Keeper().GetMimir(ctx, constants.ValidatorMaxRewardRatio.String())
		if validatorMaxRewardRatio < 0 || err != nil {
			validatorMaxRewardRatio = h.mgr.GetConstants().GetInt64Value(constants.ValidatorMaxRewardRatio)
		}

		if validatorMaxRewardRatio > 1 {
			retiring, err := h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, RetiringVault)
			if err != nil {
				return err
			}

			if len(retiring) == 0 {
				return ErrInternal(err, "cannot add bond while the network is not churning")
			}

		}
	}

	bond := msg.Bond.Add(nodeAccount.Bond)

	maxBond, err := h.mgr.Keeper().GetMimir(ctx, "MaximumBondInRune")
	if maxBond > 0 && err == nil {
		maxValidatorBond := cosmos.NewUint(uint64(maxBond))
		if bond.GT(maxValidatorBond) {
			return cosmos.ErrUnknownRequest(fmt.Sprintf("too much bond, max validator bond (%s), bond(%s)", maxValidatorBond.String(), bond))
		}
	}

	return nil
}

func (h BondHandler) handleV1(ctx cosmos.Context, msg MsgBond) error {
	// THORNode will not have pub keys at the moment, so have to leave it empty
	emptyPubKeySet := common.PubKeySet{
		Secp256k1: common.EmptyPubKey,
		Ed25519:   common.EmptyPubKey,
	}

	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node account(%s)", msg.NodeAddress))
	}

	if nodeAccount.Status == NodeUnknown {
		// white list the given bep address
		nodeAccount = NewNodeAccount(msg.NodeAddress, NodeWhiteListed, emptyPubKeySet, "", cosmos.ZeroUint(), msg.BondAddress, common.BlockHeight(ctx))
		ctx.EventManager().EmitEvent(
			cosmos.NewEvent("new_node",
				cosmos.NewAttribute("address", msg.NodeAddress.String()),
			))
	}
	nodeAccount.Bond = nodeAccount.Bond.Add(msg.Bond)

	acct := h.mgr.Keeper().GetAccount(ctx, msg.NodeAddress)

	// when node bond for the first time , send 1 RUNE to node address
	// so as the node address will be created on THORChain otherwise node account won't be able to send tx
	if acct == nil && nodeAccount.Bond.GTE(cosmos.NewUint(common.One)) {
		coin := common.NewCoin(common.RuneNative, cosmos.NewUint(common.One))
		if err := h.mgr.Keeper().SendFromModuleToAccount(ctx, BondName, msg.NodeAddress, common.NewCoins(coin)); err != nil {
			ctx.Logger().Error("fail to send one RUNE to node address", "error", err)
		}
		nodeAccount.Bond = common.SafeSub(nodeAccount.Bond, cosmos.NewUint(common.One))
	}

	if err := h.mgr.Keeper().SetNodeAccount(ctx, nodeAccount); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save node account(%s)", nodeAccount.String()))
	}

	bondEvent := NewEventBond(msg.Bond, BondPaid, msg.TxIn)
	if err := h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
		return cosmos.Wrapf(errFailSaveEvent, "fail to emit bond event: %w", err)
	}

	return nil
}

func (h BondHandler) handleV47(ctx cosmos.Context, msg MsgBond) error {
	// THORNode will not have pub keys at the moment, so have to leave it empty
	emptyPubKeySet := common.PubKeySet{
		Secp256k1: common.EmptyPubKey,
		Ed25519:   common.EmptyPubKey,
	}

	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node account(%s)", msg.NodeAddress))
	}

	if nodeAccount.Status == NodeUnknown {
		// white list the given bep address
		nodeAccount = NewNodeAccount(msg.NodeAddress, NodeWhiteListed, emptyPubKeySet, "", cosmos.ZeroUint(), msg.BondAddress, common.BlockHeight(ctx))
		ctx.EventManager().EmitEvent(
			cosmos.NewEvent("new_node",
				cosmos.NewAttribute("address", msg.NodeAddress.String()),
			))
	}
	nodeAccount.Bond = nodeAccount.Bond.Add(msg.Bond)

	acct := h.mgr.Keeper().GetAccount(ctx, msg.NodeAddress)

	// when node bond for the first time , send 1 RUNE to node address
	// so as the node address will be created on THORChain otherwise node account won't be able to send tx
	if acct == nil && nodeAccount.Bond.GTE(cosmos.NewUint(common.One)) {
		coin := common.NewCoin(common.RuneNative, cosmos.NewUint(common.One))
		if err := h.mgr.Keeper().SendFromModuleToAccount(ctx, BondName, msg.NodeAddress, common.NewCoins(coin)); err != nil {
			ctx.Logger().Error("fail to send one RUNE to node address", "error", err)
		}
		nodeAccount.Bond = common.SafeSub(nodeAccount.Bond, cosmos.NewUint(common.One))
		msg.Bond = common.SafeSub(msg.Bond, cosmos.NewUint(common.One))
	}

	if err := h.mgr.Keeper().SetNodeAccount(ctx, nodeAccount); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save node account(%s)", nodeAccount.String()))
	}

	bondEvent := NewEventBond(msg.Bond, BondPaid, msg.TxIn)
	if err := h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
		return cosmos.Wrapf(errFailSaveEvent, "fail to emit bond event: %w", err)
	}

	return nil
}

func (h BondHandler) handleV68(ctx cosmos.Context, msg MsgBond) error {
	// THORNode will not have pub keys at the moment, so have to leave it empty
	emptyPubKeySet := common.PubKeySet{
		Secp256k1: common.EmptyPubKey,
		Ed25519:   common.EmptyPubKey,
	}

	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node account(%s)", msg.NodeAddress))
	}

	if nodeAccount.Status == NodeUnknown {
		// white list the given bep address
		nodeAccount = NewNodeAccount(msg.NodeAddress, NodeWhiteListed, emptyPubKeySet, "", cosmos.ZeroUint(), msg.BondAddress, common.BlockHeight(ctx))
		ctx.EventManager().EmitEvent(
			cosmos.NewEvent("new_node",
				cosmos.NewAttribute("address", msg.NodeAddress.String()),
			))
	}
	nodeAccount.Bond = nodeAccount.Bond.Add(msg.Bond)

	acct := h.mgr.Keeper().GetAccount(ctx, msg.NodeAddress)

	// when node bond for the first time , send 1 RUNE to node address
	// so as the node address will be created on THORChain otherwise node account won't be able to send tx
	if acct == nil && nodeAccount.Bond.GTE(cosmos.NewUint(common.One)) {
		coin := common.NewCoin(common.RuneNative, cosmos.NewUint(common.One))
		if err := h.mgr.Keeper().SendFromModuleToAccount(ctx, BondName, msg.NodeAddress, common.NewCoins(coin)); err != nil {
			ctx.Logger().Error("fail to send one RUNE to node address", "error", err)
			nodeAccount.Status = NodeUnknown
		}
		nodeAccount.Bond = common.SafeSub(nodeAccount.Bond, cosmos.NewUint(common.One))
		msg.Bond = common.SafeSub(msg.Bond, cosmos.NewUint(common.One))
	}

	if err := h.mgr.Keeper().SetNodeAccount(ctx, nodeAccount); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save node account(%s)", nodeAccount.String()))
	}

	bondEvent := NewEventBond(msg.Bond, BondPaid, msg.TxIn)
	if err := h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
		return cosmos.Wrapf(errFailSaveEvent, "fail to emit bond event: %w", err)
	}

	return nil
}
