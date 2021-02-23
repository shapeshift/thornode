package thorchain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// UnBondHandler a handler to process unbond request
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

	na, err := h.keeper.GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node account(%s)", msg.NodeAddress))
	}

	if !na.BondAddress.Equals(msg.TxIn.FromAddress) {
		return cosmos.ErrUnauthorized(fmt.Sprintf("%s are not authorized to manage %s", msg.TxIn.FromAddress, msg.NodeAddress))
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
		canUnbond := true
		totalRuneValue := cosmos.ZeroUint()
		for _, c := range ygg.Coins {
			if c.Amount.IsZero() {
				continue
			}
			if !c.Asset.IsGasAsset() {
				// None gas asset has not been sent back to asgard in full
				canUnbond = false
				break
			}
			chain := c.GetAsset().Chain
			maxGas, err := h.mgr.GasMgr().GetMaxGas(ctx, chain)
			if err != nil {
				ctx.Logger().Error("fail to get max gas", "chain", chain, "error", err)
				canUnbond = false
				break
			}
			if c.Amount.GT(maxGas.Amount) {
				canUnbond = false
			}
			pool, err := h.keeper.GetPool(ctx, c.Asset)
			if err != nil {
				ctx.Logger().Error("fail to get pool", "asset", c.Asset, "error", err)
				canUnbond = false
				break
			}
			totalRuneValue = totalRuneValue.Add(pool.AssetValueInRune(c.Amount))
		}
		if !canUnbond {
			return cosmos.ErrUnknownRequest("cannot unbond while yggdrasil vault still has funds")
		}
		totalRuneValue = totalRuneValue.MulUint64(3).QuoUint64(2)
		totalAmountCanBeUnbond := common.SafeSub(na.Bond, totalRuneValue)
		if msg.Amount.GT(totalAmountCanBeUnbond) {
			return cosmos.ErrUnknownRequest(fmt.Sprintf("unbond amount %s is more than %s , not allowed", msg.Amount, totalAmountCanBeUnbond))
		}
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
	msg, ok := m.(*MsgUnBond)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive MsgUnBond",
		"node address", msg.NodeAddress,
		"request hash", msg.TxIn.ID,
		"amount", msg.Amount)
	if err := h.validate(ctx, *msg, version, constAccessor); err != nil {
		ctx.Logger().Error("msg unbond fail validation", "error", err)
		return nil, err
	}
	if version.GTE(semver.MustParse("0.1.0")) {
		if err := h.handleV1(ctx, *msg, version, constAccessor); err != nil {
			ctx.Logger().Error("fail to process msg unbond", "error", err)
			return nil, err
		}
	}

	return &cosmos.Result{}, nil
}

func (h UnBondHandler) handleV1(ctx cosmos.Context, msg MsgUnBond, version semver.Version, constAccessor constants.ConstantValues) error {
	na, err := h.keeper.GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node account(%s)", msg.NodeAddress))
	}
	vaults, err := h.keeper.GetAsgardVaultsByStatus(ctx, RetiringVault)
	if err != nil {
		return ErrInternal(err, "fail to get retiring vault")
	}
	isMemberOfRetiringVault := false
	for _, v := range vaults {
		if v.GetMembership().Contains(na.PubKeySet.Secp256k1) {
			isMemberOfRetiringVault = true
			ctx.Logger().Info("node account is still part of the retiring vault,can't return bond yet")
			break
		}
	}
	if isMemberOfRetiringVault {
		return ErrInternal(err, "fail to unbond, still part of the retiring vault")
	}
	if err := refundBond(ctx, msg.TxIn, msg.Amount, &na, h.keeper, h.mgr); err != nil {
		return ErrInternal(err, "fail to unbond")
	}

	coin := msg.TxIn.Coins.GetCoin(common.RuneAsset())
	if !coin.IsEmpty() {
		na.Bond = na.Bond.Add(coin.Amount)
		if err := h.keeper.SetNodeAccount(ctx, na); err != nil {
			return ErrInternal(err, "fail to save node account to key value store")
		}
	}

	return nil
}
