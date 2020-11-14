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

// AddLiquidityHandler is to handle add liquidity
type AddLiquidityHandler struct {
	keeper keeper.Keeper
	mgr    Manager
}

// NewAddLiquidityHandler create a new instance of AddLiquidityHandler
func NewAddLiquidityHandler(keeper keeper.Keeper, mgr Manager) AddLiquidityHandler {
	return AddLiquidityHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

func (h AddLiquidityHandler) validate(ctx cosmos.Context, msg MsgAddLiquidity, version semver.Version, constAccessor constants.ConstantValues) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg, constAccessor)
	}
	return errBadVersion
}

func (h AddLiquidityHandler) validateV1(ctx cosmos.Context, msg MsgAddLiquidity, constAccessor constants.ConstantValues) error {
	if err := msg.ValidateBasic(); err != nil {
		ctx.Logger().Error(err.Error())
		return errAddLiquidityFailValidation
	}

	ensureLiquidityNoLargerThanBond := constAccessor.GetBoolValue(constants.StrictBondLiquidityRatio)
	// the following  only applicable for chaosnet
	totalLiquidityRUNE, err := h.getTotalLiquidityRUNE(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total liquidity RUNE")
	}

	// total liquidity RUNE after current add liquidity
	totalLiquidityRUNE = totalLiquidityRUNE.Add(msg.RuneAmount)
	maximumLiquidityRune, err := h.keeper.GetMimir(ctx, constants.MaximumLiquidityRune.String())
	if maximumLiquidityRune < 0 || err != nil {
		maximumLiquidityRune = constAccessor.GetInt64Value(constants.MaximumLiquidityRune)
	}
	if maximumLiquidityRune > 0 {
		if totalLiquidityRUNE.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
			return errAddLiquidityRUNEOverLimit
		}
	}

	if !ensureLiquidityNoLargerThanBond {
		return nil
	}
	totalBondRune, err := h.getTotalBond(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total bond RUNE")
	}
	if totalLiquidityRUNE.GT(totalBondRune) {
		ctx.Logger().Info(fmt.Sprintf("total liquidity RUNE(%s) is more than total Bond(%s)", totalLiquidityRUNE, totalBondRune))
		return errAddLiquidityRUNEMoreThanBond
	}

	return nil
}

// Run execute the handler
func (h AddLiquidityHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(MsgAddLiquidity)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("received add liquidity request",
		"asset", msg.Asset.String(),
		"tx", msg.Tx)
	if err := h.validate(ctx, msg, version, constAccessor); err != nil {
		ctx.Logger().Error("msg add liquidity fail validation", "error", err)
		return nil, err
	}

	if err := h.handle(ctx, msg, version, constAccessor); err != nil {
		ctx.Logger().Error("fail to process msg add liquidity", "error", err)
		return nil, err
	}

	return &cosmos.Result{}, nil
}

func (h AddLiquidityHandler) handle(ctx cosmos.Context, msg MsgAddLiquidity, version semver.Version, constAccessor constants.ConstantValues) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, version, constAccessor)
	}
	return errBadVersion
}

func (h AddLiquidityHandler) handleV1(ctx cosmos.Context, msg MsgAddLiquidity, version semver.Version, constAccessor constants.ConstantValues) (errResult error) {
	pool, err := h.keeper.GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}

	if pool.IsEmpty() {
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
	return h.addLiquidityV1(
		ctx,
		msg.Asset,
		msg.RuneAmount,
		msg.AssetAmount,
		msg.RuneAddress,
		msg.AssetAddress,
		msg.Tx.ID,
		constAccessor)
}

// validateAddLiquidityMessage is to do some validation, and make sure it is legit
func (h AddLiquidityHandler) validateAddLiquidityMessage(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset, requestTxHash common.TxID, runeAddr, assetAddr common.Address) error {
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

func (h AddLiquidityHandler) addLiquidityV1(ctx cosmos.Context,
	asset common.Asset,
	addRuneAmount, addAssetAmount cosmos.Uint,
	runeAddr, assetAddr common.Address,
	requestTxHash common.TxID,
	constAccessor constants.ConstantValues) error {
	ctx.Logger().Info(fmt.Sprintf("%s staking %s %s", asset, addRuneAmount, addAssetAmount))
	if err := h.validateAddLiquidityMessage(ctx, h.keeper, asset, requestTxHash, runeAddr, assetAddr); err != nil {
		return fmt.Errorf("add liquidity message fail validation: %w", err)
	}
	if addRuneAmount.IsZero() && addAssetAmount.IsZero() {
		return cosmos.ErrUnknownRequest("both rune and asset is zero")
	}
	if runeAddr.IsEmpty() {
		return cosmos.ErrUnknownRequest("rune address cannot be empty")
	}

	pool, err := h.keeper.GetPool(ctx, asset)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get pool(%s)", asset))
	}
	emitPoolEnabledEvent := false
	// if THORNode have no balance, set the default pool status
	if pool.BalanceAsset.IsZero() && pool.BalanceRune.IsZero() {
		defaultPoolStatus := PoolEnabled.String()
		// if the pools is for gas asset on the chain, automatically enable it
		if !pool.Asset.Equals(pool.Asset.Chain.GetGasAsset()) {
			defaultPoolStatus = constAccessor.GetStringValue(constants.DefaultPoolStatus)
		}
		pool.Status = GetPoolStatus(defaultPoolStatus)
		if pool.Status == PoolEnabled {
			emitPoolEnabledEvent = true
		}
	}

	su, err := h.keeper.GetStaker(ctx, asset, runeAddr)
	if err != nil {
		return ErrInternal(err, "fail to get staker")
	}

	su.LastStakeHeight = common.BlockHeight(ctx)
	if su.RuneAddress.IsEmpty() {
		su.RuneAddress = runeAddr
	}
	if su.AssetAddress.IsEmpty() {
		su.AssetAddress = assetAddr
	} else {
		if !su.AssetAddress.Equals(assetAddr) {
			// mismatch of asset addresses from what is known to the address
			// given. Refund it.
			return errAddLiquidityMismatchAssetAddr
		}
	}

	if !asset.Chain.Equals(common.RuneAsset().Chain) {
		if addAssetAmount.IsZero() {
			su.PendingRune = su.PendingRune.Add(addRuneAmount)
			su.PendingTxID = requestTxHash
			h.keeper.SetStaker(ctx, su)
			// cross chain liquidity , this is the first tx
			return nil
		}
		addRuneAmount = su.PendingRune.Add(addRuneAmount)
		su.PendingRune = cosmos.ZeroUint()
	}

	ctx.Logger().Info(fmt.Sprintf("Pre-Pool: %sRUNE %sAsset", pool.BalanceRune, pool.BalanceAsset))
	ctx.Logger().Info(fmt.Sprintf("Adding Liquidity: %sRUNE %sAsset", addRuneAmount, addAssetAmount))

	balanceRune := pool.BalanceRune
	balanceAsset := pool.BalanceAsset

	oldPoolUnits := pool.PoolUnits
	newPoolUnits, liquidityUnits, err := calculatePoolUnitsV1(oldPoolUnits, balanceRune, balanceAsset, addRuneAmount, addAssetAmount)
	if err != nil {
		return ErrInternal(err, "fail to calculate pool unit")
	}

	ctx.Logger().Info(fmt.Sprintf("current pool units : %s ,liquidity units : %s", newPoolUnits, liquidityUnits))
	poolRune := balanceRune.Add(addRuneAmount)
	poolAsset := balanceAsset.Add(addAssetAmount)
	pool.PoolUnits = newPoolUnits
	pool.BalanceRune = poolRune
	pool.BalanceAsset = poolAsset
	ctx.Logger().Info(fmt.Sprintf("Post-Pool: %sRUNE %sAsset", pool.BalanceRune, pool.BalanceAsset))
	if err := h.keeper.SetPool(ctx, pool); err != nil {
		return ErrInternal(err, "fail to save pool")
	}
	if emitPoolEnabledEvent {
		poolEvent := NewEventPool(pool.Asset, PoolEnabled)
		if err := h.mgr.EventMgr().EmitEvent(ctx, poolEvent); err != nil {
			ctx.Logger().Error("fail to emit pool event", "error", err)
		}
	}

	acc, err := su.RuneAddress.AccAddress()
	if err != nil {
		return ErrInternal(err, "fail to convert rune address")
	}
	err = h.keeper.AddStake(ctx, common.NewCoin(pool.Asset.LiquidityAsset(), liquidityUnits), acc)
	if err != nil {
		return ErrInternal(err, "fail to add liquidity")
	}
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

	evt := NewEventAddLiquidity(asset, liquidityUnits, runeAddr, addRuneAmount, addAssetAmount, runeTxID, assetTxID, assetAddr)
	if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
		return ErrInternal(err, "fail to emit add liquidity event")
	}
	return nil
}

// r = rune staked;
// a = asset staked
// R = rune Balance (before)
// A = asset Balance (before)
// P = existing Pool Units
// slipAdjustment = (1 - ABS((R a - r A)/((2 r + R) (a + A))))
// units = ((P (a R + A r))/(2 A R))*slidAdjustment
func calculatePoolUnitsV1(oldPoolUnits, poolRune, poolAsset, addRune, addAsset cosmos.Uint) (cosmos.Uint, cosmos.Uint, error) {
	if addRune.Add(poolRune).IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), errors.New("total RUNE in the pool is zero")
	}
	if addAsset.Add(poolAsset).IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), errors.New("total asset in the pool is zero")
	}
	if poolRune.IsZero() || poolAsset.IsZero() {
		return addRune, addRune, nil
	}
	P := cosmos.NewDecFromBigInt(oldPoolUnits.BigInt())
	R := cosmos.NewDecFromBigInt(poolRune.BigInt())
	A := cosmos.NewDecFromBigInt(poolAsset.BigInt())
	r := cosmos.NewDecFromBigInt(addRune.BigInt())
	a := cosmos.NewDecFromBigInt(addAsset.BigInt())

	// (2 r + R) (a + A)
	slipAdjDenominator := (r.MulInt64(2).Add(R)).Mul(a.Add(A))
	// ABS((R a - r A)/((2 r + R) (a + A)))
	var slipAdjustment cosmos.Dec
	if R.Mul(a).GT(r.Mul(A)) {
		slipAdjustment = R.Mul(a).Sub(r.Mul(A)).Quo(slipAdjDenominator)
	} else {
		slipAdjustment = r.Mul(A).Sub(R.Mul(a)).Quo(slipAdjDenominator)
	}
	// (1 - ABS((R a - r A)/((2 r + R) (a + A))))
	slipAdjustment = cosmos.NewDec(1).Sub(slipAdjustment)

	// ((P (a R + A r))
	numerator := P.Mul(a.Mul(R).Add(A.Mul(r)))
	// 2AR
	denominator := cosmos.NewDec(2).Mul(A).Mul(R)
	liquidityUnits := numerator.Quo(denominator).Mul(slipAdjustment)
	newPoolUnit := P.Add(liquidityUnits)

	return cosmos.NewUintFromBigInt(newPoolUnit.TruncateInt().BigInt()), cosmos.NewUintFromBigInt(liquidityUnits.TruncateInt().BigInt()), nil
}

// getTotalBond
func (h AddLiquidityHandler) getTotalBond(ctx cosmos.Context) (cosmos.Uint, error) {
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

// getTotalLiquidityRUNE we have in all pools
func (h AddLiquidityHandler) getTotalLiquidityRUNE(ctx cosmos.Context) (cosmos.Uint, error) {
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
