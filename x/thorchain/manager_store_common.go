package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// removeTransactions is a method used to remove a tx out item in the queue
func removeTransactions(ctx cosmos.Context, mgr Manager, hashes ...string) {
	for _, txID := range hashes {
		inTxID, err := common.NewTxID(txID)
		if err != nil {
			ctx.Logger().Error("fail to parse tx id", "error", err, "tx_id", inTxID)
			continue
		}
		voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, inTxID)
		if err != nil {
			ctx.Logger().Error("fail to get observed tx voter", "error", err)
			continue
		}
		// all outbound action get removed
		voter.Actions = []TxOutItem{}
		if voter.Tx.IsEmpty() {
			continue
		}
		voter.Tx.SetDone(common.BlankTxID, 0)
		// set the tx outbound with a blank txid will mark it as down , and will be skipped in the reschedule logic
		for idx := range voter.Txs {
			voter.Txs[idx].SetDone(common.BlankTxID, 0)
		}
		mgr.Keeper().SetObservedTxInVoter(ctx, voter)
	}
}

// nolint
type adhocRefundTx struct {
	inboundHash string
	toAddr      string
	amount      float64
	asset       string
}

// refundTransactions is design to use store migration to refund adhoc transactions
// nolint
func refundTransactions(ctx cosmos.Context, mgr *Mgrs, pubKey string, adhocRefundTxes ...adhocRefundTx) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to schedule refund BNB transactions", "error", err)
		}
	}()

	if err := mgr.BeginBlock(ctx); err != nil {
		ctx.Logger().Error("fail to initialise block", "error", err)
		return
	}
	asgardPubKey, err := common.NewPubKey(pubKey)
	if err != nil {
		ctx.Logger().Error("fail to parse pub key", "error", err, "pubkey", pubKey)
		return
	}
	for _, item := range adhocRefundTxes {
		hash, err := common.NewTxID(item.inboundHash)
		if err != nil {
			ctx.Logger().Error("fail to parse hash", "hash", item.inboundHash, "error", err)
			continue
		}
		addr, err := common.NewAddress(item.toAddr)
		if err != nil {
			ctx.Logger().Error("fail to parse address", "address", item.toAddr, "error", err)
			continue
		}
		asset, err := common.NewAsset(item.asset)
		if err != nil {
			ctx.Logger().Error("fail to parse asset", "asset", item.asset, "error", err)
			continue
		}
		coin := common.NewCoin(asset, cosmos.NewUint(uint64(item.amount*common.One)))
		maxGas, err := mgr.GasMgr().GetMaxGas(ctx, coin.Asset.GetChain())
		if err != nil {
			ctx.Logger().Error("fail to get max gas", "error", err)
			continue
		}
		toi := TxOutItem{
			Chain:       coin.Asset.GetChain(),
			InHash:      hash,
			ToAddress:   addr,
			Coin:        coin,
			Memo:        NewRefundMemo(hash).String(),
			MaxGas:      common.Gas{maxGas},
			GasRate:     int64(mgr.GasMgr().GetGasRate(ctx, coin.Asset.GetChain()).Uint64()),
			VaultPubKey: asgardPubKey,
		}

		voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, toi.InHash)
		if err != nil {
			ctx.Logger().Error("fail to get observe tx in voter", "error", err)
			continue
		}
		voter.OutboundHeight = common.BlockHeight(ctx)
		mgr.Keeper().SetObservedTxInVoter(ctx, voter)

		if err := mgr.TxOutStore().UnSafeAddTxOutItem(ctx, mgr, toi); err != nil {
			ctx.Logger().Error("fail to send manual refund", "address", item.toAddr, "error", err)
		}
	}
}
