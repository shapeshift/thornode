package thorchain

import (
	"sort"
	"strconv"
	"strings"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// SwapQv1 is going to manage the swaps queue
type SwapQv1 struct {
	k keeper.Keeper
}

type swapItem struct {
	index int
	msg   MsgSwap
	fee   cosmos.Uint
	slip  cosmos.Uint
}
type swapItems []swapItem

// NewSwapQv1 create a new vault manager
func NewSwapQv1(k keeper.Keeper) *SwapQv1 {
	return &SwapQv1{k: k}
}

// FetchQueue - grabs all swap queue items from the kvstore and returns them
func (vm *SwapQv1) FetchQueue(ctx cosmos.Context) (swapItems, error) {
	items := make(swapItems, 0)
	iterator := vm.k.GetSwapQueueIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := vm.k.Cdc().UnmarshalBinaryBare(iterator.Value(), &msg); err != nil {
			ctx.Logger().Error("fail to fetch swap msg from queue", "error", err)
			continue
		}

		ss := strings.Split(string(iterator.Key()), "-")
		i, err := strconv.Atoi(ss[len(ss)-1])
		if err != nil {
			ctx.Logger().Error("fail to parse swap queue msg index", "key", iterator.Key(), "error", err)
			continue
		}

		items = append(items, swapItem{
			msg:   msg,
			index: i,
			fee:   cosmos.ZeroUint(),
			slip:  cosmos.ZeroUint(),
		})
	}

	return items, nil
}

// EndBlock trigger the real swap to be processed
func (vm *SwapQv1) EndBlock(ctx cosmos.Context, mgr Manager, version semver.Version, constAccessor constants.ConstantValues) error {
	handler := NewSwapHandler(vm.k, mgr)

	minSwapsPerBlock, err := vm.k.GetMimir(ctx, constants.MinSwapsPerBlock.String())
	if minSwapsPerBlock < 0 || err != nil {
		minSwapsPerBlock = constAccessor.GetInt64Value(constants.MinSwapsPerBlock)
	}
	maxSwapsPerBlock, err := vm.k.GetMimir(ctx, constants.MaxSwapsPerBlock.String())
	if maxSwapsPerBlock < 0 || err != nil {
		maxSwapsPerBlock = constAccessor.GetInt64Value(constants.MaxSwapsPerBlock)
	}

	swaps, err := vm.FetchQueue(ctx)
	if err != nil {
		ctx.Logger().Error("fail to fetch swap queue from store", "error", err)
		return err
	}

	swaps, err = vm.scoreMsgs(ctx, swaps)
	if err != nil {
		ctx.Logger().Error("fail to fetch swap items", "error", err)
		// continue, don't exit, just do them out of order (instead of not at all)
	}
	swaps = swaps.Sort()

	for i := int64(0); i < vm.getTodoNum(int64(len(swaps)), minSwapsPerBlock, maxSwapsPerBlock); i++ {
		pick := swaps[i]
		_, err := handler.handle(ctx, pick.msg, version, constAccessor)
		if err != nil {
			ctx.Logger().Error("fail to swap", "msg", pick.msg.Tx.String(), "error", err)
			if newErr := refundTx(ctx, ObservedTx{Tx: pick.msg.Tx}, mgr, vm.k, constAccessor, CodeSwapFail, err.Error(), ""); nil != newErr {
				ctx.Logger().Error("fail to refund swap", "error", err)
			}
		}
		vm.k.RemoveSwapQueueItem(ctx, pick.msg.Tx.ID, pick.index)
	}
	return nil
}

// getTodoNum - determine how many swaps to do.
func (vm *SwapQv1) getTodoNum(queueLen, minSwapsPerBlock, maxSwapsPerBlock int64) int64 {
	// Do half the length of the queue. Unless...
	//	1. The queue length is greater than maxSwapsPerBlock
	//  2. The queue legnth is less than minSwapsPerBlock
	todo := queueLen / 2
	if minSwapsPerBlock >= queueLen {
		todo = queueLen
	}
	if maxSwapsPerBlock < todo {
		todo = maxSwapsPerBlock
	}
	return todo
}

// scoreMsgs - this takes a list of MsgSwap, and converts them to a scored
// swapItem list
func (vm *SwapQv1) scoreMsgs(ctx cosmos.Context, items swapItems) (swapItems, error) {
	pools := make(map[common.Asset]Pool, 0)

	for i, item := range items {
		if _, ok := pools[item.msg.TargetAsset]; !ok {
			var err error
			pools[item.msg.TargetAsset], err = vm.k.GetPool(ctx, item.msg.TargetAsset)
			if err != nil {
				ctx.Logger().Error("fail to get pool", "pool", item.msg.TargetAsset, "error", err)
				continue
			}
		}

		pool := pools[item.msg.TargetAsset]
		if pool.IsEmpty() || !pool.IsAvailable() || pool.BalanceRune.IsZero() || pool.BalanceAsset.IsZero() {
			continue
		}

		sourceCoin := item.msg.Tx.Coins[0]

		// Get our X, x, Y values
		var X, x, Y cosmos.Uint
		x = sourceCoin.Amount
		if sourceCoin.Asset.IsRune() {
			X = pool.BalanceRune
			Y = pool.BalanceAsset
		} else {
			Y = pool.BalanceRune
			X = pool.BalanceAsset
		}

		items[i].fee = calcLiquidityFee(X, x, Y)
		if sourceCoin.Asset.IsRune() {
			items[i].fee = pool.AssetValueInRune(item.fee)
		}
		items[i].slip = calcSwapSlip(X, x)
	}

	return items, nil
}

func (items swapItems) Sort() swapItems {
	// sort by liquidity fee , descending
	byFee := items
	sort.SliceStable(byFee, func(i, j int) bool {
		return byFee[i].fee.GT(byFee[j].fee)
	})

	// sort by slip fee , descending
	bySlip := items
	sort.SliceStable(bySlip, func(i, j int) bool {
		return bySlip[i].slip.GT(bySlip[j].slip)
	})

	type score struct {
		msg   MsgSwap
		score int
	}

	// add liquidity fee score
	scores := make([]score, len(items))
	for i, item := range byFee {
		scores[i] = score{
			msg:   item.msg,
			score: i,
		}
	}

	// add slip score
	for i, item := range bySlip {
		for j, score := range scores {
			if score.msg.Tx.ID.Equals(item.msg.Tx.ID) {
				scores[j].score += i
				break
			}
		}
	}

	// This sorted appears to sort twice, but actually the first sort informs
	// the second. If we have multiple swaps with the same score, it will use
	// the ID sort to deterministically sort within the same score

	// sort by ID, first
	sort.SliceStable(scores, func(i, j int) bool {
		return scores[i].msg.Tx.ID.String() < scores[j].msg.Tx.ID.String()
	})

	// sort by score, second
	sort.SliceStable(scores, func(i, j int) bool {
		return scores[i].score < scores[j].score
	})

	// sort our items by score
	sorted := make(swapItems, len(items))
	for i, score := range scores {
		for _, item := range items {
			if item.msg.Tx.ID.Equals(score.msg.Tx.ID) {
				sorted[i] = item
				break
			}
		}
	}

	return sorted
}
