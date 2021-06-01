package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	memo "gitlab.com/thorchain/thornode/x/thorchain/memo"
)

// ErrataTxHandler is to handle ErrataTx message
type ErrataTxHandler struct {
	mgr Manager
}

// NewErrataTxHandler create new instance of ErrataTxHandler
func NewErrataTxHandler(mgr Manager) ErrataTxHandler {
	return ErrataTxHandler{
		mgr: mgr,
	}
}

// Run is the main entry point to execute ErrataTx logic
func (h ErrataTxHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(*MsgErrataTx)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg, version); err != nil {
		ctx.Logger().Error("msg errata tx failed validation", "error", err)
		return nil, err
	}
	return h.handle(ctx, *msg, version, constAccessor)
}

func (h ErrataTxHandler) validate(ctx cosmos.Context, msg MsgErrataTx, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h ErrataTxHandler) validateV1(ctx cosmos.Context, msg MsgErrataTx) error {
	return h.validateCurrent(ctx, msg)
}

func (h ErrataTxHandler) validateCurrent(ctx cosmos.Context, msg MsgErrataTx) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	if !isSignedByActiveNodeAccounts(ctx, h.mgr.Keeper(), msg.GetSigners()) {
		return cosmos.ErrUnauthorized(notAuthorized.Error())
	}

	return nil
}

func (h ErrataTxHandler) handle(ctx cosmos.Context, msg MsgErrataTx, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	ctx.Logger().Info("handleMsgErrataTx request", "txid", msg.TxID.String())
	if version.GTE(semver.MustParse("0.45.0")) {
		return h.handleV45(ctx, msg, version, constAccessor)
	} else if version.GTE(semver.MustParse("0.42.0")) {
		return h.handleV42(ctx, msg, version, constAccessor)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, version, constAccessor)
	}
	ctx.Logger().Error(errInvalidVersion.Error())
	return nil, errBadVersion
}

func (h ErrataTxHandler) handleV1(ctx cosmos.Context, msg MsgErrataTx, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	active, err := h.mgr.Keeper().ListActiveNodeAccounts(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}

	voter, err := h.mgr.Keeper().GetErrataTxVoter(ctx, msg.TxID, msg.Chain)
	if err != nil {
		return nil, err
	}
	observeSlashPoints := constAccessor.GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := constAccessor.GetInt64Value(constants.ObservationDelayFlexibility)
	h.mgr.Slasher().IncSlashPoints(ctx, observeSlashPoints, msg.Signer)
	if !voter.Sign(msg.Signer) {
		ctx.Logger().Info("signer already signed MsgErrataTx", "signer", msg.Signer.String(), "txid", msg.TxID)
		return &cosmos.Result{}, nil
	}
	h.mgr.Keeper().SetErrataTxVoter(ctx, voter)
	// doesn't have consensus yet
	if !voter.HasConsensus(active) {
		ctx.Logger().Info("not having consensus yet, return")
		return &cosmos.Result{}, nil
	}

	if voter.BlockHeight > 0 {
		if (voter.BlockHeight + observeFlex) >= common.BlockHeight(ctx) {
			h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, msg.Signer)
		}
		// errata tx already processed
		return &cosmos.Result{}, nil
	}

	voter.BlockHeight = common.BlockHeight(ctx)
	h.mgr.Keeper().SetErrataTxVoter(ctx, voter)
	// decrease the slash points
	h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, voter.GetSigners()...)
	observedVoter, err := h.mgr.Keeper().GetObservedTxInVoter(ctx, msg.TxID)
	if err != nil {
		return nil, err
	}

	if len(observedVoter.Txs) == 0 {
		return h.processErrataOutboundTx(ctx, msg)
	}
	// set the observed Tx to reverted
	observedVoter.SetReverted()
	h.mgr.Keeper().SetObservedTxInVoter(ctx, observedVoter)
	if observedVoter.Tx.IsEmpty() || !observedVoter.Tx.IsFinal() {
		ctx.Logger().Info("tx is not finalised, so nothing need to be done", "tx_id", msg.TxID)
		return &cosmos.Result{}, nil
	}

	tx := observedVoter.Tx.Tx
	if !tx.Chain.Equals(msg.Chain) {
		// does not match chain
		return &cosmos.Result{}, nil
	}
	if observedVoter.UpdatedVault {
		vaultPubKey := observedVoter.Tx.ObservedPubKey
		if !vaultPubKey.IsEmpty() {
			// try to deduct the asset from asgard
			vault, err := h.mgr.Keeper().GetVault(ctx, vaultPubKey)
			if err != nil {
				return nil, fmt.Errorf("fail to get active asgard vaults: %w", err)
			}
			vault.SubFunds(tx.Coins)
			if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				return nil, fmt.Errorf("fail to save vault, err: %w", err)
			}
		}
	}

	memo, _ := ParseMemo(tx.Memo)
	if !memo.IsType(TxSwap) && !memo.IsType(TxAdd) {
		// must be a swap or add transaction
		return &cosmos.Result{}, nil
	}
	// fetch pool from memo
	pool, err := h.mgr.Keeper().GetPool(ctx, memo.GetAsset())
	if err != nil {
		ctx.Logger().Error("fail to get pool for errata tx", "error", err)
		return nil, err
	}

	// subtract amounts from pool balances
	runeCoin := tx.Coins.GetCoin(common.RuneAsset())
	assetCoin := tx.Coins.GetCoin(memo.GetAsset())
	if runeCoin.Amount.GT(pool.BalanceRune) {
		runeCoin.Amount = pool.BalanceRune
	}
	if assetCoin.Amount.GT(pool.BalanceAsset) {
		assetCoin.Amount = pool.BalanceAsset
	}
	pool.BalanceRune = common.SafeSub(pool.BalanceRune, runeCoin.Amount)
	pool.BalanceAsset = common.SafeSub(pool.BalanceAsset, assetCoin.Amount)
	if memo.IsType(TxAdd) {
		lp, err := h.mgr.Keeper().GetLiquidityProvider(ctx, memo.GetAsset(), tx.FromAddress)
		if err != nil {
			return nil, fmt.Errorf("fail to get liquidity provider: %w", err)
		}

		// since this address is being malicious, zero their liquidity provider units
		pool.PoolUnits = common.SafeSub(pool.PoolUnits, lp.Units)
		lp.Units = cosmos.ZeroUint()
		lp.LastAddHeight = common.BlockHeight(ctx)

		h.mgr.Keeper().SetLiquidityProvider(ctx, lp)
	}

	if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
		ctx.Logger().Error("fail to save pool", "error", err)
	}

	// send errata event
	mods := PoolMods{
		NewPoolMod(pool.Asset, runeCoin.Amount, false, assetCoin.Amount, false),
	}

	eventErrata := NewEventErrata(msg.TxID, mods)
	if err := h.mgr.EventMgr().EmitEvent(ctx, eventErrata); err != nil {
		return nil, ErrInternal(err, "fail to emit errata event")
	}
	return &cosmos.Result{}, nil
}

func (h ErrataTxHandler) handleV42(ctx cosmos.Context, msg MsgErrataTx, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	active, err := h.mgr.Keeper().ListActiveNodeAccounts(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}

	voter, err := h.mgr.Keeper().GetErrataTxVoter(ctx, msg.TxID, msg.Chain)
	if err != nil {
		return nil, err
	}
	observeSlashPoints := constAccessor.GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := constAccessor.GetInt64Value(constants.ObservationDelayFlexibility)
	h.mgr.Slasher().IncSlashPoints(ctx, observeSlashPoints, msg.Signer)
	if !voter.Sign(msg.Signer) {
		ctx.Logger().Info("signer already signed MsgErrataTx", "signer", msg.Signer.String(), "txid", msg.TxID)
		return &cosmos.Result{}, nil
	}
	h.mgr.Keeper().SetErrataTxVoter(ctx, voter)
	// doesn't have consensus yet
	if !voter.HasConsensus(active) {
		ctx.Logger().Info("not having consensus yet, return")
		return &cosmos.Result{}, nil
	}

	if voter.BlockHeight > 0 {
		if (voter.BlockHeight + observeFlex) >= common.BlockHeight(ctx) {
			h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, msg.Signer)
		}
		// errata tx already processed
		return &cosmos.Result{}, nil
	}

	voter.BlockHeight = common.BlockHeight(ctx)
	h.mgr.Keeper().SetErrataTxVoter(ctx, voter)
	// decrease the slash points
	h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, voter.GetSigners()...)
	observedVoter, err := h.mgr.Keeper().GetObservedTxInVoter(ctx, msg.TxID)
	if err != nil {
		return nil, err
	}

	if len(observedVoter.Txs) == 0 {
		return h.processErrataOutboundTxV42(ctx, msg)
	}
	// set the observed Tx to reverted
	observedVoter.SetReverted()
	h.mgr.Keeper().SetObservedTxInVoter(ctx, observedVoter)
	if observedVoter.Tx.IsEmpty() || !observedVoter.Tx.IsFinal() {
		ctx.Logger().Info("tx is not finalised, so nothing need to be done", "tx_id", msg.TxID)
		return &cosmos.Result{}, nil
	}

	tx := observedVoter.Tx.Tx
	if !tx.Chain.Equals(msg.Chain) {
		// does not match chain
		return &cosmos.Result{}, nil
	}
	if observedVoter.UpdatedVault {
		vaultPubKey := observedVoter.Tx.ObservedPubKey
		if !vaultPubKey.IsEmpty() {
			// try to deduct the asset from asgard
			vault, err := h.mgr.Keeper().GetVault(ctx, vaultPubKey)
			if err != nil {
				return nil, fmt.Errorf("fail to get active asgard vaults: %w", err)
			}
			vault.SubFunds(tx.Coins)
			if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				return nil, fmt.Errorf("fail to save vault, err: %w", err)
			}
		}
	}

	memo, _ := ParseMemo(tx.Memo)
	// if the tx is a migration , from old valut to new vault , then the inbound tx must have a related outbound tx as well
	if memo.IsInternal() {
		return h.processErrataOutboundTxV42(ctx, msg)
	}

	if !memo.IsType(TxSwap) && !memo.IsType(TxAdd) {
		// must be a swap or add transaction
		return &cosmos.Result{}, nil
	}
	// fetch pool from memo
	pool, err := h.mgr.Keeper().GetPool(ctx, memo.GetAsset())
	if err != nil {
		ctx.Logger().Error("fail to get pool for errata tx", "error", err)
		return nil, err
	}

	// subtract amounts from pool balances
	runeCoin := tx.Coins.GetCoin(common.RuneAsset())
	assetCoin := tx.Coins.GetCoin(memo.GetAsset())
	if runeCoin.Amount.GT(pool.BalanceRune) {
		runeCoin.Amount = pool.BalanceRune
	}
	if assetCoin.Amount.GT(pool.BalanceAsset) {
		assetCoin.Amount = pool.BalanceAsset
	}
	pool.BalanceRune = common.SafeSub(pool.BalanceRune, runeCoin.Amount)
	pool.BalanceAsset = common.SafeSub(pool.BalanceAsset, assetCoin.Amount)
	if memo.IsType(TxAdd) {
		lp, err := h.mgr.Keeper().GetLiquidityProvider(ctx, memo.GetAsset(), tx.FromAddress)
		if err != nil {
			return nil, fmt.Errorf("fail to get liquidity provider: %w", err)
		}

		// since this address is being malicious, zero their liquidity provider units
		pool.PoolUnits = common.SafeSub(pool.PoolUnits, lp.Units)
		lp.Units = cosmos.ZeroUint()
		lp.LastAddHeight = common.BlockHeight(ctx)

		h.mgr.Keeper().SetLiquidityProvider(ctx, lp)
	}

	if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
		ctx.Logger().Error("fail to save pool", "error", err)
	}

	// send errata event
	mods := PoolMods{
		NewPoolMod(pool.Asset, runeCoin.Amount, false, assetCoin.Amount, false),
	}

	eventErrata := NewEventErrata(msg.TxID, mods)
	if err := h.mgr.EventMgr().EmitEvent(ctx, eventErrata); err != nil {
		return nil, ErrInternal(err, "fail to emit errata event")
	}
	return &cosmos.Result{}, nil

}

func (h ErrataTxHandler) handleV45(ctx cosmos.Context, msg MsgErrataTx, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	return h.handleCurrent(ctx, msg, version, constAccessor)
}

func (h ErrataTxHandler) handleCurrent(ctx cosmos.Context, msg MsgErrataTx, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	active, err := h.mgr.Keeper().ListActiveNodeAccounts(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}

	voter, err := h.mgr.Keeper().GetErrataTxVoter(ctx, msg.TxID, msg.Chain)
	if err != nil {
		return nil, err
	}
	observeSlashPoints := constAccessor.GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := constAccessor.GetInt64Value(constants.ObservationDelayFlexibility)
	h.mgr.Slasher().IncSlashPoints(ctx, observeSlashPoints, msg.Signer)
	if !voter.Sign(msg.Signer) {
		ctx.Logger().Info("signer already signed MsgErrataTx", "signer", msg.Signer.String(), "txid", msg.TxID)
		return &cosmos.Result{}, nil
	}
	h.mgr.Keeper().SetErrataTxVoter(ctx, voter)
	// doesn't have consensus yet
	if !voter.HasConsensus(active) {
		ctx.Logger().Info("not having consensus yet, return")
		return &cosmos.Result{}, nil
	}

	if voter.BlockHeight > 0 {
		if (voter.BlockHeight + observeFlex) >= common.BlockHeight(ctx) {
			h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, msg.Signer)
		}
		// errata tx already processed
		return &cosmos.Result{}, nil
	}

	voter.BlockHeight = common.BlockHeight(ctx)
	h.mgr.Keeper().SetErrataTxVoter(ctx, voter)
	// decrease the slash points
	h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, voter.GetSigners()...)
	observedVoter, err := h.mgr.Keeper().GetObservedTxInVoter(ctx, msg.TxID)
	if err != nil {
		return nil, err
	}

	if len(observedVoter.Txs) == 0 {
		return h.processErrataOutboundTxV42(ctx, msg)
	}
	// set the observed Tx to reverted
	observedVoter.SetReverted()
	h.mgr.Keeper().SetObservedTxInVoter(ctx, observedVoter)
	if observedVoter.Tx.IsEmpty() {
		ctx.Logger().Info("tx has not reach consensus yet, so nothing need to be done", "tx_id", msg.TxID)
		return &cosmos.Result{}, nil
	}

	tx := observedVoter.Tx.Tx
	if !tx.Chain.Equals(msg.Chain) {
		// does not match chain
		return &cosmos.Result{}, nil
	}
	if observedVoter.UpdatedVault {
		vaultPubKey := observedVoter.Tx.ObservedPubKey
		if !vaultPubKey.IsEmpty() {
			// try to deduct the asset from asgard
			vault, err := h.mgr.Keeper().GetVault(ctx, vaultPubKey)
			if err != nil {
				return nil, fmt.Errorf("fail to get active asgard vaults: %w", err)
			}
			vault.SubFunds(tx.Coins)
			if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				return nil, fmt.Errorf("fail to save vault, err: %w", err)
			}
		}
	}

	if !observedVoter.Tx.IsFinal() {
		ctx.Logger().Info("tx is not finalised, so nothing need to be done", "tx_id", msg.TxID)
		return &cosmos.Result{}, nil
	}

	memo, _ := ParseMemo(tx.Memo)
	// if the tx is a migration , from old valut to new vault , then the inbound tx must have a related outbound tx as well
	if memo.IsInternal() {
		return h.processErrataOutboundTxV42(ctx, msg)
	}

	if !memo.IsType(TxSwap) && !memo.IsType(TxAdd) {
		// must be a swap or add transaction
		return &cosmos.Result{}, nil
	}

	runeCoin := common.NoCoin
	assetCoin := common.NoCoin
	for _, coin := range tx.Coins {
		if coin.Asset.IsRune() {
			runeCoin = coin
		} else {
			assetCoin = coin
		}
	}

	// fetch pool from memo
	pool, err := h.mgr.Keeper().GetPool(ctx, assetCoin.Asset)
	if err != nil {
		ctx.Logger().Error("fail to get pool for errata tx", "error", err)
		return nil, err
	}

	// subtract amounts from pool balances
	if runeCoin.Amount.GT(pool.BalanceRune) {
		runeCoin.Amount = pool.BalanceRune
	}
	if assetCoin.Amount.GT(pool.BalanceAsset) {
		assetCoin.Amount = pool.BalanceAsset
	}
	pool.BalanceRune = common.SafeSub(pool.BalanceRune, runeCoin.Amount)
	pool.BalanceAsset = common.SafeSub(pool.BalanceAsset, assetCoin.Amount)
	if memo.IsType(TxAdd) {
		lp, err := h.mgr.Keeper().GetLiquidityProvider(ctx, pool.Asset, tx.FromAddress)
		if err != nil {
			return nil, fmt.Errorf("fail to get liquidity provider: %w", err)
		}

		// since this address is being malicious, zero their liquidity provider units
		pool.PoolUnits = common.SafeSub(pool.PoolUnits, lp.Units)
		lp.Units = cosmos.ZeroUint()
		lp.LastAddHeight = common.BlockHeight(ctx)

		h.mgr.Keeper().SetLiquidityProvider(ctx, lp)
	}

	if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
		ctx.Logger().Error("fail to save pool", "error", err)
	}

	// send errata event
	mods := PoolMods{
		NewPoolMod(pool.Asset, runeCoin.Amount, false, assetCoin.Amount, false),
	}

	eventErrata := NewEventErrata(msg.TxID, mods)
	if err := h.mgr.EventMgr().EmitEvent(ctx, eventErrata); err != nil {
		return nil, ErrInternal(err, "fail to emit errata event")
	}
	return &cosmos.Result{}, nil
}

// processErrataOutboundTx when the network detect an outbound tx which previously had been sent out to customer , however it get re-org , and it doesn't
// exist on the external chain anymore , then it will need to reschedule the tx
func (h ErrataTxHandler) processErrataOutboundTx(ctx cosmos.Context, msg MsgErrataTx) (*cosmos.Result, error) {
	txOutVoter, err := h.mgr.Keeper().GetObservedTxOutVoter(ctx, msg.GetTxID())
	if err != nil {
		return nil, fmt.Errorf("fail to get observed tx out voter for tx (%s) : %w", msg.GetTxID(), err)
	}
	if len(txOutVoter.Txs) == 0 {
		return nil, fmt.Errorf("cannot find tx: %s", msg.TxID)
	}
	if txOutVoter.Tx.IsEmpty() {
		return nil, fmt.Errorf("tx out voter is not finalised")
	}
	tx := txOutVoter.Tx.Tx
	if !tx.Chain.Equals(msg.Chain) || tx.Coins.IsEmpty() {
		return &cosmos.Result{}, nil
	}
	// parse the outbound tx memo, so we can figure out which inbound tx triggered the outbound
	m, err := memo.ParseMemo(tx.Memo)
	if err != nil {
		return nil, fmt.Errorf("fail to parse memo(%s): %w", tx.Memo, err)
	}
	if !m.IsOutbound() {
		return nil, fmt.Errorf("%s is not outbound tx", m)
	}
	vaultPubKey := txOutVoter.Tx.ObservedPubKey
	if !vaultPubKey.IsEmpty() {
		v, err := h.mgr.Keeper().GetVault(ctx, vaultPubKey)
		if err != nil {
			return nil, fmt.Errorf("fail to get vault with pubkey %s: %w", vaultPubKey, err)
		}
		compensate := true
		if v.IsAsgard() {
			if v.Status == RetiringVault || v.Status == ActiveVault {
				v.AddFunds(tx.Coins)
				compensate = false
			}
		}
		if v.IsYggdrasil() {
			node, err := h.mgr.Keeper().GetNodeAccountByPubKey(ctx, v.PubKey)
			if err != nil {
				return nil, fmt.Errorf("fail to get node account with pubkey: %s,err: %w", v.PubKey, err)
			}
			if !node.IsEmpty() && !node.Bond.IsZero() {
				// as long as the node still has bond , we can just credit it back to it's yggdrasil vault.
				// if the node request to leave , but has not refund it's bond yet , then they will be slashed,
				// if the node stay in the network , then they can still hold the fund until they leave
				// if the node already left , but only has little bond left , the slash logic will take it all , and then
				// subsidise pool with reserve
				v.AddFunds(tx.Coins)
				compensate = false
			}
		}

		if !v.IsEmpty() {
			if err := h.mgr.Keeper().SetVault(ctx, v); err != nil {
				return nil, fmt.Errorf("fail to save vault: %w", err)
			}
		}
		if compensate {
			for _, coin := range tx.Coins {
				if coin.Asset.IsRune() {
					// it is using native rune, so outbound can't be RUNE
					continue
				}
				p, err := h.mgr.Keeper().GetPool(ctx, coin.Asset)
				if err != nil {
					return nil, fmt.Errorf("fail to get pool(%s): %w", coin.Asset, err)
				}
				runeValue := p.AssetValueInRune(coin.Amount)
				p.BalanceRune = p.BalanceRune.Add(runeValue)
				p.BalanceAsset = common.SafeSub(p.BalanceAsset, coin.Amount)
				if err := h.mgr.Keeper().SendFromModuleToModule(ctx, ReserveName, AsgardName, common.Coins{
					common.NewCoin(common.RuneAsset(), runeValue),
				}); err != nil {
					return nil, fmt.Errorf("fail to send fund from reserve to asgard: %w", err)
				}
				if err := h.mgr.Keeper().SetPool(ctx, p); err != nil {
					return nil, fmt.Errorf("fail to save pool (%s) : %w", p.Asset, err)
				}
				// send errata event
				mods := PoolMods{
					NewPoolMod(p.Asset, runeValue, true, coin.Amount, false),
				}

				eventErrata := NewEventErrata(msg.TxID, mods)
				if err := h.mgr.EventMgr().EmitEvent(ctx, eventErrata); err != nil {
					return nil, ErrInternal(err, "fail to emit errata event")
				}
			}
		}
	}

	if m.IsInternal() {
		ctx.Logger().Info("%s is internal tx , don't do anything", tx.Memo)
		return &cosmos.Result{}, nil
	}
	txInVoter, err := h.mgr.Keeper().GetObservedTxInVoter(ctx, m.GetTxID())
	if err != nil {
		return nil, fmt.Errorf("fail to get tx in voter for tx (%s): %w", m.GetTxID(), err)
	}

	for _, item := range txInVoter.Actions {
		if !item.OutHash.Equals(msg.GetTxID()) {
			continue
		}
		newTxOutItem := TxOutItem{
			Chain:     item.Chain,
			InHash:    item.InHash,
			ToAddress: item.ToAddress,
			Coin:      item.Coin,
			Memo:      item.Memo,
		}
		_, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, newTxOutItem)
		if err != nil {
			return nil, fmt.Errorf("fail to reschedule tx out item: %w", err)
		}
		break
	}
	txOutVoter.SetReverted()
	h.mgr.Keeper().SetObservedTxOutVoter(ctx, txOutVoter)
	return &cosmos.Result{}, nil
}

// processErrataOutboundTx when the network detect an outbound tx which previously had been sent out to customer , however it get re-org , and it doesn't
// exist on the external chain anymore , then it will need to reschedule the tx
func (h ErrataTxHandler) processErrataOutboundTxV42(ctx cosmos.Context, msg MsgErrataTx) (*cosmos.Result, error) {
	txOutVoter, err := h.mgr.Keeper().GetObservedTxOutVoter(ctx, msg.GetTxID())
	if err != nil {
		return nil, fmt.Errorf("fail to get observed tx out voter for tx (%s) : %w", msg.GetTxID(), err)
	}
	if len(txOutVoter.Txs) == 0 {
		return nil, fmt.Errorf("cannot find tx: %s", msg.TxID)
	}
	if txOutVoter.Tx.IsEmpty() {
		return nil, fmt.Errorf("tx out voter is not finalised")
	}
	tx := txOutVoter.Tx.Tx
	if !tx.Chain.Equals(msg.Chain) || tx.Coins.IsEmpty() {
		return &cosmos.Result{}, nil
	}
	// parse the outbound tx memo, so we can figure out which inbound tx triggered the outbound
	m, err := memo.ParseMemo(tx.Memo)
	if err != nil {
		return nil, fmt.Errorf("fail to parse memo(%s): %w", tx.Memo, err)
	}
	if !m.IsOutbound() && !m.IsInternal() {
		return nil, fmt.Errorf("%s is not outbound or internal tx", m)
	}
	vaultPubKey := txOutVoter.Tx.ObservedPubKey
	if !vaultPubKey.IsEmpty() {
		v, err := h.mgr.Keeper().GetVault(ctx, vaultPubKey)
		if err != nil {
			return nil, fmt.Errorf("fail to get vault with pubkey %s: %w", vaultPubKey, err)
		}
		compensate := true
		if v.IsAsgard() {
			// if the fund is sending out from asgard , then it need to be credit back to asgard
			// if the asgard has been retired (inactive), need to set it to Retiring again , so the fund can be migrated
			v.AddFunds(tx.Coins)
			compensate = false
			if v.Status == InactiveVault {
				ctx.Logger().Info("Errata cause retired vault to be resurrect", "vault pub key", v.PubKey)
				v.UpdateStatus(RetiringVault, ctx.BlockHeight())
			}
		}

		if v.IsYggdrasil() {
			node, err := h.mgr.Keeper().GetNodeAccountByPubKey(ctx, v.PubKey)
			if err != nil {
				return nil, fmt.Errorf("fail to get node account with pubkey: %s,err: %w", v.PubKey, err)
			}
			if !node.IsEmpty() && !node.Bond.IsZero() {
				// as long as the node still has bond , we can just credit it back to it's yggdrasil vault.
				// if the node request to leave , but has not refund it's bond yet , then they will be slashed,
				// if the node stay in the network , then they can still hold the fund until they leave
				// if the node already left , but only has little bond left , the slash logic will take it all , and then
				// subsidise pool with reserve
				v.AddFunds(tx.Coins)
				compensate = false
			}
		}

		if !v.IsEmpty() {
			if err := h.mgr.Keeper().SetVault(ctx, v); err != nil {
				return nil, fmt.Errorf("fail to save vault: %w", err)
			}
		}
		if compensate {
			for _, coin := range tx.Coins {
				if coin.Asset.IsRune() {
					// it is using native rune, so outbound can't be RUNE
					continue
				}
				p, err := h.mgr.Keeper().GetPool(ctx, coin.Asset)
				if err != nil {
					return nil, fmt.Errorf("fail to get pool(%s): %w", coin.Asset, err)
				}
				runeValue := p.AssetValueInRune(coin.Amount)
				p.BalanceRune = p.BalanceRune.Add(runeValue)
				p.BalanceAsset = common.SafeSub(p.BalanceAsset, coin.Amount)
				if err := h.mgr.Keeper().SendFromModuleToModule(ctx, ReserveName, AsgardName, common.Coins{
					common.NewCoin(common.RuneAsset(), runeValue),
				}); err != nil {
					return nil, fmt.Errorf("fail to send fund from reserve to asgard: %w", err)
				}
				if err := h.mgr.Keeper().SetPool(ctx, p); err != nil {
					return nil, fmt.Errorf("fail to save pool (%s) : %w", p.Asset, err)
				}
				// send errata event
				mods := PoolMods{
					NewPoolMod(p.Asset, runeValue, true, coin.Amount, false),
				}

				eventErrata := NewEventErrata(msg.TxID, mods)
				if err := h.mgr.EventMgr().EmitEvent(ctx, eventErrata); err != nil {
					return nil, ErrInternal(err, "fail to emit errata event")
				}
			}
		}
	}

	if !m.IsInternal() {
		txInVoter, err := h.mgr.Keeper().GetObservedTxInVoter(ctx, m.GetTxID())
		if err != nil {
			return nil, fmt.Errorf("fail to get tx in voter for tx (%s): %w", m.GetTxID(), err)
		}

		for _, item := range txInVoter.Actions {
			if !item.OutHash.Equals(msg.GetTxID()) {
				continue
			}
			newTxOutItem := TxOutItem{
				Chain:     item.Chain,
				InHash:    item.InHash,
				ToAddress: item.ToAddress,
				Coin:      item.Coin,
				Memo:      item.Memo,
			}
			_, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, newTxOutItem)
			if err != nil {
				return nil, fmt.Errorf("fail to reschedule tx out item: %w", err)
			}
			break
		}
	}
	txOutVoter.SetReverted()
	h.mgr.Keeper().SetObservedTxOutVoter(ctx, txOutVoter)
	return &cosmos.Result{}, nil
}
