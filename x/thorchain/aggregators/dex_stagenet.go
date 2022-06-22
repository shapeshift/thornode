//go:build stagenet
// +build stagenet

package aggregators

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
)

func DexAggregators(version semver.Version) []Aggregator {
	return []Aggregator{
		// THORSwap Wrapped Uniswap V2 Router
		{common.ETHChain, `0xd31f7e39afECEc4855fecc51b693F9A0Cec49fd2`},
		// THORSwap Wrapped Uniswap V3 Router
		{common.ETHChain, `0x2f8aedd149afbdb5206ecaf8b1a3abb9186c8053`},
	}
}
