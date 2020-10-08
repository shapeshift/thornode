package thorchain

import (
	"errors"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	kvTypes "gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
)

type YggMgrV1 struct {
	keeper keeper.Keeper
}

func NewYggMgrV1(keeper keeper.Keeper) *YggMgrV1 {
	return &YggMgrV1{
		keeper: keeper,
	}
}

// Fund is a method to fund yggdrasil pool
func (ymgr YggMgrV1) Fund(ctx cosmos.Context, mgr Manager, constAccessor constants.ConstantValues) error {
	// Check if we have triggered the ragnarok protocol
	ragnarokHeight, err := ymgr.keeper.GetRagnarokBlockHeight(ctx)
	if err != nil {
		return fmt.Errorf("fail to get ragnarok height: %w", err)
	}
	if ragnarokHeight > 0 {
		return nil
	}

	// Check we're not migrating funds
	retiring, err := ymgr.keeper.GetAsgardVaultsByStatus(ctx, RetiringVault)
	if err != nil {
		ctx.Logger().Error("fail to get retiring vaults", "error", err)
		return err
	}
	if len(retiring) > 0 {
		// skip yggdrasil funding while a migration is in progress
		return nil
	}

	// find total bonded
	totalBond := cosmos.ZeroUint()
	nodeAccs, err := ymgr.keeper.ListActiveNodeAccounts(ctx)
	if err != nil {
		return err
	}
	minimumNodesForYggdrasil := constAccessor.GetInt64Value(constants.MinimumNodesForYggdrasil)
	if int64(len(nodeAccs)) < minimumNodesForYggdrasil {
		return nil
	}

	// Gather list of all pools
	pools, err := ymgr.keeper.GetPools(ctx)
	if err != nil {
		return err
	}

	for _, na := range nodeAccs {
		totalBond = totalBond.Add(na.Bond)
	}

	// We don't want to check all Yggdrasil pools every time THORNode run this
	// function. So THORNode use modulus to determine which Ygg THORNode process. This
	// should behave as a "round robin" approach checking one Ygg per block.
	// With 100 Ygg pools, THORNode should check each pool every 8.33 minutes.
	na := nodeAccs[common.BlockHeight(ctx)%int64(len(nodeAccs))]

	// check that we have enough bond
	minBond, err := ymgr.keeper.GetMimir(ctx, constants.MinimumBondInRune.String())
	if minBond < 0 || err != nil {
		minBond = constAccessor.GetInt64Value(constants.MinimumBondInRune)
	}
	if na.Bond.LT(cosmos.NewUint(uint64(minBond))) {
		return nil
	}

	// figure out if THORNode need to send them assets.
	// get a list of coin/amounts this yggdrasil pool should have, ideally.
	// TODO: We are assuming here that the pub key is Secp256K1
	ygg, err := ymgr.keeper.GetVault(ctx, na.PubKeySet.Secp256k1)
	if err != nil {
		if !errors.Is(err, kvTypes.ErrVaultNotFound) {
			return fmt.Errorf("fail to get yggdrasil: %w", err)
		}
		ygg = NewVault(common.BlockHeight(ctx), ActiveVault, YggdrasilVault, na.PubKeySet.Secp256k1, nil)
		ygg.Membership = append(ygg.Membership, na.PubKeySet.Secp256k1)

		if err := ymgr.keeper.SetVault(ctx, ygg); err != nil {
			return fmt.Errorf("fail to create yggdrasil pool: %w", err)
		}
	}
	if !ygg.IsYggdrasil() {
		return nil
	}
	pendingTxCount := ygg.LenPendingTxBlockHeights(common.BlockHeight(ctx), constAccessor)
	if pendingTxCount > 0 {
		return fmt.Errorf("cannot send more yggdrasil funds while transactions are pending (%s: %d)", ygg.PubKey, pendingTxCount)
	}

	yggFundLimit, err := ymgr.keeper.GetMimir(ctx, constants.YggFundLimit.String())
	if yggFundLimit < 0 || err != nil {
		yggFundLimit = constAccessor.GetInt64Value(constants.YggFundLimit)
	}
	targetCoins, err := ymgr.calcTargetYggCoins(pools, ygg, na.Bond, totalBond, cosmos.NewUint(uint64(yggFundLimit)))
	if err != nil {
		return err
	}

	var sendCoins common.Coins
	// iterate over each target coin amount and figure if THORNode need to reimburse
	// a Ygg pool of this particular asset.
	for _, targetCoin := range targetCoins {
		yggCoin := ygg.GetCoin(targetCoin.Asset)
		// check if the amount the ygg pool has is less that 50% of what
		// they are suppose to have, ideally. We refill them if they drop
		// below this line
		if yggCoin.Amount.LT(targetCoin.Amount.QuoUint64(2)) {
			sendCoins = append(
				sendCoins,
				common.NewCoin(
					targetCoin.Asset,
					common.SafeSub(targetCoin.Amount, yggCoin.Amount),
				),
			)
		}
	}

	if len(sendCoins) > 0 {
		count, err := ymgr.sendCoinsToYggdrasil(ctx, sendCoins, ygg, mgr)
		if err != nil {
			return err
		}
		for i := 0; i < count; i++ {
			ygg.AppendPendingTxBlockHeights(common.BlockHeight(ctx), constAccessor)
		}
		if err := ymgr.keeper.SetVault(ctx, ygg); err != nil {
			return fmt.Errorf("fail to create yggdrasil pool: %w", err)
		}
	}

	return nil
}

// sendCoinsToYggdrasil - adds outbound txs to send the given coins to a
// yggdrasil pool
func (ymgr YggMgrV1) sendCoinsToYggdrasil(ctx cosmos.Context, coins common.Coins, ygg Vault, mgr Manager) (int, error) {
	var count int

	active, err := ymgr.keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return count, err
	}

	for i := 1; i <= 2; i++ {
		// First iteration (1), we add gas assets. This is to ensure the vault
		// has gas to send transactions as it needs to
		// Second iteration (2), we add non-gas assets
		for _, coin := range coins {
			if i == 1 && !coin.Asset.Chain.GetGasAsset().Equals(coin.Asset) {
				continue
			}
			if i == 2 && coin.Asset.Chain.GetGasAsset().Equals(coin.Asset) {
				continue
			}

			// ignore amount 0
			if coin.Amount.Equal(cosmos.ZeroUint()) {
				continue
			}
			// select active vault to send funds from
			vault := active.SelectByMaxCoin(coin.Asset)
			if vault.IsEmpty() {
				continue
			}
			if coin.Amount.GT(vault.GetCoin(coin.Asset).Amount) {
				// not enough funds
				continue
			}

			to, err := ygg.PubKey.GetAddress(coin.Asset.Chain)
			if err != nil {
				ctx.Logger().Error("fail to get address for pubkey", "pubkey", ygg.PubKey, "chain", coin.Asset.Chain, "error", err)
				continue
			}
			gasCoin, err := mgr.GasMgr().GetMaxGas(ctx, coin.Asset.Chain)
			if err != nil {
				ctx.Logger().Error("fail to get max gas coin", "error", err)
				continue
			}
			toi := &TxOutItem{
				Chain:       coin.Asset.Chain,
				ToAddress:   to,
				InHash:      common.BlankTxID,
				Memo:        NewYggdrasilFund(common.BlockHeight(ctx)).String(),
				Coin:        coin,
				VaultPubKey: vault.PubKey,
				MaxGas: common.Gas{
					gasCoin,
				},
			}

			if err := mgr.TxOutStore().UnSafeAddTxOutItem(ctx, mgr, toi); err != nil {
				return count, err
			}
			count += 1
		}
	}

	return count, nil
}

// calcTargetYggCoins - calculate the amount of coins of each pool a yggdrasil
// pool should have, relative to how much they have bonded (which should be
// target == bond * yggFundLimit / 100).
func (ymgr YggMgrV1) calcTargetYggCoins(pools []Pool, ygg Vault, yggBond, totalBond, yggFundLimit cosmos.Uint) (common.Coins, error) {
	runeCoin := common.NewCoin(common.RuneAsset(), cosmos.ZeroUint())
	var coins common.Coins

	// calculate total staked rune in our pools
	totalStakedRune := cosmos.ZeroUint()
	for _, pool := range pools {
		totalStakedRune = totalStakedRune.Add(pool.BalanceRune)
	}
	if totalStakedRune.IsZero() {
		// if nothing is staked, no coins should be issued
		return nil, nil
	}

	// if we're under bonded, calculate as if we're not. Otherwise, we'll try
	// to send too much funds to ygg vaults
	bondVal := totalBond.MulUint64(2)
	if bondVal.LT(totalStakedRune.MulUint64(4)) {
		bondVal = totalStakedRune.MulUint64(4)
	}
	// figure out what percentage of the bond this yggdrasil pool has. They
	// should get half of that value.
	targetRune := common.GetShare(yggBond, bondVal, totalStakedRune)
	// check if more rune would be allocated to this pool than their bond allows
	if targetRune.GT(yggBond.Mul(yggFundLimit).QuoUint64(100)) {
		targetRune = yggBond.Mul(yggFundLimit).QuoUint64(100)
	}

	// track how much value (in rune) we've associated with this ygg pool. This
	// is here just to be absolutely sure THORNode never send too many assets to the
	// ygg by accident.
	counter := cosmos.ZeroUint()
	for _, pool := range pools {
		runeAmt := common.GetShare(targetRune, totalStakedRune, pool.BalanceRune)
		runeCoin.Amount = runeCoin.Amount.Add(runeAmt)
		assetAmt := common.GetShare(targetRune, totalStakedRune, pool.BalanceAsset)
		// add rune amt (not asset since the two are considered to be equal)
		// in a single pool X, the value of 1% asset X in RUNE ,equals the 1% RUNE in the same pool
		yggCoin := ygg.GetCoin(pool.Asset)
		coin := common.NewCoin(pool.Asset, common.SafeSub(assetAmt, yggCoin.Amount))
		if !coin.IsEmpty() {
			counter = counter.Add(runeAmt)
			coins = append(coins, coin)
		}
	}

	yggRune := ygg.GetCoin(common.RuneAsset())
	runeCoin.Amount = common.SafeSub(runeCoin.Amount, yggRune.Amount)
	if !runeCoin.IsEmpty() {
		counter = counter.Add(runeCoin.Amount)
		coins = append(coins, runeCoin)
	}

	// ensure THORNode don't send too much value in coins to the ygg pool
	if counter.GT(yggBond.Mul(yggFundLimit).QuoUint64(100)) {
		return nil, fmt.Errorf("exceeded safe amounts of assets for given Yggdrasil pool (%d/%d)", counter.Uint64(), yggBond.QuoUint64(2).Uint64())
	}

	return coins, nil
}
