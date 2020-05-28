package thorchain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	keeper "gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// StakeHandler is to handle stake
type StakeHandler struct {
	keeper keeper.Keeper
	mgr    Manager
}

// NewStakeHandler create a new instance of StakeHandler
func NewStakeHandler(keeper keeper.Keeper, mgr Manager) StakeHandler {
	return StakeHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

func (h StakeHandler) validate(ctx cosmos.Context, msg MsgSetStakeData, version semver.Version, constAccessor constants.ConstantValues) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg, constAccessor)
	}
	return errBadVersion
}

func (h StakeHandler) validateV1(ctx cosmos.Context, msg MsgSetStakeData, constAccessor constants.ConstantValues) error {
	if err := msg.ValidateBasic(); err != nil {
		ctx.Logger().Error(err.Error())
		return errStakeFailValidation
	}

	ensureStakeNoLargerThanBond := constAccessor.GetBoolValue(constants.StrictBondStakeRatio)
	// the following  only applicable for chaosnet
	totalStakeRUNE, err := h.getTotalStakeRUNE(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total staked RUNE")
	}

	// total staked RUNE after current stake
	totalStakeRUNE = totalStakeRUNE.Add(msg.RuneAmount)
	maximumStakeRune, err := h.keeper.GetMimir(ctx, constants.MaximumStakeRune.String())
	if maximumStakeRune < 0 || err != nil {
		maximumStakeRune = constAccessor.GetInt64Value(constants.MaximumStakeRune)
	}
	if maximumStakeRune > 0 {
		if totalStakeRUNE.GT(cosmos.NewUint(uint64(maximumStakeRune))) {
			return errStakeRUNEOverLimit
		}
	}

	if !ensureStakeNoLargerThanBond {
		return nil
	}
	totalBondRune, err := h.getTotalBond(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total bond RUNE")
	}
	if totalStakeRUNE.GT(totalBondRune) {
		ctx.Logger().Info(fmt.Sprintf("total stake RUNE(%s) is more than total Bond(%s)", totalStakeRUNE, totalBondRune))
		return errStakeRUNEMoreThanBond
	}

	return nil
}

// Run execute the handler
func (h StakeHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(MsgSetStakeData)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("received stake request",
		"asset", msg.Asset.String(),
		"tx", msg.Tx)
	if err := h.validate(ctx, msg, version, constAccessor); err != nil {
		ctx.Logger().Error("msg stake fail validation", "error", err)
		return nil, err
	}

	if err := h.handle(ctx, msg, version, constAccessor); err != nil {
		ctx.Logger().Error("fail to process msg stake", "error", err)
		return nil, err
	}

	return &cosmos.Result{}, nil
}

func (h StakeHandler) handle(ctx cosmos.Context, msg MsgSetStakeData, version semver.Version, constAccessor constants.ConstantValues) (errResult error) {
	pool, err := h.keeper.GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}

	if pool.Empty() {
		ctx.Logger().Info("pool doesn't exist yet, creating a new one...", "symbol", msg.Asset.String(), "creator", msg.RuneAddress)
		pool.Asset = msg.Asset
		if err := h.keeper.SetPool(ctx, pool); err != nil {
			return ErrInternal(err, "fail to save pool to key value store")
		}
	}
	if err := pool.EnsureValidPoolStatus(msg); err != nil {
		ctx.Logger().Error("fail to check pool status", "error", err)
		return errInvalidPoolStatus
	}
	return h.stake(
		ctx,
		msg.Asset,
		msg.RuneAmount,
		msg.AssetAmount,
		msg.RuneAddress,
		msg.AssetAddress,
		msg.Tx.ID,
		constAccessor)
}

// validateStakeMessage is to do some validation, and make sure it is legit
func (h StakeHandler) validateStakeMessage(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset, requestTxHash common.TxID, runeAddr, assetAddr common.Address) error {
	if asset.IsEmpty() {
		return errors.New("asset is empty")
	}
	if requestTxHash.IsEmpty() {
		return errors.New("request tx hash is empty")
	}
	if asset.Chain.Equals(common.RuneAsset().Chain) {
		if runeAddr.IsEmpty() {
			return errors.New("rune address is empty")
		}
	} else {
		if assetAddr.IsEmpty() {
			return errors.New("asset address is empty")
		}
	}
	if !keeper.PoolExist(ctx, asset) {
		return fmt.Errorf("%s doesn't exist", asset)
	}
	return nil
}

func (h StakeHandler) stake(ctx cosmos.Context,
	asset common.Asset,
	stakeRuneAmount, stakeAssetAmount cosmos.Uint,
	runeAddr, assetAddr common.Address,
	requestTxHash common.TxID,
	constAccessor constants.ConstantValues) error {
	ctx.Logger().Info(fmt.Sprintf("%s staking %s %s", asset, stakeRuneAmount, stakeAssetAmount))
	if err := h.validateStakeMessage(ctx, h.keeper, asset, requestTxHash, runeAddr, assetAddr); err != nil {
		return fmt.Errorf("stake message fail validation: %w", err)
	}
	if stakeRuneAmount.IsZero() && stakeAssetAmount.IsZero() {
		return cosmos.ErrUnknownRequest("both rune and asset is zero")
	}
	if runeAddr.IsEmpty() {
		return cosmos.ErrUnknownRequest("rune address cannot be empty")
	}

	pool, err := h.keeper.GetPool(ctx, asset)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get pool(%s)", asset))
	}

	// if THORNode have no balance, set the default pool status
	if pool.BalanceAsset.IsZero() && pool.BalanceRune.IsZero() {
		defaultPoolStatus := PoolEnabled.String()

		// if we have pools that are already enabled, use the default status
		iterator := h.keeper.GetPoolIterator(ctx)
		defer iterator.Close()
		for ; iterator.Valid(); iterator.Next() {
			var p Pool
			err := h.keeper.Cdc().UnmarshalBinaryBare(iterator.Value(), &p)
			if err != nil {
				continue
			}
			if p.Status == PoolEnabled {
				defaultPoolStatus = constAccessor.GetStringValue(constants.DefaultPoolStatus)
				break
			}
		}
		pool.Status = GetPoolStatus(defaultPoolStatus)
	}

	su, err := h.keeper.GetStaker(ctx, asset, runeAddr)
	if err != nil {
		return ErrInternal(err, "fail to get staker")
	}

	su.LastStakeHeight = ctx.BlockHeight()
	if su.RuneAddress.IsEmpty() {
		su.RuneAddress = runeAddr
	}
	if su.AssetAddress.IsEmpty() {
		su.AssetAddress = assetAddr
	} else {
		if !su.AssetAddress.Equals(assetAddr) {
			// mismatch of asset addresses from what is known to the address
			// given. Refund it.
			return errStakeMismatchAssetAddr
		}
	}

	if !asset.Chain.Equals(common.RuneAsset().Chain) {
		if stakeAssetAmount.IsZero() {
			su.PendingRune = su.PendingRune.Add(stakeRuneAmount)
			su.PendingTxID = requestTxHash
			h.keeper.SetStaker(ctx, su)
			// cross chain stake , this is the first tx
			return nil
		}
		stakeRuneAmount = su.PendingRune.Add(stakeRuneAmount)
		su.PendingRune = cosmos.ZeroUint()

	}

	ctx.Logger().Info(fmt.Sprintf("Pre-Pool: %sRUNE %sAsset", pool.BalanceRune, pool.BalanceAsset))
	ctx.Logger().Info(fmt.Sprintf("Staking: %sRUNE %sAsset", stakeRuneAmount, stakeAssetAmount))

	balanceRune := pool.BalanceRune
	balanceAsset := pool.BalanceAsset

	oldPoolUnits := pool.PoolUnits
	newPoolUnits, stakerUnits, err := calculatePoolUnits(oldPoolUnits, balanceRune, balanceAsset, stakeRuneAmount, stakeAssetAmount)
	if err != nil {
		return ErrInternal(err, "fail to calculate pool unit")
	}

	ctx.Logger().Info(fmt.Sprintf("current pool units : %s ,staker units : %s", newPoolUnits, stakerUnits))
	poolRune := balanceRune.Add(stakeRuneAmount)
	poolAsset := balanceAsset.Add(stakeAssetAmount)
	pool.PoolUnits = newPoolUnits
	pool.BalanceRune = poolRune
	pool.BalanceAsset = poolAsset
	ctx.Logger().Info(fmt.Sprintf("Post-Pool: %sRUNE %sAsset", pool.BalanceRune, pool.BalanceAsset))
	if err := h.keeper.SetPool(ctx, pool); err != nil {
		return ErrInternal(err, "fail to save pool")
	}
	// maintain staker structure

	fex := su.Units
	totalStakerUnits := fex.Add(stakerUnits)
	su.Units = totalStakerUnits
	h.keeper.SetStaker(ctx, su)
	runeTxID := requestTxHash
	assetTxID := requestTxHash
	if !su.PendingTxID.IsEmpty() {
		if asset.IsRune() {
			assetTxID = su.PendingTxID
		} else {
			runeTxID = su.PendingTxID
		}
	}

	evt := NewEventStake(asset, stakerUnits, runeAddr, stakeRuneAmount, stakeAssetAmount, runeTxID, assetTxID)
	if err := h.mgr.EventMgr().EmitStakeEvent(ctx, h.keeper, evt); err != nil {
		return ErrInternal(err, "fail to emit stake event")
	}
	return nil
}

// calculatePoolUnits calculate the pool units and staker units
// returns newPoolUnit,stakerUnit, error
func calculatePoolUnits(oldPoolUnits, poolRune, poolAsset, stakeRune, stakeAsset cosmos.Uint) (cosmos.Uint, cosmos.Uint, error) {
	if stakeRune.Add(poolRune).IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), errors.New("total RUNE in the pool is zero")
	}
	if stakeAsset.Add(poolAsset).IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), errors.New("total asset in the pool is zero")
	}

	poolRuneAfter := poolRune.Add(stakeRune)
	poolAssetAfter := poolAsset.Add(stakeAsset)

	// ((R + A) * (r * A + R * a))/(4 * R * A)
	nominator1 := poolRuneAfter.Add(poolAssetAfter)
	nominator2 := stakeRune.Mul(poolAssetAfter).Add(poolRuneAfter.Mul(stakeAsset))
	denominator := cosmos.NewUint(4).Mul(poolRuneAfter).Mul(poolAssetAfter)
	stakeUnits := nominator1.Mul(nominator2).Quo(denominator)
	newPoolUnit := oldPoolUnits.Add(stakeUnits)
	return newPoolUnit, stakeUnits, nil
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
