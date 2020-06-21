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

// UnstakeHandler to process unstake requests
type UnstakeHandler struct {
	keeper keeper.Keeper
	mgr    Manager
}

// NewUnstakeHandler create a new instance of UnstakeHandler to process unstake request
func NewUnstakeHandler(keeper keeper.Keeper, mgr Manager) UnstakeHandler {
	return UnstakeHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

// Run is the main entry point of unstake
func (h UnstakeHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, _ constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(MsgSetUnStake)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info(fmt.Sprintf("receive MsgSetUnstake from : %s(%s) unstake (%s)", msg, msg.RuneAddress, msg.UnstakeBasisPoints))

	if err := h.validate(ctx, msg, version); err != nil {
		ctx.Logger().Error("MsgSetUnStake failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, msg, version)
	if err != nil {
		ctx.Logger().Error("failed to process MsgSetUnStake", "error", err)
	}
	return result, err
}

func (h UnstakeHandler) validate(ctx cosmos.Context, msg MsgSetUnStake, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h UnstakeHandler) validateV1(ctx cosmos.Context, msg MsgSetUnStake) error {
	if err := msg.ValidateBasic(); err != nil {
		return errUnstakeFailValidation
	}
	pool, err := h.keeper.GetPool(ctx, msg.Asset)
	if err != nil {
		errMsg := fmt.Sprintf("fail to get pool(%s)", msg.Asset)
		return ErrInternal(err, errMsg)
	}

	if err := pool.EnsureValidPoolStatus(msg); err != nil {
		return multierror.Append(errInvalidPoolStatus, err)
	}

	return nil
}

func (h UnstakeHandler) handle(ctx cosmos.Context, msg MsgSetUnStake, version semver.Version) (*cosmos.Result, error) {
	staker, err := h.keeper.GetStaker(ctx, msg.Asset, msg.RuneAddress)
	if err != nil {
		return nil, multierror.Append(errFailGetStaker, err)
	}
	runeAmt, assetAmount, units, gasAsset, err := unstake(ctx, version, h.keeper, msg, h.mgr.EventMgr())
	if err != nil {
		return nil, ErrInternal(err, "fail to process UnStake request")
	}
	unstakeEvt := NewEventUnstake(
		msg.Asset,
		units,
		int64(msg.UnstakeBasisPoints.Uint64()),
		cosmos.ZeroDec(), // TODO: What is Asymmetry, how to calculate it?
		msg.Tx,
	)
	if err := h.mgr.EventMgr().EmitUnstakeEvent(ctx, unstakeEvt); err != nil {
		return nil, multierror.Append(errFailSaveEvent, err)
	}

	memo := ""
	if msg.Tx.ID.Equals(common.BlankTxID) {
		// tx id is blank, must be triggered by the ragnarok protocol
		memo = NewRagnarokMemo(common.BlockHeight(ctx)).String()
	}
	toi := &TxOutItem{
		Chain:     msg.Asset.Chain,
		InHash:    msg.Tx.ID,
		ToAddress: staker.AssetAddress,
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
	}

	okAsset, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi)
	if err != nil {
		if !errors.Is(err, ErrNotEnoughToPayFee) {
			// the emit asset not enough to pay fee,continue
			return nil, multierror.Append(errFailAddOutboundTx, err)
		}
		okAsset = true
	}
	if !okAsset {
		return nil, errFailAddOutboundTx
	}

	toi = &TxOutItem{
		Chain:     common.RuneAsset().Chain,
		InHash:    msg.Tx.ID,
		ToAddress: staker.RuneAddress,
		Coin:      common.NewCoin(common.RuneAsset(), runeAmt),
		Memo:      memo,
	}
	if !common.RuneAsset().Chain.Equals(common.THORChain) {
		if !gasAsset.IsZero() {
			if msg.Asset.IsBNB() {
				toi.MaxGas = common.Gas{
					common.NewCoin(common.RuneAsset().Chain.GetGasAsset(), gasAsset.QuoUint64(2)),
				}
			}
		}
	}
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
