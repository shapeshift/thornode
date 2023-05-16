package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (h VersionHandler) handleV57(ctx cosmos.Context, msg MsgSetVersion) error {
	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		return cosmos.ErrUnauthorized(fmt.Errorf("unable to find account(%s):%w", msg.Signer, err).Error())
	}

	version, err := msg.GetVersion()
	if err != nil {
		return fmt.Errorf("fail to parse version: %w", err)
	}

	if nodeAccount.GetVersion().LT(version) {
		nodeAccount.Version = version.String()
	}

	c, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || c < 0 {
		c = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
	}
	cost := cosmos.NewUint(uint64(c))
	if cost.GT(nodeAccount.Bond) {
		cost = nodeAccount.Bond
	}

	nodeAccount.Bond = common.SafeSub(nodeAccount.Bond, cost)
	if err := h.mgr.Keeper().SetNodeAccount(ctx, nodeAccount); err != nil {
		return fmt.Errorf("fail to save node account: %w", err)
	}

	// add bond to reserve
	coin := common.NewCoin(common.RuneNative, cost)
	if !cost.IsZero() {
		// cost has been deducted from node account's bond , thus just send the cost from bond to reserve
		if err := h.mgr.Keeper().SendFromModuleToModule(ctx, BondName, ReserveName, common.NewCoins(coin)); err != nil {
			ctx.Logger().Error("fail to transfer funds from bond to reserve", "error", err)
			return err
		}
	}

	tx := common.Tx{}
	tx.ID = common.BlankTxID
	tx.FromAddress = nodeAccount.BondAddress
	bondEvent := NewEventBond(cost, BondCost, tx)
	if err := h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
		return fmt.Errorf("fail to emit bond event: %w", err)
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("set_version",
			cosmos.NewAttribute("thor_address", msg.Signer.String()),
			cosmos.NewAttribute("version", msg.Version)))

	return nil
}
