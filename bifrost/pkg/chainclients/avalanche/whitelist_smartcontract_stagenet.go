//go:build stagenet
// +build stagenet

package avalanche

import (
	"gitlab.com/thorchain/thornode/common"
)

var whitelistSmartContractAddress = []common.Address{
	// Pangolin Aggregator
	`0x5afcA2485AE7f03158B7cb4558DA79f091b56256`,
}
