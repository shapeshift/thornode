//go:build stagenet
// +build stagenet

package avalanche

import (
	_ "embed"
)

//go:embed token_list_testnet.json
var tokenList []byte
