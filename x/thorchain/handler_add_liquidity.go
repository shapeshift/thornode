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
	if version.GTE(semver.MustParse("0.39.0")) {
		return h.validateCurrent(ctx, msg, constAccessor)
	} else if version.GTE(semver.MustParse("0.1.0")) {
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

func (h AddLiquidityHandler) validateCurrent(ctx cosmos.Context, msg MsgAddLiquidity, constAccessor constants.ConstantValues) error {
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
	totalBondRune, err := h.getTotalActiveBond(ctx)
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
	msg, ok := m.(*MsgAddLiquidity)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("received add liquidity request",
		"asset", msg.Asset.String(),
		"tx", msg.Tx)
	if err := h.validate(ctx, *msg, version, constAccessor); err != nil {
		ctx.Logger().Error("msg add liquidity fail validation", "error", err)
		return nil, err
	}

	if err := h.handle(ctx, *msg, version, constAccessor); err != nil {
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
	return h.handleCurrent(ctx, msg, version, constAccessor)
}

func (h AddLiquidityHandler) handleCurrent(ctx cosmos.Context, msg MsgAddLiquidity, version semver.Version, constAccessor constants.ConstantValues) (errResult error) {
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

	// if the pool decimals hasn't been set, it will still be 0. If we have a
	// pool asset coin, get the decimals from that transaction. This will only
	// set the decimals once.
	if pool.Decimals == 0 {
		coin := msg.GetTx().Coins.GetCoin(pool.Asset)
		if !coin.IsEmpty() {
			if coin.Decimals > 0 {
				pool.Decimals = coin.Decimals
			}
			ctx.Logger().Info("try update pool decimals", "asset", msg.Asset, "pool decimals", pool.Decimals)
			if err := h.keeper.SetPool(ctx, pool); err != nil {
				return ErrInternal(err, "fail to save pool to key value store")
			}
		}
	}

	if err := pool.EnsureValidPoolStatus(&msg); err != nil {
		ctx.Logger().Error("fail to check pool status", "error", err)
		return errInvalidPoolStatus
	}

	// figure out if we need to stage the funds and wait for a follow on
	// transaction to commit all funds atomically
	stage := false
	if !msg.AssetAddress.IsEmpty() && msg.AssetAmount.IsZero() {
		stage = true
	}
	if !msg.RuneAddress.IsEmpty() && msg.RuneAmount.IsZero() {
		stage = true
	}

	addLiquidity := h.addLiquidityV1
	if msg.AffiliateBasisPoints.IsZero() {
		return addLiquidity(
			ctx,
			msg.Asset,
			msg.RuneAmount,
			msg.AssetAmount,
			msg.RuneAddress,
			msg.AssetAddress,
			msg.Tx.ID,
			stage,
			constAccessor)
	}

	// add liquidity has an affiliate fee, add liquidity for both the user and their affiliate
	affiliateRune := common.GetShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.RuneAmount)
	affiliateAsset := common.GetShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.AssetAmount)
	userRune := common.SafeSub(msg.RuneAmount, affiliateRune)
	userAsset := common.SafeSub(msg.AssetAmount, affiliateAsset)

	err = addLiquidity(
		ctx,
		msg.Asset,
		userRune,
		userAsset,
		msg.RuneAddress,
		msg.AssetAddress,
		msg.Tx.ID,
		stage,
		constAccessor,
	)
	if err != nil {
		return err
	}

	err = addLiquidity(
		ctx,
		msg.Asset,
		affiliateRune,
		affiliateAsset,
		msg.AffiliateAddress,
		common.NoAddress,
		msg.Tx.ID,
		stage,
		constAccessor,
	)
	if err != nil {
		// we swallow this error so we don't trigger a refund, when we've
		// already successfully added liquidity for the user. If we were to
		// refund here, funds could be leaked from the network. In order, to
		// error here, we would need to revert the user addLiquidity
		// function first (TODO).
		ctx.Logger().Error("fail to add liquidity for affiliate", "address", msg.AffiliateAddress, "error", err)
	}
	return nil
}

// validateAddLiquidityMessage is to do some validation, and make sure it is legit
func (h AddLiquidityHandler) validateAddLiquidityMessage(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset, requestTxHash common.TxID, runeAddr, assetAddr common.Address) error {
	if asset.IsEmpty() {
		return errors.New("asset is empty")
	}
	if requestTxHash.IsEmpty() {
		return errors.New("request tx hash is empty")
	}
	if runeAddr.IsEmpty() && assetAddr.IsEmpty() {
		return errors.New("rune address and asset address is empty")
	}
	if !keeper.PoolExist(ctx, asset) {
		return fmt.Errorf("%s doesn't exist", asset)
	}
	pool, err := h.keeper.GetPool(ctx, asset)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get pool(%s)", asset))
	}
	if pool.Status == PoolStaged && (runeAddr.IsEmpty() || assetAddr.IsEmpty()) {
		return fmt.Errorf("cannot add single sided liquidity while a pool is staged")
	}
	return nil
}

// r = rune provided;
// a = asset provided
// R = rune Balance (before)
// A = asset Balance (before)
// P = existing Pool Units
// slipAdjustment = (1 - ABS((R a - r A)/((r + R) (a + A))))
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

	// (r + R) (a + A)
	slipAdjDenominator := (r.Add(R)).Mul(a.Add(A))
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

	pUnits := cosmos.NewUintFromBigInt(newPoolUnit.TruncateInt().BigInt())
	sUnits := cosmos.NewUintFromBigInt(liquidityUnits.TruncateInt().BigInt())

	return pUnits, sUnits, nil
}

func (h AddLiquidityHandler) addLiquidityV1(ctx cosmos.Context,
	asset common.Asset,
	addRuneAmount, addAssetAmount cosmos.Uint,
	runeAddr, assetAddr common.Address,
	requestTxHash common.TxID,
	stage bool,
	constAccessor constants.ConstantValues) error {
	ctx.Logger().Info(fmt.Sprintf("%s liquidity provision %s %s", asset, addRuneAmount, addAssetAmount))
	if err := h.validateAddLiquidityMessage(ctx, h.keeper, asset, requestTxHash, runeAddr, assetAddr); err != nil {
		return fmt.Errorf("add liquidity message fail validation: %w", err)
	}

	pool, err := h.keeper.GetPool(ctx, asset)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get pool(%s)", asset))
	}
	originalUnits := pool.PoolUnits
	// if THORNode have no balance, set the default pool status
	if originalUnits.IsZero() {
		defaultPoolStatus := PoolAvailable.String()
		// if the pools is for gas asset on the chain, automatically enable it
		if !pool.Asset.Equals(pool.Asset.GetChain().GetGasAsset()) {
			defaultPoolStatus = constAccessor.GetStringValue(constants.DefaultPoolStatus)
		}
		pool.Status = GetPoolStatus(defaultPoolStatus)
	}

	fetchAddr := runeAddr
	if fetchAddr.IsEmpty() {
		fetchAddr = assetAddr
	}
	su, err := h.keeper.GetLiquidityProvider(ctx, asset, fetchAddr)
	if err != nil {
		return ErrInternal(err, "fail to get liquidity provider")
	}

	su.LastAddHeight = common.BlockHeight(ctx)
	if su.RuneAddress.IsEmpty() {
		su.RuneAddress = runeAddr
	}
	if su.AssetAddress.IsEmpty() {
		su.AssetAddress = assetAddr
	} else {
		if !assetAddr.IsEmpty() && !su.AssetAddress.Equals(assetAddr) {
			// mismatch of asset addresses from what is known to the address
			// given. Refund it.
			return errAddLiquidityMismatchAssetAddr
		}
	}

	// get tx hashes
	runeTxID := requestTxHash
	assetTxID := requestTxHash
	if addRuneAmount.IsZero() {
		runeTxID = su.PendingTxID
	} else {
		assetTxID = su.PendingTxID
	}

	addRuneAmount = su.PendingRune.Add(addRuneAmount)
	addAssetAmount = su.PendingAsset.Add(addAssetAmount)

	// if we have an asset address and no asset amount, put the rune pending
	if stage && addAssetAmount.IsZero() {
		pool.PendingInboundRune = pool.PendingInboundRune.Add(common.SafeSub(addRuneAmount, su.PendingRune))
		su.PendingRune = addRuneAmount
		su.PendingTxID = requestTxHash
		h.keeper.SetLiquidityProvider(ctx, su)
		if err := h.keeper.SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound rune", "error", err)
		}
		return nil
	}

	// if we have a rune address and no rune asset, put the asset in pending
	if stage && addRuneAmount.IsZero() {
		pool.PendingInboundAsset = pool.PendingInboundAsset.Add(common.SafeSub(addAssetAmount, su.PendingAsset))
		su.PendingAsset = addAssetAmount
		su.PendingTxID = requestTxHash
		h.keeper.SetLiquidityProvider(ctx, su)
		if err := h.keeper.SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound asset", "error", err)
		}
		return nil
	}

	pool.PendingInboundRune = common.SafeSub(pool.PendingInboundRune, su.PendingRune)
	pool.PendingInboundAsset = common.SafeSub(pool.PendingInboundAsset, su.PendingAsset)
	su.PendingAsset = cosmos.ZeroUint()
	su.PendingRune = cosmos.ZeroUint()

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
	if pool.BalanceRune.IsZero() || pool.BalanceAsset.IsZero() {
		return ErrInternal(err, "pool cannot have zero rune or asset balance")
	}
	if err := h.keeper.SetPool(ctx, pool); err != nil {
		return ErrInternal(err, "fail to save pool")
	}
	if originalUnits.IsZero() && !pool.PoolUnits.IsZero() {
		poolEvent := NewEventPool(pool.Asset, pool.Status)
		if err := h.mgr.EventMgr().EmitEvent(ctx, poolEvent); err != nil {
			ctx.Logger().Error("fail to emit pool event", "error", err)
		}
	}

	su.Units = su.Units.Add(liquidityUnits)
	su.RuneDepositValue = su.RuneDepositValue.Add(common.GetShare(liquidityUnits, pool.PoolUnits, pool.BalanceRune))
	su.AssetDepositValue = su.AssetDepositValue.Add(common.GetShare(liquidityUnits, pool.PoolUnits, pool.BalanceAsset))
	h.keeper.SetLiquidityProvider(ctx, su)

	evt := NewEventAddLiquidity(asset, liquidityUnits, runeAddr, addRuneAmount, addAssetAmount, runeTxID, assetTxID, assetAddr)
	if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
		return ErrInternal(err, "fail to emit add liquidity event")
	}
	return nil
}

// getTotalBond
func (h AddLiquidityHandler) getTotalBond(ctx cosmos.Context) (cosmos.Uint, error) {
	nodeAccounts, err := h.keeper.ListNodeAccountsWithBond(ctx)
	if err != nil {
		return cosmos.ZeroUint(), err
	}
	total := cosmos.ZeroUint()
	for _, na := range nodeAccounts {
		if na.Status != NodeDisabled {
			continue
		}
		total = total.Add(na.Bond)
	}
	return total, nil
}

// getTotalActiveBond
func (h AddLiquidityHandler) getTotalActiveBond(ctx cosmos.Context) (cosmos.Uint, error) {
	nodeAccounts, err := h.keeper.ListNodeAccountsWithBond(ctx)
	if err != nil {
		return cosmos.ZeroUint(), err
	}
	total := cosmos.ZeroUint()
	for _, na := range nodeAccounts {
		if na.Status != NodeActive {
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
