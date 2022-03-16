//go:build testnet || mocknet
// +build testnet mocknet

package cmd

const (
	Bech32PrefixAccAddr         = "tthor"
	Bech32PrefixAccPub          = "tthorpub"
	Bech32PrefixValAddr         = "tthorv"
	Bech32PrefixValPub          = "tthorvpub"
	Bech32PrefixConsAddr        = "tthorc"
	Bech32PrefixConsPub         = "tthorcpub"
	DenomRegex                  = `[a-zA-Z][a-zA-Z0-9:\\/\\\-\\_\\.]{2,127}`
	THORChainCoinType    uint32 = 931
	THORChainCoinPurpose uint32 = 44
	THORChainHDPath      string = `m/44'/931'/0'/0/0`
)
