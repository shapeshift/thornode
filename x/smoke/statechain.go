package smoke

import "fmt"

var endpoints = map[string]string{
	"local": "localhost",
	"stage": "testnet-chain.bepswap.io",
	"dev":   "testnet-chain.bepswap.net",
	"prod":  "testnet-chain.bepswap.com",
}

type Statechain struct {
	Env string
}

// NewStatechain : Create a new Statechain instance.
func NewStatechain(env string) Statechain {
	return Statechain{
		Env: env,
	}
}

// Scheme : SSL or not.
func (s Statechain) scheme() string {
	scheme := "https"

	if s.Env == "local" {
		scheme = "http"
	}

	return scheme
}

// PoolURL : Return the Pool URL based on the selected environment.
func (s Statechain) PoolURL() string {
	return fmt.Sprintf("%v://%v/swapservice/pools", s.scheme(), endpoints[s.Env])
}

// StakerURL  : Return the Staker URL based on the selected environment.
func (s Statechain) StakerURL(staker string) string {
	return fmt.Sprintf("%v://%v/swapservice/staker/%v", s.scheme(), endpoints[s.Env], staker)
}
