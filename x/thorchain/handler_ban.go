package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// BanHandler is to handle Ban message
type BanHandler struct {
	keeper Keeper
	mgr    Manager
}

// NewBanHandler create new instance of BanHandler
func NewBanHandler(keeper Keeper, mgr Manager) BanHandler {
	return BanHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

// Run is the main entry point to execute Ban logic
func (h BanHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) cosmos.Result {
	msg, ok := m.(MsgBan)
	if !ok {
		return errInvalidMessage.Result()
	}
	if err := h.validate(ctx, msg, version); err != nil {
		ctx.Logger().Error("msg ban failed validation", "error", err)
		return err.Result()
	}
	return h.handle(ctx, msg, version, constAccessor)
}

func (h BanHandler) validate(ctx cosmos.Context, msg MsgBan, version semver.Version) cosmos.Error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	} else {
		return errBadVersion
	}
}

func (h BanHandler) validateV1(ctx cosmos.Context, msg MsgBan) cosmos.Error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	if !isSignedByActiveNodeAccounts(ctx, h.keeper, msg.GetSigners()) {
		return cosmos.ErrUnauthorized(notAuthorized.Error())
	}

	return nil
}

func (h BanHandler) handle(ctx cosmos.Context, msg MsgBan, version semver.Version, constAccessor constants.ConstantValues) cosmos.Result {
	ctx.Logger().Info("handleMsgBan request", "node address", msg.NodeAddress.String())
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, constAccessor)
	} else {
		ctx.Logger().Error(errInvalidVersion.Error())
		return errBadVersion.Result()
	}
}

func (h BanHandler) handleV1(ctx cosmos.Context, msg MsgBan, constAccessor constants.ConstantValues) cosmos.Result {
	toBan, err := h.keeper.GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		err = wrapError(ctx, err, "fail to get to ban node account")
		return cosmos.ErrInternal(err.Error()).Result()
	}
	if err := toBan.IsValid(); err != nil {
		return cosmos.ErrInternal(err.Error()).Result()
	}
	if toBan.ForcedToLeave {
		// already ban, no need to ban again
		return cosmos.Result{
			Code:      cosmos.CodeOK,
			Codespace: DefaultCodespace,
		}
	}
	if toBan.Status != NodeActive {
		return cosmos.ErrInternal("cannot ban a node account that is not current active").Result()
	}

	banner, err := h.keeper.GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		err = wrapError(ctx, err, "fail to get banner node account")
		return cosmos.ErrInternal(err.Error()).Result()
	}
	if err := banner.IsValid(); err != nil {
		return cosmos.ErrInternal(err.Error()).Result()
	}

	active, err := h.keeper.ListActiveNodeAccounts(ctx)
	if err != nil {
		err = wrapError(ctx, err, "fail to get list of active node accounts")
		return cosmos.ErrInternal(err.Error()).Result()
	}

	voter, err := h.keeper.GetBanVoter(ctx, msg.NodeAddress)
	if err != nil {
		return cosmos.ErrInternal(err.Error()).Result()
	}

	if !voter.HasSigned(msg.Signer) && voter.BlockHeight == 0 {
		// take 0.1% of the minimum bond, and put it into the reserve
		minBond, err := h.keeper.GetMimir(ctx, constants.MinimumBondInRune.String())
		if minBond < 0 || err != nil {
			minBond = constAccessor.GetInt64Value(constants.MinimumBondInRune)
		}
		slashAmount := cosmos.NewUint(uint64(minBond)).QuoUint64(1000)
		banner.Bond = common.SafeSub(banner.Bond, slashAmount)

		if common.RuneAsset().Chain.Equals(common.THORChain) {
			coin := common.NewCoin(common.RuneNative, slashAmount)
			if err := h.keeper.SendFromModuleToModule(ctx, BondName, ReserveName, coin); err != nil {
				ctx.Logger().Error("fail to transfer funds from bond to reserve", "error", err)
				return err.Result()
			}
		} else {
			vaultData, err := h.keeper.GetVaultData(ctx)
			if err != nil {
				err = fmt.Errorf("fail to get vault data: %w", err)
				return cosmos.ErrInternal(err.Error()).Result()
			}
			vaultData.TotalReserve = vaultData.TotalReserve.Add(slashAmount)
			if err := h.keeper.SetVaultData(ctx, vaultData); err != nil {
				err = fmt.Errorf("fail to save vault data: %w", err)
				return cosmos.ErrInternal(err.Error()).Result()
			}
		}

		if err := h.keeper.SetNodeAccount(ctx, banner); err != nil {
			err = fmt.Errorf("fail to save node account: %w", err)
			return cosmos.ErrInternal(err.Error()).Result()
		}
	}

	voter.Sign(msg.Signer)
	h.keeper.SetBanVoter(ctx, voter)
	// doesn't have consensus yet
	if !voter.HasConsensus(active) {
		ctx.Logger().Info("not having consensus yet, return")
		return cosmos.Result{
			Code:      cosmos.CodeOK,
			Codespace: DefaultCodespace,
		}
	}

	if voter.BlockHeight > 0 {
		// ban already processed
		return cosmos.Result{
			Code:      cosmos.CodeOK,
			Codespace: DefaultCodespace,
		}
	}

	voter.BlockHeight = ctx.BlockHeight()
	h.keeper.SetBanVoter(ctx, voter)

	toBan.ForcedToLeave = true
	toBan.LeaveHeight = ctx.BlockHeight()
	if err := h.keeper.SetNodeAccount(ctx, toBan); err != nil {
		err = fmt.Errorf("fail to save node account: %w", err)
		return cosmos.ErrInternal(err.Error()).Result()
	}

	return cosmos.Result{
		Code:      cosmos.CodeOK,
		Codespace: DefaultCodespace,
	}
}
