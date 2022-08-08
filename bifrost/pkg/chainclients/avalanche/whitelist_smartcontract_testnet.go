//go:build testnet || mocknet
// +build testnet mocknet

package avalanche

import (
	"gitlab.com/thorchain/thornode/common"
)

var whitelistSmartContractAddress = []common.Address{`0x1429859428C0aBc9C2C47C8Ee9FBaf82cFA0F20f`}
