package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type SetNodeKeysHandler struct {
	keeper Keeper
	mgr    Manager
}

func NewSetNodeKeysHandler(keeper Keeper, mgr Manager) SetNodeKeysHandler {
	return SetNodeKeysHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

func (h SetNodeKeysHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(MsgSetNodeKeys)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, msg, version); err != nil {
		return nil, err
	}
	return h.handle(ctx, msg, version, constAccessor)
}

func (h SetNodeKeysHandler) validate(ctx cosmos.Context, msg MsgSetNodeKeys, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	} else {
		ctx.Logger().Error(errInvalidVersion.Error())
		return errInvalidVersion
	}
}

func (h SetNodeKeysHandler) validateV1(ctx cosmos.Context, msg MsgSetNodeKeys) error {
	if err := msg.ValidateBasic(); err != nil {
		ctx.Logger().Error(err.Error())
		return err
	}

	nodeAccount, err := h.keeper.GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		ctx.Logger().Error("fail to get node account", "error", err, "address", msg.Signer.String())
		return notAuthorized
	}
	if nodeAccount.IsEmpty() {
		ctx.Logger().Error("unauthorized account", "address", msg.Signer.String())
		return notAuthorized
	}

	// You should not able to update node address when the node is in active mode
	// for example if they update observer address
	if nodeAccount.Status == NodeActive {
		ctx.Logger().Error(fmt.Sprintf("node %s is active, so it can't update itself", nodeAccount.NodeAddress))
		return fmt.Errorf("node is active can't update")
	}
	if nodeAccount.Status == NodeDisabled {
		err := fmt.Errorf("node %s is disabled, so it can't update itself", nodeAccount.NodeAddress)
		ctx.Logger().Error(err.Error())
		return err
	}
	if err := h.keeper.EnsureNodeKeysUnique(ctx, msg.ValidatorConsPubKey, msg.PubKeySetSet); err != nil {
		return err
	}

	return nil
}

func (h SetNodeKeysHandler) handle(ctx cosmos.Context, msg MsgSetNodeKeys, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	ctx.Logger().Info("handleMsgSetNodeKeys request")
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, version, constAccessor)
	} else {
		ctx.Logger().Error(errInvalidVersion.Error())
		return nil, errBadVersion
	}
}

// Handle a message to set node keys
func (h SetNodeKeysHandler) handleV1(ctx cosmos.Context, msg MsgSetNodeKeys, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	nodeAccount, err := h.keeper.GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		ctx.Logger().Error("fail to get node account", "error", err, "address", msg.Signer.String())
		return nil, cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorized", msg.Signer))
	}

	// Here make sure THORNode don't change the node account's bond
	nodeAccount.UpdateStatus(NodeStandby, ctx.BlockHeight())
	nodeAccount.PubKeySet = msg.PubKeySetSet
	nodeAccount.ValidatorConsPubKey = msg.ValidatorConsPubKey
	if err := h.keeper.SetNodeAccount(ctx, nodeAccount); err != nil {
		return nil, fmt.Errorf("fail to save node account: %w", err)
	}

	// Set version number
	setVersionMsg := NewMsgSetVersion(version, msg.Signer)
	setVersionHandler := NewVersionHandler(h.keeper, h.mgr)
	result, err := setVersionHandler.Run(ctx, setVersionMsg, version, constAccessor)
	if err != nil {
		ctx.Logger().Error("fail to set version", "version", version, "error", result.Log)
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("set_node_keys",
			cosmos.NewAttribute("node_address", msg.Signer.String()),
			cosmos.NewAttribute("node_secp256k1_pubkey", msg.PubKeySetSet.Secp256k1.String()),
			cosmos.NewAttribute("node_ed25519_pubkey", msg.PubKeySetSet.Ed25519.String()),
			cosmos.NewAttribute("validator_consensus_pub_key", msg.ValidatorConsPubKey)))

	return &cosmos.Result{}, nil
}
