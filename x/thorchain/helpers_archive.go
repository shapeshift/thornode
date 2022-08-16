package thorchain

import (
	"errors"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func subsidizePoolWithSlashBondV88(ctx cosmos.Context, ygg Vault, yggTotalStolen, slashRuneAmt cosmos.Uint, mgr Manager) error {
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

		pool, err := mgr.Keeper().GetPool(ctx, coin.Asset)
		if err != nil {
			return err
		}
		f.stolenAsset = f.stolenAsset.Add(coin.Amount)
		runeValue := pool.AssetValueInRune(coin.Amount)
		// the amount of RUNE thorchain used to subsidize the pool is calculate by ratio
		// slashRune * (stealAssetRuneValue /totalStealAssetRuneValue)
		subsidizeAmt := slashRuneAmt.Mul(runeValue).Quo(yggTotalStolen)
		f.subsidiseRune = f.subsidiseRune.Add(subsidizeAmt)
		subsidize = append(subsidize, f)
	}

	for _, f := range subsidize {
		pool, err := mgr.Keeper().GetPool(ctx, f.asset)
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
		if err := mgr.Keeper().SendFromModuleToModule(ctx, BondName, AsgardName, common.NewCoins(runeToAsgard)); err != nil {
			ctx.Logger().Error("fail to send subsidy from bond to asgard", "error", err)
			return err
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

func refundBondV81(ctx cosmos.Context, tx common.Tx, acc cosmos.AccAddress, amt cosmos.Uint, nodeAcc *NodeAccount, mgr Manager) error {
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

	// enforce node operator fee
	// TODO: allow a node to change this while node is in standby. Should also
	// have a "cool down" period where the node cannot churn in for a while to
	// enure bond providers don't get rug pulled of their rewards.
	defaultNodeOperatorFee, err := mgr.Keeper().GetMimir(ctx, constants.NodeOperatorFee.String())
	if defaultNodeOperatorFee <= 0 || err != nil {
		defaultNodeOperatorFee = mgr.GetConstants().GetInt64Value(constants.NodeOperatorFee)
	}
	bp.NodeOperatorFee = cosmos.NewUint(uint64(defaultNodeOperatorFee))

	// backfil bond provider information (passive migration code)
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
	// slashing 1.5 * yggdrasil remains
	slashRune := yggRune.MulUint64(3).QuoUint64(2)
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

func refundBondV88(ctx cosmos.Context, tx common.Tx, acc cosmos.AccAddress, amt cosmos.Uint, nodeAcc *NodeAccount, mgr Manager) error {
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
	// slashing 1.5 * yggdrasil remains
	slashRune := yggRune.MulUint64(3).QuoUint64(2)
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

func subsidizePoolWithSlashBondV74(ctx cosmos.Context, ygg Vault, yggTotalStolen, slashRuneAmt cosmos.Uint, mgr Manager) error {
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

func cyclePoolsV73(ctx cosmos.Context, maxAvailablePools, minRunePoolDepth, stagedPoolCost int64, mgr Manager) error {
	var availblePoolCount int64
	onDeck := NewPool()        // currently staged pool that could get promoted
	choppingBlock := NewPool() // currently available pool that is on the chopping block to being demoted
	minRuneDepth := cosmos.NewUint(uint64(minRunePoolDepth))

	// quick func to check the validity of a pool
	validPool := func(pool Pool) bool {
		if pool.BalanceAsset.IsZero() || pool.BalanceRune.IsZero() || pool.BalanceRune.LT(minRuneDepth) {
			return false
		}
		return true
	}

	// quick func to save a pool status and emit event
	setPool := func(pool Pool) error {
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
		pool.StatusSince = ctx.BlockHeight()
		return mgr.Keeper().SetPool(ctx, pool)
	}

	iterator := mgr.Keeper().GetPoolIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &pool); err != nil {
			return err
		}

		switch pool.Status {
		case PoolAvailable:
			// any available pools that have no asset, no rune, or less than
			// min rune, moves back to staged status
			if validPool(pool) {
				availblePoolCount += 1
			} else if !pool.Asset.IsGasAsset() {
				pool.Status = PoolStaged
				if err := setPool(pool); err != nil {
					return err
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

				if err := mgr.Keeper().AddPoolFeeToReserve(ctx, fee); err != nil {
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

			} else if validPool(pool) && onDeck.BalanceRune.LT(pool.BalanceRune) {
				onDeck = pool
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
		if err := setPool(onDeck); err != nil {
			return err
		}
	}

	if !choppingBlock.IsEmpty() {
		choppingBlock.Status = PoolStaged
		if err := setPool(choppingBlock); err != nil {
			return err
		}
	}

	return nil
}
