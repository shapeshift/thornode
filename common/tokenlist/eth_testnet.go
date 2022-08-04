//go:build testnet || mocknet
// +build testnet mocknet

package tokenlist

import (
	_ "embed"
)

//go:embed eth_testnet_V93.json
var ethTokenListRawV93 []byte

//go:embed eth_testnet_V95.json
var ethTokenListRawV95 []byte
