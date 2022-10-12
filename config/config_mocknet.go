//go:build testnet || mocknet
// +build testnet mocknet

package config

const (
	rpcPort = 26657
	p2pPort = 26656
)

func getSeedAddrs() []string {
	return []string{}
}
