//go:build mocknet
// +build mocknet

package aggregators

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
)

func DexAggregators(version semver.Version) []Aggregator {
	if version.GTE(semver.MustParse("0.1.0")) {
		return []Aggregator{
			// mocknet mock aggregator
			{common.ETHChain, `0x69800327b38A4CeF30367Dec3f64c2f2386f3848`},
		}
	}
	return nil
}
