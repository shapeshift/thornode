package thorchain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	keeper "gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// UnBondHandler a handler to process unbond
type UnBondHandler struct {
	keeper keeper.Keeper
	mgr    Manager
}

// NewUnBondHandler create new UnBondHandler
func NewUnBondHandler(keeper keeper.Keeper, mgr Manager) UnBondHandler {
	return UnBondHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

func (h UnBondHandler) validate(ctx cosmos.Context, msg MsgUnBond, version semver.Version, constAccessor constants.ConstantValues) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, version, msg, constAccessor)
	}
	return errBadVersion
}

func (h UnBondHandler) validateV1(ctx cosmos.Context, version semver.Version, msg MsgUnBond, constAccessor constants.ConstantValues) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	if !isSignedByActiveNodeAccounts(ctx, h.keeper, msg.GetSigners()) {
		return cosmos.ErrUnauthorized("msg is not signed by an active node account")
	}
	na, err := h.keeper.GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node account(%s)", msg.NodeAddress))
	}

	if na.Status == NodeActive {
		return cosmos.ErrUnknownRequest("cannot unbond while node is in active status")
	}

	ygg := Vault{}
	if h.keeper.VaultExists(ctx, na.PubKeySet.Secp256k1) {
		var err error
		ygg, err = h.keeper.GetVault(ctx, na.PubKeySet.Secp256k1)
		if err != nil {
			return err
		}
		if !ygg.IsYggdrasil() {
			return errors.New("this is not a Yggdrasil vault")
		}
	}

	if ygg.HasFunds() {
		return cosmos.ErrUnknownRequest("cannot unbond while yggdrasil vault still has funds")
	}

	jail, err := h.keeper.GetNodeAccountJail(ctx, msg.NodeAddress)
	if err != nil {
		// ignore this error and carry on. Don't want a jail bug causing node
		// accounts to not be able to get their funds out
		ctx.Logger().Error("fail to get node account jail", "error", err)
	}
	if jail.IsJailed(ctx) {
		return fmt.Errorf("failed to unbond due to jail status: (release height %d) %s", jail.ReleaseHeight, jail.Reason)
	}

	return nil
}

// Run execute the handler
func (h UnBondHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(MsgUnBond)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive MsgUnBond",
		"node address", msg.NodeAddress,
		"request hash", msg.TxIn.ID,
		"amount", msg.Amount)
	if err := h.validate(ctx, msg, version, constAccessor); err != nil {
		ctx.Logger().Error("msg unbond fail validation", "error", err)
		return nil, err
	}

	if err := h.handle(ctx, msg, version, constAccessor); err != nil {
		ctx.Logger().Error("fail to process msg unbond", "error", err)
		return nil, err
	}

	return &cosmos.Result{}, nil
}

func (h UnBondHandler) handle(ctx cosmos.Context, msg MsgUnBond, version semver.Version, constAccessor constants.ConstantValues) error {
	na, err := h.keeper.GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node account(%s)", msg.NodeAddress))
	}

	if err := refundBond(ctx, msg.TxIn, msg.Amount, na, h.keeper, h.mgr); err != nil {
		return ErrInternal(err, "fail to unbond")
	}

	return nil
}
