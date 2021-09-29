package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// handleV1 process MsgNetworkFee
func (h NetworkFeeHandler) handleV1(ctx cosmos.Context, msg MsgNetworkFee) (*cosmos.Result, error) {
	active, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		err = wrapError(ctx, err, "fail to get list of active node accounts")
		return nil, err
	}

	voter, err := h.mgr.Keeper().GetObservedNetworkFeeVoter(ctx, msg.BlockHeight, msg.Chain)
	if err != nil {
		return nil, err
	}
	observeSlashPoints := h.mgr.GetConstants().GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := h.mgr.GetConstants().GetInt64Value(constants.ObservationDelayFlexibility)
	h.mgr.Slasher().IncSlashPoints(ctx, observeSlashPoints, msg.Signer)
	if !voter.Sign(msg.Signer) {
		ctx.Logger().Info("signer already signed MsgNetworkFee", "signer", msg.Signer.String(), "block height", msg.BlockHeight, "chain", msg.Chain.String())
		return &cosmos.Result{}, nil
	}
	h.mgr.Keeper().SetObservedNetworkFeeVoter(ctx, voter)
	// doesn't have consensus yet
	if !voter.HasConsensus(active) {
		return &cosmos.Result{}, nil
	}

	if voter.BlockHeight > 0 {
		if (voter.BlockHeight + observeFlex) >= common.BlockHeight(ctx) {
			h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, msg.Signer)
		}
		// MsgNetworkFee tx already processed
		return &cosmos.Result{}, nil
	}

	voter.BlockHeight = common.BlockHeight(ctx)
	h.mgr.Keeper().SetObservedNetworkFeeVoter(ctx, voter)
	// decrease the slash points
	h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, voter.GetSigners()...)
	ctx.Logger().Info("update network fee", "chain", msg.Chain.String(), "transaction-size", msg.TransactionSize, "fee-rate", msg.TransactionFeeRate)
	if err := h.mgr.Keeper().SaveNetworkFee(ctx, msg.Chain, NetworkFee{
		Chain:              msg.Chain,
		TransactionSize:    msg.TransactionSize,
		TransactionFeeRate: msg.TransactionFeeRate,
	}); err != nil {
		return nil, ErrInternal(err, "fail to save network fee")
	}
	return &cosmos.Result{}, nil
}
