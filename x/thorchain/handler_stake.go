package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// StakeHandler is to handle stake
type StakeHandler struct {
	keeper                Keeper
	versionedEventManager VersionedEventManager
}

// NewStakeHandler create a new instance of StakeHandler
func NewStakeHandler(keeper Keeper, versionedEventManager VersionedEventManager) StakeHandler {
	return StakeHandler{
		keeper:                keeper,
		versionedEventManager: versionedEventManager,
	}
}

func (h StakeHandler) validate(ctx cosmos.Context, msg MsgSetStakeData, version semver.Version, constAccessor constants.ConstantValues) cosmos.Error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg, constAccessor)
	}
	return errBadVersion
}

func (h StakeHandler) validateV1(ctx cosmos.Context, msg MsgSetStakeData, constAccessor constants.ConstantValues) cosmos.Error {
	if err := msg.ValidateBasic(); err != nil {
		ctx.Logger().Error(err.ABCILog())
		return cosmos.NewError(DefaultCodespace, CodeStakeFailValidation, err.Error())
	}
	if !isSignedByActiveNodeAccounts(ctx, h.keeper, msg.GetSigners()) {
		return cosmos.ErrUnauthorized("msg is not signed by an active node account")
	}

	ensureStakeNoLargerThanBond := constAccessor.GetBoolValue(constants.StrictBondStakeRatio)
	// the following  only applicable for chaosnet
	totalStakeRUNE, err := h.getTotalStakeRUNE(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get total staked RUNE", err)
		return cosmos.ErrInternal("fail to get total staked RUNE")
	}

	// total staked RUNE after current stake
	totalStakeRUNE = totalStakeRUNE.Add(msg.RuneAmount)
	maximumStakeRune, err := h.keeper.GetMimir(ctx, constants.MaximumStakeRune.String())
	if maximumStakeRune < 0 || err != nil {
		maximumStakeRune = constAccessor.GetInt64Value(constants.MaximumStakeRune)
	}
	if maximumStakeRune > 0 {
		if totalStakeRUNE.GT(cosmos.NewUint(uint64(maximumStakeRune))) {
			return cosmos.NewError(DefaultCodespace, CodeStakeRUNEOverLimit, "total staked RUNE is more than %d", maximumStakeRune)
		}
	}

	if !ensureStakeNoLargerThanBond {
		return nil
	}
	totalBondRune, err := h.getTotalBond(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get total bond RUNE", err)
		return cosmos.ErrInternal("fail to get total bond RUNE")
	}
	if totalStakeRUNE.GT(totalBondRune) {
		ctx.Logger().Info(fmt.Sprintf("total stake RUNE(%s) is more than total Bond(%s)", totalStakeRUNE, totalBondRune))
		return cosmos.NewError(DefaultCodespace, CodeStakeRUNEMoreThanBond, "total stake RUNE is more than bond")
	}

	return nil
}

// Run execute the handler
func (h StakeHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) cosmos.Result {
	msg, ok := m.(MsgSetStakeData)
	if !ok {
		return errInvalidMessage.Result()
	}
	ctx.Logger().Info("received stake request",
		"asset", msg.Asset.String(),
		"tx", msg.Tx)
	if err := h.validate(ctx, msg, version, constAccessor); err != nil {
		ctx.Logger().Error("msg stake fail validation", "error", err)
		return err.Result()
	}

	if err := h.handle(ctx, msg, version, constAccessor); err != nil {
		ctx.Logger().Error("fail to process msg stake", "error", err)
		return err.Result()
	}

	return cosmos.Result{
		Code:      cosmos.CodeOK,
		Codespace: DefaultCodespace,
	}
}

func (h StakeHandler) handle(ctx cosmos.Context, msg MsgSetStakeData, version semver.Version, constAccessor constants.ConstantValues) (errResult cosmos.Error) {
	pool, err := h.keeper.GetPool(ctx, msg.Asset)
	if err != nil {
		return cosmos.ErrInternal(fmt.Errorf("fail to get pool: %w", err).Error())
	}

	if pool.Empty() {
		ctx.Logger().Info("pool doesn't exist yet, create a new one", "symbol", msg.Asset.String(), "creator", msg.RuneAddress)
		pool.Asset = msg.Asset
		if err := h.keeper.SetPool(ctx, pool); err != nil {
			return cosmos.ErrInternal(fmt.Errorf("fail to save pool to key value store: %w", err).Error())
		}
	}
	if err := pool.EnsureValidPoolStatus(msg); err != nil {
		ctx.Logger().Error("fail to check pool status", "error", err)
		return cosmos.NewError(DefaultCodespace, CodeInvalidPoolStatus, err.Error())
	}
	stakeUnits, err := stake(
		ctx,
		h.keeper,
		msg.Asset,
		msg.RuneAmount,
		msg.AssetAmount,
		msg.RuneAddress,
		msg.AssetAddress,
		msg.Tx.ID,
		constAccessor,
	)
	if err != nil {
		return cosmos.ErrUnknownRequest(fmt.Errorf("fail to process stake request: %w", err).Error())
	}

	if err := h.processStakeEvent(ctx, version, msg, stakeUnits); err != nil {
		return cosmos.ErrInternal(fmt.Errorf("fail to save stake event: %w", err).Error())
	}

	return nil
}

func (h StakeHandler) processStakeEvent(ctx cosmos.Context, version semver.Version, msg MsgSetStakeData, stakeUnits cosmos.Uint) error {
	eventMgr, err := h.versionedEventManager.GetEventManager(ctx, version)
	if err != nil {
		return errFailGetEventManager
	}

	stakeEvt := NewEventStake(
		msg.Asset,
		stakeUnits,
		msg.Tx)
	return eventMgr.EmitStakeEvent(ctx, h.keeper, msg.Tx, stakeEvt)
}

// getTotalBond
func (h StakeHandler) getTotalBond(ctx cosmos.Context) (cosmos.Uint, error) {
	nodeAccounts, err := h.keeper.ListNodeAccountsWithBond(ctx)
	if err != nil {
		return cosmos.ZeroUint(), err
	}
	total := cosmos.ZeroUint()
	for _, na := range nodeAccounts {
		if na.Status == NodeDisabled {
			continue
		}
		total = total.Add(na.Bond)
	}
	return total, nil
}

// getTotalStakeRUNE we have in all pools
func (h StakeHandler) getTotalStakeRUNE(ctx cosmos.Context) (cosmos.Uint, error) {
	pools, err := h.keeper.GetPools(ctx)
	if err != nil {
		return cosmos.ZeroUint(), fmt.Errorf("fail to get pools from data store: %w", err)
	}
	total := cosmos.ZeroUint()
	for _, p := range pools {
		// ignore suspended pools
		if p.Status == PoolSuspended {
			continue
		}
		total = total.Add(p.BalanceRune)
	}
	return total, nil
}
