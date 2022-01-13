package thorchain

import (
	"errors"
	"fmt"

	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"
	"github.com/hashicorp/go-multierror"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (h WithdrawLiquidityHandler) validateV1(ctx cosmos.Context, msg MsgWithdrawLiquidity) error {
	if err := msg.ValidateBasic(); err != nil {
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

	return nil

}

func (h WithdrawLiquidityHandler) handleV1(ctx cosmos.Context, msg MsgWithdrawLiquidity) (*cosmos.Result, error) {
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
			// the emit asset not enough to pay fee,continue
			// other situation will be none of the vault has enough fund to fulfill this withdraw
			// thus withdraw need to be revert
			if !errors.Is(err, ErrNotEnoughToPayFee) {
				// restore pool and liquidity provider
				if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
					return nil, ErrInternal(err, "fail to save pool")
				}
				h.mgr.Keeper().SetLiquidityProvider(ctx, lp)
				return nil, multierror.Append(errFailAddOutboundTx, err)
			}
			okAsset = true
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
		if err := h.mgr.Keeper().AddFeeToReserve(ctx, coin.Amount); err != nil {
			// Add to reserve
			ctx.Logger().Error("fail to add fee to reserve", "error", err)
		}
	}

	return &cosmos.Result{}, nil

}

func (h WithdrawLiquidityHandler) handleV42(ctx cosmos.Context, msg MsgWithdrawLiquidity) (*cosmos.Result, error) {
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

	inboundAsset := msg.Tx.Coins.GetCoin(msg.Asset)
	if !inboundAsset.IsEmpty() {
		assetAmount = assetAmount.Add(inboundAsset.Amount)
	}
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
		// there is much much less chance thorchain doesn't have enough RUNE
		okRune, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi)
		if err != nil {
			// emitted asset doesn't enough to cover fee, continue
			return nil, multierror.Append(errFailAddOutboundTx, err)
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
		if err := h.mgr.Keeper().AddFeeToReserve(ctx, coin.Amount); err != nil {
			// Add to reserve
			ctx.Logger().Error("fail to add fee to reserve", "error", err)
		}
	}

	return &cosmos.Result{}, nil
}

func (h WithdrawLiquidityHandler) handleV45(ctx cosmos.Context, msg MsgWithdrawLiquidity) (*cosmos.Result, error) {
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

	inboundAsset := msg.Tx.Coins.GetCoin(msg.Asset)
	if !inboundAsset.IsEmpty() {
		assetAmount = assetAmount.Add(inboundAsset.Amount)
	}

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
		// there is much much less chance thorchain doesn't have enough RUNE
		okRune, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi)
		if err != nil {
			// when assetAmount is zero , the network didn't try to send asset to customer
			// thus if it failed to send RUNE , it should restore pool here
			// this might happen when the emit RUNE from the withdraw is less than `NativeTransactionFee`
			if assetAmount.IsZero() {
				if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
					return nil, ErrInternal(err, "fail to save pool")
				}
			}
			// emitted asset doesn't enough to cover fee, continue
			return nil, multierror.Append(errFailAddOutboundTx, err)
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
		if err := h.mgr.Keeper().AddFeeToReserve(ctx, coin.Amount); err != nil {
			// Add to reserve
			ctx.Logger().Error("fail to add fee to reserve", "error", err)
		}
	}

	return &cosmos.Result{}, nil
}

func (h WithdrawLiquidityHandler) handleV47(ctx cosmos.Context, msg MsgWithdrawLiquidity) (*cosmos.Result, error) {
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

	inboundAsset := msg.Tx.Coins.GetCoin(msg.Asset)
	if !inboundAsset.IsEmpty() {
		assetAmount = assetAmount.Add(inboundAsset.Amount)
	}

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
		// there is much much less chance thorchain doesn't have enough RUNE
		okRune, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi)
		if err != nil {
			// when assetAmount is zero , the network didn't try to send asset to customer
			// thus if it failed to send RUNE , it should restore pool here
			// this might happen when the emit RUNE from the withdraw is less than `NativeTransactionFee`
			if assetAmount.IsZero() {
				if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
					return nil, ErrInternal(err, "fail to save pool")
				}
			}
			// emitted asset doesn't enough to cover fee, continue
			return nil, multierror.Append(errFailAddOutboundTx, err)
		}

		if !okRune {
			return nil, errFailAddOutboundTx
		}
	}

	if units.IsZero() {
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
	coin := msg.Tx.Coins.GetCoin(common.RuneAsset())
	if !coin.IsEmpty() {
		if err := h.mgr.Keeper().AddFeeToReserve(ctx, coin.Amount); err != nil {
			// Add to reserve
			ctx.Logger().Error("fail to add fee to reserve", "error", err)
		}
	}

	return &cosmos.Result{}, nil

}

func (h WithdrawLiquidityHandler) handleV49(ctx cosmos.Context, msg MsgWithdrawLiquidity) (*cosmos.Result, error) {
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

	inboundAsset := msg.Tx.Coins.GetCoin(msg.Asset)
	if !inboundAsset.IsEmpty() {
		assetAmount = assetAmount.Add(inboundAsset.Amount)
	}

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
		// there is much much less chance thorchain doesn't have enough RUNE
		okRune, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi)
		if err != nil {
			// when assetAmount is zero , the network didn't try to send asset to customer
			// thus if it failed to send RUNE , it should restore pool here
			// this might happen when the emit RUNE from the withdraw is less than `NativeTransactionFee`
			if assetAmount.IsZero() {
				if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
					return nil, ErrInternal(err, "fail to save pool")
				}
			}
			// emitted asset doesn't enough to cover fee, continue
			return nil, multierror.Append(errFailAddOutboundTx, err)
		}

		if !okRune {
			return nil, errFailAddOutboundTx
		}
	}

	if units.IsZero() {
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
	coin := msg.Tx.Coins.GetCoin(common.RuneAsset())
	if !coin.IsEmpty() {
		if err := h.mgr.Keeper().AddFeeToReserve(ctx, coin.Amount); err != nil {
			// Add to reserve
			ctx.Logger().Error("fail to add fee to reserve", "error", err)
		}
	}

	return &cosmos.Result{}, nil
}

func (h WithdrawLiquidityHandler) handleV50(ctx cosmos.Context, msg MsgWithdrawLiquidity) (*cosmos.Result, error) {
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

	inboundAsset := msg.Tx.Coins.GetCoin(msg.Asset)
	if !inboundAsset.IsEmpty() {
		assetAmount = assetAmount.Add(inboundAsset.Amount)
	}

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
		// there is much much less chance thorchain doesn't have enough RUNE
		okRune, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi)
		if err != nil {
			// when assetAmount is zero , the network didn't try to send asset to customer
			// thus if it failed to send RUNE , it should restore pool here
			// this might happen when the emit RUNE from the withdraw is less than `NativeTransactionFee`
			if assetAmount.IsZero() {
				if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
					return nil, ErrInternal(err, "fail to save pool")
				}
			}
			// emitted asset doesn't enough to cover fee, continue
			return nil, multierror.Append(errFailAddOutboundTx, err)
		}

		if !okRune {
			return nil, errFailAddOutboundTx
		}
	}

	if units.IsZero() {
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
	coin := msg.Tx.Coins.GetCoin(common.RuneAsset())
	if !coin.IsEmpty() {
		if err := h.mgr.Keeper().AddFeeToReserve(ctx, coin.Amount); err != nil {
			// Add to reserve
			ctx.Logger().Error("fail to add fee to reserve", "error", err)
		}
	}

	return &cosmos.Result{}, nil

}

func (h WithdrawLiquidityHandler) handleV55(ctx cosmos.Context, msg MsgWithdrawLiquidity) (*cosmos.Result, error) {
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

	inboundAsset := msg.Tx.Coins.GetCoin(msg.Asset)
	if !inboundAsset.IsEmpty() {
		assetAmount = assetAmount.Add(inboundAsset.Amount)
	}

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
		// there is much much less chance thorchain doesn't have enough RUNE
		okRune, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi)
		if err != nil {
			// when assetAmount is zero , the network didn't try to send asset to customer
			// thus if it failed to send RUNE , it should restore pool here
			// this might happen when the emit RUNE from the withdraw is less than `NativeTransactionFee`
			if assetAmount.IsZero() {
				if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
					return nil, ErrInternal(err, "fail to save pool")
				}
			}
			// emitted asset doesn't enough to cover fee, continue
			return nil, multierror.Append(errFailAddOutboundTx, err)
		}

		if !okRune {
			return nil, errFailAddOutboundTx
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
	coin := msg.Tx.Coins.GetCoin(common.RuneAsset())
	if !coin.IsEmpty() {
		if err := h.mgr.Keeper().AddFeeToReserve(ctx, coin.Amount); err != nil {
			// Add to reserve
			ctx.Logger().Error("fail to add fee to reserve", "error", err)
		}
	}

	return &cosmos.Result{}, nil
}

func (h WithdrawLiquidityHandler) handleV65(ctx cosmos.Context, msg MsgWithdrawLiquidity) (*cosmos.Result, error) {
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
		// there is much much less chance thorchain doesn't have enough RUNE
		okRune, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi)
		if err != nil {
			// when assetAmount is zero , the network didn't try to send asset to customer
			// thus if it failed to send RUNE , it should restore pool here
			// this might happen when the emit RUNE from the withdraw is less than `NativeTransactionFee`
			if assetAmount.IsZero() {
				if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
					return nil, ErrInternal(err, "fail to save pool")
				}
			}
			// emitted asset doesn't enough to cover fee, continue
			return nil, multierror.Append(errFailAddOutboundTx, err)
		}

		if !okRune {
			return nil, errFailAddOutboundTx
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
	coin := msg.Tx.Coins.GetCoin(common.RuneAsset())
	if !coin.IsEmpty() {
		if err := h.mgr.Keeper().AddFeeToReserve(ctx, coin.Amount); err != nil {
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
