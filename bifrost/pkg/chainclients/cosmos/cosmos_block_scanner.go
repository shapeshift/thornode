package cosmos

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/cosmos/cosmos-sdk/codec"
	stypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	tmtypes "github.com/tendermint/tendermint/proto/tendermint/types"

	"gitlab.com/thorchain/thornode/bifrost/blockscanner"
	"gitlab.com/thorchain/thornode/bifrost/config"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/common"
)

// SolvencyReporter is to report solvency info to THORNode
type SolvencyReporter func(int64) error

var (
	ErrInvalidScanStorage = errors.New("scan storage is empty or nil")
	ErrInvalidMetrics     = errors.New("metrics is empty or nil")
	ErrEmptyTx            = errors.New("empty tx")
)

// CosmosBlockScanner is to scan the blocks
type CosmosBlockScanner struct {
	cfg              config.BlockScannerConfiguration
	logger           zerolog.Logger
	db               blockscanner.ScannerStorage
	m                *metrics.Metrics
	errCounter       *prometheus.CounterVec
	tmService        tmservice.ServiceClient
	bridge           *thorclient.ThorchainBridge
	solvencyReporter SolvencyReporter
}

// NewCosmosBlockScanner create a new instance of BlockScan
func NewCosmosBlockScanner(cfg config.BlockScannerConfiguration,
	scanStorage blockscanner.ScannerStorage,
	isTestNet bool,
	bridge *thorclient.ThorchainBridge,
	m *metrics.Metrics,
	solvencyReporter SolvencyReporter) (*CosmosBlockScanner, error) {
	if scanStorage == nil {
		return nil, errors.New("scanStorage is nil")
	}
	if m == nil {
		return nil, errors.New("metrics is nil")
	}
	fmt.Printf("interface registry:\n%+v", bridge.GetContext().InterfaceRegistry.ListAllInterfaces())

	clientCtx := bridge.GetContext()
	tmService := tmservice.NewServiceClient(clientCtx)

	return &CosmosBlockScanner{
		cfg:              cfg,
		logger:           log.Logger.With().Str("module", "blockscanner").Str("chain", "GAIA").Logger(),
		db:               scanStorage,
		errCounter:       m.GetCounterVec(metrics.BlockScanError(common.GAIAChain)),
		tmService:        tmService,
		bridge:           bridge,
		solvencyReporter: solvencyReporter,
	}, nil
}

func (b *CosmosBlockScanner) GetHeight() (int64, error) {
	resultHeight, err := b.tmService.GetLatestBlock(
		context.Background(),
		&tmservice.GetLatestBlockRequest{},
	)
	if err != nil {
		return 0, err
	}

	return resultHeight.Block.Header.Height, nil
}

func (c *CosmosBlockScanner) FetchMemPool(height int64) (types.TxIn, error) {
	return types.TxIn{}, nil
}

// GetBlock returns a Tendermint block as a reference to a ResultBlock for a
// given height. An error is returned upon query failure.
func (b *CosmosBlockScanner) GetBlock(height int64) (*tmtypes.Block, error) {
	resultBlock, err := b.tmService.GetBlockByHeight(
		context.Background(),
		&tmservice.GetBlockByHeightRequest{Height: height},
	)

	if err != nil {
		b.logger.Error().Int64("height", height).Msgf("failed to get block: %v", err)
		b.errCounter.WithLabelValues("fail_get_block", strconv.Itoa(int(height))).Inc()

		return nil, fmt.Errorf("failed to get block: %w", err)
	}

	return resultBlock.Block, nil
}

func (b *CosmosBlockScanner) FetchTxs(height int64) (types.TxIn, error) {
	log.Info().Int64("height", height).Msg("FetchTxs")

	block, err := b.GetBlock(height)
	if err != nil {
		return types.TxIn{}, err
	}

	codec := codec.NewProtoCodec(b.bridge.GetContext().InterfaceRegistry)

	// rawTxs := make([]string, len(block.Data.Txs))
	for _, tx := range block.Data.Txs {
		fmt.Printf("raw tx:\n%s", string(tx))
		var msg stypes.Msg
		err = codec.UnmarshalInterface(tx, &msg)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to unpack any")
		}
		log.Info().Interface("msg", msg).Msg("stypes.Msg")
	}

	return types.TxIn{}, nil
}
