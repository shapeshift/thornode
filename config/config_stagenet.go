//go:build stagenet
// +build stagenet

package config

const (
	rpcPort = 27147
	p2pPort = 27146
)

func getSeedAddrs() (addrs []string) {
	return []string{"stagenet-seed.ninerealms.com"}
}
