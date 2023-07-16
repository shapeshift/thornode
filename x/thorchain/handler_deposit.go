package thorchain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"
	se "github.com/cosmos/cosmos-sdk/types/errors"
	tmtypes "github.com/tendermint/tendermint/types"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// DepositHandler is to process native messages on THORChain
type DepositHandler struct {
	mgr Manager
}

// NewDepositHandler create a new instance of DepositHandler
func NewDepositHandler(mgr Manager) DepositHandler {
	return DepositHandler{
		mgr: mgr,
	}
}

// Run is the main entry of DepositHandler
func (h DepositHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgDeposit)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgDeposit failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process MsgDeposit", "error", err)
		return nil, err
	}
	return result, nil
}

func (h DepositHandler) validate(ctx cosmos.Context, msg MsgDeposit) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errInvalidVersion
}

func (h DepositHandler) validateV1(ctx cosmos.Context, msg MsgDeposit) error {
	return msg.ValidateBasic()
}

func (h DepositHandler) handle(ctx cosmos.Context, msg MsgDeposit) (*cosmos.Result, error) {
	ctx.Logger().Info("receive MsgDeposit", "from", msg.GetSigners()[0], "coins", msg.Coins, "memo", msg.Memo)
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.115.0")):
		return h.handleV115(ctx, msg)
	case version.GTE(semver.MustParse("1.113.0")):
		return h.handleV113(ctx, msg)
	case version.GTE(semver.MustParse("1.112.0")):
		return h.handleV112(ctx, msg)
	case version.GTE(semver.MustParse("1.108.0")):
		return h.handleV108(ctx, msg)
	case version.GTE(semver.MustParse("1.105.0")):
		return h.handleV105(ctx, msg)
	case version.GTE(semver.MustParse("1.99.0")):
		return h.handleV99(ctx, msg)
	case version.GTE(semver.MustParse("1.87.0")):
		return h.handleV87(ctx, msg)
	case version.GTE(semver.MustParse("0.67.0")):
		return h.handleV67(ctx, msg)
	}
	return nil, errInvalidVersion
}

func (h DepositHandler) handleV115(ctx cosmos.Context, msg MsgDeposit) (*cosmos.Result, error) {
	if h.mgr.Keeper().IsChainHalted(ctx, common.THORChain) {
		return nil, fmt.Errorf("unable to use MsgDeposit while THORChain is halted")
	}

	coins, err := msg.Coins.Native()
	if err != nil {
		return nil, ErrInternal(err, "coins are native to THORChain")
	}

	if !h.mgr.Keeper().HasCoins(ctx, msg.GetSigners()[0], coins) {
		return nil, cosmos.ErrInsufficientCoins(err, "insufficient funds")
	}

	hash := tmtypes.Tx(ctx.TxBytes()).Hash()
	txID, err := common.NewTxID(fmt.Sprintf("%X", hash))
	if err != nil {
		return nil, fmt.Errorf("fail to get tx hash: %w", err)
	}
	existingVoter, err := h.mgr.Keeper().GetObservedTxInVoter(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("fail to get existing voter")
	}
	if len(existingVoter.Txs) > 0 {
		return nil, fmt.Errorf("txid: %s already exist", txID.String())
	}
	from, err := common.NewAddress(msg.GetSigners()[0].String())
	if err != nil {
		return nil, fmt.Errorf("fail to get from address: %w", err)
	}

	handler := NewInternalHandler(h.mgr)

	memo, _ := ParseMemoWithTHORNames(ctx, h.mgr.Keeper(), msg.Memo) // ignore err
	if memo.IsOutbound() || memo.IsInternal() {
		return nil, fmt.Errorf("cannot send inbound an outbound or internal transacion")
	}

	var targetModule string
	switch memo.GetType() {
	case TxBond, TxUnBond, TxLeave:
		targetModule = BondName
	case TxReserve, TxTHORName:
		targetModule = ReserveName
	default:
		targetModule = AsgardName
	}
	coinsInMsg := msg.Coins
	if !coinsInMsg.IsEmpty() {
		// send funds to target module
		err := h.mgr.Keeper().SendFromAccountToModule(ctx, msg.GetSigners()[0], targetModule, msg.Coins)
		if err != nil {
			return nil, err
		}
	}

	to, err := h.mgr.Keeper().GetModuleAddress(targetModule)
	if err != nil {
		return nil, fmt.Errorf("fail to get to address: %w", err)
	}

	tx := common.NewTx(txID, from, to, coinsInMsg, common.Gas{}, msg.Memo)
	tx.Chain = common.THORChain

	// construct msg from memo
	txIn := ObservedTx{Tx: tx}
	txInVoter := NewObservedTxVoter(txIn.Tx.ID, []ObservedTx{txIn})
	txInVoter.FinalisedHeight = ctx.BlockHeight()
	txInVoter.Tx = txIn
	h.mgr.Keeper().SetObservedTxInVoter(ctx, txInVoter)

	m, txErr := processOneTxIn(ctx, h.mgr.GetVersion(), h.mgr.Keeper(), txIn, msg.Signer)
	if txErr != nil {
		ctx.Logger().Error("fail to process native inbound tx", "error", txErr.Error(), "tx hash", tx.ID.String())
		if txIn.Tx.Coins.IsEmpty() {
			return &cosmos.Result{}, nil
		}
		if newErr := refundTx(ctx, txIn, h.mgr, CodeInvalidMemo, txErr.Error(), targetModule); nil != newErr {
			return nil, newErr
		}

		return &cosmos.Result{}, nil
	}

	// check if we've halted trading
	_, isSwap := m.(*MsgSwap)
	_, isAddLiquidity := m.(*MsgAddLiquidity)
	if isSwap || isAddLiquidity {
		if h.mgr.Keeper().IsTradingHalt(ctx, m) || h.mgr.Keeper().RagnarokInProgress(ctx) {
			if txIn.Tx.Coins.IsEmpty() {
				return &cosmos.Result{}, nil
			}
			if newErr := refundTx(ctx, txIn, h.mgr, se.ErrUnauthorized.ABCICode(), "trading halted", targetModule); nil != newErr {
				return nil, ErrInternal(newErr, "trading is halted, fail to refund")
			}
			return &cosmos.Result{}, nil
		}
	}

	// if its a swap, send it to our queue for processing later
	if isSwap {
		msg, ok := m.(*MsgSwap)
		if ok {
			h.addSwap(ctx, *msg)
		}
		return &cosmos.Result{}, nil
	}

	// if it is a loan, inject the TxID and ToAddress into the context
	_, isLoanOpen := m.(*MsgLoanOpen)
	_, isLoanRepayment := m.(*MsgLoanRepayment)
	mCtx := ctx
	if isLoanOpen || isLoanRepayment {
		mCtx = ctx.WithValue(constants.CtxLoanTxID, txIn.Tx.ID)
		mCtx = mCtx.WithValue(constants.CtxLoanToAddress, txIn.Tx.ToAddress)
	}

	result, err := handler(mCtx, m)
	if err != nil {
		code := uint32(1)
		var e se.Error
		if errors.As(err, &e) {
			code = e.ABCICode()
		}
		if txIn.Tx.Coins.IsEmpty() {
			return &cosmos.Result{}, nil
		}
		if err := refundTx(ctx, txIn, h.mgr, code, err.Error(), targetModule); err != nil {
			return nil, fmt.Errorf("fail to refund tx: %w", err)
		}
		return &cosmos.Result{}, nil
	}
	// for those Memo that will not have outbound at all , set the observedTx to done
	if !memo.GetType().HasOutbound() {
		txInVoter.SetDone()
		h.mgr.Keeper().SetObservedTxInVoter(ctx, txInVoter)
	}
	return result, nil
}

func (h DepositHandler) addSwap(ctx cosmos.Context, msg MsgSwap) {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.116.0")):
		h.addSwapV116(ctx, msg)
	case version.GTE(semver.MustParse("1.98.0")):
		h.addSwapV98(ctx, msg)
	default:
		h.addSwapV65(ctx, msg)
	}
}

func (h DepositHandler) addSwapV116(ctx cosmos.Context, msg MsgSwap) {
	if h.mgr.Keeper().OrderBooksEnabled(ctx) {
		source := msg.Tx.Coins[0]
		target := common.NewCoin(msg.TargetAsset, msg.TradeTarget)
		evt := NewEventLimitOrder(source, target, msg.Tx.ID)
		if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			ctx.Logger().Error("fail to emit limit order event", "error", err)
		}
		if err := h.mgr.Keeper().SetOrderBookItem(ctx, msg); err != nil {
			ctx.Logger().Error("fail to add swap to queue", "error", err)
		}
	} else {
		h.addSwapDirect(ctx, msg)
	}
}

// addSwapDirect adds the swap directly to the swap queue (no order book) - segmented
// out into its own function to allow easier maintenance of original behavior vs order
// book behavior.
func (h DepositHandler) addSwapDirect(ctx cosmos.Context, msg MsgSwap) {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.116.0")):
		h.addSwapDirectV116(ctx, msg)
	default:
		h.addSwapV65(ctx, msg)
	}
}

func (h DepositHandler) addSwapDirectV116(ctx cosmos.Context, msg MsgSwap) {
	if msg.Tx.Coins.IsEmpty() {
		return
	}
	amt := cosmos.ZeroUint()
	swapSourceAsset := msg.Tx.Coins[0].Asset

	// Check if affiliate fee should be paid out
	if !msg.AffiliateBasisPoints.IsZero() && msg.AffiliateAddress.IsChain(common.THORChain) {
		amt = common.GetSafeShare(
			msg.AffiliateBasisPoints,
			cosmos.NewUint(10000),
			msg.Tx.Coins[0].Amount,
		)
		msg.Tx.Coins[0].Amount = common.SafeSub(msg.Tx.Coins[0].Amount, amt)
	}

	// Queue the main swap
	if err := h.mgr.Keeper().SetSwapQueueItem(ctx, msg, 0); err != nil {
		ctx.Logger().Error("fail to add swap to queue", "error", err)
	}

	// Affiliate fee flow
	if !amt.IsZero() {
		toAddress, err := msg.AffiliateAddress.AccAddress()
		if err != nil {
			ctx.Logger().Error("fail to convert address into AccAddress", "msg", msg.AffiliateAddress, "error", err)
			return
		}

		memo, err := ParseMemoWithTHORNames(ctx, h.mgr.Keeper(), msg.Tx.Memo)
		if err != nil {
			ctx.Logger().Error("fail to parse swap memo", "memo", msg.Tx.Memo, "error", err)
			return
		}
		// since native transaction fee has been charged to inbound from address, thus for affiliated fee , the network doesn't need to charge it again
		coin := common.NewCoin(swapSourceAsset, amt)
		affThorname := memo.GetAffiliateTHORName()

		// PreferredAsset set, update the AffiliateCollector module
		if affThorname != nil && !affThorname.PreferredAsset.IsEmpty() && swapSourceAsset.IsNativeRune() {
			h.updateAffiliateCollector(ctx, coin, msg, affThorname)
			return
		}

		// No PreferredAsset set, normal behavior
		sdkErr := h.mgr.Keeper().SendFromModuleToAccount(ctx, AsgardName, toAddress, common.NewCoins(coin))
		if sdkErr != nil {
			ctx.Logger().Error("fail to send native asset to affiliate", "msg", msg.AffiliateAddress, "error", err, "asset", swapSourceAsset)
		}
	}
}

// updateAffiliateCollector - accrue RUNE in the AffiliateCollector module and check if
// a PreferredAsset swap should be triggered
func (h DepositHandler) updateAffiliateCollector(ctx cosmos.Context, coin common.Coin, msg MsgSwap, thorname *THORName) {
	affcol, err := h.mgr.Keeper().GetAffiliateCollector(ctx, thorname.Owner)
	if err != nil {
		ctx.Logger().Error("failed to get affiliate collector", "msg", msg.AffiliateAddress, "error", err)
	} else {
		if err := h.mgr.Keeper().SendFromModuleToModule(ctx, AsgardName, AffiliateCollectorName, common.NewCoins(coin)); err != nil {
			ctx.Logger().Error("failed to send funds to affiliate collector", "error", err)
		} else {
			affcol.RuneAmount = affcol.RuneAmount.Add(coin.Amount)
			h.mgr.Keeper().SetAffiliateCollector(ctx, affcol)
		}
	}

	// Check if accrued RUNE is 100x current outbound fee of preferred asset chain, if so
	// trigger the preferred asset swap
	ofRune := h.mgr.GasMgr().GetFee(ctx, thorname.PreferredAsset.GetChain(), common.RuneNative)
	multiplier := h.mgr.Keeper().GetConfigInt64(ctx, constants.PreferredAssetOutboundFeeMultiplier)
	if affcol.RuneAmount.GT(ofRune.Mul(cosmos.NewUint(uint64(multiplier)))) {
		if err = triggerPreferredAssetSwap(ctx, h.mgr, msg.AffiliateAddress, msg.Tx.ID, *thorname, affcol, 1); err != nil {
			ctx.Logger().Error("fail to swap to preferred asset", "thorname", thorname.Name, "err", err)
		}
	}
}

// DepositAnteHandler called by the ante handler to gate mempool entry
// and also during deliver. Store changes will persist if this function
// succeeds, regardless of the success of the transaction.
func DepositAnteHandler(ctx cosmos.Context, v semver.Version, k keeper.Keeper, msg MsgDeposit) error {
	// TODO remove on hard fork
	if v.LT(semver.MustParse("1.115.0")) {
		nativeTxFee := k.GetNativeTxFee(ctx)
		gas := common.NewCoin(common.RuneNative, nativeTxFee)
		gasFee, err := gas.Native()
		if err != nil {
			return fmt.Errorf("fail to get gas fee: %w", err)
		}
		totalCoins := cosmos.NewCoins(gasFee)
		if !k.HasCoins(ctx, msg.GetSigners()[0], totalCoins) {
			return cosmos.ErrInsufficientCoins(err, "insufficient funds")
		}
		return nil
	}

	return k.DeductNativeTxFeeFromAccount(ctx, msg.GetSigners()[0])
}
