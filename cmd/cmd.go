// +build !testnet,!mocknet

package cmd

const (
	Bech32PrefixAccAddr  = "thor"
	Bech32PrefixAccPub   = "thorpub"
	Bech32PrefixValAddr  = "thorv"
	Bech32PrefixValPub   = "thorvpub"
	Bech32PrefixConsAddr = "thorc"
	Bech32PrefixConsPub  = "thorcpub"
	DenomRegex           = `[a-zA-Z][a-zA-Z0-9:\\/\\\-\\_\\.]{2,127}`
)
