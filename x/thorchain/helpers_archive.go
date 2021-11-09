package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

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
