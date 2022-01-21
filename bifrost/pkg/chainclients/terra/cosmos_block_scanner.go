package terra

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
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
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmtypes "github.com/tendermint/tendermint/proto/tendermint/types"
	grpc "google.golang.org/grpc"

	"gitlab.com/thorchain/thornode/bifrost/blockscanner"
	"gitlab.com/thorchain/thornode/bifrost/config"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// SolvencyReporter is to report solvency info to THORNode
type SolvencyReporter func(int64) error

var (
	DefaultDecimals       = 6
	WhitelistAssets       = map[string]int{"uluna": 6, "uusd": 6}
	ErrInvalidScanStorage = errors.New("scan storage is empty or nil")
	ErrInvalidMetrics     = errors.New("metrics is empty or nil")
	ErrEmptyTx            = errors.New("empty tx")
)

// CosmosBlockScanner is to scan the blocks
type CosmosBlockScanner struct {
	cfg              config.BlockScannerConfiguration
	logger           zerolog.Logger
	feeAsset         common.Asset
	avgGasFee        ctypes.Uint
	db               blockscanner.ScannerStorage
	cdc              *codec.ProtoCodec
	errCounter       *prometheus.CounterVec
	tmService        tmservice.ServiceClient
	grpc             *grpc.ClientConn
	bridge           *thorclient.ThorchainBridge
	solvencyReporter SolvencyReporter
}

// NewCosmosBlockScanner create a new instance of BlockScan
func NewCosmosBlockScanner(cfg config.BlockScannerConfiguration,
	scanStorage blockscanner.ScannerStorage,
	bridge *thorclient.ThorchainBridge,
	m *metrics.Metrics,
	solvencyReporter SolvencyReporter) (*CosmosBlockScanner, error) {
	if scanStorage == nil {
		return nil, errors.New("scanStorage is nil")
	}
	if m == nil {
		return nil, errors.New("metrics is nil")
	}

	logger := log.Logger.With().Str("module", "blockscanner").Str("chain", string(common.TERRAChain.String())).Logger()

	registry := bridge.GetContext().InterfaceRegistry
	btypes.RegisterInterfaces(registry)

	host := strings.Replace(cfg.RPCHost, "http://", "", -1)
	conn, err := grpc.Dial(host, grpc.WithInsecure())
	if err != nil {
		logger.Fatal().Err(err).Msg("fail to dial")
	}

	tmService := tmservice.NewServiceClient(conn)
	cdc := codec.NewProtoCodec(registry)

	feeAsset := common.TERRAChain.GetGasAsset()
	return &CosmosBlockScanner{
		cfg:              cfg,
		logger:           logger,
		db:               scanStorage,
		feeAsset:         feeAsset,
		avgGasFee:        ctypes.NewUint(0),
		cdc:              cdc,
		errCounter:       m.GetCounterVec(metrics.BlockScanError(common.TERRAChain)),
		tmService:        tmService,
		grpc:             conn,
		bridge:           bridge,
		solvencyReporter: solvencyReporter,
	}, nil
}

func (c *CosmosBlockScanner) GetHeight() (int64, error) {
	resultHeight, err := c.tmService.GetLatestBlock(
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

func sdkCoinToCommonCoin(c cosmos.Coin) common.Coin {
	name := fmt.Sprintf("%s.%s", common.TERRAChain.String(), c.Denom[1:])
	asset, _ := common.NewAsset(name)

	decimals, exists := WhitelistAssets[c.Denom]
	if !exists {
		return common.NewCoin(asset, ctypes.Uint(c.Amount))
	}

	amount := c.Amount
	var value *big.Int
	// Decimals are more than native THORChain, so divide...
	if decimals > 8 {
		decimalDiff := int64(decimals - 8)
		value = value.Exp(value, big.NewInt(10), big.NewInt(decimalDiff))
		amount = c.Amount.Quo(cosmos.NewIntFromBigInt(value))
	}
	// Decimals are less than native THORChain, so multiply...
	if decimals < 8 {
		decimalDiff := int64(8 - decimals)
		value = value.Exp(value, big.NewInt(10), big.NewInt(decimalDiff))
		amount = c.Amount.Mul(cosmos.NewIntFromBigInt(value))
	}
	return common.NewCoin(asset, ctypes.NewUintFromBigInt(amount.BigInt()))
}

// GetBlock returns a Tendermint block as a reference to a ResultBlock for a
// given height. An error is returned upon query failure.
func (c *CosmosBlockScanner) GetBlock(height int64) (*tmtypes.Block, error) {
	resultBlock, err := c.tmService.GetBlockByHeight(
		context.Background(),
		&tmservice.GetBlockByHeightRequest{Height: height},
	)

	if err != nil {
		c.logger.Error().Int64("height", height).Msgf("failed to get block: %v", err)
		c.errCounter.WithLabelValues("fail_get_block", strconv.Itoa(int(height))).Inc()

		return nil, fmt.Errorf("failed to get block: %w", err)
	}

	return resultBlock.Block, nil
}

func (c *CosmosBlockScanner) updateAverageGasFees(height int64, txs []types.TxInItem) (string, error) {
	var numTxs int64

	// sum all the gas fees for the FeeAsset only
	totalGasFees := ctypes.NewUint(0)
	for _, tx := range txs {
		fee := tx.Gas.ToCoins().GetCoin(c.feeAsset)
		if err := fee.Valid(); err != nil {
			// gas asset is not preferred fee asset, skip it...
			continue
		}
		numTxs++
		totalGasFees = totalGasFees.Add(fee.Amount)
	}

	if numTxs == 0 {
		return "", nil
	}

	// compute the average (total / numTxs)
	avgGasFeesAmt := (ctypes.NewDecFromBigInt(totalGasFees.BigInt()).QuoInt64(numTxs)).TruncateInt()
	if avgGasFeesAmt.IsZero() {
		return "", nil
	}

	if !avgGasFeesAmt.IsUint64() {
		return "", fmt.Errorf("average gas fee exceeds uint64: %s", avgGasFeesAmt)
	}

	// post the gas fee if it changed since last calculation
	if !c.avgGasFee.Equal(ctypes.Uint(avgGasFeesAmt)) {
		feeTx, err := c.bridge.PostNetworkFee(height, common.TERRAChain, 1, avgGasFeesAmt.Uint64())
		if err != nil {
			return "", err
		}
		c.avgGasFee = ctypes.NewUint(avgGasFeesAmt.Uint64())
		return feeTx.String(), nil
	}

	return "", nil
}

func (c *CosmosBlockScanner) FetchTxs(height int64) (types.TxIn, error) {
	block, err := c.GetBlock(height)
	if err != nil {
		return types.TxIn{}, err
	}

	decoder := tx.DefaultTxDecoder(c.cdc)
	var txs []types.TxInItem

	for _, rawTx := range block.Data.Txs {
		hash := hex.EncodeToString(tmhash.Sum(rawTx))
		tx, err := decoder(rawTx)
		if err != nil {
			if strings.Contains(err.Error(), "unable to resolve type URL") {
				// couldn't find msg type in the interface registry, probably not relevant
				if !strings.Contains(err.Error(), "MsgSend") {
					// double check to make sure MsgSend isn't mentioned
					// if it's not, we can safely ignore
					continue
				}
			}
			// else we should log this as an error and continue
			c.logger.Error().Str("tx", string(rawTx)).Err(err).Msg("unable to decode msg")
			continue
		}

		memo := tx.(ctypes.TxWithMemo).GetMemo()
		fees := tx.(ctypes.FeeTx).GetFee()

		for _, msg := range tx.GetMsgs() {
			switch msg := msg.(type) {
			case *btypes.MsgSend:
				coins := common.Coins{}
				for _, coin := range msg.Amount {

					// ignore first character of denom, which is usually "u" in cosmos
					if _, whitelisted := WhitelistAssets[coin.Denom]; !whitelisted {
						c.logger.Info().Str("tx", hash).Interface("coins", c).Msg("coin is not whitelisted, skipping")
						continue
					}

					cCoin := sdkCoinToCommonCoin(coin)
					coins = append(coins, cCoin)
				}

				// ignore the tx when no coins exist
				if coins.IsEmpty() {
					continue
				}

				gasFees := common.Gas{}
				for _, fee := range fees {
					cCoin := sdkCoinToCommonCoin(fee)
					gasFees = append(gasFees, cCoin)
				}

				txs = append(txs, types.TxInItem{
					Tx:          hash[:],
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
		Count:    strconv.Itoa(len(txs)),
		Chain:    common.TERRAChain,
		TxArray:  txs,
		Filtered: false,
		MemPool:  false,
	}

	feeTx, err := c.updateAverageGasFees(block.Header.Height, txIn.TxArray)
	if err != nil {
		c.logger.Err(err).Int64("height", height).Msg("failed to post average network fee")
	}
	if feeTx != "" {
		c.logger.Info().
			Str("tx", feeTx).
			Int64("height", height).
			Int64("gasFeeAmt", c.avgGasFee.BigInt().Int64()).
			Msg("sent network fee to THORChain")
	}

	return txIn, nil
}
