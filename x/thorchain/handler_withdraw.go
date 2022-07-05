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
	switch {
	case version.GTE(semver.MustParse("0.80.0")):
		return h.validateV80(ctx, msg)
	default:
		return errBadVersion
	}
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
	switch {
	case version.GTE(semver.MustParse("1.93.0")):
		return h.handleV93(ctx, msg)
	case version.GTE(semver.MustParse("1.88.1")):
		return h.handleV88(ctx, msg)
	case version.GTE(semver.MustParse("1.88.0")):
		// only change in 1.88.1 is moving cacheCtx to the caller
		cacheCtx, commit := ctx.CacheContext()
		res, err := h.handleV88(cacheCtx, msg)
		if err == nil {
			commit()
			ctx.EventManager().EmitEvents(cacheCtx.EventManager().Events())
		}
		return res, err
	case version.GTE(semver.MustParse("0.75.0")):
		return h.handleV75(ctx, msg)
	}
	return nil, errBadVersion
}

func (h WithdrawLiquidityHandler) handleV93(ctx cosmos.Context, msg MsgWithdrawLiquidity) (*cosmos.Result, error) {
	lp, err := h.mgr.Keeper().GetLiquidityProvider(ctx, msg.Asset, msg.WithdrawAddress)
	if err != nil {
		return nil, multierror.Append(errFailGetLiquidityProvider, err)
	}
	runeAmt, assetAmt, impLossProtection, units, gasAsset, err := withdraw(ctx, msg, h.mgr)
	if err != nil {
		return nil, ErrInternal(err, "fail to process withdraw request")
	}

	memo := ""
	if msg.Tx.ID.Equals(common.BlankTxID) {
		// tx id is blank, must be triggered by the ragnarok protocol
		memo = NewRagnarokMemo(common.BlockHeight(ctx)).String()
	}

	transfer := func(coin common.Coin, addr common.Address) error {
		toi := TxOutItem{
			Chain:     coin.Asset.GetChain(),
			InHash:    msg.Tx.ID,
			ToAddress: addr,
			Coin:      coin,
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

		ok, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi, cosmos.ZeroUint())
		if err != nil {
			return multierror.Append(errFailAddOutboundTx, err)
		}
		if !ok {
			return errFailAddOutboundTx
		}

		return nil
	}

	if !assetAmt.IsZero() {
		coin := common.NewCoin(msg.Asset, assetAmt)
		if !msg.Asset.IsNativeRune() && !lp.AssetAddress.IsChain(msg.Asset.Chain) {
			if err := h.swapV93(ctx, msg, coin, lp.AssetAddress); err != nil {
				return nil, err
			}
		} else {
			if err := transfer(coin, lp.AssetAddress); err != nil {
				return nil, err
			}
		}
	}

	if !runeAmt.IsZero() {
		coin := common.NewCoin(common.RuneAsset(), runeAmt)
		if err := transfer(coin, lp.RuneAddress); err != nil {
			return nil, err
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
			assetAmt,
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
			assetAmt,
			runeAmt,
			impLossProtection,
		)
		if err := h.mgr.EventMgr().EmitEvent(ctx, withdrawEvt); err != nil {
			return nil, multierror.Append(errFailSaveEvent, err)
		}
	}

	// any extra rune in the transaction will be donated to reserve
	reserveCoin := msg.Tx.Coins.GetCoin(common.RuneAsset())
	if !reserveCoin.IsEmpty() {
		if err := h.mgr.Keeper().AddPoolFeeToReserve(ctx, reserveCoin.Amount); err != nil {
			ctx.Logger().Error("fail to add fee to reserve", "error", err)
			return nil, err
		}
	}

	telemetry.IncrCounterWithLabels(
		[]string{"thornode", "withdraw", "implossprotection"},
		telem(impLossProtection),
		[]metrics.Label{telemetry.NewLabel("asset", msg.Asset.String())},
	)

	return &cosmos.Result{}, nil
}

func (h WithdrawLiquidityHandler) swapV93(ctx cosmos.Context, msg MsgWithdrawLiquidity, coin common.Coin, addr common.Address) error {
	// ensure TxID does NOT have a collision with another swap, this could
	// happen if the user submits two identical loan requests in the same
	// block
	if ok := h.mgr.Keeper().HasSwapQueueItem(ctx, msg.Tx.ID, 0); ok {
		return fmt.Errorf("txn hash conflict")
	}

	target := addr.GetChain().GetGasAsset()
	memo := fmt.Sprintf("=:%s:%s", target, addr)
	msg.Tx.Memo = memo
	msg.Tx.Coins = common.NewCoins(coin)
	swapMsg := NewMsgSwap(msg.Tx, target, addr, cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", nil, msg.Signer)

	// sanity check swap msg
	handler := NewSwapHandler(h.mgr)
	if err := handler.validate(ctx, *swapMsg); err != nil {
		return err
	}

	if err := h.mgr.Keeper().SetSwapQueueItem(ctx, *swapMsg, 0); err != nil {
		ctx.Logger().Error("fail to add swap to queue", "error", err)
		return err
	}

	return nil
}
