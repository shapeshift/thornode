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
	if version.GTE(semver.MustParse("0.63.0")) {
		return h.handleV63(ctx, msg)
	} else if version.GTE(semver.MustParse("0.61.0")) {
		return h.handleV61(ctx, msg)
	} else if version.GTE(semver.MustParse("0.58.0")) {
		return h.handleV58(ctx, msg)
	} else if version.GTE(semver.MustParse("0.47.0")) {
		return h.handleV47(ctx, msg)
	} else if version.GTE(semver.MustParse("0.46.0")) {
		return h.handleV46(ctx, msg)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg)
	}
	return nil, errInvalidVersion
}

func (h DepositHandler) handleV1(ctx cosmos.Context, msg MsgDeposit) (*cosmos.Result, error) {
	nativeTxFee, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || nativeTxFee < 0 {
		nativeTxFee = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
	}
	gas := common.NewCoin(common.RuneNative, cosmos.NewUint(uint64(nativeTxFee)))
	gasFee, err := gas.Native()
	if err != nil {
		return nil, fmt.Errorf("fail to get gas fee: %w", err)
	}

	coins, err := msg.Coins.Native()
	if err != nil {
		return nil, ErrInternal(err, "coins are native to THORChain")
	}

	totalCoins := cosmos.NewCoins(gasFee).Add(coins...)
	if !h.mgr.Keeper().HasCoins(ctx, msg.GetSigners()[0], totalCoins) {
		return nil, cosmos.ErrInsufficientCoins(err, "insufficient funds")
	}

	// send gas to reserve
	sdkErr := h.mgr.Keeper().SendFromAccountToModule(ctx, msg.GetSigners()[0], ReserveName, common.NewCoins(gas))
	if sdkErr != nil {
		return nil, fmt.Errorf("unable to send gas to reserve: %w", sdkErr)
	}

	hash := tmtypes.Tx(ctx.TxBytes()).Hash()
	txID, err := common.NewTxID(fmt.Sprintf("%X", hash))
	if err != nil {
		return nil, fmt.Errorf("fail to get tx hash: %w", err)
	}
	from, err := common.NewAddress(msg.GetSigners()[0].String())
	if err != nil {
		return nil, fmt.Errorf("fail to get from address: %w", err)
	}
	to, err := h.mgr.Keeper().GetModuleAddress(AsgardName)
	if err != nil {
		return nil, fmt.Errorf("fail to get to address: %w", err)
	}

	handler := NewInternalHandler(h.mgr)

	memo, _ := ParseMemo(msg.Memo) // ignore err
	var targetModule string
	switch memo.GetType() {
	case TxBond:
		targetModule = BondName
	case TxReserve:
		targetModule = ReserveName
	default:
		targetModule = AsgardName
	}
	coinsInMsg := msg.Coins
	if !coinsInMsg.IsEmpty() {
		// send funds to target module
		sdkErr = h.mgr.Keeper().SendFromAccountToModule(ctx, msg.GetSigners()[0], targetModule, msg.Coins)
		if sdkErr != nil {
			return nil, sdkErr
		}
	}

	tx := common.NewTx(txID, from, to, coinsInMsg, common.Gas{gas}, msg.Memo)
	tx.Chain = common.THORChain

	// construct msg from memo
	txIn := ObservedTx{Tx: tx}
	txInVoter := NewObservedTxVoter(txIn.Tx.ID, []ObservedTx{txIn})
	activeNodes, err := h.mgr.Keeper().ListActiveNodeAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get all active nodes: %w", err)
	}
	for _, node := range activeNodes {
		txInVoter.Add(txIn, node.NodeAddress)
	}
	txInVoter.FinalisedHeight = common.BlockHeight(ctx)
	txInVoter.Tx = txInVoter.GetTx(activeNodes)
	h.mgr.Keeper().SetObservedTxInVoter(ctx, txInVoter)
	m, txErr := processOneTxIn(ctx, h.mgr.GetVersion(), h.mgr.Keeper(), txIn, msg.Signer)
	if txErr != nil {
		ctx.Logger().Error("fail to process native inbound tx", "error", txErr.Error(), "tx hash", tx.ID.String())
		if txIn.Tx.Coins.IsEmpty() {
			return &cosmos.Result{}, nil
		}
		if newErr := refundTx(ctx, txIn, h.mgr, h.mgr.GetConstants(), CodeInvalidMemo, txErr.Error(), targetModule); nil != newErr {
			return nil, newErr
		}

		return &cosmos.Result{}, nil
	}

	// check if we've halted trading
	_, isSwap := m.(*MsgSwap)
	_, isAddLiquidity := m.(*MsgAddLiquidity)
	haltTrading, err := h.mgr.Keeper().GetMimir(ctx, "HaltTrading")
	if isSwap || isAddLiquidity {
		if (haltTrading > 0 && haltTrading < common.BlockHeight(ctx) && err == nil) || h.mgr.Keeper().RagnarokInProgress(ctx) {
			ctx.Logger().Info("trading is halted!!")
			if txIn.Tx.Coins.IsEmpty() {
				return &cosmos.Result{}, nil
			}
			if newErr := refundTx(ctx, txIn, h.mgr, h.mgr.GetConstants(), se.ErrUnauthorized.ABCICode(), "trading halted", targetModule); nil != newErr {
				return nil, ErrInternal(newErr, "trading is halted, fail to refund")
			}
			return &cosmos.Result{}, nil
		}
	}

	// if its a swap, send it to our queue for processing later
	if isSwap {
		msg := m.(*MsgSwap)
		h.addSwapV1(ctx, *msg)
		return &cosmos.Result{}, nil
	}

	result, err := handler(ctx, m)
	if err != nil {
		code := uint32(1)
		var e se.Error
		if errors.As(err, &e) {
			code = e.ABCICode()
		}
		if txIn.Tx.Coins.IsEmpty() {
			return &cosmos.Result{}, nil
		}
		if err := refundTx(ctx, txIn, h.mgr, h.mgr.GetConstants(), code, err.Error(), targetModule); err != nil {
			return nil, fmt.Errorf("fail to refund tx: %w", err)
		}
	}
	// for those Memo that will not have outbound at all , set the observedTx to done
	if !memo.GetType().HasOutbound() {
		txInVoter.SetDone()
		h.mgr.Keeper().SetObservedTxInVoter(ctx, txInVoter)
	}
	return result, nil

}

func (h DepositHandler) handleV46(ctx cosmos.Context, msg MsgDeposit) (*cosmos.Result, error) {
	nativeTxFee, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || nativeTxFee < 0 {
		nativeTxFee = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
	}
	gas := common.NewCoin(common.RuneNative, cosmos.NewUint(uint64(nativeTxFee)))
	gasFee, err := gas.Native()
	if err != nil {
		return nil, fmt.Errorf("fail to get gas fee: %w", err)
	}

	coins, err := msg.Coins.Native()
	if err != nil {
		return nil, ErrInternal(err, "coins are native to THORChain")
	}

	totalCoins := cosmos.NewCoins(gasFee).Add(coins...)
	if !h.mgr.Keeper().HasCoins(ctx, msg.GetSigners()[0], totalCoins) {
		return nil, cosmos.ErrInsufficientCoins(err, "insufficient funds")
	}

	// send gas to reserve
	sdkErr := h.mgr.Keeper().SendFromAccountToModule(ctx, msg.GetSigners()[0], ReserveName, common.NewCoins(gas))
	if sdkErr != nil {
		return nil, fmt.Errorf("unable to send gas to reserve: %w", sdkErr)
	}

	hash := tmtypes.Tx(ctx.TxBytes()).Hash()
	txID, err := common.NewTxID(fmt.Sprintf("%X", hash))
	if err != nil {
		return nil, fmt.Errorf("fail to get tx hash: %w", err)
	}
	from, err := common.NewAddress(msg.GetSigners()[0].String())
	if err != nil {
		return nil, fmt.Errorf("fail to get from address: %w", err)
	}
	to, err := h.mgr.Keeper().GetModuleAddress(AsgardName)
	if err != nil {
		return nil, fmt.Errorf("fail to get to address: %w", err)
	}

	handler := NewInternalHandler(h.mgr)

	memo, _ := ParseMemo(msg.Memo) // ignore err
	var targetModule string
	switch memo.GetType() {
	case TxBond:
		targetModule = BondName
	case TxReserve:
		targetModule = ReserveName
	default:
		targetModule = AsgardName
	}
	coinsInMsg := msg.Coins
	if !coinsInMsg.IsEmpty() {
		// send funds to target module
		sdkErr = h.mgr.Keeper().SendFromAccountToModule(ctx, msg.GetSigners()[0], targetModule, msg.Coins)
		if sdkErr != nil {
			return nil, sdkErr
		}
	}

	tx := common.NewTx(txID, from, to, coinsInMsg, common.Gas{gas}, msg.Memo)
	tx.Chain = common.THORChain

	// construct msg from memo
	txIn := ObservedTx{Tx: tx}
	txInVoter := NewObservedTxVoter(txIn.Tx.ID, []ObservedTx{txIn})
	activeNodes, err := h.mgr.Keeper().ListActiveNodeAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get all active nodes: %w", err)
	}
	for _, node := range activeNodes {
		txInVoter.Add(txIn, node.NodeAddress)
	}
	txInVoter.FinalisedHeight = common.BlockHeight(ctx)
	txInVoter.Tx = txInVoter.GetTx(activeNodes)
	h.mgr.Keeper().SetObservedTxInVoter(ctx, txInVoter)
	m, txErr := processOneTxInV46(ctx, h.mgr.Keeper(), txIn, msg.Signer)
	if txErr != nil {
		ctx.Logger().Error("fail to process native inbound tx", "error", txErr.Error(), "tx hash", tx.ID.String())
		if txIn.Tx.Coins.IsEmpty() {
			return &cosmos.Result{}, nil
		}
		if newErr := refundTx(ctx, txIn, h.mgr, h.mgr.GetConstants(), CodeInvalidMemo, txErr.Error(), targetModule); nil != newErr {
			return nil, newErr
		}

		return &cosmos.Result{}, nil
	}

	// check if we've halted trading
	_, isSwap := m.(*MsgSwap)
	_, isAddLiquidity := m.(*MsgAddLiquidity)
	haltTrading, err := h.mgr.Keeper().GetMimir(ctx, "HaltTrading")
	if isSwap || isAddLiquidity {
		if (haltTrading > 0 && haltTrading < common.BlockHeight(ctx) && err == nil) || h.mgr.Keeper().RagnarokInProgress(ctx) {
			ctx.Logger().Info("trading is halted!!")
			if txIn.Tx.Coins.IsEmpty() {
				return &cosmos.Result{}, nil
			}
			if newErr := refundTx(ctx, txIn, h.mgr, h.mgr.GetConstants(), se.ErrUnauthorized.ABCICode(), "trading halted", targetModule); nil != newErr {
				return nil, ErrInternal(newErr, "trading is halted, fail to refund")
			}
			return &cosmos.Result{}, nil
		}
	}

	// if its a swap, send it to our queue for processing later
	if isSwap {
		msg := m.(*MsgSwap)
		h.addSwapV1(ctx, *msg)
		return &cosmos.Result{}, nil
	}

	result, err := handler(ctx, m)
	if err != nil {
		code := uint32(1)
		var e se.Error
		if errors.As(err, &e) {
			code = e.ABCICode()
		}
		if txIn.Tx.Coins.IsEmpty() {
			return &cosmos.Result{}, nil
		}
		if err := refundTx(ctx, txIn, h.mgr, h.mgr.GetConstants(), code, err.Error(), targetModule); err != nil {
			return nil, fmt.Errorf("fail to refund tx: %w", err)
		}
	}
	// for those Memo that will not have outbound at all , set the observedTx to done
	if !memo.GetType().HasOutbound() {
		txInVoter.SetDone()
		h.mgr.Keeper().SetObservedTxInVoter(ctx, txInVoter)
	}
	return result, nil

}

func (h DepositHandler) handleV47(ctx cosmos.Context, msg MsgDeposit) (*cosmos.Result, error) {
	nativeTxFee, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || nativeTxFee < 0 {
		nativeTxFee = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
	}
	gas := common.NewCoin(common.RuneNative, cosmos.NewUint(uint64(nativeTxFee)))
	gasFee, err := gas.Native()
	if err != nil {
		return nil, fmt.Errorf("fail to get gas fee: %w", err)
	}

	coins, err := msg.Coins.Native()
	if err != nil {
		return nil, ErrInternal(err, "coins are native to THORChain")
	}

	totalCoins := cosmos.NewCoins(gasFee).Add(coins...)
	if !h.mgr.Keeper().HasCoins(ctx, msg.GetSigners()[0], totalCoins) {
		return nil, cosmos.ErrInsufficientCoins(err, "insufficient funds")
	}

	// send gas to reserve
	sdkErr := h.mgr.Keeper().SendFromAccountToModule(ctx, msg.GetSigners()[0], ReserveName, common.NewCoins(gas))
	if sdkErr != nil {
		return nil, fmt.Errorf("unable to send gas to reserve: %w", sdkErr)
	}

	hash := tmtypes.Tx(ctx.TxBytes()).Hash()
	txID, err := common.NewTxID(fmt.Sprintf("%X", hash))
	if err != nil {
		return nil, fmt.Errorf("fail to get tx hash: %w", err)
	}
	from, err := common.NewAddress(msg.GetSigners()[0].String())
	if err != nil {
		return nil, fmt.Errorf("fail to get from address: %w", err)
	}
	to, err := h.mgr.Keeper().GetModuleAddress(AsgardName)
	if err != nil {
		return nil, fmt.Errorf("fail to get to address: %w", err)
	}

	handler := NewInternalHandler(h.mgr)

	memo, _ := ParseMemo(msg.Memo) // ignore err
	var targetModule string
	switch memo.GetType() {
	case TxBond:
		targetModule = BondName
	case TxReserve:
		targetModule = ReserveName
	default:
		targetModule = AsgardName
	}
	coinsInMsg := msg.Coins
	if !coinsInMsg.IsEmpty() {
		// send funds to target module
		sdkErr = h.mgr.Keeper().SendFromAccountToModule(ctx, msg.GetSigners()[0], targetModule, msg.Coins)
		if sdkErr != nil {
			return nil, sdkErr
		}
	}

	tx := common.NewTx(txID, from, to, coinsInMsg, common.Gas{gas}, msg.Memo)
	tx.Chain = common.THORChain

	// construct msg from memo
	txIn := ObservedTx{Tx: tx}
	txInVoter := NewObservedTxVoter(txIn.Tx.ID, []ObservedTx{txIn})
	activeNodes, err := h.mgr.Keeper().ListActiveNodeAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get all active nodes: %w", err)
	}
	for _, node := range activeNodes {
		txInVoter.Add(txIn, node.NodeAddress)
	}
	txInVoter.FinalisedHeight = common.BlockHeight(ctx)
	txInVoter.Tx = txInVoter.GetTx(activeNodes)
	h.mgr.Keeper().SetObservedTxInVoter(ctx, txInVoter)
	m, txErr := processOneTxInV46(ctx, h.mgr.Keeper(), txIn, msg.Signer)
	if txErr != nil {
		ctx.Logger().Error("fail to process native inbound tx", "error", txErr.Error(), "tx hash", tx.ID.String())
		if txIn.Tx.Coins.IsEmpty() {
			return &cosmos.Result{}, nil
		}
		if newErr := refundTxV47(ctx, txIn, h.mgr, h.mgr.GetConstants(), CodeInvalidMemo, txErr.Error(), targetModule); nil != newErr {
			return nil, newErr
		}

		return &cosmos.Result{}, nil
	}

	// check if we've halted trading
	_, isSwap := m.(*MsgSwap)
	_, isAddLiquidity := m.(*MsgAddLiquidity)
	haltTrading, err := h.mgr.Keeper().GetMimir(ctx, "HaltTrading")
	if isSwap || isAddLiquidity {
		if (haltTrading > 0 && haltTrading < common.BlockHeight(ctx) && err == nil) || h.mgr.Keeper().RagnarokInProgress(ctx) {
			ctx.Logger().Info("trading is halted!!")
			if txIn.Tx.Coins.IsEmpty() {
				return &cosmos.Result{}, nil
			}
			if newErr := refundTxV47(ctx, txIn, h.mgr, h.mgr.GetConstants(), se.ErrUnauthorized.ABCICode(), "trading halted", targetModule); nil != newErr {
				return nil, ErrInternal(newErr, "trading is halted, fail to refund")
			}
			return &cosmos.Result{}, nil
		}
	}

	// if its a swap, send it to our queue for processing later
	if isSwap {
		msg := m.(*MsgSwap)
		h.addSwapV1(ctx, *msg)
		return &cosmos.Result{}, nil
	}

	result, err := handler(ctx, m)
	if err != nil {
		code := uint32(1)
		var e se.Error
		if errors.As(err, &e) {
			code = e.ABCICode()
		}
		if txIn.Tx.Coins.IsEmpty() {
			return &cosmos.Result{}, nil
		}
		if err := refundTxV47(ctx, txIn, h.mgr, h.mgr.GetConstants(), code, err.Error(), targetModule); err != nil {
			return nil, fmt.Errorf("fail to refund tx: %w", err)
		}
	}
	// for those Memo that will not have outbound at all , set the observedTx to done
	if !memo.GetType().HasOutbound() {
		txInVoter.SetDone()
		h.mgr.Keeper().SetObservedTxInVoter(ctx, txInVoter)
	}
	return result, nil

}

func (h DepositHandler) handleV58(ctx cosmos.Context, msg MsgDeposit) (*cosmos.Result, error) {
	nativeTxFee, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || nativeTxFee < 0 {
		nativeTxFee = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
	}
	gas := common.NewCoin(common.RuneNative, cosmos.NewUint(uint64(nativeTxFee)))
	gasFee, err := gas.Native()
	if err != nil {
		return nil, fmt.Errorf("fail to get gas fee: %w", err)
	}

	coins, err := msg.Coins.Native()
	if err != nil {
		return nil, ErrInternal(err, "coins are native to THORChain")
	}

	totalCoins := cosmos.NewCoins(gasFee).Add(coins...)
	if !h.mgr.Keeper().HasCoins(ctx, msg.GetSigners()[0], totalCoins) {
		return nil, cosmos.ErrInsufficientCoins(err, "insufficient funds")
	}

	// send gas to reserve
	sdkErr := h.mgr.Keeper().SendFromAccountToModule(ctx, msg.GetSigners()[0], ReserveName, common.NewCoins(gas))
	if sdkErr != nil {
		return nil, fmt.Errorf("unable to send gas to reserve: %w", sdkErr)
	}

	hash := tmtypes.Tx(ctx.TxBytes()).Hash()
	txID, err := common.NewTxID(fmt.Sprintf("%X", hash))
	if err != nil {
		return nil, fmt.Errorf("fail to get tx hash: %w", err)
	}
	from, err := common.NewAddress(msg.GetSigners()[0].String())
	if err != nil {
		return nil, fmt.Errorf("fail to get from address: %w", err)
	}
	to, err := h.mgr.Keeper().GetModuleAddress(AsgardName)
	if err != nil {
		return nil, fmt.Errorf("fail to get to address: %w", err)
	}

	handler := NewInternalHandler(h.mgr)

	memo, _ := ParseMemoWithTHORNames(ctx, h.mgr.Keeper(), msg.Memo) // ignore err
	var targetModule string
	switch memo.GetType() {
	case TxBond:
		targetModule = BondName
	case TxReserve:
		targetModule = ReserveName
	default:
		targetModule = AsgardName
	}
	coinsInMsg := msg.Coins
	if !coinsInMsg.IsEmpty() {
		// send funds to target module
		sdkErr = h.mgr.Keeper().SendFromAccountToModule(ctx, msg.GetSigners()[0], targetModule, msg.Coins)
		if sdkErr != nil {
			return nil, sdkErr
		}
	}

	tx := common.NewTx(txID, from, to, coinsInMsg, common.Gas{gas}, msg.Memo)
	tx.Chain = common.THORChain

	// construct msg from memo
	txIn := ObservedTx{Tx: tx}
	txInVoter := NewObservedTxVoter(txIn.Tx.ID, []ObservedTx{txIn})
	activeNodes, err := h.mgr.Keeper().ListActiveNodeAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get all active nodes: %w", err)
	}
	for _, node := range activeNodes {
		txInVoter.Add(txIn, node.NodeAddress)
	}
	txInVoter.FinalisedHeight = common.BlockHeight(ctx)
	txInVoter.Tx = txInVoter.GetTx(activeNodes)
	h.mgr.Keeper().SetObservedTxInVoter(ctx, txInVoter)
	m, txErr := processOneTxIn(ctx, h.mgr.GetVersion(), h.mgr.Keeper(), txIn, msg.Signer)
	if txErr != nil {
		ctx.Logger().Error("fail to process native inbound tx", "error", txErr.Error(), "tx hash", tx.ID.String())
		if txIn.Tx.Coins.IsEmpty() {
			return &cosmos.Result{}, nil
		}
		if newErr := refundTx(ctx, txIn, h.mgr, h.mgr.GetConstants(), CodeInvalidMemo, txErr.Error(), targetModule); nil != newErr {
			return nil, newErr
		}

		return &cosmos.Result{}, nil
	}

	// check if we've halted trading
	_, isSwap := m.(*MsgSwap)
	_, isAddLiquidity := m.(*MsgAddLiquidity)
	haltTrading, err := h.mgr.Keeper().GetMimir(ctx, "HaltTrading")
	if isSwap || isAddLiquidity {
		if (haltTrading > 0 && haltTrading < common.BlockHeight(ctx) && err == nil) || h.mgr.Keeper().RagnarokInProgress(ctx) {
			ctx.Logger().Info("trading is halted!!")
			if txIn.Tx.Coins.IsEmpty() {
				return &cosmos.Result{}, nil
			}
			if newErr := refundTx(ctx, txIn, h.mgr, h.mgr.GetConstants(), se.ErrUnauthorized.ABCICode(), "trading halted", targetModule); nil != newErr {
				return nil, ErrInternal(newErr, "trading is halted, fail to refund")
			}
			return &cosmos.Result{}, nil
		}
	}

	// if its a swap, send it to our queue for processing later
	if isSwap {
		msg := m.(*MsgSwap)
		h.addSwapV1(ctx, *msg)
		return &cosmos.Result{}, nil
	}

	result, err := handler(ctx, m)
	if err != nil {
		code := uint32(1)
		var e se.Error
		if errors.As(err, &e) {
			code = e.ABCICode()
		}
		if txIn.Tx.Coins.IsEmpty() {
			return &cosmos.Result{}, nil
		}
		if err := refundTx(ctx, txIn, h.mgr, h.mgr.GetConstants(), code, err.Error(), targetModule); err != nil {
			return nil, fmt.Errorf("fail to refund tx: %w", err)
		}
	}
	// for those Memo that will not have outbound at all , set the observedTx to done
	if !memo.GetType().HasOutbound() {
		txInVoter.SetDone()
		h.mgr.Keeper().SetObservedTxInVoter(ctx, txInVoter)
	}
	return result, nil
}

func (h DepositHandler) handleV61(ctx cosmos.Context, msg MsgDeposit) (*cosmos.Result, error) {
	nativeTxFee, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || nativeTxFee < 0 {
		nativeTxFee = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
	}
	gas := common.NewCoin(common.RuneNative, cosmos.NewUint(uint64(nativeTxFee)))
	gasFee, err := gas.Native()
	if err != nil {
		return nil, fmt.Errorf("fail to get gas fee: %w", err)
	}

	coins, err := msg.Coins.Native()
	if err != nil {
		return nil, ErrInternal(err, "coins are native to THORChain")
	}

	totalCoins := cosmos.NewCoins(gasFee).Add(coins...)
	if !h.mgr.Keeper().HasCoins(ctx, msg.GetSigners()[0], totalCoins) {
		return nil, cosmos.ErrInsufficientCoins(err, "insufficient funds")
	}

	// send gas to reserve
	sdkErr := h.mgr.Keeper().SendFromAccountToModule(ctx, msg.GetSigners()[0], ReserveName, common.NewCoins(gas))
	if sdkErr != nil {
		return nil, fmt.Errorf("unable to send gas to reserve: %w", sdkErr)
	}

	hash := tmtypes.Tx(ctx.TxBytes()).Hash()
	txID, err := common.NewTxID(fmt.Sprintf("%X", hash))
	if err != nil {
		return nil, fmt.Errorf("fail to get tx hash: %w", err)
	}
	from, err := common.NewAddress(msg.GetSigners()[0].String())
	if err != nil {
		return nil, fmt.Errorf("fail to get from address: %w", err)
	}
	to, err := h.mgr.Keeper().GetModuleAddress(AsgardName)
	if err != nil {
		return nil, fmt.Errorf("fail to get to address: %w", err)
	}

	handler := NewInternalHandler(h.mgr)

	memo, _ := ParseMemoWithTHORNames(ctx, h.mgr.Keeper(), msg.Memo) // ignore err
	var targetModule string
	switch memo.GetType() {
	case TxBond:
		targetModule = BondName
	case TxReserve:
		targetModule = ReserveName
	default:
		targetModule = AsgardName
	}
	coinsInMsg := msg.Coins
	if !coinsInMsg.IsEmpty() {
		// send funds to target module
		sdkErr = h.mgr.Keeper().SendFromAccountToModule(ctx, msg.GetSigners()[0], targetModule, msg.Coins)
		if sdkErr != nil {
			return nil, sdkErr
		}
	}

	tx := common.NewTx(txID, from, to, coinsInMsg, common.Gas{gas}, msg.Memo)
	tx.Chain = common.THORChain

	// construct msg from memo
	txIn := ObservedTx{Tx: tx}
	txInVoter := NewObservedTxVoter(txIn.Tx.ID, []ObservedTx{txIn})
	activeNodes, err := h.mgr.Keeper().ListActiveNodeAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get all active nodes: %w", err)
	}
	for _, node := range activeNodes {
		txInVoter.Add(txIn, node.NodeAddress)
	}
	txInVoter.FinalisedHeight = common.BlockHeight(ctx)
	txInVoter.Tx = txInVoter.GetTx(activeNodes)
	h.mgr.Keeper().SetObservedTxInVoter(ctx, txInVoter)
	m, txErr := processOneTxIn(ctx, h.mgr.GetVersion(), h.mgr.Keeper(), txIn, msg.Signer)
	if txErr != nil {
		ctx.Logger().Error("fail to process native inbound tx", "error", txErr.Error(), "tx hash", tx.ID.String())
		if txIn.Tx.Coins.IsEmpty() {
			return &cosmos.Result{}, nil
		}
		if newErr := refundTx(ctx, txIn, h.mgr, h.mgr.GetConstants(), CodeInvalidMemo, txErr.Error(), targetModule); nil != newErr {
			return nil, newErr
		}

		return &cosmos.Result{}, nil
	}

	// check if we've halted trading
	_, isSwap := m.(*MsgSwap)
	_, isAddLiquidity := m.(*MsgAddLiquidity)
	if isSwap || isAddLiquidity {
		if isTradingHalt(ctx, m, h.mgr) || h.mgr.Keeper().RagnarokInProgress(ctx) {
			if txIn.Tx.Coins.IsEmpty() {
				return &cosmos.Result{}, nil
			}
			if newErr := refundTx(ctx, txIn, h.mgr, h.mgr.GetConstants(), se.ErrUnauthorized.ABCICode(), "trading halted", targetModule); nil != newErr {
				return nil, ErrInternal(newErr, "trading is halted, fail to refund")
			}
			return &cosmos.Result{}, nil
		}
	}

	// if its a swap, send it to our queue for processing later
	if isSwap {
		msg := m.(*MsgSwap)
		h.addSwapV1(ctx, *msg)
		return &cosmos.Result{}, nil
	}

	result, err := handler(ctx, m)
	if err != nil {
		code := uint32(1)
		var e se.Error
		if errors.As(err, &e) {
			code = e.ABCICode()
		}
		if txIn.Tx.Coins.IsEmpty() {
			return &cosmos.Result{}, nil
		}
		if err := refundTx(ctx, txIn, h.mgr, h.mgr.GetConstants(), code, err.Error(), targetModule); err != nil {
			return nil, fmt.Errorf("fail to refund tx: %w", err)
		}
	}
	// for those Memo that will not have outbound at all , set the observedTx to done
	if !memo.GetType().HasOutbound() {
		txInVoter.SetDone()
		h.mgr.Keeper().SetObservedTxInVoter(ctx, txInVoter)
	}
	return result, nil

}

func (h DepositHandler) handleV63(ctx cosmos.Context, msg MsgDeposit) (*cosmos.Result, error) {
	return h.handleCurrent(ctx, msg)
}

func (h DepositHandler) handleCurrent(ctx cosmos.Context, msg MsgDeposit) (*cosmos.Result, error) {
	haltHeight, err := h.mgr.Keeper().GetMimir(ctx, "HaltTHORChain")
	if err != nil {
		return nil, fmt.Errorf("failed to get mimir setting: %w", err)
	}
	if haltHeight > 0 && common.BlockHeight(ctx) > haltHeight {
		return nil, fmt.Errorf("mimir has halted THORChain transactions")
	}

	nativeTxFee, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || nativeTxFee < 0 {
		nativeTxFee = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
	}
	gas := common.NewCoin(common.RuneNative, cosmos.NewUint(uint64(nativeTxFee)))
	gasFee, err := gas.Native()
	if err != nil {
		return nil, fmt.Errorf("fail to get gas fee: %w", err)
	}

	coins, err := msg.Coins.Native()
	if err != nil {
		return nil, ErrInternal(err, "coins are native to THORChain")
	}

	totalCoins := cosmos.NewCoins(gasFee).Add(coins...)
	if !h.mgr.Keeper().HasCoins(ctx, msg.GetSigners()[0], totalCoins) {
		return nil, cosmos.ErrInsufficientCoins(err, "insufficient funds")
	}

	// send gas to reserve
	sdkErr := h.mgr.Keeper().SendFromAccountToModule(ctx, msg.GetSigners()[0], ReserveName, common.NewCoins(gas))
	if sdkErr != nil {
		return nil, fmt.Errorf("unable to send gas to reserve: %w", sdkErr)
	}

	hash := tmtypes.Tx(ctx.TxBytes()).Hash()
	txID, err := common.NewTxID(fmt.Sprintf("%X", hash))
	if err != nil {
		return nil, fmt.Errorf("fail to get tx hash: %w", err)
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
	case TxBond:
		targetModule = BondName
	case TxReserve:
		targetModule = ReserveName
	default:
		targetModule = AsgardName
	}
	coinsInMsg := msg.Coins
	if !coinsInMsg.IsEmpty() {
		// send funds to target module
		sdkErr = h.mgr.Keeper().SendFromAccountToModule(ctx, msg.GetSigners()[0], targetModule, msg.Coins)
		if sdkErr != nil {
			return nil, sdkErr
		}
	}

	to, err := h.mgr.Keeper().GetModuleAddress(targetModule)
	if err != nil {
		return nil, fmt.Errorf("fail to get to address: %w", err)
	}

	tx := common.NewTx(txID, from, to, coinsInMsg, common.Gas{gas}, msg.Memo)
	tx.Chain = common.THORChain

	// construct msg from memo
	txIn := ObservedTx{Tx: tx}
	txInVoter := NewObservedTxVoter(txIn.Tx.ID, []ObservedTx{txIn})
	activeNodes, err := h.mgr.Keeper().ListActiveNodeAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get all active nodes: %w", err)
	}
	for _, node := range activeNodes {
		txInVoter.Add(txIn, node.NodeAddress)
	}
	txInVoter.FinalisedHeight = common.BlockHeight(ctx)
	txInVoter.Tx = txInVoter.GetTx(activeNodes)
	h.mgr.Keeper().SetObservedTxInVoter(ctx, txInVoter)
	m, txErr := processOneTxIn(ctx, h.mgr.GetVersion(), h.mgr.Keeper(), txIn, msg.Signer)
	if txErr != nil {
		ctx.Logger().Error("fail to process native inbound tx", "error", txErr.Error(), "tx hash", tx.ID.String())
		if txIn.Tx.Coins.IsEmpty() {
			return &cosmos.Result{}, nil
		}
		if newErr := refundTx(ctx, txIn, h.mgr, h.mgr.GetConstants(), CodeInvalidMemo, txErr.Error(), targetModule); nil != newErr {
			return nil, newErr
		}

		return &cosmos.Result{}, nil
	}

	// check if we've halted trading
	_, isSwap := m.(*MsgSwap)
	_, isAddLiquidity := m.(*MsgAddLiquidity)
	if isSwap || isAddLiquidity {
		if isTradingHalt(ctx, m, h.mgr) || h.mgr.Keeper().RagnarokInProgress(ctx) {
			if txIn.Tx.Coins.IsEmpty() {
				return &cosmos.Result{}, nil
			}
			if newErr := refundTx(ctx, txIn, h.mgr, h.mgr.GetConstants(), se.ErrUnauthorized.ABCICode(), "trading halted", targetModule); nil != newErr {
				return nil, ErrInternal(newErr, "trading is halted, fail to refund")
			}
			return &cosmos.Result{}, nil
		}
	}
	withdrawMsg, isWithdrawLiquidity := m.(*MsgWithdrawLiquidity)
	if isWithdrawLiquidity && isChainHalted(ctx, h.mgr, withdrawMsg.Asset.Chain) {
		return nil, fmt.Errorf("unable to withdraw liquidity while chain is halted")
	}

	// if its a swap, send it to our queue for processing later
	if isSwap {
		msg := m.(*MsgSwap)
		h.addSwap(ctx, *msg)
		return &cosmos.Result{}, nil
	}

	result, err := handler(ctx, m)
	if err != nil {
		code := uint32(1)
		var e se.Error
		if errors.As(err, &e) {
			code = e.ABCICode()
		}
		if txIn.Tx.Coins.IsEmpty() {
			return &cosmos.Result{}, nil
		}
		if err := refundTx(ctx, txIn, h.mgr, h.mgr.GetConstants(), code, err.Error(), targetModule); err != nil {
			return nil, fmt.Errorf("fail to refund tx: %w", err)
		}
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
	if version.GTE(semver.MustParse("0.63.0")) {
		h.addSwapV63(ctx, msg)
	} else {
		h.addSwapV1(ctx, msg)
	}
}
func (h DepositHandler) addSwapV1(ctx cosmos.Context, msg MsgSwap) {
	amt := cosmos.ZeroUint()
	if !msg.AffiliateBasisPoints.IsZero() && msg.AffiliateAddress.IsChain(common.THORChain) {
		amt = common.GetShare(
			msg.AffiliateBasisPoints,
			cosmos.NewUint(10000),
			msg.Tx.Coins[0].Amount,
		)
		msg.Tx.Coins[0].Amount = common.SafeSub(msg.Tx.Coins[0].Amount, amt)
	}

	if err := h.mgr.Keeper().SetSwapQueueItem(ctx, msg, 0); err != nil {
		ctx.Logger().Error("fail to add swap to queue", "error", err)
	}

	if !amt.IsZero() {
		to_address, err := msg.AffiliateAddress.AccAddress()
		if err != nil {
			ctx.Logger().Error("fail to convert address into AccAddress", "msg", msg.AffiliateAddress, "error", err)
		} else {
			nativeTxFee, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
			if err != nil || nativeTxFee < 0 {
				nativeTxFee = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
			}
			amt = common.SafeSub(amt, cosmos.NewUint(uint64(nativeTxFee)))

			coin := common.NewCoin(common.RuneNative, amt)
			sdkErr := h.mgr.Keeper().SendFromModuleToAccount(ctx, AsgardName, to_address, common.NewCoins(coin))
			if sdkErr != nil {
				ctx.Logger().Error("fail to send native rune to affiliate", "msg", msg.AffiliateAddress, "error", err)
			}
		}
	}
}
func (h DepositHandler) addSwapV63(ctx cosmos.Context, msg MsgSwap) {
	amt := cosmos.ZeroUint()
	if !msg.AffiliateBasisPoints.IsZero() && msg.AffiliateAddress.IsChain(common.THORChain) {
		amt = common.GetSafeShare(
			msg.AffiliateBasisPoints,
			cosmos.NewUint(10000),
			msg.Tx.Coins[0].Amount,
		)
		msg.Tx.Coins[0].Amount = common.SafeSub(msg.Tx.Coins[0].Amount, amt)
	}

	if err := h.mgr.Keeper().SetSwapQueueItem(ctx, msg, 0); err != nil {
		ctx.Logger().Error("fail to add swap to queue", "error", err)
	}

	if !amt.IsZero() {
		to_address, err := msg.AffiliateAddress.AccAddress()
		if err != nil {
			ctx.Logger().Error("fail to convert address into AccAddress", "msg", msg.AffiliateAddress, "error", err)
		} else {
			nativeTxFee, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
			if err != nil || nativeTxFee < 0 {
				nativeTxFee = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
			}
			amt = common.SafeSub(amt, cosmos.NewUint(uint64(nativeTxFee)))

			coin := common.NewCoin(common.RuneNative, amt)
			sdkErr := h.mgr.Keeper().SendFromModuleToAccount(ctx, AsgardName, to_address, common.NewCoins(coin))
			if sdkErr != nil {
				ctx.Logger().Error("fail to send native rune to affiliate", "msg", msg.AffiliateAddress, "error", err)
			}
		}
	}
}
