package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// LeaveHandler a handler to process leave request
// if an operator of THORChain node would like to leave and get their bond back , they have to
// send a Leave request through Binance Chain
type LeaveHandler struct {
	keeper keeper.Keeper
	mgr    Manager
}

// NewLeaveHandler create a new LeaveHandler
func NewLeaveHandler(keeper keeper.Keeper, mgr Manager) LeaveHandler {
	return LeaveHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

func (h LeaveHandler) validate(ctx cosmos.Context, msg MsgLeave, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h LeaveHandler) validateV1(ctx cosmos.Context, msg MsgLeave) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	return nil
}

// Run execute the handler
func (h LeaveHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, _ constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(MsgLeave)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive MsgLeave",
		"sender", msg.Tx.FromAddress.String(),
		"request tx hash", msg.Tx.ID)
	if err := h.validate(ctx, msg, version); err != nil {
		ctx.Logger().Error("msg leave fail validation", "error", err)
		return nil, err
	}

	if err := h.handle(ctx, msg, version); err != nil {
		ctx.Logger().Error("fail to process msg leave", "error", err)
		return nil, err
	}

	return &cosmos.Result{}, nil
}

func (h LeaveHandler) handle(ctx cosmos.Context, msg MsgLeave, version semver.Version) error {
	nodeAcc, err := h.keeper.GetNodeAccount(ctx, msg.NodeAddress)
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
		if nodeAcc.LeaveHeight == 0 {
			nodeAcc.LeaveHeight = common.BlockHeight(ctx)
		}
	} else {
		// NOTE: there is an edge case, where the first node doesn't have a
		// vault (it was destroyed when we successfully migrated funds from
		// their address to a new TSS vault
		if !h.keeper.VaultExists(ctx, nodeAcc.PubKeySet.Secp256k1) {
			if err := refundBond(ctx, msg.Tx, nodeAcc, h.keeper, h.mgr); err != nil {
				return ErrInternal(err, "fail to refund bond")
			}
		} else {
			// given the node is not active, they should not have Yggdrasil pool either
			// but let's check it anyway just in case
			vault, err := h.keeper.GetVault(ctx, nodeAcc.PubKeySet.Secp256k1)
			if err != nil {
				return ErrInternal(err, "fail to get vault pool")
			}
			if vault.IsYggdrasil() {
				if !vault.HasFunds() {
					// node is not active , they are free to leave , refund them
					if err := refundBond(ctx, msg.Tx, nodeAcc, h.keeper, h.mgr); err != nil {
						return ErrInternal(err, "fail to refund bond")
					}
				} else {
					if err := h.mgr.ValidatorMgr().RequestYggReturn(ctx, nodeAcc, h.mgr); err != nil {
						return ErrInternal(err, "fail to request yggdrasil return fund")
					}
				}
			}
		}
	}

	nodeAcc.RequestedToLeave = true
	if err := h.keeper.SetNodeAccount(ctx, nodeAcc); err != nil {
		return ErrInternal(err, "fail to save node account to key value store")
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("validator_request_leave",
			cosmos.NewAttribute("signer bnb address", msg.Tx.FromAddress.String()),
			cosmos.NewAttribute("destination", nodeAcc.BondAddress.String()),
			cosmos.NewAttribute("tx", msg.Tx.ID.String())))

	return nil
}
