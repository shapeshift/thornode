//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package bsctokens

import (
	_ "embed"
)

//go:embed bsc_mainnet_latest.json
var BSCTokenListRawV111 []byte
