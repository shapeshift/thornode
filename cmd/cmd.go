//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package cmd

const (
	Bech32PrefixAccAddr         = "thor"
	Bech32PrefixAccPub          = "thorpub"
	Bech32PrefixValAddr         = "thorv"
	Bech32PrefixValPub          = "thorvpub"
	Bech32PrefixConsAddr        = "thorc"
	Bech32PrefixConsPub         = "thorcpub"
	DenomRegex                  = `[a-zA-Z][a-zA-Z0-9:\\/\\\-\\_\\.]{2,127}`
	THORChainCoinType    uint32 = 931
	THORChainHDPath      string = `m/44'/931'/0'/0/0`
)
