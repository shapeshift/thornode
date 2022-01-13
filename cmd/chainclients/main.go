package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	golog "github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	flag "github.com/spf13/pflag"

	"gitlab.com/thorchain/thornode/bifrost/blockscanner"
	"gitlab.com/thorchain/thornode/bifrost/config"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/terra"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/common"
)

func initLog(level string, pretty bool) {
	l, err := zerolog.ParseLevel(level)
	if err != nil {
		log.Warn().Msgf("%s is not a valid log-level, falling back to 'info'", level)
	}
	var out io.Writer = os.Stdout
	if pretty {
		out = zerolog.ConsoleWriter{Out: os.Stdout}
	}
	zerolog.SetGlobalLevel(l)
	log.Logger = log.Output(out).With().Str("service", "bifrost/chainclients").Logger()

	logLevel := golog.LevelInfo
	switch l {
	case zerolog.DebugLevel:
		logLevel = golog.LevelDebug
	case zerolog.InfoLevel:
		logLevel = golog.LevelInfo
	case zerolog.ErrorLevel:
		logLevel = golog.LevelError
	case zerolog.FatalLevel:
		logLevel = golog.LevelFatal
	case zerolog.PanicLevel:
		logLevel = golog.LevelPanic
	}
	golog.SetAllLoggers(logLevel)
	if err := golog.SetLogLevel("tss-lib", level); err != nil {
		log.Fatal().Err(err).Msg("fail to set tss-lib loglevel")
	}
}

func main() {
	logLevel := flag.StringP("log-level", "l", "info", "Log Level")
	pretty := flag.BoolP("pretty-log", "p", false, "Enables unstructured prettified logging. This is useful for local debugging")
	cfgFile := flag.StringP("cfg", "c", "config", "configuration file with extension")
	flag.Parse()

	initLog(*logLevel, *pretty)

	// load configuration file
	cfg, err := config.LoadBiFrostConfig(*cfgFile)
	if err != nil {
		log.Fatal().Err(err).Msg("fail to load config ")
	}

	if len(cfg.Chains) == 0 {
		log.Fatal().Err(err).Msg("missing chains")
		return
	}

	cfg.Thorchain.SignerPasswd = os.Getenv("SIGNER_PASSWD")

	// metrics
	m, err := metrics.NewMetrics(cfg.Metrics)
	if err != nil {
		log.Fatal().Err(err).Msg("fail to create metric instance")
	}
	if err := m.Start(); err != nil {
		log.Fatal().Err(err).Msg("fail to start metric collector")
	}
	if len(cfg.Thorchain.SignerName) == 0 {
		log.Fatal().Msg("signer name is empty")
	}
	if len(cfg.Thorchain.SignerPasswd) == 0 {
		log.Fatal().Msg("signer password is empty")
	}
	kb, _, err := thorclient.GetKeyringKeybase(cfg.Thorchain.ChainHomeFolder, cfg.Thorchain.SignerName, cfg.Thorchain.SignerPasswd)
	if err != nil {
		log.Fatal().Err(err).Msg("fail to get keyring keybase")
	}

	k := thorclient.NewKeysWithKeybase(kb, cfg.Thorchain.SignerName, cfg.Thorchain.SignerPasswd)
	// thorchain bridge
	thorchainBridge, err := thorclient.NewThorchainBridge(cfg.Thorchain, m, k)
	if err != nil {
		log.Fatal().Err(err).Msg("fail to create new thorchain bridge")
	}

	// ensure we have a protocol for chain RPC Hosts
	for _, chainCfg := range cfg.Chains {
		if len(chainCfg.RPCHost) == 0 {
			log.Fatal().Err(err).Msg("missing chain RPC host")
			return
		}
		if !strings.HasPrefix(chainCfg.RPCHost, "http") {
			chainCfg.RPCHost = fmt.Sprintf("http://%s", chainCfg.RPCHost)
		}

		if len(chainCfg.BlockScanner.RPCHost) == 0 {
			log.Fatal().Err(err).Msg("missing chain RPC host")
			return
		}
		if !strings.HasPrefix(chainCfg.BlockScanner.RPCHost, "http") {
			chainCfg.BlockScanner.RPCHost = fmt.Sprintf("http://%s", chainCfg.BlockScanner.RPCHost)
		}

		log.Info().
			Str("chainCfg.RPCHost", chainCfg.RPCHost).
			Str("chainCfg.BlockScanner.RPCHost", chainCfg.BlockScanner.RPCHost).
			Msg("chain cfg")

		if chainCfg.ChainID == common.TERRAChain {

			var path string // if not set later, will in memory storage
			if len(chainCfg.BlockScanner.DBPath) > 0 {
				path = fmt.Sprintf("%s/%s", chainCfg.BlockScanner.DBPath, chainCfg.BlockScanner.ChainID)
			}
			storage, err := blockscanner.NewBlockScannerStorage(path)
			if err != nil {
				log.Fatal().Err(err).Msg("fail to create scan storage")
			}

			cosmosScanner, err := terra.NewCosmosBlockScanner(
				chainCfg.BlockScanner,
				storage,
				thorchainBridge,
				m,
				func(int64) error {
					return nil
				},
			)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to create cosmos block scanner")
			}

			blockScanner, err := blockscanner.NewBlockScanner(chainCfg.BlockScanner, storage, m, thorchainBridge, cosmosScanner)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to create block scanner")
			}
			globalTxsQueue := make(chan types.TxIn)
			go blockScanner.Start(globalTxsQueue)
			for {
				for msg := range globalTxsQueue {
					log.Printf("global tx:\n%s", msg)
				}
			}
		}
	}

}
