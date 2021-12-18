package thorchain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// UnBondHandler a handler to process unbond request
type UnBondHandler struct {
	mgr Manager
}

// NewUnBondHandler create new UnBondHandler
func NewUnBondHandler(mgr Manager) UnBondHandler {
	return UnBondHandler{
		mgr: mgr,
	}
}

// Run execute the handler
func (h UnBondHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgUnBond)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive MsgUnBond",
		"node address", msg.NodeAddress,
		"request hash", msg.TxIn.ID,
		"amount", msg.Amount)
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg unbond fail validation", "error", err)
		return nil, err
	}
	if err := h.handle(ctx, *msg); err != nil {
		ctx.Logger().Error("msg unbond fail handler", "error", err)
		return nil, err
	}

	return &cosmos.Result{}, nil
}

func (h UnBondHandler) validate(ctx cosmos.Context, msg MsgUnBond) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.77.0")) {
		return h.validateV55(ctx, msg)
	} else if version.GTE(semver.MustParse("0.55.0")) {
		return h.validateV55(ctx, msg)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h UnBondHandler) validateV77(ctx cosmos.Context, msg MsgUnBond) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	na, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node account(%s)", msg.NodeAddress))
	}

	if na.Status == NodeActive || na.Status == NodeReady {
		return cosmos.ErrUnknownRequest("cannot unbond while node is in active or ready status")
	}

	ygg := Vault{}
	if h.mgr.Keeper().VaultExists(ctx, na.PubKeySet.Secp256k1) {
		var err error
		ygg, err = h.mgr.Keeper().GetVault(ctx, na.PubKeySet.Secp256k1)
		if err != nil {
			return err
		}
		if !ygg.IsYggdrasil() {
			return errors.New("this is not a Yggdrasil vault")
		}
	}

	jail, err := h.mgr.Keeper().GetNodeAccountJail(ctx, msg.NodeAddress)
	if err != nil {
		// ignore this error and carry on. Don't want a jail bug causing node
		// accounts to not be able to get their funds out
		ctx.Logger().Error("fail to get node account jail", "error", err)
	}
	if jail.IsJailed(ctx) {
		return fmt.Errorf("failed to unbond due to jail status: (release height %d) %s", jail.ReleaseHeight, jail.Reason)
	}

	bp, err := h.mgr.Keeper().GetBondProviders(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get bond providers(%s)", msg.NodeAddress))
	}
	from, err := msg.BondAddress.AccAddress()
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to parse bond address(%s)", msg.BondAddress))
	}
	if !bp.Has(from) && !na.BondAddress.Equals(msg.BondAddress) {
		return cosmos.ErrUnauthorized(fmt.Sprintf("%s are not authorized to manage %s", msg.BondAddress, msg.NodeAddress))
	}

	return nil
}

func (h UnBondHandler) handle(ctx cosmos.Context, msg MsgUnBond) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.77.0")) {
		return h.handleV77(ctx, msg)
	} else if version.GTE(semver.MustParse("0.76.0")) {
		return h.handleV76(ctx, msg)
	} else if version.GTE(semver.MustParse("0.55.0")) {
		return h.handleV55(ctx, msg)
	} else if version.GTE(semver.MustParse("0.46.0")) {
		return h.handleV46(ctx, msg)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg)
	}
	return errBadVersion
}

func (h UnBondHandler) handleV77(ctx cosmos.Context, msg MsgUnBond) error {
	na, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node account(%s)", msg.NodeAddress))
	}

	ygg := Vault{}
	if h.mgr.Keeper().VaultExists(ctx, na.PubKeySet.Secp256k1) {
		var err error
		ygg, err = h.mgr.Keeper().GetVault(ctx, na.PubKeySet.Secp256k1)
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
			chain := c.Asset.GetChain()
			maxGas, err := h.mgr.GasMgr().GetMaxGas(ctx, chain)
			if err != nil {
				ctx.Logger().Error("fail to get max gas", "chain", chain, "error", err)
				canUnbond = false
				break
			}
			// 10x the maxGas , if the amount of gas asset left in the yggdrasil vault is larger than 10x of the MaxGas , then we don't allow node to unbond
			if c.Amount.GT(maxGas.Amount.MulUint64(10)) {
				canUnbond = false
			}
			pool, err := h.mgr.Keeper().GetPool(ctx, c.Asset)
			if err != nil {
				ctx.Logger().Error("fail to get pool", "asset", c.Asset, "error", err)
				canUnbond = false
				break
			}
			totalRuneValue = totalRuneValue.Add(pool.AssetValueInRune(c.Amount))
		}
		if !canUnbond {
			ctx.Logger().Error("cannot unbond while yggdrasil vault still has funds")
			if err := h.mgr.ValidatorMgr().RequestYggReturn(ctx, na, h.mgr, h.mgr.GetConstants()); err != nil {
				return ErrInternal(err, "fail to request yggdrasil return fund")
			}
			return nil
		}
		totalRuneValue = totalRuneValue.MulUint64(3).QuoUint64(2)
		totalAmountCanBeUnbond := common.SafeSub(na.Bond, totalRuneValue)
		if msg.Amount.GT(totalAmountCanBeUnbond) {
			return cosmos.ErrUnknownRequest(fmt.Sprintf("unbond amount %s is more than %s , not allowed", msg.Amount, totalAmountCanBeUnbond))
		}
	}

	bondLockPeriod, err := h.mgr.Keeper().GetMimir(ctx, constants.BondLockupPeriod.String())
	if err != nil || bondLockPeriod < 0 {
		bondLockPeriod = h.mgr.GetConstants().GetInt64Value(constants.BondLockupPeriod)
	}
	if common.BlockHeight(ctx)-na.StatusSince < bondLockPeriod {
		return fmt.Errorf("node can not unbond before %d", na.StatusSince+bondLockPeriod)
	}
	vaults, err := h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, RetiringVault)
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

	from, err := cosmos.AccAddressFromBech32(msg.BondAddress.String())
	if err != nil {
		return ErrInternal(err, "fail to parse from address")
	}

	// remove/unbonding bond provider
	// check that 1) requester is node operator, 2) references
	if msg.BondAddress.Equals(na.BondAddress) && !msg.BondProviderAddress.Empty() {
		if err := refundBond(ctx, msg.TxIn, msg.BondProviderAddress, msg.Amount, &na, h.mgr); err != nil {
			return ErrInternal(err, "fail to unbond")
		}

		// remove bond provider (if bond is now zero)
		if !na.NodeAddress.Equals(msg.BondProviderAddress) {
			bp, err := h.mgr.Keeper().GetBondProviders(ctx, na.NodeAddress)
			if err != nil {
				return ErrInternal(err, fmt.Sprintf("fail to get bond providers(%s)", na.NodeAddress))
			}
			provider := bp.Get(msg.BondProviderAddress)
			if !provider.IsEmpty() && !provider.Bond.IsZero() {
				if ok := bp.Remove(msg.BondProviderAddress); ok {
					if err := h.mgr.Keeper().SetBondProviders(ctx, bp); err != nil {
						return ErrInternal(err, fmt.Sprintf("fail to save bond providers(%s)", bp.NodeAddress.String()))
					}
				}
			}
		}
	} else {
		if err := refundBond(ctx, msg.TxIn, from, msg.Amount, &na, h.mgr); err != nil {
			return ErrInternal(err, "fail to unbond")
		}
	}

	coin := msg.TxIn.Coins.GetCoin(common.RuneAsset())
	if !coin.IsEmpty() {

		na.Bond = na.Bond.Add(coin.Amount)
		if err := h.mgr.Keeper().SetNodeAccount(ctx, na); err != nil {
			return ErrInternal(err, "fail to save node account to key value store")
		}
	}

	return nil
}
