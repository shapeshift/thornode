package thorchain

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/armon/go-metrics"
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/telemetry"
	"github.com/hashicorp/go-multierror"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

var WhitelistedArbs = []string{ // treasury addresses
	"thor1egxvam70a86jafa8gcg3kqfmfax3s0m2g3m754",
	"bc1qq2z2f4gs4nd7t0a9jjp90y9l9zzjtegu4nczha",
	"qz7262r7uufxk89ematxrf6yquk7zfwrjqm97vskzw",
	"0x04c5998ded94f89263370444ce64a99b7dbc9f46",
	"bnb1pa6hpjs7qv0vkd5ks5tqa2xtt2gk5n08yw7v7f",
	"ltc1qaa064vvv4d6stgywnf777j6dl8rd3tt93fp6jx",
}

func refundTx(ctx cosmos.Context, tx ObservedTx, mgr Manager, refundCode uint32, refundReason, nativeRuneModuleName string) error {
	version := mgr.GetVersion()
	if version.GTE(semver.MustParse("0.47.0")) {
		return refundTxV47(ctx, tx, mgr, refundCode, refundReason, nativeRuneModuleName)
	}
	return errBadVersion
}

func refundTxV47(ctx cosmos.Context, tx ObservedTx, mgr Manager, refundCode uint32, refundReason, nativeRuneModuleName string) error {
	// If THORNode recognize one of the coins, and therefore able to refund
	// withholding fees, refund all coins.

	addEvent := func(refundCoins common.Coins) error {
		eventRefund := NewEventRefund(refundCode, refundReason, tx.Tx, common.NewFee(common.Coins{}, cosmos.ZeroUint()))
		if len(refundCoins) > 0 {
			// create a new TX based on the coins thorchain refund , some of the coins thorchain doesn't refund
			// coin thorchain doesn't have pool with , likely airdrop
			newTx := common.NewTx(tx.Tx.ID, tx.Tx.FromAddress, tx.Tx.ToAddress, tx.Tx.Coins, tx.Tx.Gas, tx.Tx.Memo)

			// all the coins in tx.Tx should belongs to the same chain
			transactionFee := mgr.GasMgr().GetFee(ctx, tx.Tx.Chain, common.RuneAsset())
			fee := getFee(tx.Tx.Coins, refundCoins, transactionFee)
			eventRefund = NewEventRefund(refundCode, refundReason, newTx, fee)
		}
		if err := mgr.EventMgr().EmitEvent(ctx, eventRefund); err != nil {
			return fmt.Errorf("fail to emit refund event: %w", err)
		}
		return nil
	}

	// for THORChain transactions, create the event before we txout. For other
	// chains, do it after. The reason for this is we need to make sure the
	// first event (refund) is created, before we create the outbound events
	// (second). Because its THORChain, its safe to assume all the coins are
	// safe to send back. Where as for external coins, we cannot make this
	// assumption (ie coins we don't have pools for and therefore, don't know
	// the value of it relative to rune)
	if tx.Tx.Chain.Equals(common.THORChain) {
		if err := addEvent(tx.Tx.Coins); err != nil {
			return err
		}
	}
	refundCoins := make(common.Coins, 0)
	for _, coin := range tx.Tx.Coins {
		if coin.Asset.IsRune() && coin.Asset.GetChain().Equals(common.ETHChain) {
			continue
		}
		pool, err := mgr.Keeper().GetPool(ctx, coin.Asset.GetLayer1Asset())
		if err != nil {
			return fmt.Errorf("fail to get pool: %w", err)
		}

		if coin.Asset.IsRune() || !pool.BalanceRune.IsZero() {
			toi := TxOutItem{
				Chain:       coin.Asset.GetChain(),
				InHash:      tx.Tx.ID,
				ToAddress:   tx.Tx.FromAddress,
				VaultPubKey: tx.ObservedPubKey,
				Coin:        coin,
				Memo:        NewRefundMemo(tx.Tx.ID).String(),
				ModuleName:  nativeRuneModuleName,
			}

			success, err := mgr.TxOutStore().TryAddTxOutItem(ctx, mgr, toi, cosmos.ZeroUint())
			if err != nil {
				ctx.Logger().Error("fail to prepare outbund tx", "error", err)
				// concatenate the refund failure to refundReason
				refundReason = fmt.Sprintf("%s; fail to refund (%s): %s", refundReason, toi.Coin.String(), err)
			}
			if success {
				refundCoins = append(refundCoins, toi.Coin)
			}
		}
		// Zombie coins are just dropped.
	}
	if !tx.Tx.Chain.Equals(common.THORChain) {
		if err := addEvent(refundCoins); err != nil {
			return err
		}
	}

	return nil
}

func getFee(input, output common.Coins, transactionFee cosmos.Uint) common.Fee {
	var fee common.Fee
	assetTxCount := 0
	for _, out := range output {
		if !out.Asset.IsRune() {
			assetTxCount++
		}
	}
	for _, in := range input {
		outCoin := common.NoCoin
		for _, out := range output {
			if out.Asset.Equals(in.Asset) {
				outCoin = out
				break
			}
		}
		if outCoin.IsEmpty() {
			if !in.Amount.IsZero() {
				fee.Coins = append(fee.Coins, common.NewCoin(in.Asset, in.Amount))
			}
		} else {
			if !in.Amount.Sub(outCoin.Amount).IsZero() {
				fee.Coins = append(fee.Coins, common.NewCoin(in.Asset, in.Amount.Sub(outCoin.Amount)))
			}
		}
	}
	fee.PoolDeduct = transactionFee.MulUint64(uint64(assetTxCount))
	return fee
}

func subsidizePoolWithSlashBond(ctx cosmos.Context, ygg Vault, yggTotalStolen, slashRuneAmt cosmos.Uint, mgr Manager) error {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.92.0")):
		return subsidizePoolWithSlashBondV92(ctx, ygg, yggTotalStolen, slashRuneAmt, mgr)
	case version.GTE(semver.MustParse("1.88.0")):
		return subsidizePoolWithSlashBondV88(ctx, ygg, yggTotalStolen, slashRuneAmt, mgr)
	case version.GTE(semver.MustParse("0.74.0")):
		return subsidizePoolWithSlashBondV74(ctx, ygg, yggTotalStolen, slashRuneAmt, mgr)
	default:
		return errBadVersion
	}
}

func subsidizePoolWithSlashBondV92(ctx cosmos.Context, ygg Vault, yggTotalStolen, slashRuneAmt cosmos.Uint, mgr Manager) error {
	// Thorchain did not slash the node account
	if slashRuneAmt.IsZero() {
		return nil
	}
	stolenRUNE := ygg.GetCoin(common.RuneAsset()).Amount
	slashRuneAmt = common.SafeSub(slashRuneAmt, stolenRUNE)
	yggTotalStolen = common.SafeSub(yggTotalStolen, stolenRUNE)

	// Should never happen, but this prevents a divide-by-zero panic in case it does
	if yggTotalStolen.IsZero() {
		return nil
	}

	type fund struct {
		asset         common.Asset
		stolenAsset   cosmos.Uint
		subsidiseRune cosmos.Uint
	}
	// here need to use a map to hold on to the amount of RUNE need to be subsidized to each pool
	// reason being , if ygg pool has both RUNE and BNB coin left, these two coin share the same pool
	// which is BNB pool , if add the RUNE directly back to pool , it will affect BNB price , which will affect the result
	subsidize := make([]fund, 0)
	for _, coin := range ygg.Coins {
		if coin.IsEmpty() {
			continue
		}
		if coin.Asset.IsRune() {
			// when the asset is RUNE, thorchain don't need to update the RUNE balance on pool
			continue
		}
		f := fund{
			asset:         coin.Asset,
			stolenAsset:   cosmos.ZeroUint(),
			subsidiseRune: cosmos.ZeroUint(),
		}

		pool, err := mgr.Keeper().GetPool(ctx, coin.Asset.GetLayer1Asset())
		if err != nil {
			return err
		}
		f.stolenAsset = f.stolenAsset.Add(coin.Amount)
		runeValue := pool.AssetValueInRune(coin.Amount)
		if runeValue.IsZero() {
			ctx.Logger().Info("rune value of stolen asset is 0", "pool", pool.Asset, "asset amount", coin.Amount.String())
			continue
		}
		// the amount of RUNE thorchain used to subsidize the pool is calculate by ratio
		// slashRune * (stealAssetRuneValue /totalStealAssetRuneValue)
		subsidizeAmt := slashRuneAmt.Mul(runeValue).Quo(yggTotalStolen)
		f.subsidiseRune = f.subsidiseRune.Add(subsidizeAmt)
		subsidize = append(subsidize, f)
	}

	for _, f := range subsidize {
		pool, err := mgr.Keeper().GetPool(ctx, f.asset.GetLayer1Asset())
		if err != nil {
			ctx.Logger().Error("fail to get pool", "asset", f.asset, "error", err)
			continue
		}
		if pool.IsEmpty() {
			continue
		}

		pool.BalanceRune = pool.BalanceRune.Add(f.subsidiseRune)
		pool.BalanceAsset = common.SafeSub(pool.BalanceAsset, f.stolenAsset)

		if err := mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool", "asset", pool.Asset, "error", err)
			continue
		}

		// Send the subsidized RUNE from the Bond module to Asgard
		runeToAsgard := common.NewCoin(common.RuneNative, f.subsidiseRune)
		if !runeToAsgard.Amount.IsZero() {
			if err := mgr.Keeper().SendFromModuleToModule(ctx, BondName, AsgardName, common.NewCoins(runeToAsgard)); err != nil {
				ctx.Logger().Error("fail to send subsidy from bond to asgard", "error", err)
				return err
			}
		}

		poolSlashAmt := []PoolAmt{
			{
				Asset:  pool.Asset,
				Amount: 0 - int64(f.stolenAsset.Uint64()),
			},
			{
				Asset:  common.RuneAsset(),
				Amount: int64(f.subsidiseRune.Uint64()),
			},
		}
		eventSlash := NewEventSlash(pool.Asset, poolSlashAmt)
		if err := mgr.EventMgr().EmitEvent(ctx, eventSlash); err != nil {
			ctx.Logger().Error("fail to emit slash event", "error", err)
		}
	}
	return nil
}

// getTotalYggValueInRune will go through all the coins in ygg , and calculate the total value in RUNE
// return value will be totalValueInRune,error
func getTotalYggValueInRune(ctx cosmos.Context, keeper keeper.Keeper, ygg Vault) (cosmos.Uint, error) {
	yggRune := cosmos.ZeroUint()
	for _, coin := range ygg.Coins {
		if coin.Asset.IsRune() {
			yggRune = yggRune.Add(coin.Amount)
		} else {
			pool, err := keeper.GetPool(ctx, coin.Asset)
			if err != nil {
				return cosmos.ZeroUint(), err
			}
			yggRune = yggRune.Add(pool.AssetValueInRune(coin.Amount))
		}
	}
	return yggRune, nil
}

func refundBond(ctx cosmos.Context, tx common.Tx, acc cosmos.AccAddress, amt cosmos.Uint, nodeAcc *NodeAccount, mgr Manager) error {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.92.0")):
		return refundBondV92(ctx, tx, acc, amt, nodeAcc, mgr)
	case version.GTE(semver.MustParse("1.88.0")):
		return refundBondV88(ctx, tx, acc, amt, nodeAcc, mgr)
	case version.GTE(semver.MustParse("0.81.0")):
		return refundBondV81(ctx, tx, acc, amt, nodeAcc, mgr)
	default:
		return errBadVersion
	}
}

func refundBondV92(ctx cosmos.Context, tx common.Tx, acc cosmos.AccAddress, amt cosmos.Uint, nodeAcc *NodeAccount, mgr Manager) error {
	if nodeAcc.Status == NodeActive {
		ctx.Logger().Info("node still active, cannot refund bond", "node address", nodeAcc.NodeAddress, "node pub key", nodeAcc.PubKeySet.Secp256k1)
		return nil
	}

	// ensures nodes don't return bond while being churned into the network
	// (removing their bond last second)
	if nodeAcc.Status == NodeReady {
		ctx.Logger().Info("node ready, cannot refund bond", "node address", nodeAcc.NodeAddress, "node pub key", nodeAcc.PubKeySet.Secp256k1)
		return nil
	}

	if amt.IsZero() || amt.GT(nodeAcc.Bond) {
		amt = nodeAcc.Bond
	}

	ygg := Vault{}
	if mgr.Keeper().VaultExists(ctx, nodeAcc.PubKeySet.Secp256k1) {
		var err error
		ygg, err = mgr.Keeper().GetVault(ctx, nodeAcc.PubKeySet.Secp256k1)
		if err != nil {
			return err
		}
		if !ygg.IsYggdrasil() {
			return errors.New("this is not a Yggdrasil vault")
		}
	}

	bp, err := mgr.Keeper().GetBondProviders(ctx, nodeAcc.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get bond providers(%s)", nodeAcc.NodeAddress))
	}

	// backfill bond provider information (passive migration code)
	if len(bp.Providers) == 0 {
		// no providers yet, add node operator bond address to the bond provider list
		nodeOpBondAddr, err := nodeAcc.BondAddress.AccAddress()
		if err != nil {
			return ErrInternal(err, fmt.Sprintf("fail to parse bond address(%s)", nodeAcc.BondAddress))
		}
		p := NewBondProvider(nodeOpBondAddr)
		p.Bond = nodeAcc.Bond
		bp.Providers = append(bp.Providers, p)
	}

	// Calculate total value (in rune) the Yggdrasil pool has
	yggRune, err := getTotalYggValueInRune(ctx, mgr.Keeper(), ygg)
	if err != nil {
		return fmt.Errorf("fail to get total ygg value in RUNE: %w", err)
	}

	if nodeAcc.Bond.LT(yggRune) {
		ctx.Logger().Error("Node Account left with more funds in their Yggdrasil vault than their bond's value", "address", nodeAcc.NodeAddress, "ygg-value", yggRune, "bond", nodeAcc.Bond)
	}
	// slash yggdrasil remains
	penaltyPts := fetchConfigInt64(ctx, mgr, constants.SlashPenalty)
	slashRune := common.GetUncappedShare(cosmos.NewUint(uint64(penaltyPts)), cosmos.NewUint(10_000), yggRune)
	if slashRune.GT(nodeAcc.Bond) {
		slashRune = nodeAcc.Bond
	}
	bondBeforeSlash := nodeAcc.Bond
	nodeAcc.Bond = common.SafeSub(nodeAcc.Bond, slashRune)
	bp.Adjust(nodeAcc.Bond) // redistribute node bond amongst bond providers
	provider := bp.Get(acc)

	if !provider.IsEmpty() && !provider.Bond.IsZero() {
		if amt.GT(provider.Bond) {
			amt = provider.Bond
		}

		bp.Unbond(amt, provider.BondAddress)

		toAddress, err := common.NewAddress(provider.BondAddress.String())
		if err != nil {
			return fmt.Errorf("fail to parse bond address: %w", err)
		}

		// refund bond
		txOutItem := TxOutItem{
			Chain:      common.RuneAsset().Chain,
			ToAddress:  toAddress,
			InHash:     tx.ID,
			Coin:       common.NewCoin(common.RuneAsset(), amt),
			ModuleName: BondName,
		}
		_, err = mgr.TxOutStore().TryAddTxOutItem(ctx, mgr, txOutItem, cosmos.ZeroUint())
		if err != nil {
			return fmt.Errorf("fail to add outbound tx: %w", err)
		}

		bondEvent := NewEventBond(amt, BondReturned, tx)
		if err := mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
			ctx.Logger().Error("fail to emit bond event", "error", err)
		}

		nodeAcc.Bond = common.SafeSub(nodeAcc.Bond, amt)
	} else {
		// if it get into here that means the node account doesn't have any bond left after slash.
		// which means the real slashed RUNE could be the bond they have before slash
		slashRune = bondBeforeSlash
	}

	if nodeAcc.RequestedToLeave {
		// when node already request to leave , it can't come back , here means the node already unbond
		// so set the node to disabled status
		nodeAcc.UpdateStatus(NodeDisabled, ctx.BlockHeight())
	}
	if err := mgr.Keeper().SetNodeAccount(ctx, *nodeAcc); err != nil {
		ctx.Logger().Error(fmt.Sprintf("fail to save node account(%s)", nodeAcc), "error", err)
		return err
	}
	if err := mgr.Keeper().SetBondProviders(ctx, bp); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save bond providers(%s)", bp.NodeAddress.String()))
	}

	if err := subsidizePoolWithSlashBond(ctx, ygg, yggRune, slashRune, mgr); err != nil {
		ctx.Logger().Error("fail to subsidize pool with slashed bond", "error", err)
		return err
	}

	// at this point , all coins in yggdrasil vault has been accounted for , and node already been slashed
	ygg.SubFunds(ygg.Coins)
	if err := mgr.Keeper().SetVault(ctx, ygg); err != nil {
		ctx.Logger().Error("fail to save yggdrasil vault", "error", err)
		return err
	}

	if err := mgr.Keeper().DeleteVault(ctx, ygg.PubKey); err != nil {
		return err
	}

	// Output bond events for the slashed and returned bond.
	if !slashRune.IsZero() {
		fakeTx := common.Tx{}
		fakeTx.ID = common.BlankTxID
		fakeTx.FromAddress = nodeAcc.BondAddress
		bondEvent := NewEventBond(slashRune, BondCost, fakeTx)
		if err := mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
			ctx.Logger().Error("fail to emit bond event", "error", err)
		}
	}
	return nil
}

// isSignedByActiveNodeAccounts check if all signers are active validator nodes
func isSignedByActiveNodeAccounts(ctx cosmos.Context, k keeper.Keeper, signers []cosmos.AccAddress) bool {
	if len(signers) == 0 {
		return false
	}
	for _, signer := range signers {
		if signer.Equals(k.GetModuleAccAddress(AsgardName)) {
			continue
		}
		nodeAccount, err := k.GetNodeAccount(ctx, signer)
		if err != nil {
			ctx.Logger().Error("unauthorized account", "address", signer.String(), "error", err)
			return false
		}
		if nodeAccount.IsEmpty() {
			ctx.Logger().Error("unauthorized account", "address", signer.String())
			return false
		}
		if nodeAccount.Status != NodeActive {
			ctx.Logger().Error("unauthorized account, node account not active",
				"address", signer.String(),
				"status", nodeAccount.Status)
			return false
		}
		if nodeAccount.Type != NodeTypeValidator {
			ctx.Logger().Error("unauthorized account, node account must be a validator",
				"address", signer.String(),
				"type", nodeAccount.Type)
			return false
		}
	}
	return true
}

func fetchConfigInt64(ctx cosmos.Context, mgr Manager, key constants.ConstantName) int64 {
	val, err := mgr.Keeper().GetMimir(ctx, key.String())
	if val < 0 || err != nil {
		val = mgr.GetConstants().GetInt64Value(key)
		if err != nil {
			ctx.Logger().Error("fail to fetch mimir value", "key", key.String(), "error", err)
		}
	}
	return val
}

// polPoolValue - calculates how much the POL is worth in rune
func polPoolValue(ctx cosmos.Context, mgr Manager) (cosmos.Uint, error) {
	total := cosmos.ZeroUint()

	polAddress, err := mgr.Keeper().GetModuleAddress(ReserveName)
	if err != nil {
		return total, err
	}

	pools, err := mgr.Keeper().GetPools(ctx)
	if err != nil {
		return total, err
	}
	for _, pool := range pools {
		if pool.Asset.IsNative() {
			continue
		}
		if pool.BalanceRune.IsZero() {
			continue
		}
		synthSupply := mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
		pool.CalcUnits(mgr.GetVersion(), synthSupply)
		lp, err := mgr.Keeper().GetLiquidityProvider(ctx, pool.Asset, polAddress)
		if err != nil {
			return total, err
		}
		share := common.GetSafeShare(lp.Units, pool.GetPoolUnits(), pool.BalanceRune)
		total = total.Add(share.MulUint64(2))
	}

	return total, nil
}

func wrapError(ctx cosmos.Context, err error, wrap string) error {
	err = fmt.Errorf("%s: %w", wrap, err)
	ctx.Logger().Error(err.Error())
	return multierror.Append(errInternal, err)
}

func addGasFees(ctx cosmos.Context, mgr Manager, tx ObservedTx) error {
	version := mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return addGasFeesV1(ctx, mgr, tx)
	}
	return errBadVersion
}

// addGasFees to vault
func addGasFeesV1(ctx cosmos.Context, mgr Manager, tx ObservedTx) error {
	if len(tx.Tx.Gas) == 0 {
		return nil
	}
	if mgr.Keeper().RagnarokInProgress(ctx) {
		// when ragnarok is in progress, if the tx is for gas coin then doesn't subsidise the pool with reserve
		// liquidity providers they need to pay their own gas
		// if the outbound coin is not gas asset, then reserve will subsidise it , otherwise the gas asset pool will be in a loss
		gasAsset := tx.Tx.Chain.GetGasAsset()
		if tx.Tx.Coins.GetCoin(gasAsset).IsEmpty() {
			mgr.GasMgr().AddGasAsset(tx.Tx.Gas, true)
		}
	} else {
		mgr.GasMgr().AddGasAsset(tx.Tx.Gas, true)
	}
	// Subtract from the vault
	if mgr.Keeper().VaultExists(ctx, tx.ObservedPubKey) {
		vault, err := mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			return err
		}

		vault.SubFunds(tx.Tx.Gas.ToCoins())

		if err := mgr.Keeper().SetVault(ctx, vault); err != nil {
			return err
		}
	}
	return nil
}

func emitPoolBalanceChangedEvent(ctx cosmos.Context, poolMod PoolMod, reason string, mgr Manager) {
	evt := NewEventPoolBalanceChanged(poolMod, reason)
	if err := mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
		ctx.Logger().Error("fail to emit pool balance changed event", "error", err)
	}
}

// isTradingHalt is to check the given msg against the key value store to decide it can be processed
// if trade is halt across all chain , then the message should be refund
// if trade for the target chain is halt , then the message should be refund as well
// isTradingHalt has been used in two handlers , thus put it here
func isTradingHalt(ctx cosmos.Context, msg cosmos.Msg, mgr Manager) bool {
	version := mgr.GetVersion()
	if version.GTE(semver.MustParse("0.65.0")) {
		return isTradingHaltV65(ctx, msg, mgr)
	}
	return false
}

func isTradingHaltV65(ctx cosmos.Context, msg cosmos.Msg, mgr Manager) bool {
	switch m := msg.(type) {
	case *MsgSwap:
		for _, raw := range WhitelistedArbs {
			address, err := common.NewAddress(strings.TrimSpace(raw))
			if err != nil {
				ctx.Logger().Error("failt to parse address for trading halt check", "address", raw, "error", err)
				continue
			}
			if address.Equals(m.Tx.FromAddress) {
				return false
			}
		}
		source := common.EmptyChain
		if len(m.Tx.Coins) > 0 {
			source = m.Tx.Coins[0].Asset.GetLayer1Asset().Chain
		}
		target := m.TargetAsset.GetLayer1Asset().Chain
		return isChainTradingHalted(ctx, mgr, source) || isChainTradingHalted(ctx, mgr, target) || isGlobalTradingHalted(ctx, mgr)
	case *MsgAddLiquidity:
		return isChainTradingHalted(ctx, mgr, m.Asset.Chain) || isGlobalTradingHalted(ctx, mgr)
	default:
		return isGlobalTradingHalted(ctx, mgr)
	}
}

// isGlobalTradingHalted check whether trading has been halt at global level
func isGlobalTradingHalted(ctx cosmos.Context, mgr Manager) bool {
	haltTrading, err := mgr.Keeper().GetMimir(ctx, "HaltTrading")
	if err == nil && ((haltTrading > 0 && haltTrading < ctx.BlockHeight()) || mgr.Keeper().RagnarokInProgress(ctx)) {
		return true
	}
	return false
}

// isChainTradingHalted check whether trading on the given chain is halted
func isChainTradingHalted(ctx cosmos.Context, mgr Manager, chain common.Chain) bool {
	mimirKey := fmt.Sprintf("Halt%sTrading", chain)
	haltChainTrading, err := mgr.Keeper().GetMimir(ctx, mimirKey)
	if err == nil && (haltChainTrading > 0 && haltChainTrading < ctx.BlockHeight()) {
		ctx.Logger().Info("trading is halt", "chain", chain)
		return true
	}
	// further to check whether the chain is halted
	return isChainHalted(ctx, mgr, chain)
}

func isChainHalted(ctx cosmos.Context, mgr Manager, chain common.Chain) bool {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.87.0")):
		return isChainHaltedV87(ctx, mgr, chain)
	case version.GTE(semver.MustParse("0.65.0")):
		return isChainHaltedV65(ctx, mgr, chain)
	}
	return false
}

// isChainHalted check whether the given chain is halt
// chain halt is different as halt trading , when a chain is halt , there is no observation on the given chain
// outbound will not be signed and broadcast
func isChainHaltedV87(ctx cosmos.Context, mgr Manager, chain common.Chain) bool {
	haltChain, err := mgr.Keeper().GetMimir(ctx, "HaltChainGlobal")
	if err == nil && (haltChain > 0 && haltChain < ctx.BlockHeight()) {
		ctx.Logger().Info("global is halt")
		return true
	}

	haltChain, err = mgr.Keeper().GetMimir(ctx, "NodePauseChainGlobal")
	if err == nil && haltChain > ctx.BlockHeight() {
		ctx.Logger().Info("node global is halt")
		return true
	}

	haltMimirKey := fmt.Sprintf("Halt%sChain", chain)
	haltChain, err = mgr.Keeper().GetMimir(ctx, haltMimirKey)
	if err == nil && (haltChain > 0 && haltChain < ctx.BlockHeight()) {
		ctx.Logger().Info("chain is halt via admin or double-spend check", "chain", chain)
		return true
	}

	solvencyHaltMimirKey := fmt.Sprintf("SolvencyHalt%sChain", chain)
	haltChain, err = mgr.Keeper().GetMimir(ctx, solvencyHaltMimirKey)
	if err == nil && (haltChain > 0 && haltChain < ctx.BlockHeight()) {
		ctx.Logger().Info("chain is halt via solvency check", "chain", chain)
		return true
	}
	return false
}

// isChainHalted check whether the given chain is halt
// chain halt is different as halt trading , when a chain is halt , there is no observation on the given chain
// outbound will not be signed and broadcast
func isChainHaltedV65(ctx cosmos.Context, mgr Manager, chain common.Chain) bool {
	haltChain, err := mgr.Keeper().GetMimir(ctx, "HaltChainGlobal")
	if err == nil && (haltChain > 0 && haltChain < ctx.BlockHeight()) {
		ctx.Logger().Info("global is halt")
		return true
	}

	haltChain, err = mgr.Keeper().GetMimir(ctx, "NodePauseChainGlobal")
	if err == nil && haltChain > ctx.BlockHeight() {
		ctx.Logger().Info("node global is halt")
		return true
	}

	mimirKey := fmt.Sprintf("Halt%sChain", chain)
	haltChain, err = mgr.Keeper().GetMimir(ctx, mimirKey)
	if err == nil && (haltChain > 0 && haltChain < ctx.BlockHeight()) {
		ctx.Logger().Info("chain is halt", "chain", chain)
		return true
	}
	return false
}

func isLPPaused(ctx cosmos.Context, chain common.Chain, mgr Manager) bool {
	version := mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return isLPPausedV1(ctx, chain, mgr)
	}
	return false
}

func isLPPausedV1(ctx cosmos.Context, chain common.Chain, mgr Manager) bool {
	// check if global LP is paused
	pauseLPGlobal, err := mgr.Keeper().GetMimir(ctx, "PauseLP")
	if err == nil && pauseLPGlobal > 0 && pauseLPGlobal < ctx.BlockHeight() {
		return true
	}

	pauseLP, err := mgr.Keeper().GetMimir(ctx, fmt.Sprintf("PauseLP%s", chain))
	if err == nil && pauseLP > 0 && pauseLP < ctx.BlockHeight() {
		ctx.Logger().Info("chain has paused LP actions", "chain", chain)
		return true
	}
	return false
}

// DollarInRune gets the amount of rune that is equal to 1 USD
func DollarInRune(ctx cosmos.Context, mgr Manager) cosmos.Uint {
	// check for mimir override
	dollarInRune, err := mgr.Keeper().GetMimir(ctx, "DollarInRune")
	if err == nil && dollarInRune > 0 {
		return cosmos.NewUint(uint64(dollarInRune))
	}

	busd, _ := common.NewAsset("BNB.BUSD-BD1")
	usdc, _ := common.NewAsset("ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48")
	usdt, _ := common.NewAsset("ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7")
	usdAssets := []common.Asset{busd, usdc, usdt}

	usd := make([]cosmos.Uint, 0)
	for _, asset := range usdAssets {
		if isGlobalTradingHalted(ctx, mgr) || isChainTradingHalted(ctx, mgr, asset.Chain) {
			continue
		}
		pool, err := mgr.Keeper().GetPool(ctx, asset.GetLayer1Asset())
		if err != nil {
			ctx.Logger().Error("fail to get usd pool", "asset", asset.String(), "error", err)
			continue
		}
		if pool.Status != PoolAvailable {
			continue
		}
		value := pool.AssetValueInRune(cosmos.NewUint(common.One))
		if !value.IsZero() {
			usd = append(usd, value)
		}
	}

	if len(usd) == 0 {
		return cosmos.ZeroUint()
	}

	sort.SliceStable(usd, func(i, j int) bool {
		return usd[i].Uint64() < usd[j].Uint64()
	})

	// calculate median of our USD figures
	var median cosmos.Uint
	if len(usd)%2 > 0 {
		// odd number of figures in our slice. Take the middle figure. Since
		// slices start with an index of zero, just need to length divide by two.
		medianSpot := len(usd) / 2
		median = usd[medianSpot]
	} else {
		// even number of figures in our slice. Average the middle two figures.
		pt1 := usd[len(usd)/2-1]
		pt2 := usd[len(usd)/2]
		median = pt1.Add(pt2).QuoUint64(2)
	}
	return median
}

func telem(input cosmos.Uint) float32 {
	if !input.BigInt().IsUint64() {
		return 0
	}
	i := input.Uint64()
	return float32(i) / 100000000
}

func telemInt(input cosmos.Int) float32 {
	if !input.BigInt().IsInt64() {
		return 0
	}
	i := input.Int64()
	return float32(i) / 100000000
}

func emitEndBlockTelemetry(ctx cosmos.Context, mgr Manager) error {
	// capture panics
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("panic while emitting end block telemetry", "error", err)
		}
	}()

	// emit network data
	network, err := mgr.Keeper().GetNetwork(ctx)
	if err != nil {
		return err
	}

	telemetry.SetGauge(telem(network.BondRewardRune), "thornode", "network", "bond_reward_rune")
	telemetry.SetGauge(float32(network.TotalBondUnits.Uint64()), "thornode", "network", "total_bond_units")
	telemetry.SetGauge(telem(network.BurnedBep2Rune), "thornode", "network", "rune", "burned", "bep2")
	telemetry.SetGauge(telem(network.BurnedErc20Rune), "thornode", "network", "rune", "burned", "erc20")

	// emit protocol owned liquidity data
	pol, err := mgr.Keeper().GetPOL(ctx)
	if err != nil {
		return err
	}
	telemetry.SetGauge(telem(pol.RuneDeposited), "thornode", "pol", "rune_deposited")
	telemetry.SetGauge(telem(pol.RuneWithdrawn), "thornode", "pol", "rune_withdrawn")
	telemetry.SetGauge(telemInt(pol.CurrentDeposit()), "thornode", "pol", "current_deposit")
	polValue, err := polPoolValue(ctx, mgr)
	if err == nil {
		telemetry.SetGauge(telem(polValue), "thornode", "pol", "value")
		telemetry.SetGauge(telemInt(pol.PnL(polValue)), "thornode", "pol", "pnl")
	}

	// emit module balances
	for _, name := range []string{ReserveName, AsgardName, BondName} {
		modAddr := mgr.Keeper().GetModuleAccAddress(name)
		bal := mgr.Keeper().GetBalance(ctx, modAddr)
		for _, coin := range bal {
			modLabel := telemetry.NewLabel("module", name)
			denom := telemetry.NewLabel("denom", coin.Denom)
			telemetry.SetGaugeWithLabels(
				[]string{"thornode", "module", "balance"},
				telem(cosmos.NewUint(coin.Amount.Uint64())),
				[]metrics.Label{modLabel, denom},
			)
		}
	}

	// emit node metrics
	yggs := make(Vaults, 0)
	nodes, err := mgr.Keeper().ListValidatorsWithBond(ctx)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if node.Status == NodeActive {
			ygg, err := mgr.Keeper().GetVault(ctx, node.PubKeySet.Secp256k1)
			if err != nil {
				continue
			}
			yggs = append(yggs, ygg)
		}
		telemetry.SetGaugeWithLabels(
			[]string{"thornode", "node", "bond"},
			telem(cosmos.NewUint(node.Bond.Uint64())),
			[]metrics.Label{telemetry.NewLabel("node_address", node.NodeAddress.String()), telemetry.NewLabel("status", node.Status.String())},
		)
		pts, err := mgr.Keeper().GetNodeAccountSlashPoints(ctx, node.NodeAddress)
		if err != nil {
			continue
		}
		telemetry.SetGaugeWithLabels(
			[]string{"thornode", "node", "slash_points"},
			float32(pts),
			[]metrics.Label{telemetry.NewLabel("node_address", node.NodeAddress.String())},
		)

		age := cosmos.NewUint(uint64((ctx.BlockHeight() - node.StatusSince) * common.One))
		if pts > 0 {
			leaveScore := age.QuoUint64(uint64(pts))
			telemetry.SetGaugeWithLabels(
				[]string{"thornode", "node", "leave_score"},
				float32(leaveScore.Uint64()),
				[]metrics.Label{telemetry.NewLabel("node_address", node.NodeAddress.String())},
			)
		}
	}

	// get 1 RUNE price in USD
	runeUSDPrice := 1 / telem(DollarInRune(ctx, mgr))
	telemetry.SetGauge(runeUSDPrice, "thornode", "price", "usd", "thor", "rune")

	// emit pool metrics
	pools, err := mgr.Keeper().GetPools(ctx)
	if err != nil {
		return err
	}
	for _, pool := range pools {
		if pool.LPUnits.IsZero() {
			continue
		}
		synthSupply := mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
		labels := []metrics.Label{telemetry.NewLabel("pool", pool.Asset.String()), telemetry.NewLabel("status", pool.Status.String())}
		telemetry.SetGaugeWithLabels([]string{"thornode", "pool", "balance", "synth"}, telem(synthSupply), labels)
		telemetry.SetGaugeWithLabels([]string{"thornode", "pool", "balance", "rune"}, telem(pool.BalanceRune), labels)
		telemetry.SetGaugeWithLabels([]string{"thornode", "pool", "balance", "asset"}, telem(pool.BalanceAsset), labels)
		telemetry.SetGaugeWithLabels([]string{"thornode", "pool", "pending", "rune"}, telem(pool.PendingInboundRune), labels)
		telemetry.SetGaugeWithLabels([]string{"thornode", "pool", "pending", "asset"}, telem(pool.PendingInboundAsset), labels)

		telemetry.SetGaugeWithLabels([]string{"thornode", "pool", "units", "pool"}, telem(pool.CalcUnits(mgr.GetVersion(), synthSupply)), labels)
		telemetry.SetGaugeWithLabels([]string{"thornode", "pool", "units", "lp"}, telem(pool.LPUnits), labels)
		telemetry.SetGaugeWithLabels([]string{"thornode", "pool", "units", "synth"}, telem(pool.SynthUnits), labels)

		// pricing
		price := float32(0)
		if !pool.BalanceAsset.IsZero() {
			price = runeUSDPrice * telem(pool.BalanceRune) / telem(pool.BalanceAsset)
		}
		telemetry.SetGaugeWithLabels([]string{"thornode", "pool", "price", "usd"}, price, labels)
	}

	// emit vault metrics
	asgards, _ := mgr.Keeper().GetAsgardVaults(ctx)
	for _, vault := range append(asgards, yggs...) {
		if vault.Status != ActiveVault && vault.Status != RetiringVault {
			continue
		}

		// calculate the total value of this yggdrasil vault
		totalValue := cosmos.ZeroUint()
		for _, coin := range vault.Coins {
			if coin.Asset.IsRune() {
				totalValue = totalValue.Add(coin.Amount)
			} else {
				pool, err := mgr.Keeper().GetPool(ctx, coin.Asset.GetLayer1Asset())
				if err != nil {
					continue
				}
				totalValue = totalValue.Add(pool.AssetValueInRune(coin.Amount))
			}
		}
		labels := []metrics.Label{telemetry.NewLabel("vault_type", vault.Type.String()), telemetry.NewLabel("pubkey", vault.PubKey.String())}
		telemetry.SetGaugeWithLabels([]string{"thornode", "vault", "total_value"}, telem(totalValue), labels)

		for _, coin := range vault.Coins {
			labels := []metrics.Label{
				telemetry.NewLabel("vault_type", vault.Type.String()),
				telemetry.NewLabel("pubkey", vault.PubKey.String()),
				telemetry.NewLabel("asset", coin.Asset.String()),
			}
			telemetry.SetGaugeWithLabels([]string{"thornode", "vault", "balance"}, telem(coin.Amount), labels)
		}
	}

	// emit queue metrics
	signingTransactionPeriod := mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
	startHeight := ctx.BlockHeight() - signingTransactionPeriod
	txOutDelayMax, err := mgr.Keeper().GetMimir(ctx, constants.TxOutDelayMax.String())
	if txOutDelayMax <= 0 || err != nil {
		txOutDelayMax = mgr.GetConstants().GetInt64Value(constants.TxOutDelayMax)
	}
	maxTxOutOffset, err := mgr.Keeper().GetMimir(ctx, constants.MaxTxOutOffset.String())
	if maxTxOutOffset <= 0 || err != nil {
		maxTxOutOffset = mgr.GetConstants().GetInt64Value(constants.MaxTxOutOffset)
	}
	query := QueryQueue{
		ScheduledOutboundValue: cosmos.ZeroUint(),
	}
	iterator := mgr.Keeper().GetSwapQueueIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &msg); err != nil {
			continue
		}
		query.Swap++
	}
	for height := startHeight; height <= ctx.BlockHeight(); height++ {
		txs, err := mgr.Keeper().GetTxOut(ctx, height)
		if err != nil {
			continue
		}
		for _, tx := range txs.TxArray {
			if tx.OutHash.IsEmpty() {
				memo, _ := ParseMemo(mgr.GetVersion(), tx.Memo)
				if memo.IsInternal() {
					query.Internal++
				} else if memo.IsOutbound() {
					query.Outbound++
				}
			}
		}
	}
	for height := ctx.BlockHeight() + 1; height <= ctx.BlockHeight()+txOutDelayMax; height++ {
		value, err := mgr.Keeper().GetTxOutValue(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			continue
		}
		if height > ctx.BlockHeight()+maxTxOutOffset && value.IsZero() {
			// we've hit our max offset, and an empty block, we can assume the
			// rest will be empty as well
			break
		}
		query.ScheduledOutboundValue = query.ScheduledOutboundValue.Add(value)
	}
	telemetry.SetGauge(float32(query.Internal), "thornode", "queue", "internal")
	telemetry.SetGauge(float32(query.Outbound), "thornode", "queue", "outbound")
	telemetry.SetGauge(float32(query.Swap), "thornode", "queue", "swap")
	telemetry.SetGauge(telem(query.ScheduledOutboundValue), "thornode", "queue", "scheduled", "value", "rune")
	telemetry.SetGauge(telem(query.ScheduledOutboundValue)*runeUSDPrice, "thornode", "queue", "scheduled", "value", "usd")

	return nil
}

// get the total bond of the bottom 2/3rds active nodes
func getEffectiveSecurityBond(nas NodeAccounts) cosmos.Uint {
	amt := cosmos.ZeroUint()
	sort.SliceStable(nas, func(i, j int) bool {
		return nas[i].Bond.LT(nas[j].Bond)
	})
	t := len(nas) * 2 / 3
	if len(nas)%3 == 0 {
		t -= 1
	}
	for i, na := range nas {
		if i <= t {
			amt = amt.Add(na.Bond)
		}
	}
	return amt
}

// find the bond size the highest of the bottom 2/3rds node bonds
func getHardBondCap(nas NodeAccounts) cosmos.Uint {
	if len(nas) == 0 {
		return cosmos.ZeroUint()
	}
	sort.SliceStable(nas, func(i, j int) bool {
		return nas[i].Bond.LT(nas[j].Bond)
	})
	i := len(nas) * 2 / 3
	if len(nas)%3 == 0 {
		i -= 1
	}
	return nas[i].Bond
}

// In the case where the max gas of the chain of a queued outbound tx has changed
// Update the ObservedTxVoter so the network can still match the outbound with
// the observed inbound
func updateTxOutGas(ctx cosmos.Context, keeper keeper.Keeper, txOut types.TxOutItem, gas common.Gas) error {
	version := keeper.GetVersion()
	if keeper.GetVersion().LT(semver.MustParse("1.90.0")) {
		version = keeper.GetLowestActiveVersion(ctx) // TODO remove me on hard fork
	}
	switch {
	case version.GTE(semver.MustParse("1.88.0")):
		return updateTxOutGasV88(ctx, keeper, txOut, gas)
	case version.GTE(semver.MustParse("0.1.0")):
		return updateTxOutGasV1(ctx, keeper, txOut, gas)
	default:
		return fmt.Errorf("updateTxOutGas: invalid version")
	}
}

func updateTxOutGasV88(ctx cosmos.Context, keeper keeper.Keeper, txOut types.TxOutItem, gas common.Gas) error {
	// When txOut.InHash is 0000000000000000000000000000000000000000000000000000000000000000 , which means the outbound is trigger by the network internally
	// For example , migration , yggdrasil funding etc. there is no related inbound observation , thus doesn't need to try to find it and update anything
	if txOut.InHash == common.BlankTxID {
		return nil
	}
	voter, err := keeper.GetObservedTxInVoter(ctx, txOut.InHash)
	if err != nil {
		return err
	}

	txOutIndex := -1
	for i, tx := range voter.Actions {
		if tx.Equals(txOut) {
			txOutIndex = i
			voter.Actions[txOutIndex].MaxGas = gas
			keeper.SetObservedTxInVoter(ctx, voter)
			break
		}
	}

	if txOutIndex == -1 {
		return fmt.Errorf("fail to find tx out in ObservedTxVoter %s", txOut.InHash)
	}

	return nil
}

// No-op
func updateTxOutGasV1(ctx cosmos.Context, keeper keeper.Keeper, txOut types.TxOutItem, gas common.Gas) error {
	return nil
}
