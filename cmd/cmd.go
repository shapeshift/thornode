//go:build !testnet && !mocknet && !stagenet
// +build !testnet,!mocknet,!stagenet

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
	THORChainCoinPurpose uint32 = 44
	THORChainHDPath      string = `m/44'/931'/0'/0/0`
)
