package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// SetNodeKeysHandler process MsgSetNodeKeys
// MsgSetNodeKeys is used by operators after the node account had been white list , to update the consensus pubkey and node account pubkey
type SetNodeKeysHandler struct {
	mgr Manager
}

// NewSetNodeKeysHandler create a new instance of SetNodeKeysHandler
func NewSetNodeKeysHandler(mgr Manager) SetNodeKeysHandler {
	return SetNodeKeysHandler{
		mgr: mgr,
	}
}

// Run is the main entry point to process MsgSetNodeKeys
func (h SetNodeKeysHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgSetNodeKeys)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgSetNodeKeys failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process MsgSetNodeKey", "error", err)
	}
	return result, err
}

func (h SetNodeKeysHandler) validate(ctx cosmos.Context, msg MsgSetNodeKeys) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errInvalidVersion
}

func (h SetNodeKeysHandler) validateV1(ctx cosmos.Context, msg MsgSetNodeKeys) error {
	return h.validateCurrent(ctx, msg)
}

func (h SetNodeKeysHandler) validateCurrent(ctx cosmos.Context, msg MsgSetNodeKeys) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		return cosmos.ErrUnauthorized(fmt.Sprintf("fail to get node account(%s):%s", msg.Signer.String(), err)) // notAuthorized
	}
	if nodeAccount.IsEmpty() {
		return cosmos.ErrUnauthorized(fmt.Sprintf("unauthorized account(%s)", msg.Signer))
	}

	cost, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || cost < 0 {
		cost = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
	}
	if nodeAccount.Bond.LT(cosmos.NewUint(uint64(cost))) {
		return cosmos.ErrUnauthorized("not enough bond")
	}

	// You should not able to update node address when the node is in active mode
	// for example if they update observer address
	if nodeAccount.Status == NodeActive {
		return fmt.Errorf("node %s is active, so it can't update itself", nodeAccount.NodeAddress)
	}
	if nodeAccount.Status == NodeDisabled {
		return fmt.Errorf("node %s is disabled, so it can't update itself", nodeAccount.NodeAddress)
	}
	if err := h.mgr.Keeper().EnsureNodeKeysUnique(ctx, msg.ValidatorConsPubKey, msg.PubKeySetSet); err != nil {
		return err
	}

	return nil
}

func (h SetNodeKeysHandler) handle(ctx cosmos.Context, msg MsgSetNodeKeys) (*cosmos.Result, error) {
	ctx.Logger().Info("handleMsgSetNodeKeys request")
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.57.0")) {
		return h.handleV57(ctx, msg)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg)
	}
	return nil, errBadVersion
}

// handleV1 a message to set node keys
func (h SetNodeKeysHandler) handleV1(ctx cosmos.Context, msg MsgSetNodeKeys) (*cosmos.Result, error) {
	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		ctx.Logger().Error("fail to get node account", "error", err, "address", msg.Signer.String())
		return nil, cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorized", msg.Signer))
	}

	c, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || c < 0 {
		c = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
	}
	cost := cosmos.NewUint(uint64(c))
	if cost.GT(nodeAccount.Bond) {
		cost = nodeAccount.Bond
	}
	// Here make sure THORNode don't change the node account's bond
	nodeAccount.UpdateStatus(NodeStandby, common.BlockHeight(ctx))
	nodeAccount.PubKeySet = msg.PubKeySetSet
	nodeAccount.Bond = common.SafeSub(nodeAccount.Bond, cost)
	nodeAccount.ValidatorConsPubKey = msg.ValidatorConsPubKey
	if err := h.mgr.Keeper().SetNodeAccount(ctx, nodeAccount); err != nil {
		return nil, fmt.Errorf("fail to save node account: %w", err)
	}

	// add 10 bond to reserve
	coin := common.NewCoin(common.RuneNative, cost)
	if !cost.IsZero() {
		if err := h.mgr.Keeper().SendFromAccountToModule(ctx, msg.Signer, ReserveName, common.NewCoins(coin)); err != nil {
			ctx.Logger().Error("fail to transfer funds from bond to reserve", "error", err)
			return nil, err
		}
	}

	tx := common.Tx{}
	tx.ID = common.BlankTxID
	tx.FromAddress = nodeAccount.BondAddress
	bondEvent := NewEventBond(cost, BondCost, tx)
	if err := h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
		return nil, fmt.Errorf("fail to emit bond event: %w", err)
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("set_node_keys",
			cosmos.NewAttribute("node_address", msg.Signer.String()),
			cosmos.NewAttribute("node_secp256k1_pubkey", msg.PubKeySetSet.Secp256k1.String()),
			cosmos.NewAttribute("node_ed25519_pubkey", msg.PubKeySetSet.Ed25519.String()),
			cosmos.NewAttribute("validator_consensus_pub_key", msg.ValidatorConsPubKey)))

	return &cosmos.Result{}, nil
}

// handleV57 a message to set node keys
func (h SetNodeKeysHandler) handleV57(ctx cosmos.Context, msg MsgSetNodeKeys) (*cosmos.Result, error) {
	return h.handleCurrent(ctx, msg)
}

func (h SetNodeKeysHandler) handleCurrent(ctx cosmos.Context, msg MsgSetNodeKeys) (*cosmos.Result, error) {
	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		ctx.Logger().Error("fail to get node account", "error", err, "address", msg.Signer.String())
		return nil, cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorized", msg.Signer))
	}

	c, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || c < 0 {
		c = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
	}
	cost := cosmos.NewUint(uint64(c))
	if cost.GT(nodeAccount.Bond) {
		cost = nodeAccount.Bond
	}
	// Here make sure THORNode don't change the node account's bond
	nodeAccount.UpdateStatus(NodeStandby, common.BlockHeight(ctx))
	nodeAccount.PubKeySet = msg.PubKeySetSet
	nodeAccount.Bond = common.SafeSub(nodeAccount.Bond, cost)
	nodeAccount.ValidatorConsPubKey = msg.ValidatorConsPubKey
	if err := h.mgr.Keeper().SetNodeAccount(ctx, nodeAccount); err != nil {
		return nil, fmt.Errorf("fail to save node account: %w", err)
	}

	// add 10 bond to reserve
	coin := common.NewCoin(common.RuneNative, cost)
	if !cost.IsZero() {
		if err := h.mgr.Keeper().SendFromModuleToModule(ctx, BondName, ReserveName, common.NewCoins(coin)); err != nil {
			ctx.Logger().Error("fail to transfer funds from bond to reserve", "error", err)
			return nil, err
		}
	}

	tx := common.Tx{}
	tx.ID = common.BlankTxID
	tx.FromAddress = nodeAccount.BondAddress
	bondEvent := NewEventBond(cost, BondCost, tx)
	if err := h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
		return nil, fmt.Errorf("fail to emit bond event: %w", err)
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("set_node_keys",
			cosmos.NewAttribute("node_address", msg.Signer.String()),
			cosmos.NewAttribute("node_secp256k1_pubkey", msg.PubKeySetSet.Secp256k1.String()),
			cosmos.NewAttribute("node_ed25519_pubkey", msg.PubKeySetSet.Ed25519.String()),
			cosmos.NewAttribute("validator_consensus_pub_key", msg.ValidatorConsPubKey)))

	return &cosmos.Result{}, nil
}
