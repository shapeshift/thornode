package cosmos

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"

	ctypes "github.com/cosmos/cosmos-sdk/types"
	btypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	tmtypes "github.com/tendermint/tendermint/proto/tendermint/types"
	grpc "google.golang.org/grpc"

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
	FeeAssetMap = map[string]string{
		"testnet": "umuon",
		"mainnet": "uatom",
	}
	ErrInvalidScanStorage = errors.New("scan storage is empty or nil")
	ErrInvalidMetrics     = errors.New("metrics is empty or nil")
	ErrEmptyTx            = errors.New("empty tx")
)

// CosmosBlockScanner is to scan the blocks
type CosmosBlockScanner struct {
	cfg              config.BlockScannerConfiguration
	logger           zerolog.Logger
	feeAsset         common.Asset
	avgGasFee        common.Coin
	db               blockscanner.ScannerStorage
	cdc              *codec.ProtoCodec
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

	var feeAssetStr string
	if isTestNet {
		feeAssetStr = FeeAssetMap["testnet"]
	} else {
		feeAssetStr = FeeAssetMap["mainnet"]
	}

	feeAsset, err := common.NewAsset(feeAssetStr)
	if err != nil {
		return nil, fmt.Errorf("failed to create asset (%s): %w", feeAssetStr, err)
	}

	registry := bridge.GetContext().InterfaceRegistry
	btypes.RegisterInterfaces(registry)

	host := strings.Replace(cfg.RPCHost, "http://", "", -1)
	conn, err := grpc.Dial(host, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("fail to dial")
	}

	tmService := tmservice.NewServiceClient(conn)
	cdc := codec.NewProtoCodec(registry)

	return &CosmosBlockScanner{
		cfg:              cfg,
		logger:           log.Logger.With().Str("module", "blockscanner").Str("chain", "GAIA").Logger(),
		db:               scanStorage,
		feeAsset:         feeAsset,
		avgGasFee:        common.NewCoin(feeAsset, ctypes.NewUint(0)),
		cdc:              cdc,
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

func (b *CosmosBlockScanner) updateAverageGasFees(height int64, txs []types.TxInItem) error {
	numTxs := int64(len(txs))
	if numTxs == 0 {
		return nil
	}

	// sum all the gas fees for the FeeAsset only
	totalGasFees := ctypes.NewUint(0)
	for _, tx := range txs {
		fee := tx.Gas.ToCoins().GetCoin(b.feeAsset)
		if err := fee.Valid(); err != nil {
			return fmt.Errorf("invalid fee (%s): %w", fee, err)
		}

		totalGasFees = totalGasFees.Add(fee.Amount)
	}

	// compute the average (total / numTxs)
	avgGasFeesAmt := (ctypes.NewDecFromBigInt(totalGasFees.BigInt()).QuoInt64(numTxs)).TruncateInt()
	if avgGasFeesAmt.IsZero() {
		return nil
	}

	if !avgGasFeesAmt.IsUint64() {
		return fmt.Errorf("average gas fee exceeds uint64: %s", avgGasFeesAmt)
	}

	log.Info().Int64("height", height).Int64("gasFeeAmt", avgGasFeesAmt.Int64()).Msg("calculate gas fee")
	// if _, err := b.bridge.PostNetworkFee(height, common.GAIAChain, 1, avgGasFeesAmt.Uint64()); err != nil {
	// 	b.logger.Err(err).Int64("height", height).Msg("failed to post average network fee")
	// 	return err
	// }

	return nil
}

func sdkCoinToCommonCoin(c ctypes.Coin) (common.Coin, error) {
	asset, err := common.NewAsset(c.Denom)
	if err != nil {
		return common.Coin{}, fmt.Errorf("failed to create asset (%s): %w", c.Denom, err)
	}

	return common.NewCoin(asset, ctypes.NewUintFromBigInt(c.Amount.BigInt())), nil
}

func (b *CosmosBlockScanner) FetchTxs(height int64) (types.TxIn, error) {
	block, err := b.GetBlock(height)
	if err != nil {
		return types.TxIn{}, err
	}

	decoder := tx.DefaultTxDecoder(b.cdc)
	var txs []types.TxInItem

	for _, rawTx := range block.Data.Txs {
		tx, err := decoder(rawTx)

		if err != nil {
			if strings.Contains(err.Error(), "unable to resolve type URL") {
				// couldn't find msg type in the interface registry, probably not relevant
				if !strings.Contains(err.Error(), "MsgSend") {
					// double check to make sure MsgSend isn't mentioned
					// if it's not, we can safely ignore
					log.Debug().Str("tx", string(rawTx)).Err(err).Msg("could not decode msg not existing in interface registry")
					continue
				}
			}
			// else we should log this as an error and continue
			log.Error().Str("tx", string(rawTx)).Err(err).Msg("unable to decode msg")
			continue
		}

		memo := tx.(ctypes.TxWithMemo).GetMemo()
		fees := tx.(ctypes.FeeTx).GetFee()

		for _, msg := range tx.GetMsgs() {
			switch msg := msg.(type) {
			case *btypes.MsgSend:
				coins := common.Coins{}
				for _, c := range msg.Amount {
					cCoin, err := sdkCoinToCommonCoin(c)
					if err != nil {
						return types.TxIn{}, fmt.Errorf("failed to create asset; %s is not valid: %w", c, err)
					}

					coins = append(coins, cCoin)
				}

				// ignore the tx when no coins exist
				if coins.IsEmpty() {
					continue
				}

				gasFees := common.Gas{}
				for _, fee := range fees {
					cCoin, err := sdkCoinToCommonCoin(fee)
					if err != nil {
						return types.TxIn{}, fmt.Errorf("failed to create fee asset; %s is not valid: %w", fee, err)
					}

					gasFees = append(gasFees, cCoin)
				}

				txs = append(txs, types.TxInItem{
					Tx:          "",
					BlockHeight: height,
					Memo:        memo,
					Sender:      msg.FromAddress,
					To:          msg.ToAddress,
					Coins:       coins,
					Gas:         gasFees,
				})

			default:
				continue
			}
		}
	}

	txIn := types.TxIn{
		Count:    strconv.Itoa(int(len(txs))),
		Chain:    common.GAIAChain,
		TxArray:  txs,
		Filtered: false,
		MemPool:  false,
	}

	if len(txs) > 0 {
		log.Info().Interface("txIn", txIn).Msg("processed txIn")
	}

	b.updateAverageGasFees(block.Header.Height, txIn.TxArray)
	return txIn, nil
}
