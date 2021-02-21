package thorchain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"
	"github.com/hashicorp/go-multierror"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// WithdrawLiqudityHandler to process withdraw requests
type WithdrawLiquidityHandler struct {
	keeper keeper.Keeper
	mgr    Manager
}

// NewWithdrawLiquidityHandler create a new instance of WithdrawLiquidityHandler to process withdraw request
func NewWithdrawLiquidityHandler(keeper keeper.Keeper, mgr Manager) WithdrawLiquidityHandler {
	return WithdrawLiquidityHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

// Run is the main entry point of withdraw
func (h WithdrawLiquidityHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(*MsgWithdrawLiquidity)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info(fmt.Sprintf("receive MsgWithdrawLiquidity from : %s(%s) withdraw (%s)", *msg, msg.WithdrawAddress, msg.BasisPoints))

	if err := h.validate(ctx, *msg, version); err != nil {
		ctx.Logger().Error("MsgWithdrawLiquidity failed validation", "error", err)
		return nil, err
	}

	result, err := h.handle(ctx, *msg, version, constAccessor)
	if err != nil {
		ctx.Logger().Error("fail to process msg withdraw", "error", err)
		return nil, err
	}
	return result, err
}

func (h WithdrawLiquidityHandler) validate(ctx cosmos.Context, msg MsgWithdrawLiquidity, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h WithdrawLiquidityHandler) validateV1(ctx cosmos.Context, msg MsgWithdrawLiquidity) error {
	if err := msg.ValidateBasic(); err != nil {
		return errWithdrawFailValidation
	}
	pool, err := h.keeper.GetPool(ctx, msg.Asset)
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

	return nil
}

func (h WithdrawLiquidityHandler) handle(ctx cosmos.Context, msg MsgWithdrawLiquidity, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	if version.GTE(semver.MustParse("0.24.0")) {
		return h.handleV24(ctx, msg, version, constAccessor)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, version, constAccessor)
	}
	return nil, errBadVersion
}

func (h WithdrawLiquidityHandler) handleV1(ctx cosmos.Context, msg MsgWithdrawLiquidity, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	lp, err := h.keeper.GetLiquidityProvider(ctx, msg.Asset, msg.WithdrawAddress)
	if err != nil {
		return nil, multierror.Append(errFailGetLiquidityProvider, err)
	}
	pool, err := h.keeper.GetPool(ctx, msg.Asset)
	if err != nil {
		return nil, ErrInternal(err, "fail to get pool")
	}
	runeAmt, assetAmount, units, gasAsset, err := withdrawV1(ctx, version, h.keeper, msg, h.mgr)
	if err != nil {
		return nil, ErrInternal(err, "fail to process withdraw request")
	}

	memo := ""
	if msg.Tx.ID.Equals(common.BlankTxID) {
		// tx id is blank, must be triggered by the ragnarok protocol
		memo = NewRagnarokMemo(common.BlockHeight(ctx)).String()
	}
	if !assetAmount.IsZero() {
		toi := TxOutItem{
			Chain:     msg.Asset.Chain,
			InHash:    msg.Tx.ID,
			ToAddress: lp.AssetAddress,
			Coin:      common.NewCoin(msg.Asset, assetAmount),
			Memo:      memo,
		}
		if !gasAsset.IsZero() {
			// TODO: chain specific logic should be in a single location
			if msg.Asset.IsBNB() {
				toi.MaxGas = common.Gas{
					common.NewCoin(common.RuneAsset().Chain.GetGasAsset(), gasAsset.QuoUint64(2)),
				}
			} else if msg.Asset.Chain.GetGasAsset().Equals(msg.Asset) {
				toi.MaxGas = common.Gas{
					common.NewCoin(msg.Asset.Chain.GetGasAsset(), gasAsset),
				}
			}
			toi.GasRate = int64(h.mgr.GasMgr().GetGasRate(ctx, msg.Asset.Chain).Uint64())
		}

		okAsset, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi)
		if err != nil {
			// the emit asset not enough to pay fee,continue
			// other situation will be none of the vault has enough fund to fulfill this withdraw
			// thus withdraw need to be revert
			if !errors.Is(err, ErrNotEnoughToPayFee) {
				// restore pool and liquidity provider
				if err := h.keeper.SetPool(ctx, pool); err != nil {
					return nil, ErrInternal(err, "fail to save pool")
				}
				h.keeper.SetLiquidityProvider(ctx, lp)
				return nil, multierror.Append(errFailAddOutboundTx, err)
			}
			okAsset = true
		}
		if !okAsset {
			return nil, errFailAddOutboundTx
		}
	}

	// If the pool is in staged status, then we won't have withheld the asset
	// above from being deducted a fee/gas from the transaction. So we have to
	// do it here, on the rune side.
	if pool.Status != PoolAvailable {
		transactionFee := h.mgr.GasMgr().GetFee(ctx, msg.Asset.Chain, common.RuneAsset())
		runeAmt = common.SafeSub(runeAmt, transactionFee)
	}

	if !runeAmt.IsZero() {
		toi := TxOutItem{
			Chain:     common.RuneAsset().Chain,
			InHash:    msg.Tx.ID,
			ToAddress: lp.RuneAddress,
			Coin:      common.NewCoin(common.RuneAsset(), runeAmt),
			Memo:      memo,
		}
		// there is much much less chance thorchain doesn't have enough RUNE
		okRune, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi)
		if err != nil {
			// emitted asset doesn't enough to cover fee, continue
			if !errors.Is(err, ErrNotEnoughToPayFee) {
				return nil, multierror.Append(errFailAddOutboundTx, err)
			}
			okRune = true
		}

		if !okRune {
			return nil, errFailAddOutboundTx
		}
	}

	withdrawEvt := NewEventWithdraw(
		msg.Asset,
		units,
		int64(msg.BasisPoints.Uint64()),
		cosmos.ZeroDec(),
		msg.Tx,
		assetAmount,
		runeAmt,
		cosmos.ZeroUint(),
	)
	if err := h.mgr.EventMgr().EmitEvent(ctx, withdrawEvt); err != nil {
		return nil, multierror.Append(errFailSaveEvent, err)
	}
	// Get rune (if any) and donate it to the reserve
	coin := msg.Tx.Coins.GetCoin(common.RuneAsset())
	if !coin.IsEmpty() {
		if err := h.keeper.AddFeeToReserve(ctx, coin.Amount); err != nil {
			// Add to reserve
			ctx.Logger().Error("fail to add fee to reserve", "error", err)
		}
	}

	return &cosmos.Result{}, nil
}

func (h WithdrawLiquidityHandler) handleV24(ctx cosmos.Context, msg MsgWithdrawLiquidity, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	lp, err := h.keeper.GetLiquidityProvider(ctx, msg.Asset, msg.WithdrawAddress)
	if err != nil {
		return nil, multierror.Append(errFailGetLiquidityProvider, err)
	}
	pool, err := h.keeper.GetPool(ctx, msg.Asset)
	if err != nil {
		return nil, ErrInternal(err, "fail to get pool")
	}
	runeAmt, assetAmount, impLossProtection, units, gasAsset, err := withdrawV24(ctx, version, h.keeper, msg, h.mgr)
	if err != nil {
		return nil, ErrInternal(err, "fail to process withdraw request")
	}

	memo := ""
	if msg.Tx.ID.Equals(common.BlankTxID) {
		// tx id is blank, must be triggered by the ragnarok protocol
		memo = NewRagnarokMemo(common.BlockHeight(ctx)).String()
	}
	if !assetAmount.IsZero() {
		toi := TxOutItem{
			Chain:     msg.Asset.Chain,
			InHash:    msg.Tx.ID,
			ToAddress: lp.AssetAddress,
			Coin:      common.NewCoin(msg.Asset, assetAmount),
			Memo:      memo,
		}
		if !gasAsset.IsZero() {
			// TODO: chain specific logic should be in a single location
			if msg.Asset.IsBNB() {
				toi.MaxGas = common.Gas{
					common.NewCoin(common.RuneAsset().Chain.GetGasAsset(), gasAsset.QuoUint64(2)),
				}
			} else if msg.Asset.Chain.GetGasAsset().Equals(msg.Asset) {
				toi.MaxGas = common.Gas{
					common.NewCoin(msg.Asset.Chain.GetGasAsset(), gasAsset),
				}
			}
			toi.GasRate = int64(h.mgr.GasMgr().GetGasRate(ctx, msg.Asset.Chain).Uint64())
		}

		okAsset, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi)
		if err != nil {
			// the emit asset not enough to pay fee,continue
			// other situation will be none of the vault has enough fund to fulfill this withdraw
			// thus withdraw need to be revert
			if !errors.Is(err, ErrNotEnoughToPayFee) {
				// restore pool and liquidity provider
				if err := h.keeper.SetPool(ctx, pool); err != nil {
					return nil, ErrInternal(err, "fail to save pool")
				}
				h.keeper.SetLiquidityProvider(ctx, lp)
				return nil, multierror.Append(errFailAddOutboundTx, err)
			}
			okAsset = true
		}
		if !okAsset {
			return nil, errFailAddOutboundTx
		}
	}

	// If the pool is in staged status, then we won't have withheld the asset
	// above from being deducted a fee/gas from the transaction. So we have to
	// do it here, on the rune side.
	if pool.Status != PoolAvailable {
		transactionFee := h.mgr.GasMgr().GetFee(ctx, msg.Asset.Chain, common.RuneAsset())
		runeAmt = common.SafeSub(runeAmt, transactionFee)
	}

	if !runeAmt.IsZero() {
		toi := TxOutItem{
			Chain:     common.RuneAsset().Chain,
			InHash:    msg.Tx.ID,
			ToAddress: lp.RuneAddress,
			Coin:      common.NewCoin(common.RuneAsset(), runeAmt),
			Memo:      memo,
		}
		// there is much much less chance thorchain doesn't have enough RUNE
		okRune, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi)
		if err != nil {
			// emitted asset doesn't enough to cover fee, continue
			if !errors.Is(err, ErrNotEnoughToPayFee) {
				return nil, multierror.Append(errFailAddOutboundTx, err)
			}
			okRune = true
		}

		if !okRune {
			return nil, errFailAddOutboundTx
		}
	}

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
	// Get rune (if any) and donate it to the reserve
	coin := msg.Tx.Coins.GetCoin(common.RuneAsset())
	if !coin.IsEmpty() {
		if err := h.keeper.AddFeeToReserve(ctx, coin.Amount); err != nil {
			// Add to reserve
			ctx.Logger().Error("fail to add fee to reserve", "error", err)
		}
	}

	return &cosmos.Result{}, nil
}
