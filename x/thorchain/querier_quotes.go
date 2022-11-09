package thorchain

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog"
	abci "github.com/tendermint/tendermint/abci/types"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/log"
	openapi "gitlab.com/thorchain/thornode/openapi/gen"
	mem "gitlab.com/thorchain/thornode/x/thorchain/memo"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

// -------------------------------------------------------------------------------------
// Config
// -------------------------------------------------------------------------------------

const (
	fromAssetParam            = "from_asset"
	toAssetParam              = "to_asset"
	assetParam                = "asset"
	addressParam              = "address"
	withdrawBasisPointsParam  = "withdraw_bps"
	amountParam               = "amount"
	destinationParam          = "destination"
	toleranceBasisPointsParam = "tolerance_bps"
	affiliateParam            = "affiliate"
	affiliateBpsParam         = "affiliate_bps"
)

var nullLogger = &log.TendermintLogWrapper{Logger: zerolog.New(ioutil.Discard)}

// -------------------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------------------

func quoteErrorResponse(err error) ([]byte, error) {
	return json.Marshal(map[string]string{"error": err.Error()})
}

func quoteParseParams(data []byte) (params url.Values, err error) {
	// parse the query parameters
	u, err := url.ParseRequestURI(string(data))
	if err != nil {
		return nil, fmt.Errorf("bad params: %w", err)
	}

	// error if parameters were not provided
	if len(u.Query()) == 0 {
		return nil, fmt.Errorf("no parameters provided")
	}

	return u.Query(), nil
}

func quoteParseAddress(ctx cosmos.Context, mgr *Mgrs, addrString string, chain common.Chain) (common.Address, error) {
	if addrString == "" {
		return common.NoAddress, nil
	}

	// attempt to parse a raw address
	addr, err := common.NewAddress(addrString)
	if err == nil {
		return addr, nil
	}

	// attempt to lookup a thorname address
	name, err := mgr.Keeper().GetTHORName(ctx, addrString)
	if err != nil {
		return common.NoAddress, fmt.Errorf("unable to parse address: %w", err)
	}

	// find the address for the correct chain
	for _, alias := range name.Aliases {
		if alias.Chain.Equals(chain) {
			return alias.Address, nil
		}
	}

	return common.NoAddress, fmt.Errorf("no thorname alias for chain %s", chain)
}

func quoteHandleAffiliate(ctx cosmos.Context, mgr *Mgrs, params url.Values, amount sdk.Uint) (affiliate common.Address, memo string, bps, newAmount sdk.Uint, err error) {
	// parse affiliate
	memo = "" // do not resolve thorname for the memo
	if len(params[affiliateParam]) > 0 {
		affiliate, err = quoteParseAddress(ctx, mgr, params[affiliateParam][0], common.THORChain)
		if err != nil {
			err = fmt.Errorf("bad affiliate address: %w", err)
			return
		}
		memo = params[affiliateParam][0]
	}

	// parse affiliate fee
	bps = sdk.NewUint(0)
	if len(params[affiliateBpsParam]) > 0 {
		bps, err = sdk.ParseUint(params[affiliateBpsParam][0])
		if err != nil {
			err = fmt.Errorf("bad affiliate fee: %w", err)
			return
		}
	}

	// verify affiliate fee
	if bps.GT(sdk.NewUint(10000)) {
		err = fmt.Errorf("affiliate fee must be less than 10000 bps")
		return
	}

	// compute the new swap amount if an affiliate fee will be taken first
	if affiliate != common.NoAddress && !bps.IsZero() {
		// affiliate fee modifies amount at observation before the swap
		amount = common.GetSafeShare(
			cosmos.NewUint(10000).Sub(bps),
			cosmos.NewUint(10000),
			amount,
		)
	}

	return affiliate, memo, bps, amount, nil
}

func hasPrefixMatch(prefix string, values []string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func quoteReverseFuzzyAsset(ctx cosmos.Context, mgr *Mgrs, asset common.Asset) (common.Asset, error) {
	// get all pools
	pools, err := mgr.Keeper().GetPools(ctx)
	if err != nil {
		return asset, fmt.Errorf("failed to get pools: %w", err)
	}

	// get all other assets
	assets := []string{}
	for _, p := range pools {
		if p.IsAvailable() && !p.IsEmpty() && !p.Asset.Equals(asset) {
			assets = append(assets, p.Asset.String())
		}
	}

	// find the shortest unique prefix of the memo asset
	as := asset.String()
	for i := 1; i < len(as); i++ {
		if !hasPrefixMatch(as[:i], assets) {
			return common.NewAsset(as[:i])
		}
	}

	return asset, nil
}

func quoteSimulateSwap(ctx cosmos.Context, mgr *Mgrs, amount sdk.Uint, msg *MsgSwap) (res *openapi.QuoteSwapResponse, emitAmount sdk.Uint, err error) {
	// if the generated memo is too long for the source chain send error
	maxMemoLength := msg.Tx.Coins[0].Asset.Chain.MaxMemoLength()
	if maxMemoLength > 0 && len(msg.Tx.Memo) > maxMemoLength {
		return nil, sdk.ZeroUint(), fmt.Errorf("generated memo too long for source chain")
	}

	// use the first active node account as the signer
	nodeAccounts, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, sdk.ZeroUint(), fmt.Errorf("no active node accounts: %w", err)
	}
	msg.Signer = nodeAccounts[0].NodeAddress

	// simulate the swap
	events, err := simulateInternal(ctx, mgr, msg)
	if err != nil {
		return nil, sdk.ZeroUint(), err
	}

	// extract events
	var swaps []map[string]string
	var fee map[string]string
	for _, e := range events {
		switch e.Type {
		case "swap":
			swaps = append(swaps, eventMap(e))
		case "fee":
			fee = eventMap(e)
		}
	}
	finalSwap := swaps[len(swaps)-1]

	// parse outbound fee from event
	outboundFeeCoin, err := common.ParseCoin(fee["coins"])
	if err != nil {
		return nil, sdk.ZeroUint(), fmt.Errorf("unable to parse outbound fee coin: %w", err)
	}
	outboundFeeAmount := outboundFeeCoin.Amount

	// parse outbound amount from event
	emitCoin, err := common.ParseCoin(finalSwap["emit_asset"])
	if err != nil {
		return nil, sdk.ZeroUint(), fmt.Errorf("unable to parse emit coin: %w", err)
	}
	emitAmount = emitCoin.Amount

	// approximate the affiliate fee in the target asset
	affiliateFee := sdk.ZeroUint()
	if msg.AffiliateAddress != common.NoAddress && !msg.AffiliateBasisPoints.IsZero() {
		affiliateFee = common.GetUncappedShare(msg.AffiliateBasisPoints, cosmos.NewUint(10_000), amount)
		affiliateFee = affiliateFee.Mul(emitAmount).Quo(msg.Tx.Coins[0].Amount)

		// undo the approximate slip fee since the affiliate fee is taken first
		factor := sdk.NewUint(10_000)
		for _, s := range swaps {
			factor.Add(sdk.NewUintFromString(s["swap_slip"]))
		}
		affiliateFee = affiliateFee.Mul(factor).Quo(sdk.NewUint(10_000))
	}

	// sum the slip fees
	slippageBps := sdk.ZeroUint()
	for _, s := range swaps {
		slippageBps = slippageBps.Add(sdk.NewUintFromString(s["swap_slip"]))
	}

	// build response from simulation result events
	return &openapi.QuoteSwapResponse{
		ExpectedAmountOut: emitAmount.Sub(outboundFeeAmount).String(),
		Fees: openapi.QuoteFees{
			Asset:     msg.TargetAsset.String(),
			Affiliate: affiliateFee.String(),
			Outbound:  outboundFeeAmount.String(),
		},
		SlippageBps: slippageBps.BigInt().Int64(),
	}, emitAmount, nil
}

func quoteInboundInfo(ctx cosmos.Context, mgr *Mgrs, amount sdk.Uint, chain common.Chain) (address common.Address, confirmations int64, err error) {
	// get the most secure vault for inbound
	active, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return common.NoAddress, 0, err
	}
	constAccessor := mgr.GetConstants()
	signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	vault := mgr.Keeper().GetMostSecure(ctx, active, signingTransactionPeriod)
	address, err = vault.PubKey.GetAddress(chain)
	if err != nil {
		return common.NoAddress, 0, err
	}

	// estimate the inbound confirmation count blocks: ceil(amount/coinbase)
	if chain.DefaultCoinbase() > 0 {
		coinbase := cosmos.NewUint(uint64(chain.DefaultCoinbase()) * common.One)
		confirmations = amount.Quo(coinbase).BigInt().Int64()
		if !amount.Mod(coinbase).IsZero() {
			confirmations++
		}
	}

	return address, confirmations, nil
}

func quoteOutboundInfo(ctx cosmos.Context, mgr *Mgrs, coin common.Coin) (int64, error) {
	toi := TxOutItem{
		Memo: "OUT:-",
		Coin: coin,
	}
	outboundHeight, err := mgr.txOutStore.CalcTxOutHeight(ctx, mgr.GetVersion(), toi)
	if err != nil {
		return 0, err
	}
	return outboundHeight - ctx.BlockHeight(), nil
}

// -------------------------------------------------------------------------------------
// Swap
// -------------------------------------------------------------------------------------

func queryQuoteSwap(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	// extract parameters
	params, err := quoteParseParams(req.Data)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// validate required parameters
	for _, p := range []string{fromAssetParam, toAssetParam, amountParam} {
		if len(params[p]) == 0 {
			return quoteErrorResponse(fmt.Errorf("missing required parameter %s", p))
		}
	}

	// parse assets
	fromAsset, err := common.NewAsset(params[fromAssetParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad from asset: %w", err))
	}
	toAsset, err := common.NewAsset(params[toAssetParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad to asset: %w", err))
	}

	// parse amount
	amount, err := cosmos.ParseUint(params[amountParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad amount: %w", err))
	}

	// parse affiliate
	affiliate, affiliateMemo, affiliateBps, swapAmount, err := quoteHandleAffiliate(ctx, mgr, params, amount)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// parse destination address or generate a random one
	sendMemo := true
	var destination common.Address
	if len(params[destinationParam]) > 0 {
		destination, err = quoteParseAddress(ctx, mgr, params[destinationParam][0], toAsset.Chain)
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("bad destination address: %w", err))
		}

	} else {
		chain := common.THORChain
		if !toAsset.IsSyntheticAsset() {
			chain = toAsset.Chain
		}
		destination, err = types.GetRandomPubKey().GetAddress(chain)
		if err != nil {
			return nil, fmt.Errorf("failed to generate address: %w", err)
		}
		sendMemo = false // do not send memo if destination was random
	}

	// parse tolerance basis points
	limit := sdk.ZeroUint()
	if len(params[toleranceBasisPointsParam]) > 0 {
		// validate tolerance basis points
		toleranceBasisPoints, err := sdk.ParseUint(params[toleranceBasisPointsParam][0])
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("bad tolerance basis points: %w", err))
		}
		if toleranceBasisPoints.GT(sdk.NewUint(10000)) {
			return quoteErrorResponse(fmt.Errorf("tolerance basis points must be less than 10000"))
		}

		// get from asset pool
		fromPool, err := mgr.Keeper().GetPool(ctx, fromAsset)
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("failed to get pool: %w", err))
		}

		// get to asset pool
		toPool, err := mgr.Keeper().GetPool(ctx, toAsset)
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("failed to get pool: %w", err))
		}

		// convert to a limit of target asset amount assuming zero fees and slip
		feelessEmit := amount.Mul(fromPool.BalanceRune).Quo(fromPool.BalanceAsset).Mul(toPool.BalanceAsset).Quo(toPool.BalanceRune)
		limit = feelessEmit.MulUint64(10000 - toleranceBasisPoints.Uint64()).QuoUint64(10000)
	}

	// create the memo
	memo := &SwapMemo{
		MemoBase: mem.MemoBase{
			TxType: TxSwap,
			Asset:  toAsset,
		},
		Destination:          destination,
		SlipLimit:            limit,
		AffiliateAddress:     common.Address(affiliateMemo),
		AffiliateBasisPoints: affiliateBps,
	}

	// if from asset chain has memo length restrictions use a prefix
	if fromAsset.Chain.MaxMemoLength() > 0 && len(memo.String()) > fromAsset.Chain.MaxMemoLength() {
		memo.Asset, err = quoteReverseFuzzyAsset(ctx, mgr, toAsset)
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("failed to reverse fuzzy asset: %w", err))
		}
	}

	// create the swap message
	msg := &types.MsgSwap{
		Tx: common.Tx{
			ID:          common.BlankTxID,
			Chain:       fromAsset.Chain,
			FromAddress: common.NoopAddress,
			ToAddress:   common.NoopAddress,
			Coins: []common.Coin{
				{
					Asset:  fromAsset,
					Amount: swapAmount,
				},
			},
			Gas: []common.Coin{{
				Asset:  common.RuneAsset(),
				Amount: sdk.NewUint(1),
			}},
			Memo: memo.String(),
		},
		TargetAsset:          toAsset,
		TradeTarget:          limit,
		Destination:          destination,
		AffiliateAddress:     affiliate,
		AffiliateBasisPoints: affiliateBps,
	}

	// simulate the swap
	res, emitAmount, err := quoteSimulateSwap(ctx, mgr, amount, msg)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("failed to simulate swap: %w", err))
	}

	// estimate the inbound info
	inboundAddress, inboundConfirmations, err := quoteInboundInfo(ctx, mgr, amount, msg.Tx.Chain)
	if err != nil {
		return quoteErrorResponse(err)
	}
	res.InboundAddress = inboundAddress.String()
	if inboundConfirmations > 0 {
		res.InboundConfirmationBlocks = wrapInt64(inboundConfirmations)
		res.InboundConfirmationSeconds = wrapInt64(inboundConfirmations * msg.Tx.Chain.ApproximateBlockMilliseconds() / 1000)
	}

	// estimate the outbound info
	outboundDelay, err := quoteOutboundInfo(ctx, mgr, common.Coin{Asset: toAsset, Amount: emitAmount})
	if err != nil {
		return quoteErrorResponse(err)
	}
	res.OutboundDelayBlocks = outboundDelay
	res.OutboundDelaySeconds = outboundDelay * toAsset.Chain.ApproximateBlockMilliseconds() / 1000

	// send memo if the destination was provided
	if sendMemo {
		res.Memo = wrapString(memo.String())
	}

	return json.MarshalIndent(res, "", "  ")
}

// -------------------------------------------------------------------------------------
// Saver Deposit
// -------------------------------------------------------------------------------------

func queryQuoteSaverDeposit(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	// extract parameters
	params, err := quoteParseParams(req.Data)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// validate required parameters
	for _, p := range []string{assetParam, amountParam} {
		if len(params[p]) == 0 {
			return quoteErrorResponse(fmt.Errorf("missing required parameter %s", p))
		}
	}

	// parse asset
	asset, err := common.NewAsset(params[assetParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad asset: %w", err))
	}

	// parse amount
	amount, err := cosmos.ParseUint(params[amountParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad amount: %w", err))
	}

	// parse affiliate
	affiliate, affiliateMemo, affiliateBps, depositAmount, err := quoteHandleAffiliate(ctx, mgr, params, amount)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// create the memo
	memo := &SwapMemo{
		MemoBase: mem.MemoBase{
			TxType: TxSwap, // swap and add uses swap handler
			Asset:  asset.GetSyntheticAsset(),
		},
		SlipLimit:            sdk.ZeroUint(),
		AffiliateAddress:     common.Address(affiliateMemo),
		AffiliateBasisPoints: affiliateBps,
	}

	// use random destination address
	destination, err := types.GetRandomPubKey().GetAddress(common.THORChain)
	if err != nil {
		return nil, fmt.Errorf("failed to generate address: %w", err)
	}

	// create the message
	msg := &types.MsgSwap{
		Tx: common.Tx{
			ID:          common.BlankTxID,
			Chain:       asset.Chain,
			FromAddress: common.NoopAddress,
			ToAddress:   common.NoopAddress,
			Coins: []common.Coin{
				{
					Asset:  asset,
					Amount: depositAmount,
				},
			},
			Gas: []common.Coin{
				{
					Asset:  common.RuneAsset(),
					Amount: sdk.NewUint(1),
				},
			},
			Memo: memo.String(),
		},
		TargetAsset:          asset.GetSyntheticAsset(),
		TradeTarget:          sdk.ZeroUint(),
		AffiliateAddress:     affiliate,
		AffiliateBasisPoints: affiliateBps,
		Destination:          destination,
	}

	// get the swap result
	swapRes, _, err := quoteSimulateSwap(ctx, mgr, amount, msg)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("failed to simulate swap: %w", err))
	}

	// generate deposit memo
	depositMemoComponents := []string{
		"+",
		asset.GetSyntheticAsset().String(),
		"",
		affiliateMemo,
		affiliateBps.String(),
	}
	depositMemo := strings.Join(depositMemoComponents[:2], ":")
	if affiliate != common.NoAddress && !affiliateBps.IsZero() {
		depositMemo = strings.Join(depositMemoComponents, ":")
	}

	// use the swap result info to generate the deposit quote
	res := &openapi.QuoteSaverDepositResponse{
		ExpectedAmountOut:          swapRes.ExpectedAmountOut,
		Fees:                       swapRes.Fees,
		SlippageBps:                swapRes.SlippageBps,
		InboundConfirmationBlocks:  swapRes.InboundConfirmationBlocks,
		InboundConfirmationSeconds: swapRes.InboundConfirmationSeconds,
		Memo:                       depositMemo,
	}

	// estimate the inbound info
	inboundAddress, inboundConfirmations, err := quoteInboundInfo(ctx, mgr, amount, asset.GetLayer1Asset().Chain)
	if err != nil {
		return quoteErrorResponse(err)
	}
	res.InboundAddress = inboundAddress.String()
	res.InboundConfirmationBlocks = wrapInt64(inboundConfirmations)

	return json.MarshalIndent(res, "", "  ")
}

// -------------------------------------------------------------------------------------
// Saver Withdraw
// -------------------------------------------------------------------------------------

func queryQuoteSaverWithdraw(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	// extract parameters
	params, err := quoteParseParams(req.Data)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// validate required parameters
	for _, p := range []string{assetParam, addressParam, withdrawBasisPointsParam} {
		if len(params[p]) == 0 {
			return quoteErrorResponse(fmt.Errorf("missing required parameter %s", p))
		}
	}

	// parse asset
	asset, err := common.NewAsset(params[assetParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad asset: %w", err))
	}
	asset = asset.GetSyntheticAsset() // always use the vault asset

	// parse address
	address, err := common.NewAddress(params[addressParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad address: %w", err))
	}

	// parse basis points
	basisPoints, err := cosmos.ParseUint(params[withdrawBasisPointsParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad basis points: %w", err))
	}

	// validate basis points
	if basisPoints.GT(sdk.NewUint(10_000)) {
		return quoteErrorResponse(fmt.Errorf("basis points must be less than 10000"))
	}

	// get liquidity provider
	lp, err := mgr.Keeper().GetLiquidityProvider(ctx, asset, address)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("failed to get liquidity provider: %w", err))
	}

	// get the pool
	pool, err := mgr.Keeper().GetPool(ctx, asset)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("failed to get pool: %w", err))
	}

	// get the liquidity provider share of the pool
	lpShare := common.GetSafeShare(lp.Units, pool.LPUnits, pool.BalanceAsset)

	// calculate the withdraw amount
	amount := common.GetSafeShare(basisPoints, sdk.NewUint(10_000), lpShare)

	// create the memo
	memo := &SwapMemo{
		MemoBase: mem.MemoBase{
			TxType: TxSwap,
			Asset:  asset,
		},
		SlipLimit: sdk.ZeroUint(),
	}

	// create the message
	msg := &types.MsgSwap{
		Tx: common.Tx{
			ID:          common.BlankTxID,
			Chain:       common.THORChain,
			FromAddress: common.NoopAddress,
			ToAddress:   common.NoopAddress,
			Coins: []common.Coin{
				{
					Asset:  asset,
					Amount: amount,
				},
			},
			Gas: []common.Coin{
				{
					Asset:  common.RuneAsset(),
					Amount: sdk.NewUint(1),
				},
			},
			Memo: memo.String(),
		},
		TargetAsset:          asset.GetLayer1Asset(),
		TradeTarget:          sdk.ZeroUint(),
		AffiliateAddress:     common.NoAddress,
		AffiliateBasisPoints: sdk.ZeroUint(),
		Destination:          address,
	}

	// get the swap result
	swapRes, emitAmount, err := quoteSimulateSwap(ctx, mgr, amount, msg)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("failed to simulate swap: %w", err))
	}

	// use the swap result info to generate the withdraw quote
	res := &openapi.QuoteSaverWithdrawResponse{
		ExpectedAmountOut: swapRes.ExpectedAmountOut,
		Fees:              swapRes.Fees,
		SlippageBps:       swapRes.SlippageBps,
		Memo:              fmt.Sprintf("-:%s:%s", asset.String(), basisPoints.String()),
		DustAmount:        asset.GetLayer1Asset().Chain.DustThreshold().Add(basisPoints).String(),
	}

	// estimate the inbound info
	inboundAddress, _, err := quoteInboundInfo(ctx, mgr, amount, asset.GetLayer1Asset().Chain)
	if err != nil {
		return quoteErrorResponse(err)
	}
	res.InboundAddress = inboundAddress.String()

	// estimate the outbound info
	outboundCoin := common.Coin{Asset: asset.GetLayer1Asset(), Amount: emitAmount}
	outboundDelay, err := quoteOutboundInfo(ctx, mgr, outboundCoin)
	if err != nil {
		return quoteErrorResponse(err)
	}
	res.OutboundDelayBlocks = outboundDelay
	res.OutboundDelaySeconds = outboundDelay * asset.GetLayer1Asset().Chain.ApproximateBlockMilliseconds() / 1000

	return json.MarshalIndent(res, "", "  ")
}
