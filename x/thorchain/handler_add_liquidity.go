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
	mgr Manager
}

// NewAddLiquidityHandler create a new instance of AddLiquidityHandler
func NewAddLiquidityHandler(mgr Manager) AddLiquidityHandler {
	return AddLiquidityHandler{
		mgr: mgr,
	}
}

// Run execute the handler
func (h AddLiquidityHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgAddLiquidity)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("received add liquidity request",
		"asset", msg.Asset.String(),
		"tx", msg.Tx)
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg add liquidity fail validation", "error", err)
		return nil, err
	}

	if err := h.handle(ctx, *msg); err != nil {
		ctx.Logger().Error("fail to process msg add liquidity", "error", err)
		return nil, err
	}

	return &cosmos.Result{}, nil
}

func (h AddLiquidityHandler) validate(ctx cosmos.Context, msg MsgAddLiquidity) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.76.0")) {
		return h.validateV76(ctx, msg)
	} else if version.GTE(semver.MustParse("0.68.0")) {
		return h.validateV68(ctx, msg)
	} else if version.GTE(semver.MustParse("0.65.0")) {
		return h.validateV65(ctx, msg)
	} else if version.GTE(semver.MustParse("0.63.0")) {
		return h.validateV63(ctx, msg)
	} else if version.GTE(semver.MustParse("0.55.0")) {
		return h.validateV55(ctx, msg)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h AddLiquidityHandler) validateV76(ctx cosmos.Context, msg MsgAddLiquidity) error {
	if err := msg.ValidateBasicV63(); err != nil {
		ctx.Logger().Error(err.Error())
		return errAddLiquidityFailValidation
	}

	if msg.Asset.IsSyntheticAsset() {
		ctx.Logger().Error("asset cannot be synth", "error", errAddLiquidityFailValidation)
		return errAddLiquidityFailValidation
	}

	// Synths coins are not compatible with add liquidity
	if msg.Tx.Coins.HasSynthetic() {
		ctx.Logger().Error("asset coins cannot be synth", "error", errAddLiquidityFailValidation)
		return errAddLiquidityFailValidation
	}

	if !msg.AssetAddress.IsEmpty() && !msg.AssetAddress.IsChain(msg.Asset.Chain) {
		ctx.Logger().Error("asset address must match asset chain")
		return errAddLiquidityFailValidation
	}

	if !msg.RuneAddress.IsEmpty() && !msg.RuneAddress.IsChain(common.THORChain) {
		ctx.Logger().Error("rune address must be THORChain")
		return errAddLiquidityFailValidation
	}

	if isChainHalted(ctx, h.mgr, msg.Asset.Chain) || isLPPaused(ctx, msg.Asset.Chain, h.mgr) {
		return fmt.Errorf("unable to add liquidity while chain has paused LP actions")
	}

	ensureLiquidityNoLargerThanBond := h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
	// the following  only applicable for chaosnet
	totalLiquidityRUNE, err := h.getTotalLiquidityRUNE(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total liquidity RUNE")
	}

	// total liquidity RUNE after current add liquidity
	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	totalLiquidityRUNE = totalLiquidityRUNE.Add(msg.RuneAmount)
	totalLiquidityRUNE = totalLiquidityRUNE.Add(pool.AssetValueInRune(msg.AssetAmount))
	maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityRune.String())
	if maximumLiquidityRune < 0 || err != nil {
		maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityRune)
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
		ctx.Logger().Info("total liquidity RUNE is more than total Bond", "rune", totalLiquidityRUNE.String(), "bond", totalBondRune.String())
		return errAddLiquidityRUNEMoreThanBond
	}

	return nil
}

func (h AddLiquidityHandler) handle(ctx cosmos.Context, msg MsgAddLiquidity) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.63.0")) {
		return h.handleV63(ctx, msg)
	} else if version.GTE(semver.MustParse("0.55.0")) {
		return h.handleV55(ctx, msg)
	} else if version.GTE(semver.MustParse("0.47.0")) {
		return h.handleV47(ctx, msg)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg)
	}
	return errBadVersion
}

func (h AddLiquidityHandler) handleV63(ctx cosmos.Context, msg MsgAddLiquidity) (errResult error) {
	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}

	if pool.IsEmpty() {
		ctx.Logger().Info("pool doesn't exist yet, creating a new one...", "symbol", msg.Asset.String(), "creator", msg.RuneAddress)
		pool.Asset = msg.Asset
		if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
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
			if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
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

	if msg.AffiliateBasisPoints.IsZero() {
		return h.addLiquidity(
			ctx,
			msg.Asset,
			msg.RuneAmount,
			msg.AssetAmount,
			msg.RuneAddress,
			msg.AssetAddress,
			msg.Tx.ID,
			stage,
			h.mgr.GetConstants())
	}

	// add liquidity has an affiliate fee, add liquidity for both the user and their affiliate
	affiliateRune := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.RuneAmount)
	affiliateAsset := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.AssetAmount)
	userRune := common.SafeSub(msg.RuneAmount, affiliateRune)
	userAsset := common.SafeSub(msg.AssetAmount, affiliateAsset)

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		userRune,
		userAsset,
		msg.RuneAddress,
		msg.AssetAddress,
		msg.Tx.ID,
		stage,
		h.mgr.GetConstants(),
	)
	if err != nil {
		return err
	}

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		affiliateRune,
		affiliateAsset,
		msg.AffiliateAddress,
		common.NoAddress,
		msg.Tx.ID,
		stage,
		h.mgr.GetConstants(),
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
	pool, err := h.mgr.Keeper().GetPool(ctx, asset)
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

func (h AddLiquidityHandler) addLiquidity(ctx cosmos.Context,
	asset common.Asset,
	addRuneAmount, addAssetAmount cosmos.Uint,
	runeAddr, assetAddr common.Address,
	requestTxHash common.TxID,
	stage bool,
	constAccessor constants.ConstantValues,
) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.79.0")) {
		return h.addLiquidityV79(ctx, asset, addRuneAmount, addAssetAmount, runeAddr, assetAddr, requestTxHash, stage, constAccessor)
	} else if version.GTE(semver.MustParse("0.76.0")) {
		return h.addLiquidityV76(ctx, asset, addRuneAmount, addAssetAmount, runeAddr, assetAddr, requestTxHash, stage, constAccessor)
	} else if version.GTE(semver.MustParse("0.68.0")) {
		return h.addLiquidityV68(ctx, asset, addRuneAmount, addAssetAmount, runeAddr, assetAddr, requestTxHash, stage, constAccessor)
	} else if version.GTE(semver.MustParse("0.63.0")) {
		return h.addLiquidityV63(ctx, asset, addRuneAmount, addAssetAmount, runeAddr, assetAddr, requestTxHash, stage, constAccessor)
	} else if version.GTE(semver.MustParse("0.55.0")) {
		return h.addLiquidityV55(ctx, asset, addRuneAmount, addAssetAmount, runeAddr, assetAddr, requestTxHash, stage, constAccessor)
	} else if version.GTE(semver.MustParse("0.46.0")) {
		return h.addLiquidityV46(ctx, asset, addRuneAmount, addAssetAmount, runeAddr, assetAddr, requestTxHash, stage, constAccessor)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.addLiquidityV1(ctx, asset, addRuneAmount, addAssetAmount, runeAddr, assetAddr, requestTxHash, stage, constAccessor)
	}
	return errBadVersion
}

func (h AddLiquidityHandler) addLiquidityV79(ctx cosmos.Context,
	asset common.Asset,
	addRuneAmount, addAssetAmount cosmos.Uint,
	runeAddr, assetAddr common.Address,
	requestTxHash common.TxID,
	stage bool,
	constAccessor constants.ConstantValues,
) error {
	ctx.Logger().Info("liquidity provision", "asset", asset, "rune amount", addRuneAmount, "asset amount", addAssetAmount)
	if err := h.validateAddLiquidityMessage(ctx, h.mgr.Keeper(), asset, requestTxHash, runeAddr, assetAddr); err != nil {
		return fmt.Errorf("add liquidity message fail validation: %w", err)
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, asset)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get pool(%s)", asset))
	}
	ver := h.mgr.Keeper().GetLowestActiveVersion(ctx)
	synthSupply := h.mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	originalUnits := pool.CalcUnits(ver, synthSupply)

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
	su, err := h.mgr.Keeper().GetLiquidityProvider(ctx, asset, fetchAddr)
	if err != nil {
		return ErrInternal(err, "fail to get liquidity provider")
	}

	su.LastAddHeight = common.BlockHeight(ctx)
	if su.Units.IsZero() {
		if su.PendingTxID.IsEmpty() {
			if su.RuneAddress.IsEmpty() {
				su.RuneAddress = runeAddr
			}
			if su.AssetAddress.IsEmpty() {
				su.AssetAddress = assetAddr
			}
		}

		// ensure input addresses match LP position addresses
		if !runeAddr.Equals(su.RuneAddress) {
			return errAddLiquidityMismatchAddr
		}
		if !assetAddr.Equals(su.AssetAddress) {
			return errAddLiquidityMismatchAddr
		}
	}

	if !assetAddr.IsEmpty() && !su.AssetAddress.Equals(assetAddr) {
		// mismatch of asset addresses from what is known to the address
		// given. Refund it.
		return errAddLiquidityMismatchAddr
	}

	// get tx hashes
	runeTxID := requestTxHash
	assetTxID := requestTxHash
	if addRuneAmount.IsZero() {
		runeTxID = su.PendingTxID
	} else {
		assetTxID = su.PendingTxID
	}

	pendingRuneAmt := su.PendingRune.Add(addRuneAmount)
	pendingAssetAmt := su.PendingAsset.Add(addAssetAmount)

	// if we have an asset address and no asset amount, put the rune pending
	if stage && pendingAssetAmt.IsZero() {
		pool.PendingInboundRune = pool.PendingInboundRune.Add(addRuneAmount)
		su.PendingRune = pendingRuneAmt
		su.PendingTxID = requestTxHash
		h.mgr.Keeper().SetLiquidityProvider(ctx, su)
		if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound rune", "error", err)
		}

		// add pending liquidity event
		evt := NewEventPendingLiquidity(pool.Asset, AddPendingLiquidity, su.RuneAddress, addRuneAmount, su.AssetAddress, cosmos.ZeroUint(), requestTxHash, common.TxID(""))
		if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return ErrInternal(err, "fail to emit partial add liquidity event")
		}
		return nil
	}

	// if we have a rune address and no rune asset, put the asset in pending
	if stage && pendingRuneAmt.IsZero() {
		pool.PendingInboundAsset = pool.PendingInboundAsset.Add(addAssetAmount)
		su.PendingAsset = pendingAssetAmt
		su.PendingTxID = requestTxHash
		h.mgr.Keeper().SetLiquidityProvider(ctx, su)
		if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound asset", "error", err)
		}
		evt := NewEventPendingLiquidity(pool.Asset, AddPendingLiquidity, su.RuneAddress, cosmos.ZeroUint(), su.AssetAddress, addAssetAmount, common.TxID(""), requestTxHash)
		if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return ErrInternal(err, "fail to emit partial add liquidity event")
		}
		return nil
	}

	pool.PendingInboundRune = common.SafeSub(pool.PendingInboundRune, su.PendingRune)
	pool.PendingInboundAsset = common.SafeSub(pool.PendingInboundAsset, su.PendingAsset)
	su.PendingAsset = cosmos.ZeroUint()
	su.PendingRune = cosmos.ZeroUint()
	su.PendingTxID = ""

	ctx.Logger().Info("pre add liquidity", "pool", pool.Asset, "rune", pool.BalanceRune, "asset", pool.BalanceAsset, "LP units", pool.LPUnits, "synth units", pool.SynthUnits)
	ctx.Logger().Info("adding liquidity", "rune", addRuneAmount, "asset", addAssetAmount)

	balanceRune := pool.BalanceRune
	balanceAsset := pool.BalanceAsset

	oldPoolUnits := pool.GetPoolUnits()
	newPoolUnits, liquidityUnits, err := calculatePoolUnitsV1(oldPoolUnits, balanceRune, balanceAsset, pendingRuneAmt, pendingAssetAmt)
	if err != nil {
		return ErrInternal(err, "fail to calculate pool unit")
	}
	ctx.Logger().Info("current pool status", "pool units", newPoolUnits, "liquidity units", liquidityUnits)
	poolRune := balanceRune.Add(pendingRuneAmt)
	poolAsset := balanceAsset.Add(pendingAssetAmt)
	pool.LPUnits = pool.LPUnits.Add(liquidityUnits)
	pool.BalanceRune = poolRune
	pool.BalanceAsset = poolAsset
	ctx.Logger().Info("post add liquidity", "pool", pool.Asset, "rune", pool.BalanceRune, "asset", pool.BalanceAsset, "LP units", pool.LPUnits, "synth units", pool.SynthUnits, "add liquidity units", liquidityUnits)
	if pool.BalanceRune.IsZero() || pool.BalanceAsset.IsZero() {
		return ErrInternal(err, "pool cannot have zero rune or asset balance")
	}
	if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
		return ErrInternal(err, "fail to save pool")
	}
	if originalUnits.IsZero() && !pool.GetPoolUnits().IsZero() {
		poolEvent := NewEventPool(pool.Asset, pool.Status)
		if err := h.mgr.EventMgr().EmitEvent(ctx, poolEvent); err != nil {
			ctx.Logger().Error("fail to emit pool event", "error", err)
		}
	}

	su.Units = su.Units.Add(liquidityUnits)
	if pool.Status == PoolAvailable {
		if su.AssetDepositValue.IsZero() && su.RuneDepositValue.IsZero() {
			su.RuneDepositValue = common.GetSafeShare(su.Units, pool.GetPoolUnits(), pool.BalanceRune)
			su.AssetDepositValue = common.GetSafeShare(su.Units, pool.GetPoolUnits(), pool.BalanceAsset)
		} else {
			su.RuneDepositValue = su.RuneDepositValue.Add(common.GetSafeShare(liquidityUnits, pool.GetPoolUnits(), pool.BalanceRune))
			su.AssetDepositValue = su.AssetDepositValue.Add(common.GetSafeShare(liquidityUnits, pool.GetPoolUnits(), pool.BalanceAsset))
		}
	}
	h.mgr.Keeper().SetLiquidityProvider(ctx, su)

	evt := NewEventAddLiquidity(asset, liquidityUnits, su.RuneAddress, pendingRuneAmt, pendingAssetAmt, runeTxID, assetTxID, su.AssetAddress)
	if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
		return ErrInternal(err, "fail to emit add liquidity event")
	}
	return nil
}

// getTotalActiveBond
func (h AddLiquidityHandler) getTotalActiveBond(ctx cosmos.Context) (cosmos.Uint, error) {
	nodeAccounts, err := h.mgr.Keeper().ListValidatorsWithBond(ctx)
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
	pools, err := h.mgr.Keeper().GetPools(ctx)
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
