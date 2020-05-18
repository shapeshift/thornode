package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// Calculate pool deficit based on the pool's accrued fees compared with total fees.
func calcPoolDeficit(stakerDeficit, totalFees, poolFees cosmos.Uint) cosmos.Uint {
	return common.GetShare(poolFees, totalFees, stakerDeficit)
}

// Calculate the block rewards that bonders and stakers should receive
func calcBlockRewards(totalStaked, totalBonded, totalReserve, totalLiquidityFees cosmos.Uint, emissionCurve, blocksPerYear int64) (cosmos.Uint, cosmos.Uint, cosmos.Uint) {
	// Block Rewards will take the latest reserve, divide it by the emission
	// curve factor, then divide by blocks per year
	trD := cosmos.NewDec(int64(totalReserve.Uint64()))
	ecD := cosmos.NewDec(emissionCurve)
	bpyD := cosmos.NewDec(blocksPerYear)
	blockRewardD := trD.Quo(ecD).Quo(bpyD)
	blockReward := cosmos.NewUint(uint64((blockRewardD).RoundInt64()))

	systemIncome := blockReward.Add(totalLiquidityFees)                 // Get total system income for block
	stakerSplit := getPoolShare(totalStaked, totalBonded, systemIncome) // Get staker share
	bonderSplit := common.SafeSub(systemIncome, stakerSplit)            // Remainder to Bonders

	stakerDeficit := cosmos.ZeroUint()
	poolReward := cosmos.ZeroUint()

	if stakerSplit.GTE(totalLiquidityFees) {
		// Stakers have not been paid enough already, pay more
		poolReward = common.SafeSub(stakerSplit, totalLiquidityFees) // Get how much to divert to add to staker split
	} else {
		// Stakers have been paid too much, calculate deficit
		stakerDeficit = common.SafeSub(totalLiquidityFees, stakerSplit) // Deduct existing income from split
	}

	return bonderSplit, poolReward, stakerDeficit
}

func getPoolShare(totalStaked, totalBonded, totalRewards cosmos.Uint) cosmos.Uint {
	// Targets a linear change in rewards from 0% staked, 33% staked, 100% staked.
	// 0% staked: All rewards to stakers
	// 33% staked: 33% to stakers
	// 100% staked: All rewards to Bonders

	if totalStaked.GTE(totalBonded) { // Zero payments to stakers when staked == bonded
		return cosmos.ZeroUint()
	}
	factor := totalBonded.Add(totalStaked).Quo(common.SafeSub(totalBonded, totalStaked)) // (y + x) / (y - x)
	return totalRewards.Quo(factor)
}
