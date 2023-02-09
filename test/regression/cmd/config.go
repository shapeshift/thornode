package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	tmhttp "github.com/tendermint/tendermint/rpc/client/http"
	"gitlab.com/thorchain/thornode/app"
	"gitlab.com/thorchain/thornode/app/params"
	"gitlab.com/thorchain/thornode/cmd"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

////////////////////////////////////////////////////////////////////////////////////////
// Cosmos
////////////////////////////////////////////////////////////////////////////////////////

var (
	encodingConfig params.EncodingConfig
	clientCtx      client.Context
	txFactory      tx.Factory
)

func init() {
	// initialize the bech32 prefix for testnet/mocknet
	config := cosmos.GetConfig()
	config.SetBech32PrefixForAccount("tthor", "tthorpub")
	config.SetBech32PrefixForValidator("tthorv", "tthorvpub")
	config.SetBech32PrefixForConsensusNode("tthorc", "tthorcpub")
	config.Seal()

	// initialize the codec
	encodingConfig = app.MakeEncodingConfig()

	// create new rpc client
	rpcClient, err := tmhttp.New("http://localhost:26657", "/websocket")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create tendermint client")
	}

	// create cosmos-sdk client context
	clientCtx = client.Context{
		Client:            rpcClient,
		ChainID:           "thorchain",
		JSONCodec:         encodingConfig.Marshaler,
		Codec:             encodingConfig.Marshaler,
		InterfaceRegistry: encodingConfig.InterfaceRegistry,
		Keyring:           keyRing,
		BroadcastMode:     flags.BroadcastSync,
		SkipConfirm:       true,
		TxConfig:          encodingConfig.TxConfig,
		AccountRetriever:  authtypes.AccountRetriever{},
		NodeURI:           "http://localhost:26657",
		LegacyAmino:       encodingConfig.Amino,
	}

	// create tx factory
	txFactory = txFactory.WithKeybase(clientCtx.Keyring)
	txFactory = txFactory.WithTxConfig(clientCtx.TxConfig)
	txFactory = txFactory.WithAccountRetriever(clientCtx.AccountRetriever)
	txFactory = txFactory.WithChainID(clientCtx.ChainID)
	txFactory = txFactory.WithGas(1e8)
	txFactory = txFactory.WithSignMode(signing.SignMode_SIGN_MODE_DIRECT)
}

////////////////////////////////////////////////////////////////////////////////////////
// Logging
////////////////////////////////////////////////////////////////////////////////////////

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	log.Logger = log.With().Caller().Logger()

	// set to info level if DEBUG is not set (debug is the default level)
	if os.Getenv("DEBUG") == "" {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

////////////////////////////////////////////////////////////////////////////////////////
// Colors
////////////////////////////////////////////////////////////////////////////////////////

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorPurple = "\033[35m"

	// save for later
	// ColorYellow = "\033[33m"
	// ColorBlue   = "\033[34m"
	// ColorCyan   = "\033[36m"
	// ColorGray   = "\033[37m"
	// ColorWhite  = "\033[97m"
)

////////////////////////////////////////////////////////////////////////////////////////
// HTTP
////////////////////////////////////////////////////////////////////////////////////////

var httpClient = &http.Client{
	Transport: &http.Transport{
		Dial: (&net.Dialer{
			Timeout: time.Second,
		}).Dial,
	},
	Timeout: 3 * time.Second,
}

////////////////////////////////////////////////////////////////////////////////////////
// Thorchain Module Addresses
////////////////////////////////////////////////////////////////////////////////////////

// TODO: determine how to return these programmatically without keeper
const (
	ModuleAddrThorchain    = "tthor1v8ppstuf6e3x0r4glqc68d5jqcs2tf38ulmsrp"
	ModuleAddrAsgard       = "tthor1g98cy3n9mmjrpn0sxmn63lztelera37nrytwp2"
	ModuleAddrBond         = "tthor17gw75axcnr8747pkanye45pnrwk7p9c3uhzgff"
	ModuleAddrTransfer     = "tthor1yl6hdjhmkf37639730gffanpzndzdpmhv07zme"
	ModuleAddrReserve      = "tthor1dheycdevq39qlkxs2a6wuuzyn4aqxhve3hhmlw"
	ModuleAddrFeeCollector = "tthor17xpfvakm2amg962yls6f84z3kell8c5ljftt88"
)

////////////////////////////////////////////////////////////////////////////////////////
// Keys
////////////////////////////////////////////////////////////////////////////////////////

var (
	keyRing         = keyring.NewInMemory()
	addressToName   = map[string]string{} // thor...->dog, 0x...->dog
	templateAddress = map[string]string{} // addr_thor_dog->thor..., addr_eth_dog->0x...
	templatePubKey  = map[string]string{} // pubkey_dog->thorpub...

	dogMnemonic = strings.Repeat("dog ", 23) + "fossil"
	catMnemonic = strings.Repeat("cat ", 23) + "crawl"
	foxMnemonic = strings.Repeat("fox ", 23) + "filter"
	pigMnemonic = strings.Repeat("pig ", 23) + "quick"

	// mnemonics contains the set of all mnemonics for accounts used in tests
	mnemonics = [...]string{
		dogMnemonic,
		catMnemonic,
		foxMnemonic,
		pigMnemonic,
	}
)

func init() {
	// register functions for all mnemonic-chain addresses
	for _, m := range mnemonics {
		name := strings.Split(m, " ")[0]

		// create pubkey for mnemonic
		derivedPriv, err := hd.Secp256k1.Derive()(m, "", cmd.THORChainHDPath)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to derive private key")
		}
		privKey := hd.Secp256k1.Generate()(derivedPriv)
		s, err := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeAccPub, privKey.PubKey())
		if err != nil {
			log.Fatal().Err(err).Msg("failed to bech32ify pubkey")
		}
		pk := common.PubKey(s)

		// add key to keyring
		_, err = keyRing.NewAccount(name, m, "", cmd.THORChainHDPath, hd.Secp256k1)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to add account to keyring")
		}

		for _, chain := range common.AllChains {

			// register template address for all chains
			addr, err := pk.GetAddress(chain)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to get address")
			}
			lowerChain := strings.ToLower(chain.String())
			templateAddress[fmt.Sprintf("addr_%s_%s", lowerChain, name)] = addr.String()

			// register address to name
			addressToName[addr.String()] = name

			// register pubkey for thorchain
			if chain == common.THORChain {
				templatePubKey[fmt.Sprintf("pubkey_%s", name)] = pk.String()
			}
		}
	}
}
