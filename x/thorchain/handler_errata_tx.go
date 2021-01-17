package thorchain

import (
	"fmt"

	"github.com/blang/semver"
	se "github.com/cosmos/cosmos-sdk/types/errors"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// ErrataTxHandler is to handle ErrataTx message
type ErrataTxHandler struct {
	keeper keeper.Keeper
	mgr    Manager
}

// NewErrataTxHandler create new instance of ErrataTxHandler
func NewErrataTxHandler(keeper keeper.Keeper, mgr Manager) ErrataTxHandler {
	return ErrataTxHandler{
		keeper: keeper,
		mgr:    mgr,
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
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	if !isSignedByActiveNodeAccounts(ctx, h.keeper, msg.GetSigners()) {
		return cosmos.ErrUnauthorized(notAuthorized.Error())
	}

	return nil
}

func (h ErrataTxHandler) handle(ctx cosmos.Context, msg MsgErrataTx, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	ctx.Logger().Info("handleMsgErrataTx request", "txid", msg.TxID.String())
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, version, constAccessor)
	}
	ctx.Logger().Error(errInvalidVersion.Error())
	return nil, errBadVersion
}

func (h ErrataTxHandler) handleV1(ctx cosmos.Context, msg MsgErrataTx, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	active, err := h.keeper.ListActiveNodeAccounts(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}

	voter, err := h.keeper.GetErrataTxVoter(ctx, msg.TxID, msg.Chain)
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
	h.keeper.SetErrataTxVoter(ctx, voter)
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
	h.keeper.SetErrataTxVoter(ctx, voter)
	// decrease the slash points
	h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, voter.GetSigners()...)
	observedVoter, err := h.keeper.GetObservedTxInVoter(ctx, msg.TxID)
	if err != nil {
		return nil, err
	}

	if len(observedVoter.Txs) == 0 {
		return nil, se.Wrap(errInternal, fmt.Sprintf("cannot find tx: %s", msg.TxID))
	}
	// set the observed Tx to reverted
	observedVoter.SetReverted()
	h.keeper.SetObservedTxInVoter(ctx, observedVoter)
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
			vault, err := h.keeper.GetVault(ctx, vaultPubKey)
			if err != nil {
				return nil, fmt.Errorf("fail to get active asgard vaults: %w", err)
			}
			vault.SubFunds(tx.Coins)
			if err := h.keeper.SetVault(ctx, vault); err != nil {
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
	pool, err := h.keeper.GetPool(ctx, memo.GetAsset())
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
		lp, err := h.keeper.GetLiquidityProvider(ctx, memo.GetAsset(), tx.FromAddress)
		if err != nil {
			return nil, fmt.Errorf("fail to get liquidity provider: %w", err)
		}

		// since this address is being malicious, zero their liquidity provider units
		pool.PoolUnits = common.SafeSub(pool.PoolUnits, lp.Units)
		lp.Units = cosmos.ZeroUint()
		lp.LastAddHeight = common.BlockHeight(ctx)

		h.keeper.SetLiquidityProvider(ctx, lp)
	}

	if err := h.keeper.SetPool(ctx, pool); err != nil {
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
