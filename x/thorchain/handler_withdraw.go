package thorchain

import (
	"fmt"

	"github.com/armon/go-metrics"
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/telemetry"
	"github.com/hashicorp/go-multierror"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// WithdrawLiquidityHandler to process withdraw requests
type WithdrawLiquidityHandler struct {
	mgr Manager
}

// NewWithdrawLiquidityHandler create a new instance of WithdrawLiquidityHandler to process withdraw request
func NewWithdrawLiquidityHandler(mgr Manager) WithdrawLiquidityHandler {
	return WithdrawLiquidityHandler{
		mgr: mgr,
	}
}

// Run is the main entry point of withdraw
func (h WithdrawLiquidityHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgWithdrawLiquidity)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive MsgWithdrawLiquidity", "withdraw address", msg.WithdrawAddress, "withdraw basis points", msg.BasisPoints)
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgWithdrawLiquidity failed validation", "error", err)
		return nil, err
	}

	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process msg withdraw", "error", err)
		return nil, err
	}
	return result, err
}

func (h WithdrawLiquidityHandler) validate(ctx cosmos.Context, msg MsgWithdrawLiquidity) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.80.0")) {
		return h.validateV80(ctx, msg)
	} else if version.GTE(semver.MustParse("0.65.0")) {
		return h.validateV65(ctx, msg)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h WithdrawLiquidityHandler) validateV80(ctx cosmos.Context, msg MsgWithdrawLiquidity) error {
	if err := msg.ValidateBasic(); err != nil {
		return errWithdrawFailValidation
	}
	if msg.Asset.IsSyntheticAsset() {
		ctx.Logger().Error("asset cannot be synth", "error", errWithdrawFailValidation)
		return errWithdrawFailValidation
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		errMsg := fmt.Sprintf("fail to get pool(%s)", msg.Asset)
		return ErrInternal(err, errMsg)
	}

	if err := pool.EnsureValidPoolStatus(&msg); err != nil {
		return multierror.Append(errInvalidPoolStatus, err)
	}

	// when ragnarok kicks off,  all pool will be set PoolStaged , the ragnarok tx's hash will be common.BlankTxID
	if pool.Status != PoolAvailable && !msg.WithdrawalAsset.IsEmpty() && !msg.Tx.ID.Equals(common.BlankTxID) {
		return fmt.Errorf("cannot specify a withdrawal asset while the pool is not available")
	}

	if isChainHalted(ctx, h.mgr, msg.Asset.Chain) || isLPPaused(ctx, msg.Asset.Chain, h.mgr) {
		return fmt.Errorf("unable to withdraw liquidity while chain is halted or paused LP actions")
	}

	return nil
}

func (h WithdrawLiquidityHandler) handle(ctx cosmos.Context, msg MsgWithdrawLiquidity) (*cosmos.Result, error) {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.75.0")) {
		return h.handleV75(ctx, msg)
	} else if version.GTE(semver.MustParse("0.65.0")) {
		return h.handleV65(ctx, msg)
	} else if version.GTE(semver.MustParse("0.55.0")) {
		return h.handleV55(ctx, msg)
	} else if version.GTE(semver.MustParse("0.50.0")) {
		return h.handleV50(ctx, msg)
	} else if version.GTE(semver.MustParse("0.49.0")) {
		return h.handleV49(ctx, msg)
	} else if version.GTE(semver.MustParse("0.47.0")) {
		return h.handleV47(ctx, msg)
	} else if version.GTE(semver.MustParse("0.45.0")) {
		return h.handleV45(ctx, msg)
	} else if version.GTE(semver.MustParse("0.42.0")) {
		return h.handleV42(ctx, msg)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg)
	}
	return nil, errBadVersion
}

func (h WithdrawLiquidityHandler) handleV75(ctx cosmos.Context, msg MsgWithdrawLiquidity) (*cosmos.Result, error) {
	lp, err := h.mgr.Keeper().GetLiquidityProvider(ctx, msg.Asset, msg.WithdrawAddress)
	if err != nil {
		return nil, multierror.Append(errFailGetLiquidityProvider, err)
	}
	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return nil, ErrInternal(err, "fail to get pool")
	}
	runeAmt, assetAmount, impLossProtection, units, gasAsset, err := withdraw(ctx, h.mgr.GetVersion(), msg, h.mgr)
	if err != nil {
		return nil, ErrInternal(err, "fail to process withdraw request")
	}

	memo := ""
	if msg.Tx.ID.Equals(common.BlankTxID) {
		// tx id is blank, must be triggered by the ragnarok protocol
		memo = NewRagnarokMemo(common.BlockHeight(ctx)).String()
	}

	// any extra rune in the transaction will be donated to reserve
	reserveCoin := msg.Tx.Coins.GetCoin(common.RuneAsset())

	if !assetAmount.IsZero() {
		toi := TxOutItem{
			Chain:     msg.Asset.GetChain(),
			InHash:    msg.Tx.ID,
			ToAddress: lp.AssetAddress,
			Coin:      common.NewCoin(msg.Asset, assetAmount),
			Memo:      memo,
		}
		if !gasAsset.IsZero() {
			// TODO: chain specific logic should be in a single location
			if msg.Asset.IsBNB() {
				toi.MaxGas = common.Gas{
					common.NewCoin(common.RuneAsset().GetChain().GetGasAsset(), gasAsset.QuoUint64(2)),
				}
			} else if msg.Asset.GetChain().GetGasAsset().Equals(msg.Asset) {
				toi.MaxGas = common.Gas{
					common.NewCoin(msg.Asset.GetChain().GetGasAsset(), gasAsset),
				}
			}
			toi.GasRate = int64(h.mgr.GasMgr().GetGasRate(ctx, msg.Asset.GetChain()).Uint64())
		}

		okAsset, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi)
		if err != nil {
			// restore pool and liquidity provider
			if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
				return nil, ErrInternal(err, "fail to save pool")
			}
			h.mgr.Keeper().SetLiquidityProvider(ctx, lp)
			return nil, multierror.Append(errFailAddOutboundTx, err)
		}
		if !okAsset {
			return nil, errFailAddOutboundTx
		}
	}

	if !runeAmt.IsZero() {
		toi := TxOutItem{
			Chain:     common.RuneAsset().GetChain(),
			InHash:    msg.Tx.ID,
			ToAddress: lp.RuneAddress,
			Coin:      common.NewCoin(common.RuneAsset(), runeAmt),
			Memo:      memo,
		}
		okRune, txOutErr := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi)
		if txOutErr != nil || !okRune {
			// when assetAmount is zero , the network didn't try to send asset to customer
			// thus if it failed to send RUNE , it should restore pool here
			// this might happen when the emit RUNE from the withdraw is less than `NativeTransactionFee`
			if assetAmount.IsZero() {
				if poolErr := h.mgr.Keeper().SetPool(ctx, pool); poolErr != nil {
					return nil, ErrInternal(poolErr, "fail to save pool")
				}
				h.mgr.Keeper().SetLiquidityProvider(ctx, lp)
				if txOutErr != nil {
					return nil, multierror.Append(errFailAddOutboundTx, txOutErr)
				}
				return nil, errFailAddOutboundTx
			}
			// asset success/rune failed: send rune to reserve and emit events
			reserveCoin.Amount = reserveCoin.Amount.Add(toi.Coin.Amount)
			ctx.Logger().Error("rune side failed, add to reserve", "error", txOutErr, "coin", toi.Coin)
		}
	}

	if units.IsZero() && impLossProtection.IsZero() {
		// withdraw pending liquidity event
		runeHash := common.TxID("")
		assetHash := common.TxID("")
		if msg.Tx.Chain.Equals(common.THORChain) {
			runeHash = msg.Tx.ID
		} else {
			assetHash = msg.Tx.ID
		}
		evt := NewEventPendingLiquidity(
			msg.Asset,
			WithdrawPendingLiquidity,
			lp.RuneAddress,
			runeAmt,
			lp.AssetAddress,
			assetAmount,
			runeHash,
			assetHash,
		)
		if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return nil, multierror.Append(errFailSaveEvent, err)
		}
	} else {
		withdrawEvt := NewEventWithdraw(
			msg.Asset,
			units,
			int64(msg.BasisPoints.Uint64()),
			cosmos.ZeroDec(),
			msg.Tx,
			assetAmount,
			runeAmt,
			impLossProtection,
		)
		if err := h.mgr.EventMgr().EmitEvent(ctx, withdrawEvt); err != nil {
			return nil, multierror.Append(errFailSaveEvent, err)
		}
	}

	// Get rune (if any) and donate it to the reserve
	if !reserveCoin.IsEmpty() {
		if err := h.mgr.Keeper().AddFeeToReserve(ctx, reserveCoin.Amount); err != nil {
			// Add to reserve
			ctx.Logger().Error("fail to add fee to reserve", "error", err)
		}
	}

	telemetry.IncrCounterWithLabels(
		[]string{"thornode", "withdraw", "implossprotection"},
		telem(impLossProtection),
		[]metrics.Label{telemetry.NewLabel("asset", msg.Asset.String())},
	)

	return &cosmos.Result{}, nil
}
