//go:build !testnet && !mocknet && !stagenet
// +build !testnet,!mocknet,!stagenet

package avalanche

import (
	"gitlab.com/thorchain/thornode/common"
)

var whitelistSmartContractAddress = []common.Address{}
