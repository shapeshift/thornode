package thorchain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/armon/go-metrics"
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/telemetry"
	"github.com/hashicorp/go-multierror"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

func refundTx(ctx cosmos.Context, tx ObservedTx, mgr Manager, constAccessor constants.ConstantValues, refundCode uint32, refundReason, nativeRuneModuleName string) error {
	version := mgr.GetVersion()
	if version.GTE(semver.MustParse("0.47.0")) {
		return refundTxV47(ctx, tx, mgr, constAccessor, refundCode, refundReason, nativeRuneModuleName)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return refundTxV1(ctx, tx, mgr, constAccessor, refundCode, refundReason, nativeRuneModuleName)
	}
	return errBadVersion
}

// nativeRuneModuleName will be empty if it is not NATIVE Rune
func refundTxV1(ctx cosmos.Context, tx ObservedTx, mgr Manager, constAccessor constants.ConstantValues, refundCode uint32, refundReason, nativeRuneModuleName string) error {
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
		pool, err := mgr.Keeper().GetPool(ctx, coin.Asset)
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

			success, err := mgr.TxOutStore().TryAddTxOutItem(ctx, mgr, toi)
			if err != nil {
				ctx.Logger().Error("fail to prepare outbund tx", "error", err)
			}
			if success {
				refundCoins = append(refundCoins, toi.Coin)
			}
		}
		// Zombie coins are just dropped.
	}
	if !tx.Tx.Chain.Equals(common.THORChain) && !refundCoins.IsEmpty() {
		if err := addEvent(refundCoins); err != nil {
			return err
		}
	}

	return nil
}

func refundTxV47(ctx cosmos.Context, tx ObservedTx, mgr Manager, constAccessor constants.ConstantValues, refundCode uint32, refundReason, nativeRuneModuleName string) error {
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
		pool, err := mgr.Keeper().GetPool(ctx, coin.Asset)
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

			success, err := mgr.TxOutStore().TryAddTxOutItem(ctx, mgr, toi)
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
	if version.GTE(semver.MustParse("0.46.0")) {
		return subsidizePoolWithSlashBondV46(ctx, ygg, yggTotalStolen, slashRuneAmt, mgr)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return subsidizePoolWithSlashBondV1(ctx, ygg, yggTotalStolen, slashRuneAmt, mgr)
	}
	return errBadVersion
}

func subsidizePoolWithSlashBondV1(ctx cosmos.Context, ygg Vault, yggTotalStolen, slashRuneAmt cosmos.Uint, mgr Manager) error {
	// Thorchain did not slash the node account
	if slashRuneAmt.IsZero() {
		return nil
	}
	stolenRUNE := ygg.GetCoin(common.RuneAsset()).Amount
	slashRuneAmt = common.SafeSub(slashRuneAmt, stolenRUNE)
	yggTotalStolen = common.SafeSub(yggTotalStolen, stolenRUNE)
	type fund struct {
		stolenAsset   cosmos.Uint
		subsidiseRune cosmos.Uint
	}
	// here need to use a map to hold on to the amount of RUNE need to be subsidized to each pool
	// reason being , if ygg pool has both RUNE and BNB coin left, these two coin share the same pool
	// which is BNB pool , if add the RUNE directly back to pool , it will affect BNB price , which will affect the result
	subsidizeAmounts := make(map[common.Asset]fund)
	for _, coin := range ygg.Coins {
		asset := coin.Asset
		if coin.Asset.IsRune() {
			// when the asset is RUNE, thorchain don't need to update the RUNE balance on pool
			continue
		}
		f, ok := subsidizeAmounts[asset]
		if !ok {
			f = fund{
				stolenAsset:   cosmos.ZeroUint(),
				subsidiseRune: cosmos.ZeroUint(),
			}
		}

		pool, err := mgr.Keeper().GetPool(ctx, asset)
		if err != nil {
			return err
		}
		f.stolenAsset = f.stolenAsset.Add(coin.Amount)
		runeValue := pool.AssetValueInRune(coin.Amount)
		// the amount of RUNE thorchain used to subsidize the pool is calculate by ratio
		// slashRune * (stealAssetRuneValue /totalStealAssetRuneValue)
		subsidizeAmt := slashRuneAmt.Mul(runeValue).Quo(yggTotalStolen)
		f.subsidiseRune = f.subsidiseRune.Add(subsidizeAmt)
		subsidizeAmounts[asset] = f
	}

	for asset, f := range subsidizeAmounts {
		pool, err := mgr.Keeper().GetPool(ctx, asset)
		if err != nil {
			return err
		}
		pool.BalanceRune = pool.BalanceRune.Add(f.subsidiseRune)
		pool.BalanceAsset = common.SafeSub(pool.BalanceAsset, f.stolenAsset)

		if err := mgr.Keeper().SetPool(ctx, pool); err != nil {
			return fmt.Errorf("fail to save pool: %w", err)
		}
	}
	return nil
}

func subsidizePoolWithSlashBondV46(ctx cosmos.Context, ygg Vault, yggTotalStolen, slashRuneAmt cosmos.Uint, mgr Manager) error {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to subsidize pool", "error", err)
		}
	}()

	// Thorchain did not slash the node account
	if slashRuneAmt.IsZero() {
		return nil
	}
	stolenRUNE := ygg.GetCoin(common.RuneAsset()).Amount
	slashRuneAmt = common.SafeSub(slashRuneAmt, stolenRUNE)
	yggTotalStolen = common.SafeSub(yggTotalStolen, stolenRUNE)
	type fund struct {
		stolenAsset   cosmos.Uint
		subsidiseRune cosmos.Uint
	}
	// here need to use a map to hold on to the amount of RUNE need to be subsidized to each pool
	// reason being , if ygg pool has both RUNE and BNB coin left, these two coin share the same pool
	// which is BNB pool , if add the RUNE directly back to pool , it will affect BNB price , which will affect the result
	subsidizeAmounts := make(map[common.Asset]fund)
	for _, coin := range ygg.Coins {
		asset := coin.Asset
		if coin.Asset.IsRune() {
			// when the asset is RUNE, thorchain don't need to update the RUNE balance on pool
			continue
		}
		f, ok := subsidizeAmounts[asset]
		if !ok {
			f = fund{
				stolenAsset:   cosmos.ZeroUint(),
				subsidiseRune: cosmos.ZeroUint(),
			}
		}

		pool, err := mgr.Keeper().GetPool(ctx, asset)
		if err != nil {
			return err
		}
		f.stolenAsset = f.stolenAsset.Add(coin.Amount)
		runeValue := pool.AssetValueInRune(coin.Amount)
		// the amount of RUNE thorchain used to subsidize the pool is calculate by ratio
		// slashRune * (stealAssetRuneValue /totalStealAssetRuneValue)
		subsidizeAmt := slashRuneAmt.Mul(runeValue).Quo(yggTotalStolen)
		f.subsidiseRune = f.subsidiseRune.Add(subsidizeAmt)
		subsidizeAmounts[asset] = f
	}

	for asset, f := range subsidizeAmounts {
		pool, err := mgr.Keeper().GetPool(ctx, asset)
		if err != nil {
			return err
		}
		pool.BalanceRune = pool.BalanceRune.Add(f.subsidiseRune)
		pool.BalanceAsset = common.SafeSub(pool.BalanceAsset, f.stolenAsset)

		if err := mgr.Keeper().SetPool(ctx, pool); err != nil {
			return fmt.Errorf("fail to save pool: %w", err)
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

func refundBond(ctx cosmos.Context, tx common.Tx, amt cosmos.Uint, nodeAcc *NodeAccount, mgr Manager) error {
	version := mgr.GetVersion()
	if version.GTE(semver.MustParse("0.46.0")) {
		return refundBondV46(ctx, tx, amt, nodeAcc, mgr)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return refundBondV1(ctx, tx, amt, nodeAcc, mgr)
	}
	return errBadVersion
}

func refundBondV1(ctx cosmos.Context, tx common.Tx, amt cosmos.Uint, nodeAcc *NodeAccount, mgr Manager) error {
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

	// Calculate total value (in rune) the Yggdrasil pool has
	yggRune, err := getTotalYggValueInRune(ctx, mgr.Keeper(), ygg)
	if err != nil {
		return fmt.Errorf("fail to get total ygg value in RUNE: %w", err)
	}

	if nodeAcc.Bond.LT(yggRune) {
		ctx.Logger().Error("Node Account left with more funds in their Yggdrasil vault than their bond's value", "address", nodeAcc.NodeAddress, "ygg-value", yggRune, "bond", nodeAcc.Bond)
	}
	// slashing 1.5 * yggdrasil remains
	slashRune := yggRune.MulUint64(3).QuoUint64(2)
	if slashRune.GT(nodeAcc.Bond) {
		slashRune = nodeAcc.Bond
	}
	bondBeforeSlash := nodeAcc.Bond
	nodeAcc.Bond = common.SafeSub(nodeAcc.Bond, slashRune)

	if !nodeAcc.Bond.IsZero() {
		active, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
		if err != nil {
			ctx.Logger().Error("fail to get active vaults", "error", err)
			return err
		}

		version := mgr.Keeper().GetLowestActiveVersion(ctx)
		constAccessor := constants.GetConstantValues(version)
		signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
		vault := mgr.Keeper().GetLeastSecure(ctx, active, signingTransactionPeriod)
		if vault.IsEmpty() {
			return fmt.Errorf("unable to determine asgard vault to send funds")
		}

		bondEvent := NewEventBond(amt, BondReturned, tx)
		if err := mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
			return fmt.Errorf("fail to emit bond event: %w", err)
		}

		refundAddress := common.Address(nodeAcc.BondAddress.String())

		// refund bond
		txOutItem := TxOutItem{
			Chain:       common.RuneAsset().Chain,
			ToAddress:   refundAddress,
			VaultPubKey: vault.PubKey,
			InHash:      tx.ID,
			Coin:        common.NewCoin(common.RuneAsset(), amt),
			ModuleName:  BondName,
		}
		_, err = mgr.TxOutStore().TryAddTxOutItem(ctx, mgr, txOutItem)
		if err != nil {
			return fmt.Errorf("fail to add outbound tx: %w", err)
		}
	} else {
		// if it get into here that means the node account doesn't have any bond left after slash.
		// which means the real slashed RUNE could be the bond they have before slash
		slashRune = bondBeforeSlash
	}

	nodeAcc.Bond = common.SafeSub(nodeAcc.Bond, amt)
	if err := mgr.Keeper().SetNodeAccount(ctx, *nodeAcc); err != nil {
		ctx.Logger().Error(fmt.Sprintf("fail to save node account(%s)", nodeAcc), "error", err)
		return err
	}
	if err := subsidizePoolWithSlashBond(ctx, ygg, yggRune, slashRune, mgr); err != nil {
		ctx.Logger().Error("fail to subsidize pool with slashed bond", "error", err)
		return err
	}
	// delete the ygg vault, there is nothing left in the ygg vault
	if !ygg.HasFunds() {
		return mgr.Keeper().DeleteVault(ctx, ygg.PubKey)
	}
	return nil
}

func refundBondV46(ctx cosmos.Context, tx common.Tx, amt cosmos.Uint, nodeAcc *NodeAccount, mgr Manager) error {
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

	// Calculate total value (in rune) the Yggdrasil pool has
	yggRune, err := getTotalYggValueInRune(ctx, mgr.Keeper(), ygg)
	if err != nil {
		return fmt.Errorf("fail to get total ygg value in RUNE: %w", err)
	}

	if nodeAcc.Bond.LT(yggRune) {
		ctx.Logger().Error("Node Account left with more funds in their Yggdrasil vault than their bond's value", "address", nodeAcc.NodeAddress, "ygg-value", yggRune, "bond", nodeAcc.Bond)
	}
	// slashing 1.5 * yggdrasil remains
	slashRune := yggRune.MulUint64(3).QuoUint64(2)
	if slashRune.GT(nodeAcc.Bond) {
		slashRune = nodeAcc.Bond
	}
	bondBeforeSlash := nodeAcc.Bond
	nodeAcc.Bond = common.SafeSub(nodeAcc.Bond, slashRune)

	if !nodeAcc.Bond.IsZero() {
		if amt.GT(nodeAcc.Bond) {
			amt = nodeAcc.Bond
		}
		active, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
		if err != nil {
			ctx.Logger().Error("fail to get active vaults", "error", err)
			return err
		}

		version := mgr.Keeper().GetLowestActiveVersion(ctx)
		constAccessor := constants.GetConstantValues(version)
		signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
		vault := mgr.Keeper().GetLeastSecure(ctx, active, signingTransactionPeriod)
		if vault.IsEmpty() {
			return fmt.Errorf("unable to determine asgard vault to send funds")
		}

		bondEvent := NewEventBond(amt, BondReturned, tx)
		if err := mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
			return fmt.Errorf("fail to emit bond event: %w", err)
		}

		refundAddress := common.Address(nodeAcc.BondAddress.String())

		// refund bond
		txOutItem := TxOutItem{
			Chain:       common.RuneAsset().Chain,
			ToAddress:   refundAddress,
			VaultPubKey: vault.PubKey,
			InHash:      tx.ID,
			Coin:        common.NewCoin(common.RuneAsset(), amt),
			ModuleName:  BondName,
		}
		_, err = mgr.TxOutStore().TryAddTxOutItem(ctx, mgr, txOutItem)
		if err != nil {
			return fmt.Errorf("fail to add outbound tx: %w", err)
		}
	} else {
		// if it get into here that means the node account doesn't have any bond left after slash.
		// which means the real slashed RUNE could be the bond they have before slash
		slashRune = bondBeforeSlash
	}

	nodeAcc.Bond = common.SafeSub(nodeAcc.Bond, amt)
	if err := mgr.Keeper().SetNodeAccount(ctx, *nodeAcc); err != nil {
		ctx.Logger().Error(fmt.Sprintf("fail to save node account(%s)", nodeAcc), "error", err)
		return err
	}
	if err := subsidizePoolWithSlashBondV46(ctx, ygg, yggRune, slashRune, mgr); err != nil {
		ctx.Logger().Error("fail to subsidize pool with slashed bond", "error", err)
		return err
	}
	// delete the ygg vault, there is nothing left in the ygg vault
	if !ygg.HasFunds() {
		return mgr.Keeper().DeleteVault(ctx, ygg.PubKey)
	}
	return nil
}

func isSignedByActiveNodeAccounts(ctx cosmos.Context, mgr Manager, signers []cosmos.AccAddress) bool {
	version := mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return isSignedByActiveNodeAccountsV1(ctx, mgr, signers)
	}
	return false
}

func isSignedByActiveNodeAccountsV1(ctx cosmos.Context, mgr Manager, signers []cosmos.AccAddress) bool {
	if len(signers) == 0 {
		return false
	}
	for _, signer := range signers {
		if signer.Equals(mgr.Keeper().GetModuleAccAddress(AsgardName)) {
			continue
		}
		nodeAccount, err := mgr.Keeper().GetNodeAccount(ctx, signer)
		if err != nil {
			ctx.Logger().Error("unauthorized account", "address", signer.String(), "error", err)
			return false
		}
		if nodeAccount.IsEmpty() {
			ctx.Logger().Error("unauthorized account", "address", signer.String())
			return false
		}
		if nodeAccount.Status != NodeActive {
			ctx.Logger().Error("unauthorized account, node account not active", "address", signer.String(), "status", nodeAccount.Status)
			return false
		}
	}
	return true
}

func cyclePools(ctx cosmos.Context, maxAvailablePools, minRunePoolDepth, stagedPoolCost int64, mgr Manager) error {
	version := mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return cyclePoolsV1(ctx, maxAvailablePools, minRunePoolDepth, stagedPoolCost, mgr)
	}
	return errBadVersion
}

func cyclePoolsV1(ctx cosmos.Context, maxAvailablePools, minRunePoolDepth, stagedPoolCost int64, mgr Manager) error {
	var availblePoolCount int64
	onDeck := NewPool()        // currently staged pool that could get promoted
	choppingBlock := NewPool() // currently available pool that is on the chopping block to being demoted
	minRuneDepth := cosmos.NewUint(uint64(minRunePoolDepth))

	// quick func to check the validity of a pool
	valid_pool := func(pool Pool) bool {
		if pool.BalanceAsset.IsZero() || pool.BalanceRune.IsZero() || pool.BalanceRune.LT(minRuneDepth) {
			return false
		}
		return true
	}

	// quick func to save a pool status and emit event
	set_pool := func(pool Pool) error {
		poolEvt := NewEventPool(pool.Asset, pool.Status)
		if err := mgr.EventMgr().EmitEvent(ctx, poolEvt); err != nil {
			return fmt.Errorf("fail to emit pool event: %w", err)
		}

		switch pool.Status {
		case PoolAvailable:
			ctx.Logger().Info("New available pool", "pool", pool.Asset)
		case PoolStaged:
			ctx.Logger().Info("Pool demoted to staged status", "pool", pool.Asset)
		}
		return mgr.Keeper().SetPool(ctx, pool)
	}

	iterator := mgr.Keeper().GetPoolIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		if err := mgr.Keeper().Cdc().UnmarshalBinaryBare(iterator.Value(), &pool); err != nil {
			return err
		}

		switch pool.Status {
		case PoolAvailable:
			// any available pools that have no asset, no rune, or less than
			// min rune, moves back to staged status
			if valid_pool(pool) {
				availblePoolCount += 1
			} else {
				if !pool.Asset.IsGasAsset() {
					pool.Status = PoolStaged
					if err := set_pool(pool); err != nil {
						return err
					}
				}
			}
			if pool.BalanceRune.LT(choppingBlock.BalanceRune) || choppingBlock.IsEmpty() {
				// omit pools that are gas assets from being on the chopping
				// block, removing these pool requires a chain ragnarok, and
				// cannot be handled individually
				if !pool.Asset.IsGasAsset() {
					choppingBlock = pool
				}
			}
		case PoolStaged:
			// deduct staged pool rune fee
			fee := cosmos.NewUint(uint64(stagedPoolCost))
			if fee.GT(pool.BalanceRune) {
				fee = pool.BalanceRune
			}
			if !fee.IsZero() {
				pool.BalanceRune = common.SafeSub(pool.BalanceRune, fee)
				if err := mgr.Keeper().SetPool(ctx, pool); err != nil {
					ctx.Logger().Error("fail to save pool", "pool", pool.Asset, "err", err)
				}

				if err := mgr.Keeper().AddFeeToReserve(ctx, fee); err != nil {
					ctx.Logger().Error("fail to add rune to reserve", "from pool", pool.Asset, "err", err)
				}

				emitPoolBalanceChangedEvent(ctx,
					NewPoolMod(pool.Asset, fee, false, cosmos.ZeroUint(), false),
					"pool stage cost",
					mgr)
			}
			// check if the rune balance is zero, and asset balance IS NOT
			// zero. This is because we don't want to abandon a pool that is in
			// the process of being created (race condition). We can safely
			// assume, if a pool has asset, but no rune, it should be
			// abandoned.
			if pool.BalanceRune.IsZero() && !pool.BalanceAsset.IsZero() {
				// the staged pool no longer has any rune, abandon the pool
				// and liquidity provider, and burn the asset (via zero'ing
				// the vaults for the asset, and churning away from the
				// tokens)
				ctx.Logger().Info("burning pool", "pool", pool.Asset)

				// remove LPs
				removeLiquidityProviders(ctx, pool.Asset, mgr)

				// delete the pool
				mgr.Keeper().RemovePool(ctx, pool.Asset)

				poolEvent := NewEventPool(pool.Asset, PoolSuspended)
				if err := mgr.EventMgr().EmitEvent(ctx, poolEvent); err != nil {
					ctx.Logger().Error("fail to emit pool event", "error", err)
				}
				// remove asset from Vault
				removeAssetFromVault(ctx, pool.Asset, mgr)

			} else {
				if valid_pool(pool) && onDeck.BalanceRune.LT(pool.BalanceRune) {
					onDeck = pool
				}
			}
		}
	}

	if availblePoolCount >= maxAvailablePools {
		// if we've hit our max available pools, and the onDeck pool is less
		// than the chopping block pool, then we do make no changes, by
		// resetting the variables
		if onDeck.BalanceRune.LTE(choppingBlock.BalanceRune) {
			onDeck = NewPool()        // reset
			choppingBlock = NewPool() // reset
		}
	} else {
		// since we haven't hit the max number of available pools, there is no
		// available pool on the chopping block
		choppingBlock = NewPool() // reset
	}

	if !onDeck.IsEmpty() {
		onDeck.Status = PoolAvailable
		if err := set_pool(onDeck); err != nil {
			return err
		}
	}

	if !choppingBlock.IsEmpty() {
		choppingBlock.Status = PoolStaged
		if err := set_pool(choppingBlock); err != nil {
			return err
		}
	}

	return nil
}
func removeAssetFromVault(ctx cosmos.Context, asset common.Asset, mgr Manager) {
	// zero vaults with the pool asset
	vaultIter := mgr.Keeper().GetVaultIterator(ctx)
	defer vaultIter.Close()
	for ; vaultIter.Valid(); vaultIter.Next() {
		var vault Vault
		if err := mgr.Keeper().Cdc().UnmarshalBinaryBare(vaultIter.Value(), &vault); err != nil {
			ctx.Logger().Error("fail to unmarshal vault", "error", err)
			continue
		}
		if vault.HasAsset(asset) {
			for i, coin := range vault.Coins {
				if asset.Equals(coin.Asset) {
					vault.Coins[i].Amount = cosmos.ZeroUint()
					if err := mgr.Keeper().SetVault(ctx, vault); err != nil {
						ctx.Logger().Error("fail to save vault", "error", err)
					}
					break
				}
			}
		}
	}
}
func removeLiquidityProviders(ctx cosmos.Context, asset common.Asset, mgr Manager) {
	iterator := mgr.Keeper().GetLiquidityProviderIterator(ctx, asset)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var lp LiquidityProvider
		if err := mgr.Keeper().Cdc().UnmarshalBinaryBare(iterator.Value(), &lp); err != nil {
			ctx.Logger().Error("fail to unmarshal liquidity provider", "error", err)
			continue
		}
		withdrawEvt := NewEventWithdraw(
			asset,
			lp.Units,
			int64(0),
			cosmos.ZeroDec(),
			common.Tx{},
			cosmos.ZeroUint(),
			cosmos.ZeroUint(),
			cosmos.ZeroUint(),
		)
		if err := mgr.EventMgr().EmitEvent(ctx, withdrawEvt); err != nil {
			ctx.Logger().Error("fail to emit pool withdraw event", "error", err)
		}
		mgr.Keeper().RemoveLiquidityProvider(ctx, lp)
	}
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
	if version.GTE(semver.MustParse("0.63.0")) {
		return isTradingHaltV63(ctx, msg, mgr)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return isTradingHaltV1(ctx, msg, mgr)
	}
	return false
}

func isTradingHaltV1(ctx cosmos.Context, msg cosmos.Msg, mgr Manager) bool {
	if isGlobalTradingHalted(ctx, mgr) {
		return true
	}
	var targetChain common.Chain
	switch m := msg.(type) {
	case *MsgSwap:
		sourceChain := m.Tx.Chain
		// check the source chain is halted or not
		if isChainTradingHalted(ctx, mgr, sourceChain) {
			return true
		}
		if m.TargetAsset.IsSyntheticAsset() {
			targetChain = m.TargetAsset.GetLayer1Asset().Chain
		} else {
			targetChain = m.TargetAsset.GetChain()
		}
	case *MsgAddLiquidity:
		targetChain = m.Asset.GetChain()
	default:
		return false
	}
	return isChainTradingHalted(ctx, mgr, targetChain)

}

func isTradingHaltV63(ctx cosmos.Context, msg cosmos.Msg, mgr Manager) bool {
	if isGlobalTradingHalted(ctx, mgr) {
		return true
	}
	switch m := msg.(type) {
	case *MsgSwap:
		source := common.EmptyChain
		if len(m.Tx.Coins) > 0 {
			source = m.Tx.Coins[0].Asset.GetLayer1Asset().Chain
		}
		target := m.TargetAsset.GetLayer1Asset().Chain
		return isChainTradingHalted(ctx, mgr, source) || isChainTradingHalted(ctx, mgr, target)
	case *MsgAddLiquidity:
		return isChainTradingHalted(ctx, mgr, m.Asset.Chain)
	default:
		return false
	}
}

// isGlobalTradingHalted check whether trading has been halt at global level
func isGlobalTradingHalted(ctx cosmos.Context, mgr Manager) bool {
	haltTrading, err := mgr.Keeper().GetMimir(ctx, "HaltTrading")
	if err == nil && ((haltTrading > 0 && haltTrading < common.BlockHeight(ctx)) || mgr.Keeper().RagnarokInProgress(ctx)) {
		return true
	}
	return false
}

// isChainTradingHalted check whether trading on the given chain is halted
func isChainTradingHalted(ctx cosmos.Context, mgr Manager, chain common.Chain) bool {
	mimirKey := fmt.Sprintf("Halt%sTrading", chain)
	haltChainTrading, err := mgr.Keeper().GetMimir(ctx, mimirKey)
	if err == nil && (haltChainTrading > 0 && haltChainTrading < common.BlockHeight(ctx)) {
		ctx.Logger().Info("trading is halt", "chain", chain)
		return true
	}
	// further to check whether the chain is halted
	return isChainHalted(ctx, mgr, chain)
}

// isChainHalted check whether the given chain is halt
// chain halt is different as halt trading , when a chain is halt , there is no observation on the given chain
// outbound will not be signed and broadcast
func isChainHalted(ctx cosmos.Context, mgr Manager, chain common.Chain) bool {
	mimirKey := fmt.Sprintf("Halt%sChain", chain)
	haltChain, err := mgr.Keeper().GetMimir(ctx, mimirKey)
	if err == nil && (haltChain > 0 && haltChain < common.BlockHeight(ctx)) {
		ctx.Logger().Info("chain is halt", "chain", chain)
		return true
	}
	return false
}

func telem(input cosmos.Uint) float32 {
	i := input.Uint64()
	return float32(i / 100000000)
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
	nodes, err := mgr.Keeper().ListNodeAccountsWithBond(ctx)
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

		age := cosmos.NewUint(uint64((common.BlockHeight(ctx) - node.StatusSince) * common.One))
		if pts > 0 {
			leaveScore := age.QuoUint64(uint64(pts))
			telemetry.SetGaugeWithLabels(
				[]string{"thornode", "node", "leave_score"},
				float32(leaveScore.Uint64()),
				[]metrics.Label{telemetry.NewLabel("node_address", node.NodeAddress.String())},
			)
		}
	}

	// get rune price
	busd, _ := common.NewAsset("BNB.BUSD-BD1")
	pool, err := mgr.Keeper().GetPool(ctx, busd)
	if err != nil {
		return err
	}
	runeUSDPrice := float32(0)
	if !pool.IsEmpty() && !pool.BalanceRune.IsZero() {
		runeUSDPrice = telem(pool.BalanceAsset) / telem(pool.BalanceRune)
		telemetry.SetGauge(runeUSDPrice, "thornode", "price", "usd", "thor", "rune")
	}

	// emit pool metrics
	pools, err := mgr.Keeper().GetPools(ctx)
	if err != nil {
		return err
	}
	for _, pool := range pools {
		synthSupply := mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
		labels := []metrics.Label{telemetry.NewLabel("pool", pool.Asset.String())}
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
		telemetry.SetGauge(price, "thornode", "price", "usd", strings.ToLower(pool.Asset.Chain.String()), strings.ToLower(pool.Asset.Symbol.String()))
	}

	// emit vault metrics
	asgards, err := mgr.Keeper().GetAsgardVaults(ctx)
	if err != nil {

	}
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
				pool, err := mgr.Keeper().GetPool(ctx, coin.Asset)
				if err != nil {
					continue
				}
				totalValue = totalValue.Add(pool.AssetValueInRune(coin.Amount))
			}
		}
		labels := []metrics.Label{telemetry.NewLabel("vault_type", vault.Type.String()), telemetry.NewLabel("pubkey", vault.PubKey.String())}
		telemetry.SetGaugeWithLabels([]string{"thornode", "vault", "total_value"}, telem(totalValue), labels)

		for _, coin := range vault.Coins {
			telemetry.SetGaugeWithLabels([]string{"thornode", "vault", "balance", coin.Asset.String()}, telem(coin.Amount), labels)
		}
	}

	// emit queue metrics
	signingTransactionPeriod := mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
	startHeight := common.BlockHeight(ctx) - signingTransactionPeriod
	query := QueryQueue{}
	iterator := mgr.Keeper().GetSwapQueueIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := mgr.Keeper().Cdc().UnmarshalBinaryBare(iterator.Value(), &msg); err != nil {
			continue
		}
		query.Swap++
	}
	for height := startHeight; height <= common.BlockHeight(ctx); height++ {
		txs, err := mgr.Keeper().GetTxOut(ctx, height)
		if err != nil {
			continue
		}
		for _, tx := range txs.TxArray {
			if tx.OutHash.IsEmpty() {
				memo, _ := ParseMemo(tx.Memo)
				if memo.IsInternal() {
					query.Internal++
				} else if memo.IsOutbound() {
					query.Outbound++
				}
			}
		}
	}
	telemetry.SetGauge(float32(query.Internal), "thornode", "queue", "internal")
	telemetry.SetGauge(float32(query.Outbound), "thornode", "queue", "outbound")
	telemetry.SetGauge(float32(query.Swap), "thornode", "queue", "swap")

	return nil
}
