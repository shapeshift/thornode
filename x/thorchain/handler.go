package thorchain

import (
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/x/auth/legacy/legacytx"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// MsgHandler is an interface expect all handler to implement
type MsgHandler interface {
	Run(ctx cosmos.Context, msg cosmos.Msg) (*cosmos.Result, error)
}

// NewExternalHandler returns a handler for "thorchain" type messages.
func NewExternalHandler(mgr Manager) cosmos.Handler {
	return func(ctx cosmos.Context, msg cosmos.Msg) (*cosmos.Result, error) {
		ctx = ctx.WithEventManager(cosmos.NewEventManager())
		version := mgr.Keeper().GetLowestActiveVersion(ctx)
		constantValues := constants.GetConstantValues(version)
		if constantValues == nil {
			return nil, errConstNotAvailable
		}
		handlerMap := getHandlerMapping(mgr)
		legacyMsg, ok := msg.(legacytx.LegacyMsg)
		if !ok {
			return nil, cosmos.ErrUnknownRequest("unknown message type")
		}
		h, ok := handlerMap[legacyMsg.Type()]
		if !ok {
			errMsg := fmt.Sprintf("Unrecognized thorchain Msg type: %v", legacyMsg.Type())
			return nil, cosmos.ErrUnknownRequest(errMsg)
		}
		result, err := h.Run(ctx, msg)
		if err != nil {
			return nil, err
		}
		if result == nil {
			result = &cosmos.Result{}
		}
		if len(ctx.EventManager().Events()) > 0 {
			result.Events = ctx.EventManager().ABCIEvents()
		}
		return result, nil
	}
}

func getHandlerMapping(mgr Manager) map[string]MsgHandler {
	return getHandlerMappingV65(mgr)
}

func getHandlerMappingV65(mgr Manager) map[string]MsgHandler {
	// New arch handlers
	m := make(map[string]MsgHandler)

	// consensus handlers
	m[MsgTssPool{}.Type()] = NewTssHandler(mgr)
	m[MsgObservedTxIn{}.Type()] = NewObservedTxInHandler(mgr)
	m[MsgObservedTxOut{}.Type()] = NewObservedTxOutHandler(mgr)
	m[MsgTssKeysignFail{}.Type()] = NewTssKeysignHandler(mgr)
	m[MsgErrataTx{}.Type()] = NewErrataTxHandler(mgr)
	m[MsgBan{}.Type()] = NewBanHandler(mgr)
	m[MsgNetworkFee{}.Type()] = NewNetworkFeeHandler(mgr)
	m[MsgSolvency{}.Type()] = NewSolvencyHandler(mgr)

	// cli handlers (non-consensus)
	m[MsgMimir{}.Type()] = NewMimirHandler(mgr)
	m[MsgSetNodeKeys{}.Type()] = NewSetNodeKeysHandler(mgr)
	m[MsgSetVersion{}.Type()] = NewVersionHandler(mgr)
	m[MsgSetIPAddress{}.Type()] = NewIPAddressHandler(mgr)
	m[MsgNodePauseChain{}.Type()] = NewNodePauseChainHandler(mgr)

	// native handlers (non-consensus)
	m[MsgSend{}.Type()] = NewSendHandler(mgr)
	m[MsgDeposit{}.Type()] = NewDepositHandler(mgr)
	return m
}

// NewInternalHandler returns a handler for "thorchain" internal type messages.
func NewInternalHandler(mgr Manager) cosmos.Handler {
	return func(ctx cosmos.Context, msg cosmos.Msg) (*cosmos.Result, error) {
		version := mgr.Keeper().GetLowestActiveVersion(ctx)
		constantValues := constants.GetConstantValues(version)
		if constantValues == nil {
			return nil, errConstNotAvailable
		}
		handlerMap := getInternalHandlerMapping(mgr)
		legacyMsg, ok := msg.(legacytx.LegacyMsg)
		if !ok {
			return nil, cosmos.ErrUnknownRequest("invalid message type")
		}
		h, ok := handlerMap[legacyMsg.Type()]
		if !ok {
			errMsg := fmt.Sprintf("Unrecognized thorchain Msg type: %v", legacyMsg.Type())
			return nil, cosmos.ErrUnknownRequest(errMsg)
		}
		if version.GTE(semver.MustParse("1.88.1")) {
			// CacheContext() returns a context which caches all changes and only forwards
			// to the underlying context when commit() is called. Call commit() only when
			// the handler succeeds, otherwise return error and the changes will be discarded.
			// On commit, cached events also have to be explicitly emitted.
			cacheCtx, commit := ctx.CacheContext()
			res, err := h.Run(cacheCtx, msg)
			if err == nil {
				// Success, commit the cached changes and events
				commit()
				ctx.EventManager().EmitEvents(cacheCtx.EventManager().Events())
			}
			return res, err
		}
		return h.Run(ctx, msg)
	}
}

func getInternalHandlerMapping(mgr Manager) map[string]MsgHandler {
	// New arch handlers
	m := make(map[string]MsgHandler)
	m[MsgOutboundTx{}.Type()] = NewOutboundTxHandler(mgr)
	m[MsgYggdrasil{}.Type()] = NewYggdrasilHandler(mgr)
	m[MsgSwap{}.Type()] = NewSwapHandler(mgr)
	m[MsgReserveContributor{}.Type()] = NewReserveContributorHandler(mgr)
	m[MsgBond{}.Type()] = NewBondHandler(mgr)
	m[MsgUnBond{}.Type()] = NewUnBondHandler(mgr)
	m[MsgLeave{}.Type()] = NewLeaveHandler(mgr)
	m[MsgDonate{}.Type()] = NewDonateHandler(mgr)
	m[MsgWithdrawLiquidity{}.Type()] = NewWithdrawLiquidityHandler(mgr)
	m[MsgAddLiquidity{}.Type()] = NewAddLiquidityHandler(mgr)
	m[MsgRefundTx{}.Type()] = NewRefundHandler(mgr)
	m[MsgMigrate{}.Type()] = NewMigrateHandler(mgr)
	m[MsgRagnarok{}.Type()] = NewRagnarokHandler(mgr)
	m[MsgSwitch{}.Type()] = NewSwitchHandler(mgr)
	m[MsgNoOp{}.Type()] = NewNoOpHandler(mgr)
	m[MsgConsolidate{}.Type()] = NewConsolidateHandler(mgr)
	m[MsgManageTHORName{}.Type()] = NewManageTHORNameHandler(mgr)
	return m
}

func getMsgSwapFromMemo(memo SwapMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	if memo.Destination.IsEmpty() {
		memo.Destination = tx.Tx.FromAddress
	}
	return NewMsgSwap(tx.Tx, memo.GetAsset(), memo.Destination, memo.SlipLimit, memo.AffiliateAddress, memo.AffiliateBasisPoints, signer), nil
}

func getMsgWithdrawFromMemo(memo WithdrawLiquidityMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	withdrawAmount := cosmos.NewUint(MaxWithdrawBasisPoints)
	if !memo.GetAmount().IsZero() {
		withdrawAmount = memo.GetAmount()
	}
	return NewMsgWithdrawLiquidity(tx.Tx, tx.Tx.FromAddress, withdrawAmount, memo.GetAsset(), memo.GetWithdrawalAsset(), signer), nil
}

func getMsgAddLiquidityFromMemo(ctx cosmos.Context, memo AddLiquidityMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	// Extract the Rune amount and the asset amount from the transaction. At least one of them must be
	// nonzero. If THORNode saw two types of coins, one of them must be the asset coin.
	runeCoin := tx.Tx.Coins.GetCoin(common.RuneAsset())
	assetCoin := tx.Tx.Coins.GetCoin(memo.GetAsset())

	var runeAddr common.Address
	var assetAddr common.Address
	if tx.Tx.Chain.Equals(common.THORChain) {
		runeAddr = tx.Tx.FromAddress
		assetAddr = memo.GetDestination()
	} else {
		runeAddr = memo.GetDestination()
		assetAddr = tx.Tx.FromAddress
	}
	// in case we are providing native rune and another native asset
	if memo.GetAsset().Chain.Equals(common.THORChain) {
		assetAddr = runeAddr
	}

	return NewMsgAddLiquidity(tx.Tx, memo.GetAsset(), runeCoin.Amount, assetCoin.Amount, runeAddr, assetAddr, memo.AffiliateAddress, memo.AffiliateBasisPoints, signer), nil
}

func getMsgDonateFromMemo(memo DonateMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	runeCoin := tx.Tx.Coins.GetCoin(common.RuneAsset())
	assetCoin := tx.Tx.Coins.GetCoin(memo.GetAsset())
	return NewMsgDonate(tx.Tx, memo.GetAsset(), runeCoin.Amount, assetCoin.Amount, signer), nil
}

func getMsgRefundFromMemo(memo RefundMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	return NewMsgRefundTx(tx, memo.GetTxID(), signer), nil
}

func getMsgOutboundFromMemo(memo OutboundMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	return NewMsgOutboundTx(tx, memo.GetTxID(), signer), nil
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
	coin := tx.Tx.Coins.GetCoin(common.RuneAsset())
	return NewMsgBond(tx.Tx, memo.GetAccAddress(), coin.Amount, tx.Tx.FromAddress, memo.BondProviderAddress, signer, memo.NodeOperatorFee), nil
}

func getMsgUnbondFromMemo(memo UnbondMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	return NewMsgUnBond(tx.Tx, memo.GetAccAddress(), memo.GetAmount(), tx.Tx.FromAddress, memo.BondProviderAddress, signer), nil
}

func getMsgManageTHORNameFromMemo(memo ManageTHORNameMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	if len(tx.Tx.Coins) == 0 {
		return nil, fmt.Errorf("transaction must have rune in it")
	}
	return NewMsgManageTHORName(memo.Name, memo.Chain, memo.Address, tx.Tx.Coins[0], memo.Expire, memo.PreferredAsset, memo.Owner, signer), nil
}

func processOneTxIn(ctx cosmos.Context, version semver.Version, keeper keeper.Keeper, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	if version.GTE(semver.MustParse("0.63.0")) {
		return processOneTxInV63(ctx, keeper, tx, signer)
	}
	return nil, errBadVersion
}

func processOneTxInV63(ctx cosmos.Context, keeper keeper.Keeper, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	memo, err := ParseMemoWithTHORNames(ctx, keeper, tx.Tx.Memo)
	if err != nil {
		ctx.Logger().Error("fail to parse memo", "error", err)
		return nil, err
	}
	// THORNode should not have one tx across chain, if it is cross chain it should be separate tx
	var newMsg cosmos.Msg
	// interpret the memo and initialize a corresponding msg event
	switch m := memo.(type) {
	case AddLiquidityMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		newMsg, err = getMsgAddLiquidityFromMemo(ctx, m, tx, signer)
	case WithdrawLiquidityMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		newMsg, err = getMsgWithdrawFromMemo(m, tx, signer)
	case SwapMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		newMsg, err = getMsgSwapFromMemo(m, tx, signer)
	case DonateMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		newMsg, err = getMsgDonateFromMemo(m, tx, signer)
	case RefundMemo:
		newMsg, err = getMsgRefundFromMemo(m, tx, signer)
	case OutboundMemo:
		newMsg, err = getMsgOutboundFromMemo(m, tx, signer)
	case MigrateMemo:
		newMsg, err = getMsgMigrateFromMemo(m, tx, signer)
	case BondMemo:
		newMsg, err = getMsgBondFromMemo(m, tx, signer)
	case UnbondMemo:
		newMsg, err = getMsgUnbondFromMemo(m, tx, signer)
	case RagnarokMemo:
		newMsg, err = getMsgRagnarokFromMemo(m, tx, signer)
	case LeaveMemo:
		newMsg, err = getMsgLeaveFromMemo(m, tx, signer)
	case YggdrasilFundMemo:
		newMsg = NewMsgYggdrasil(tx.Tx, tx.ObservedPubKey, m.GetBlockHeight(), true, tx.Tx.Coins, signer)
	case YggdrasilReturnMemo:
		newMsg = NewMsgYggdrasil(tx.Tx, tx.ObservedPubKey, m.GetBlockHeight(), false, tx.Tx.Coins, signer)
	case ReserveMemo:
		res := NewReserveContributor(tx.Tx.FromAddress, tx.Tx.Coins.GetCoin(common.RuneAsset()).Amount)
		newMsg = NewMsgReserveContributor(tx.Tx, res, signer)
	case SwitchMemo:
		newMsg = NewMsgSwitch(tx.Tx, memo.GetDestination(), signer)
	case NoOpMemo:
		newMsg = NewMsgNoOp(tx, signer, m.Action)
	case ConsolidateMemo:
		newMsg = NewMsgConsolidate(tx, signer)
	case ManageTHORNameMemo:
		newMsg, err = getMsgManageTHORNameFromMemo(m, tx, signer)
	default:
		return nil, errInvalidMemo
	}

	if err != nil {
		return newMsg, err
	}
	// MsgAddLiquidity & MsgSwap has a new version of validateBasic
	switch m := newMsg.(type) {
	case *MsgAddLiquidity:
		return newMsg, m.ValidateBasicV63()
	case *MsgSwap:
		return newMsg, m.ValidateBasicV63()
	}
	return newMsg, newMsg.ValidateBasic()
}

func fuzzyAssetMatch(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset) common.Asset {
	version := keeper.Version()
	if version.GTE(semver.MustParse("1.83.0")) {
		return fuzzyAssetMatchV83(ctx, keeper, asset)
	}
	return fuzzyAssetMatchV1(ctx, keeper, asset)
}

func fuzzyAssetMatchV83(ctx cosmos.Context, keeper keeper.Keeper, origAsset common.Asset) common.Asset {
	asset := origAsset.GetLayer1Asset()
	// if its already an exact match, return it immediately
	if keeper.PoolExist(ctx, asset) {
		return origAsset
	}

	matches := make(Pools, 0)

	iterator := keeper.GetPoolIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		if err := keeper.Cdc().Unmarshal(iterator.Value(), &pool); err != nil {
			ctx.Logger().Error("fail to fetch pool", "asset", asset, "err", err)
			continue
		}

		// check chain match
		if !asset.Chain.Equals(pool.Asset.Chain) {
			continue
		}

		// check ticker match
		if !asset.Ticker.Equals(pool.Asset.Ticker) {
			continue
		}

		// check symbol
		parts := strings.Split(asset.Symbol.String(), "-")
		// check if no symbol given (ie "USDT" or "USDT-")
		if len(parts) < 2 || strings.EqualFold(parts[1], "") {
			matches = append(matches, pool)
			continue
		}

		if strings.HasSuffix(strings.ToLower(pool.Asset.Symbol.String()), strings.ToLower(parts[1])) {
			matches = append(matches, pool)
			continue
		}
	}

	// if we found no matches, return the argument given
	if len(matches) == 0 {
		return origAsset
	}

	// find the deepest pool
	winner := NewPool()
	for _, pool := range matches {
		if winner.BalanceRune.LT(pool.BalanceRune) {
			winner = pool
		}
	}

	winner.Asset.Synth = origAsset.Synth

	return winner.Asset
}
