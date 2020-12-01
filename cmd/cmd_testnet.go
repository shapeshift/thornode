// +build testnet mocknet

package cmd

const (
	Bech32PrefixAccAddr  = "tthor"
	Bech32PrefixAccPub   = "tthorpub"
	Bech32PrefixValAddr  = "tthorv"
	Bech32PrefixValPub   = "tthorvpub"
	Bech32PrefixConsAddr = "tthorc"
	Bech32PrefixConsPub  = "tthorcpub"
	DenomRegex           = `[a-zA-Z][a-zA-Z0-9:\\/\\\-\\_\\.]{2,127}`
)
