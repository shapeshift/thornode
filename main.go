package main

import (
	"os"

	"gitlab.com/thorchain/bepswap/observe/x/observer"
	// "gitlab.com/thorchain/bepswap/observe/x/signer"
)

func main() {
	chainHost := os.Getenv("CHAIN_HOST")
	poolAddress := os.Getenv("POOL_ADDRESS")
	dexHost := os.Getenv("DEX_HOST")
	rpcHost := os.Getenv("RPC_HOST")
	runeAddress := os.Getenv("RUNE_ADDRESS")

	// signer.NewSigner(poolAddress, dexHost)
	observer.NewApp(poolAddress, dexHost, rpcHost, chainHost, runeAddress).Start()
	observer.StartWebServer()
}
