package cosmos

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
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
	cfg    config.BlockScannerConfiguration
	logger zerolog.Logger
	db     blockscanner.ScannerStorage
	// m                *metrics.Metrics
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
	block, err := b.GetBlock(height)
	if err != nil {
		return types.TxIn{}, err
	}

	rawTxs := make([]string, len(block.Data.Txs))
	for _, tx := range block.Data.Txs {
		fmt.Println(string(tx))
	}

	_ = blockscanner.Block{Height: block.Header.Height, Txs: rawTxs}

	// txIn, err := b.GetTxInFromBlock(scanBlock, block.Block.Data.Txs)
	// if err != nil {
	// 	if errStatus := b.db.SetBlockScanStatus(scanBlock, blockscanner.Failed); errStatus != nil {
	// 		b.errCounter.WithLabelValues("fail_set_block_status", "").Inc()
	// 		b.logger.Error().Err(err).Int64("height", scanBlock.Height).Msg("failed to set block to fail status")
	// 	}

	// 	b.logger.Error().Err(err).Int64("height", scanBlock.Height).Msg("failed convert block txs")
	// 	return txIn, err
	// }

	// // mark the block as successful
	// if err := b.db.RemoveBlockStatus(scanBlock.Height); err != nil {
	// 	b.errCounter.WithLabelValues("fail_remove_block_status", "").Inc()
	// 	b.logger.Error().Err(err).Int64("block", scanBlock.Height).Msg("failed to remove block status from data store")
	// }

	return types.TxIn{}, nil
}
