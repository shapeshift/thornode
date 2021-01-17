package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// BondHandler a handler to process bond
type BondHandler struct {
	keeper keeper.Keeper
	mgr    Manager
}

// NewBondHandler create new BondHandler
func NewBondHandler(keeper keeper.Keeper, mgr Manager) BondHandler {
	return BondHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

func (h BondHandler) validate(ctx cosmos.Context, msg MsgBond, version semver.Version, constAccessor constants.ConstantValues) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, version, msg, constAccessor)
	}
	return errBadVersion
}

func (h BondHandler) validateV1(ctx cosmos.Context, version semver.Version, msg MsgBond, constAccessor constants.ConstantValues) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	// When RUNE is on thorchain , pay bond doesn't need to be active node
	// in fact , usually the node will not be active at the time it bond

	nodeAccount, err := h.keeper.GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node account(%s)", msg.NodeAddress))
	}

	bond := msg.Bond.Add(nodeAccount.Bond)

	maxBond, err := h.keeper.GetMimir(ctx, "MaximumBondInRune")
	if maxBond > 0 && err == nil {
		maxValidatorBond := cosmos.NewUint(uint64(maxBond))
		if bond.GT(maxValidatorBond) {
			return cosmos.ErrUnknownRequest(fmt.Sprintf("too much bond, max validator bond (%s), bond(%s)", maxValidatorBond.String(), bond))
		}
	}

	return nil
}

// Run execute the handler
func (h BondHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(*MsgBond)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive MsgBond",
		"node address", msg.NodeAddress,
		"request hash", msg.TxIn.ID,
		"bond", msg.Bond)
	if err := h.validate(ctx, *msg, version, constAccessor); err != nil {
		ctx.Logger().Error("msg bond fail validation", "error", err)
		return nil, err
	}

	result, err := h.handle(ctx, *msg, version, constAccessor)
	if err != nil {
		ctx.Logger().Error("fail to process msg bond", "error", err)
		return nil, err
	}
	bondEvent := NewEventBond(msg.Bond, BondPaid, msg.TxIn)
	if err := h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
		return nil, cosmos.Wrapf(errFailSaveEvent, "fail to emit bond event: %w", err)
	}

	return result, nil
}

func (h BondHandler) handle(ctx cosmos.Context, msg MsgBond, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	if version.GTE(semver.MustParse("0.1.0")) {
		if err := h.handleV1(ctx, msg, version, constAccessor); err != nil {
			ctx.Logger().Error("fail to process msg bond", "error", err)
			return nil, err
		}
	}
	return &cosmos.Result{}, nil
}

func (h BondHandler) handleV1(ctx cosmos.Context, msg MsgBond, version semver.Version, constAccessor constants.ConstantValues) error {
	// THORNode will not have pub keys at the moment, so have to leave it empty
	emptyPubKeySet := common.PubKeySet{
		Secp256k1: common.EmptyPubKey,
		Ed25519:   common.EmptyPubKey,
	}

	nodeAccount, err := h.keeper.GetNodeAccount(ctx, msg.NodeAddress)
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

	acct := h.keeper.GetAccount(ctx, msg.NodeAddress)

	// when node bond for the first time , send 1 RUNE to node address
	// so as the node address will be created on THORChain otherwise node account won't be able to send tx
	if acct == nil && nodeAccount.Bond.GTE(cosmos.NewUint(common.One)) {
		coin := common.NewCoin(common.RuneNative, cosmos.NewUint(common.One))
		if err := h.keeper.SendFromModuleToAccount(ctx, BondName, msg.NodeAddress, common.NewCoins(coin)); err != nil {
			ctx.Logger().Error("fail to send one RUNE to node address", "error", err)
		}
		nodeAccount.Bond = common.SafeSub(nodeAccount.Bond, cosmos.NewUint(common.One))
	}

	if err := h.keeper.SetNodeAccount(ctx, nodeAccount); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save node account(%s)", nodeAccount.String()))
	}
	return nil
}
