//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package ethereum

import (
	_ "embed"
)

//go:embed token_list.json
var tokenList []byte
