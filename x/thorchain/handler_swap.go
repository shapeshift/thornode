package thorchain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// SwapHandler is the handler to process swap request
type SwapHandler struct {
	mgr Manager
}

// NewSwapHandler create a new instance of swap handler
func NewSwapHandler(mgr Manager) SwapHandler {
	return SwapHandler{
		mgr: mgr,
	}
}

// Run is the main entry point of swap message
func (h SwapHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgSwap)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgSwap failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to handle MsgSwap", "error", err)
		return nil, err
	}
	return result, err
}

func (h SwapHandler) validate(ctx cosmos.Context, msg MsgSwap) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.99.0")):
		return h.validateV99(ctx, msg)
	case version.GTE(semver.MustParse("1.98.0")):
		return h.validateV98(ctx, msg)
	case version.GTE(semver.MustParse("1.95.0")):
		return h.validateV95(ctx, msg)
	case version.GTE(semver.MustParse("1.93.0")):
		return h.validateV93(ctx, msg)
	case version.GTE(semver.MustParse("1.92.0")):
		return h.validateV92(ctx, msg)
	case version.GTE(semver.MustParse("1.88.1")):
		return h.validateV88(ctx, msg)
	case version.GTE(semver.MustParse("0.65.0")):
		return h.validateV65(ctx, msg)
	default:
		return errInvalidVersion
	}
}

func (h SwapHandler) validateV99(ctx cosmos.Context, msg MsgSwap) error {
	if err := msg.ValidateBasicV63(); err != nil {
		return err
	}

	target := msg.TargetAsset
	if isTradingHalt(ctx, &msg, h.mgr) {
		return errors.New("trading is halted, can't process swap")
	}
	if target.IsSyntheticAsset() {
		// the following  only applicable for chaosnet
		totalLiquidityRUNE, err := h.getTotalLiquidityRUNE(ctx)
		if err != nil {
			return ErrInternal(err, "fail to get total liquidity RUNE")
		}

		// total liquidity RUNE after current add liquidity
		if len(msg.Tx.Coins) > 0 {
			// calculate rune value on incoming swap, and add to total liquidity.
			coin := msg.Tx.Coins[0]
			runeVal := coin.Amount
			if !coin.Asset.IsRune() {
				pool, err := h.mgr.Keeper().GetPool(ctx, coin.Asset.GetLayer1Asset())
				if err != nil {
					return ErrInternal(err, "fail to get pool")
				}
				runeVal = pool.AssetValueInRune(coin.Amount)
			}
			totalLiquidityRUNE = totalLiquidityRUNE.Add(runeVal)
		}
		maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityRune.String())
		if maximumLiquidityRune < 0 || err != nil {
			maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityRune)
		}
		if maximumLiquidityRune > 0 {
			if totalLiquidityRUNE.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
				return errAddLiquidityRUNEOverLimit
			}
		}

		// fail validation if synth supply is already too high, relative to pool depth
		err = isSynthMintPaused(ctx, h.mgr, target)
		if err != nil {
			return err
		}

		ensureLiquidityNoLargerThanBond := h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
		if !ensureLiquidityNoLargerThanBond {
			return nil
		}
		securityBond, err := h.getEffectiveSecurityBond(ctx)
		if err != nil {
			return ErrInternal(err, "fail to get security bond RUNE")
		}
		if totalLiquidityRUNE.GT(securityBond) {
			ctx.Logger().Info("total liquidity RUNE is more than effective security bond", "liquidity rune", totalLiquidityRUNE, "effective security bond", securityBond)
			return errAddLiquidityRUNEMoreThanBond
		}
	}

	if len(msg.Aggregator) > 0 {
		swapOutDisabled := fetchConfigInt64(ctx, h.mgr, constants.SwapOutDexAggregationDisabled)
		if swapOutDisabled > 0 {
			return errors.New("swap out dex integration disabled")
		}
		if !msg.TargetAsset.Equals(msg.TargetAsset.Chain.GetGasAsset()) {
			return fmt.Errorf("target asset (%s) is not gas asset , can't use dex feature", msg.TargetAsset)
		}
		// validate that a referenced dex aggregator is legit
		addr, err := FetchDexAggregator(h.mgr.GetVersion(), target.Chain, msg.Aggregator)
		if err != nil {
			return err
		}
		if addr == "" {
			return fmt.Errorf("aggregator address is empty")
		}
		if len(msg.AggregatorTargetAddress) == 0 {
			return fmt.Errorf("aggregator target address is empty")
		}
	}

	return nil
}

func (h SwapHandler) handle(ctx cosmos.Context, msg MsgSwap) (*cosmos.Result, error) {
	ctx.Logger().Info("receive MsgSwap", "request tx hash", msg.Tx.ID, "source asset", msg.Tx.Coins[0].Asset, "target asset", msg.TargetAsset, "signer", msg.Signer.String())
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.98.0")):
		return h.handleV98(ctx, msg)
	case version.GTE(semver.MustParse("1.95.0")):
		return h.handleV95(ctx, msg)
	case version.GTE(semver.MustParse("1.93.0")):
		return h.handleV93(ctx, msg)
	case version.GTE(semver.MustParse("1.92.0")):
		return h.handleV92(ctx, msg)
	case version.GTE(semver.MustParse("0.81.0")):
		return h.handleV81(ctx, msg)
	default:
		return nil, errBadVersion
	}
}

func (h SwapHandler) handleV98(ctx cosmos.Context, msg MsgSwap) (*cosmos.Result, error) {
	// test that the network we are running matches the destination network
	// Don't change msg.Destination here; this line was introduced to avoid people from swapping mainnet asset,
	// but using testnet address.
	if !common.GetCurrentChainNetwork().SoftEquals(msg.Destination.GetNetwork(h.mgr.GetVersion(), msg.Destination.GetChain())) {
		return nil, fmt.Errorf("address(%s) is not same network", msg.Destination)
	}
	transactionFee := h.mgr.GasMgr().GetFee(ctx, msg.TargetAsset.GetChain(), common.RuneAsset())
	synthVirtualDepthMult, err := h.mgr.Keeper().GetMimir(ctx, constants.VirtualMultSynthsBasisPoints.String())
	if synthVirtualDepthMult < 1 || err != nil {
		synthVirtualDepthMult = h.mgr.GetConstants().GetInt64Value(constants.VirtualMultSynthsBasisPoints)
	}

	if msg.TargetAsset.IsRune() && !msg.TargetAsset.IsNativeRune() {
		return nil, fmt.Errorf("target asset can't be %s", msg.TargetAsset.String())
	}

	dexAgg := ""
	dexAggTargetAsset := ""
	if len(msg.Aggregator) > 0 {
		dexAgg, err = FetchDexAggregator(h.mgr.GetVersion(), msg.TargetAsset.Chain, msg.Aggregator)
		if err != nil {
			return nil, err
		}
	}
	dexAggTargetAsset = msg.AggregatorTargetAddress

	swapper, err := GetSwapper(h.mgr.Keeper().GetVersion())
	if err != nil {
		return nil, err
	}

	emit, _, swapErr := swapper.Swap(
		ctx,
		h.mgr.Keeper(),
		msg.Tx,
		msg.TargetAsset,
		msg.Destination,
		msg.TradeTarget,
		dexAgg,
		dexAggTargetAsset,
		msg.AggregatorTargetLimit,
		transactionFee,
		synthVirtualDepthMult,
		h.mgr)
	if swapErr != nil {
		return nil, swapErr
	}

	mem, err := ParseMemoWithTHORNames(ctx, h.mgr.Keeper(), msg.Tx.Memo)
	if err != nil {
		ctx.Logger().Error("swap handler failed to parse memo", "memo", msg.Tx.Memo, "error", err)
		return nil, err
	}
	if mem.IsType(TxAdd) {
		m, ok := mem.(AddLiquidityMemo)
		if !ok {
			return nil, fmt.Errorf("fail to cast add liquidity memo")
		}
		m.Asset = fuzzyAssetMatch(ctx, h.mgr.Keeper(), m.Asset)
		msg.Tx.Coins = common.NewCoins(common.NewCoin(m.Asset, emit))
		obTx := ObservedTx{Tx: msg.Tx}
		msg, err := getMsgAddLiquidityFromMemo(ctx, m, obTx, msg.Signer)
		if err != nil {
			return nil, err
		}
		handler := NewAddLiquidityHandler(h.mgr)
		_, err = handler.Run(ctx, msg)
		if err != nil {
			ctx.Logger().Error("swap handler failed to add liquidity", "error", err)
			return nil, err
		}
	}

	return &cosmos.Result{}, nil
}

// get the total bond of the bottom 2/3rds active validators
func (h SwapHandler) getEffectiveSecurityBond(ctx cosmos.Context) (cosmos.Uint, error) {
	nodeAccounts, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return cosmos.ZeroUint(), err
	}
	return getEffectiveSecurityBond(nodeAccounts), nil
}

// getTotalLiquidityRUNE we have in all pools
func (h SwapHandler) getTotalLiquidityRUNE(ctx cosmos.Context) (cosmos.Uint, error) {
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
		if p.Asset.IsNative() {
			continue
		}
		total = total.Add(p.BalanceRune)
	}
	return total, nil
}
