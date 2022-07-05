//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package tokenlist

import (
	_ "embed"
)

//go:embed eth_mainnet_V93.json
var ethTokenListRawV93 []byte
