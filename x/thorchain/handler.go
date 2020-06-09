package thorchain

import (
	"errors"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// NewExternalHandler returns a handler for "thorchain" type messages.
func NewExternalHandler(keeper keeper.Keeper, mgr Manager) cosmos.Handler {
	return func(ctx cosmos.Context, msg cosmos.Msg) (*cosmos.Result, error) {
		ctx = ctx.WithEventManager(cosmos.NewEventManager())
		version := keeper.GetLowestActiveVersion(ctx)
		constantValues := constants.GetConstantValues(version)
		if constantValues == nil {
			return nil, errConstNotAvailable
		}
		handlerMap := getHandlerMapping(keeper, mgr)
		h, ok := handlerMap[msg.Type()]
		if !ok {
			errMsg := fmt.Sprintf("Unrecognized thorchain Msg type: %v", msg.Type())
			return nil, cosmos.ErrUnknownRequest(errMsg)
		}
		result, err := h.Run(ctx, msg, version, constantValues)
		if err != nil {
			return nil, err
		}
		if result == nil {
			result = &cosmos.Result{}
		}
		if len(ctx.EventManager().Events()) > 0 {
			result.Events = result.Events.AppendEvents(ctx.EventManager().Events())
		}
		return result, nil
	}
}

func getHandlerMapping(keeper keeper.Keeper, mgr Manager) map[string]MsgHandler {
	// New arch handlers
	m := make(map[string]MsgHandler)
	m[MsgTssPool{}.Type()] = NewTssHandler(keeper, mgr)
	m[MsgSetNodeKeys{}.Type()] = NewSetNodeKeysHandler(keeper, mgr)
	m[MsgSetVersion{}.Type()] = NewVersionHandler(keeper, mgr)
	m[MsgSetIPAddress{}.Type()] = NewIPAddressHandler(keeper, mgr)
	m[MsgNativeTx{}.Type()] = NewNativeTxHandler(keeper, mgr)
	m[MsgObservedTxIn{}.Type()] = NewObservedTxInHandler(keeper, mgr)
	m[MsgObservedTxOut{}.Type()] = NewObservedTxOutHandler(keeper, mgr)
	m[MsgTssKeysignFail{}.Type()] = NewTssKeysignHandler(keeper, mgr)
	m[MsgErrataTx{}.Type()] = NewErrataTxHandler(keeper, mgr)
	m[MsgSend{}.Type()] = NewSendHandler(keeper, mgr)
	m[MsgMimir{}.Type()] = NewMimirHandler(keeper, mgr)
	m[MsgBan{}.Type()] = NewBanHandler(keeper, mgr)
	m[MsgNetworkFee{}.Type()] = NewNetworkFeeHandler(keeper, mgr)
	return m
}

// NewInternalHandler returns a handler for "thorchain" internal type messages.
func NewInternalHandler(keeper keeper.Keeper, mgr Manager) cosmos.Handler {
	return func(ctx cosmos.Context, msg cosmos.Msg) (*cosmos.Result, error) {
		version := keeper.GetLowestActiveVersion(ctx)
		constantValues := constants.GetConstantValues(version)
		if constantValues == nil {
			return nil, errConstNotAvailable
		}
		handlerMap := getInternalHandlerMapping(keeper, mgr)
		h, ok := handlerMap[msg.Type()]
		if !ok {
			errMsg := fmt.Sprintf("Unrecognized thorchain Msg type: %v", msg.Type())
			return nil, cosmos.ErrUnknownRequest(errMsg)
		}
		return h.Run(ctx, msg, version, constantValues)
	}
}

func getInternalHandlerMapping(keeper keeper.Keeper, mgr Manager) map[string]MsgHandler {
	// New arch handlers
	m := make(map[string]MsgHandler)
	m[MsgOutboundTx{}.Type()] = NewOutboundTxHandler(keeper, mgr)
	m[MsgYggdrasil{}.Type()] = NewYggdrasilHandler(keeper, mgr)
	m[MsgSwap{}.Type()] = NewSwapHandler(keeper, mgr)
	m[MsgReserveContributor{}.Type()] = NewReserveContributorHandler(keeper, mgr)
	m[MsgBond{}.Type()] = NewBondHandler(keeper, mgr)
	m[MsgLeave{}.Type()] = NewLeaveHandler(keeper, mgr)
	m[MsgAdd{}.Type()] = NewAddHandler(keeper, mgr)
	m[MsgSetUnStake{}.Type()] = NewUnstakeHandler(keeper, mgr)
	m[MsgSetStakeData{}.Type()] = NewStakeHandler(keeper, mgr)
	m[MsgRefundTx{}.Type()] = NewRefundHandler(keeper, mgr)
	m[MsgMigrate{}.Type()] = NewMigrateHandler(keeper, mgr)
	m[MsgRagnarok{}.Type()] = NewRagnarokHandler(keeper, mgr)
	m[MsgSwitch{}.Type()] = NewSwitchHandler(keeper, mgr)
	return m
}

func fetchMemo(ctx cosmos.Context, constAccessor constants.ConstantValues, keeper keeper.Keeper, tx common.Tx) string {
	if len(tx.Memo) > 0 {
		return tx.Memo
	}

	var memo string
	// attempt to pull memo from tx marker
	hash := tx.Hash()
	marks, err := keeper.ListTxMarker(ctx, hash)
	if err != nil {
		ctx.Logger().Error("fail to get tx marker", "error", err)
	}
	if len(marks) > 0 {
		// filter out expired tx markers
		period := constAccessor.GetInt64Value(constants.SigningTransactionPeriod) * 3
		marks = marks.FilterByMinHeight(common.BlockHeight(ctx) - period)

		// if we still have a marker, add the memo
		if len(marks) > 0 {
			var mark TxMarker
			mark, marks = marks.Pop()
			memo = mark.Memo
		}

		// update our marker list
		if err := keeper.SetTxMarkers(ctx, hash, marks); err != nil {
			ctx.Logger().Error("fail to set tx markers", "error", err)
		}
	}
	return memo
}

func processOneTxIn(ctx cosmos.Context, keeper keeper.Keeper, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	if len(tx.Tx.Coins) == 0 {
		return nil, cosmos.ErrUnknownRequest("no coin found")
	}

	memo, err := ParseMemo(tx.Tx.Memo)
	if err != nil {
		ctx.Logger().Error("fail to parse memo", "error", err)
		return nil, err
	}
	// THORNode should not have one tx across chain, if it is cross chain it should be separate tx
	var newMsg cosmos.Msg
	// interpret the memo and initialize a corresponding msg event
	switch m := memo.(type) {
	case StakeMemo:
		newMsg, err = getMsgStakeFromMemo(ctx, m, tx, signer)
		if err != nil {
			return nil, fmt.Errorf("invalid stake memo:%w", err)
		}

	case UnstakeMemo:
		newMsg, err = getMsgUnstakeFromMemo(m, tx, signer)
		if err != nil {
			return nil, err
		}
	case SwapMemo:
		newMsg, err = getMsgSwapFromMemo(m, tx, signer)
		if err != nil {
			return nil, fmt.Errorf("invalid swap memo:%w", err)
		}
	case AddMemo:
		newMsg, err = getMsgAddFromMemo(m, tx, signer)
		if err != nil {
			return nil, err
		}
	case GasMemo:
		newMsg, err = getMsgNoOpFromMemo(tx, signer)
		if err != nil {
			return nil, err
		}
	case RefundMemo:
		newMsg, err = getMsgRefundFromMemo(m, tx, signer)
		if err != nil {
			return nil, err
		}
	case OutboundMemo:
		newMsg, err = getMsgOutboundFromMemo(m, tx, signer)
		if err != nil {
			return nil, err
		}
	case MigrateMemo:
		newMsg, err = getMsgMigrateFromMemo(m, tx, signer)
		if err != nil {
			return nil, err
		}
	case BondMemo:
		newMsg, err = getMsgBondFromMemo(m, tx, signer)
		if err != nil {
			return nil, err
		}
	case RagnarokMemo:
		newMsg, err = getMsgRagnarokFromMemo(m, tx, signer)
		if err != nil {
			return nil, err
		}
	case LeaveMemo:
		newMsg, err = getMsgLeaveFromMemo(m, tx, signer)
		if err != nil {
			return nil, err
		}
	case YggdrasilFundMemo:
		newMsg = NewMsgYggdrasil(tx.Tx, tx.ObservedPubKey, m.GetBlockHeight(), true, tx.Tx.Coins, signer)
	case YggdrasilReturnMemo:
		newMsg = NewMsgYggdrasil(tx.Tx, tx.ObservedPubKey, m.GetBlockHeight(), false, tx.Tx.Coins, signer)
	case ReserveMemo:
		res := NewReserveContributor(tx.Tx.FromAddress, tx.Tx.Coins[0].Amount)
		newMsg = NewMsgReserveContributor(tx.Tx, res, signer)
	case SwitchMemo:
		newMsg = NewMsgSwitch(tx.Tx, memo.GetDestination(), signer)
	default:
		return nil, errInvalidMemo
	}

	if err := newMsg.ValidateBasic(); err != nil {
		return nil, err
	}
	return newMsg, nil
}

func getMsgNoOpFromMemo(tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	for _, coin := range tx.Tx.Coins {
		if !coin.Asset.Chain.Equals(common.RuneAsset().Chain) {
			return nil, fmt.Errorf("only accepts %s coins", common.RuneAsset().Chain)
		}
	}
	return NewMsgNoOp(tx, signer), nil
}

func getMsgSwapFromMemo(memo SwapMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	if len(tx.Tx.Coins) > 1 {
		return nil, errors.New("not expecting multiple coins in a swap")
	}
	if memo.Destination.IsEmpty() {
		memo.Destination = tx.Tx.FromAddress
	}

	coin := tx.Tx.Coins[0]
	if memo.Asset.Equals(coin.Asset) {
		return nil, fmt.Errorf("swap from %s to %s is noop, refund", memo.Asset.String(), coin.Asset.String())
	}

	// Looks like at the moment THORNode can only process ont ty
	return NewMsgSwap(tx.Tx, memo.GetAsset(), memo.Destination, memo.SlipLimit, signer), nil
}

func getMsgUnstakeFromMemo(memo UnstakeMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	withdrawAmount := cosmos.NewUint(MaxUnstakeBasisPoints)
	if len(memo.GetAmount()) > 0 {
		withdrawAmount = cosmos.NewUintFromString(memo.GetAmount())
	}
	return NewMsgSetUnStake(tx.Tx, tx.Tx.FromAddress, withdrawAmount, memo.GetAsset(), signer), nil
}

func getMsgStakeFromMemo(ctx cosmos.Context, memo StakeMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	// when staker stake to a pool ,usually it will be two coins, RUNE and the asset of the pool.
	// if it is multi-chain , like NOT Binance chain , it is using two asymmetric staking
	if len(tx.Tx.Coins) > 2 {
		return nil, errors.New("not expecting more than two coins in a stake")
	}
	runeAmount := cosmos.ZeroUint()
	assetAmount := cosmos.ZeroUint()
	asset := memo.GetAsset()
	if asset.IsEmpty() {
		return nil, errors.New("unable to determine the intended pool for this stake")
	}
	// There is no dedicate pool for RUNE ,because every pool will have RUNE , that's by design
	if asset.IsRune() {
		return nil, errors.New("invalid pool asset")
	}
	// Extract the Rune amount and the asset amount from the transaction. At least one of them must be
	// nonzero. If THORNode saw two types of coins, one of them must be the asset coin.
	for _, coin := range tx.Tx.Coins {
		ctx.Logger().Info("coin", "asset", coin.Asset.String(), "amount", coin.Amount.String())
		if coin.Asset.IsRune() {
			runeAmount = coin.Amount
		}
		if asset.Equals(coin.Asset) {
			assetAmount = coin.Amount
		}
	}

	if runeAmount.IsZero() && assetAmount.IsZero() {
		return nil, errors.New("did not find any valid coins for stake")
	}

	// when THORNode receive two coins, but THORNode didn't find the coin specify by asset, then user might send in the wrong coin
	if assetAmount.IsZero() && len(tx.Tx.Coins) == 2 {
		return nil, fmt.Errorf("did not find %s ", asset)
	}

	runeAddr := tx.Tx.FromAddress
	assetAddr := memo.GetDestination()
	// this is to cover multi-chain scenario, for example BTC , staker who would like to stake in BTC pool,  will have to complete
	// the stake operation by sending in two asymmetric stake tx, one tx on BTC chain with memo stake:BTC:<RUNE address> ,
	// and another one on Binance chain with stake:BTC , with only RUNE as the coin
	// Thorchain will use the <RUNE address> to match these two together , and consider it as one stake.
	if !runeAddr.IsChain(common.RuneAsset().Chain) {
		runeAddr = memo.GetDestination()
		assetAddr = tx.Tx.FromAddress
	} else {
		// if it is on THOR chain , while the asset addr is empty, then the asset addr is runeAddr
		if assetAddr.IsEmpty() {
			assetAddr = runeAddr
		}
	}

	return NewMsgSetStakeData(
		tx.Tx,
		asset,
		runeAmount,
		assetAmount,
		runeAddr,
		assetAddr,
		signer,
	), nil
}

func getMsgAddFromMemo(memo AddMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	runeAmount := cosmos.ZeroUint()
	assetAmount := cosmos.ZeroUint()
	for _, coin := range tx.Tx.Coins {
		if coin.Asset.IsRune() {
			runeAmount = coin.Amount
		} else if memo.GetAsset().Equals(coin.Asset) {
			assetAmount = coin.Amount
		}
	}
	return NewMsgAdd(
		tx.Tx,
		memo.GetAsset(),
		runeAmount,
		assetAmount,
		signer,
	), nil
}

func getMsgRefundFromMemo(memo RefundMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	return NewMsgRefundTx(
		tx,
		memo.GetTxID(),
		signer,
	), nil
}

func getMsgOutboundFromMemo(memo OutboundMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	return NewMsgOutboundTx(
		tx,
		memo.GetTxID(),
		signer,
	), nil
}

func getMsgMigrateFromMemo(memo MigrateMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	return NewMsgMigrate(tx, memo.GetBlockHeight(), signer), nil
}

func getMsgRagnarokFromMemo(memo RagnarokMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	return NewMsgRagnarok(tx, memo.GetBlockHeight(), signer), nil
}

func getMsgLeaveFromMemo(memo LeaveMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	return NewMsgLeave(tx.Tx, memo.GetAccAddress(), signer), nil
}

func getMsgBondFromMemo(memo BondMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	runeAmount := cosmos.ZeroUint()
	for _, coin := range tx.Tx.Coins {
		if coin.Asset.IsRune() {
			runeAmount = coin.Amount
		}
	}
	if runeAmount.IsZero() {
		return nil, errors.New("RUNE amount is 0")
	}
	return NewMsgBond(tx.Tx, memo.GetAccAddress(), runeAmount, tx.Tx.FromAddress, signer), nil
}
