//go:build !testnet && !mocknet && !stagenet
// +build !testnet,!mocknet,!stagenet

package config

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
)

const (
	rpcPort = 27147
	p2pPort = 27146
)

func getSeedAddrs() (addrs []string) {
	// fetch seeds
	res, err := http.Get("https://api.ninerealms.com/thorchain/seeds")
	if err != nil {
		log.Error().Err(err).Msg("failed to get seeds")
		return
	}

	// unmarshal seeds response
	var seedsResponse []string
	dec := json.NewDecoder(res.Body)
	err = dec.Decode(&seedsResponse)
	if err != nil {
		log.Error().Err(err).Msg("failed to unmarshal seeds response")
	}

	return seedsResponse
}
