package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// LeaveHandler a handler to process leave request
// if an operator of THORChain node would like to leave and get their bond back , they have to
// send a Leave request through Binance Chain
type LeaveHandler struct {
	keeper Keeper
	mgr    Manager
}

// NewLeaveHandler create a new LeaveHandler
func NewLeaveHandler(keeper Keeper, mgr Manager) LeaveHandler {
	return LeaveHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

func (h LeaveHandler) validate(ctx cosmos.Context, msg MsgLeave, version semver.Version) cosmos.Error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h LeaveHandler) validateV1(ctx cosmos.Context, msg MsgLeave) cosmos.Error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	return nil
}

// Run execute the handler
func (h LeaveHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, _ constants.ConstantValues) cosmos.Result {
	msg, ok := m.(MsgLeave)
	if !ok {
		return errInvalidMessage.Result()
	}
	ctx.Logger().Info("receive MsgLeave",
		"sender", msg.Tx.FromAddress.String(),
		"request tx hash", msg.Tx.ID)
	if err := h.validate(ctx, msg, version); err != nil {
		ctx.Logger().Error("msg leave fail validation", "error", err)
		return err.Result()
	}

	if err := h.handle(ctx, msg, version); err != nil {
		ctx.Logger().Error("fail to process msg leave", "error", err)
		return err.Result()
	}

	return cosmos.Result{
		Code:      cosmos.CodeOK,
		Codespace: DefaultCodespace,
	}
}

func (h LeaveHandler) handle(ctx cosmos.Context, msg MsgLeave, version semver.Version) cosmos.Error {
	nodeAcc, err := h.keeper.GetNodeAccountByBondAddress(ctx, msg.Tx.FromAddress)
	if err != nil {
		return cosmos.ErrInternal(fmt.Errorf("fail to get node account by bond address: %w", err).Error())
	}
	if nodeAcc.IsEmpty() {
		return cosmos.ErrUnknownRequest("node account doesn't exist")
	}
	// THORNode add the node to leave queue

	coin := msg.Tx.Coins.GetCoin(common.RuneAsset())
	if !coin.IsEmpty() {
		nodeAcc.Bond = nodeAcc.Bond.Add(coin.Amount)
	}

	if nodeAcc.Status == NodeActive {
		if nodeAcc.LeaveHeight == 0 {
			nodeAcc.LeaveHeight = ctx.BlockHeight()
		}
	} else {
		// NOTE: there is an edge case, where the first node doesn't have a
		// vault (it was destroyed when we successfully migrated funds from
		// their address to a new TSS vault
		if !h.keeper.VaultExists(ctx, nodeAcc.PubKeySet.Secp256k1) {
			if err := refundBond(ctx, msg.Tx, nodeAcc, h.keeper, h.mgr); err != nil {
				return cosmos.ErrInternal(fmt.Errorf("fail to refund bond: %w", err).Error())
			}
		} else {
			// given the node is not active, they should not have Yggdrasil pool either
			// but let's check it anyway just in case
			vault, err := h.keeper.GetVault(ctx, nodeAcc.PubKeySet.Secp256k1)
			if err != nil {
				return cosmos.ErrInternal(fmt.Errorf("fail to get vault pool: %w", err).Error())
			}
			if vault.IsYggdrasil() {
				if !vault.HasFunds() {
					// node is not active , they are free to leave , refund them
					if err := refundBond(ctx, msg.Tx, nodeAcc, h.keeper, h.mgr); err != nil {
						return cosmos.ErrInternal(fmt.Errorf("fail to refund bond: %w", err).Error())
					}
				} else {
					if err := h.mgr.ValidatorMgr().RequestYggReturn(ctx, nodeAcc, h.mgr); err != nil {
						return cosmos.ErrInternal(fmt.Errorf("fail to request yggdrasil return fund: %w", err).Error())
					}
				}
			}
		}
	}

	nodeAcc.RequestedToLeave = true
	if err := h.keeper.SetNodeAccount(ctx, nodeAcc); err != nil {
		return cosmos.ErrInternal(fmt.Errorf("fail to save node account to key value store: %w", err).Error())
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("validator_request_leave",
			cosmos.NewAttribute("signer bnb address", msg.Tx.FromAddress.String()),
			cosmos.NewAttribute("destination", nodeAcc.BondAddress.String()),
			cosmos.NewAttribute("tx", msg.Tx.ID.String())))

	return nil
}
