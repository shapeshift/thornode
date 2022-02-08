package terra

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"

	ctypes "github.com/cosmos/cosmos-sdk/types"
	btypes "github.com/cosmos/cosmos-sdk/x/bank/types"
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
)

// SolvencyReporter is to report solvency info to THORNode
type SolvencyReporter func(int64) error

const (
	GasUpdatePeriodBlocks = 10
	GasMethodAverage      = iota
	GasMethodSimulate
)

var (
	ErrInvalidScanStorage = errors.New("scan storage is empty or nil")
	ErrInvalidMetrics     = errors.New("metrics is empty or nil")
	ErrEmptyTx            = errors.New("empty tx")
)

// CosmosBlockScanner is to scan the blocks
type CosmosBlockScanner struct {
	cfg              config.BlockScannerConfiguration
	logger           zerolog.Logger
	feeAsset         common.Asset
	db               blockscanner.ScannerStorage
	cdc              *codec.ProtoCodec
	txClient         txtypes.ServiceClient
	txConfig         client.TxConfig
	gasMethod        int
	gasCacheSquares  []ctypes.Int
	gasCacheNum      int64
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

	logger := log.Logger.With().Str("module", "blockscanner").Str("chain", common.TERRAChain.String()).Logger()

	registry := bridge.GetContext().InterfaceRegistry
	btypes.RegisterInterfaces(registry)

	host := strings.ReplaceAll(cfg.RPCHost, "http://", "")
	conn, err := grpc.Dial(host, grpc.WithInsecure())
	if err != nil {
		logger.Fatal().Err(err).Msg("fail to dial")
	}

	tmService := tmservice.NewServiceClient(conn)
	cdc := codec.NewProtoCodec(registry)

	interfaceRegistry := codectypes.NewInterfaceRegistry()
	interfaceRegistry.RegisterImplementations((*ctypes.Msg)(nil), &btypes.MsgSend{})
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	txConfig := tx.NewTxConfig(marshaler, []signingtypes.SignMode{signingtypes.SignMode_SIGN_MODE_DIRECT})

	feeAsset := common.TERRAChain.GetGasAsset()
	return &CosmosBlockScanner{
		cfg:              cfg,
		logger:           logger,
		db:               scanStorage,
		feeAsset:         feeAsset,
		cdc:              cdc,
		txClient:         txtypes.NewServiceClient(conn),
		txConfig:         txConfig,
		tmService:        tmService,
		gasMethod:        GasMethodAverage,
		gasCacheSquares:  make([]ctypes.Int, 0),
		gasCacheNum:      0,
		grpc:             conn,
		bridge:           bridge,
		solvencyReporter: solvencyReporter,
	}, nil
}

func (c *CosmosBlockScanner) GetHeight() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	resultHeight, err := c.tmService.GetLatestBlock(
		ctx,
		&tmservice.GetLatestBlockRequest{})
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
func (c *CosmosBlockScanner) GetBlock(height int64) (*tmtypes.Block, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	resultBlock, err := c.tmService.GetBlockByHeight(
		ctx,
		&tmservice.GetBlockByHeightRequest{Height: height})
	if err != nil {
		c.logger.Error().Int64("height", height).Msgf("failed to get block: %v", err)
		return nil, fmt.Errorf("failed to get block: %w", err)
	}

	return resultBlock.Block, nil
}

func (c *CosmosBlockScanner) updateGasCache(txs []types.TxInItem) error {
	// add gas fees for the FeeAsset only
	for _, tx := range txs {
		fee := tx.Gas.ToCoins().GetCoin(c.feeAsset)
		if err := fee.Valid(); err != nil {
			// gas asset is not preferred fee asset, skip it...
			continue
		}
		c.gasCacheNum++
		c.gasCacheSquares = append(c.gasCacheSquares, ctypes.Int(fee.Amount).Mul(ctypes.Int(fee.Amount)))
	}

	return nil
}

func (c *CosmosBlockScanner) getAverageFromCache() ctypes.Int {
	if len(c.gasCacheSquares) == 0 || c.gasCacheNum == 0 {
		return ctypes.NewInt(0)
	}

	sumOfSquares := ctypes.NewInt(0)
	for _, val := range c.gasCacheSquares {
		sumOfSquares = sumOfSquares.Add(val)
	}

	averageSquares := sumOfSquares.Quo(ctypes.NewIntFromBigInt(big.NewInt(c.gasCacheNum)))
	return ctypes.NewIntFromBigInt(big.NewInt(0).Sqrt(averageSquares.BigInt()))
}

func (c *CosmosBlockScanner) updateGasFees(height int64) error {
	// post the gas fee over every cache period

	if height%GasUpdatePeriodBlocks == 0 {
		var gas common.Coin
		// Use the SimulateTx method with a dummy tx to calculate appropriate fee
		if c.gasMethod == GasMethodSimulate {
			dummyTxb, err := getDummyTxBuilderForSimulate(c.txConfig)
			if err != nil {
				return fmt.Errorf("unable to getDummyTxBuilderForSimulate: %w", err)
			}

			simRes, err := simulateTx(dummyTxb, c.txClient)
			if err != nil {
				return fmt.Errorf("unable to SimulateTx: %w", err)
			}

			gasAsset, exists := GetAssetByThorchainSymbol(c.feeAsset.String())
			if !exists {
				return fmt.Errorf("unable to get asset by thorchain symbol: %s", c.feeAsset.Symbol.String())
			}

			gas, err = fromCosmosToThorchain(ctypes.NewCoin(gasAsset.CosmosDenom, ctypes.NewInt(int64(simRes.GasInfo.GasUsed))))
			if err != nil {
				return fmt.Errorf("unable to convert cosmos coins to thorchain: %w", err)
			}
		} else if c.gasMethod == GasMethodAverage {
			gasInt := c.getAverageFromCache()
			gas = common.NewCoin(c.feeAsset, ctypes.NewUint(gasInt.Uint64()))
			c.gasCacheSquares = make([]ctypes.Int, 0)
			c.gasCacheNum = 0
		}

		feeTx, err := c.bridge.PostNetworkFee(height, common.TERRAChain, 1, gas.Amount.Uint64())
		if err != nil {
			return err
		}
		c.logger.Info().
			Str("tx", feeTx.String()).
			Int64("amount", gas.Amount.BigInt().Int64()).
			Int64("height", height).
			Msg("sent network fee to THORChain")

	}

	return nil
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

					if _, whitelisted := GetAssetByCosmosDenom(coin.Denom); !whitelisted {
						c.logger.Debug().Str("tx", hash).Interface("coins", c).Msg("coin is not whitelisted, skipping")
						continue
					}
					cCoin, err := fromCosmosToThorchain(coin)
					if err != nil {
						c.logger.Warn().Err(err).Interface("coins", c).Msg("wasn't able to convert coins that passed whitelist")
						continue
					}
					coins = append(coins, cCoin)
				}

				// ignore the tx when no coins exist
				if coins.IsEmpty() {
					continue
				}

				gasFees := common.Gas{}
				for _, fee := range fees {
					cCoin, err := fromCosmosToThorchain(fee)
					if err != nil {
						c.logger.Warn().Err(err).Interface("fees", fees).Msg("wasn't able to convert coins that passed whitelist")
						continue
					}
					gasFees = append(gasFees, cCoin)
				}

				txs = append(txs, types.TxInItem{
					Tx:          hash,
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

	err = c.updateGasCache(txs)
	if err != nil {
		c.logger.Err(err).Int64("height", height).Msg("unable to update gas cache")
	}

	err = c.updateGasFees(height)
	if err != nil {
		c.logger.Err(err).Int64("height", height).Msg("unable to update network fee")
	}

	return txIn, nil
}
