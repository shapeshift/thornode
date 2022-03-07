//go:build stagenet
// +build stagenet

package cmd

const (
	Bech32PrefixAccAddr         = "sthor"
	Bech32PrefixAccPub          = "sthorpub"
	Bech32PrefixValAddr         = "sthorv"
	Bech32PrefixValPub          = "sthorvpub"
	Bech32PrefixConsAddr        = "sthorc"
	Bech32PrefixConsPub         = "sthorcpub"
	DenomRegex                  = `[a-zA-Z][a-zA-Z0-9:\\/\\\-\\_\\.]{2,127}`
	THORChainCoinType    uint32 = 931
	THORChainCoinPurpose uint32 = 44
	THORChainHDPath      string = `m/44'/931'/0'/0/0`
)
