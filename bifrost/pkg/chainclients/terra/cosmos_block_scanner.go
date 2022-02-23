package terra

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/cosmos/cosmos-sdk/codec"
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
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/terra/wasm"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/common"
)

// SolvencyReporter is to report solvency info to THORNode
type SolvencyReporter func(int64) error

const (
	// GasUpdatePeriodBlocks is the block interval at which we report gas fee changes.
	GasUpdatePeriodBlocks = 10

	// GasPriceFactor is a multiplier applied to the gas amount before dividing by the gas
	// limit to determine the gas price, and later used as a divisor on the final fee -
	// this avoid the integer division going to zero, and can be thought of as the
	// reciprocal of the gas price precision.
	GasPriceFactor = uint64(1e9)

	// GasLimit is the default gas limit we will use for all outbound transactions.
	GasLimit = 200000

	// GasCacheTransactions is the number of transactions over which we compute an average
	// (mean) gas price to use for outbound transactions. Note that only transactions
	// using the chain fee asset will be considered.
	GasCacheTransactions = 100
)

var (
	_                     ctypes.Msg = &wasm.MsgExecuteContract{}
	ErrInvalidScanStorage            = errors.New("scan storage is empty or nil")
	ErrInvalidMetrics                = errors.New("metrics is empty or nil")
	ErrEmptyTx                       = errors.New("empty tx")
)

// CosmosBlockScanner is to scan the blocks
type CosmosBlockScanner struct {
	cfg              config.BlockScannerConfiguration
	logger           zerolog.Logger
	db               blockscanner.ScannerStorage
	cdc              *codec.ProtoCodec
	txConfig         client.TxConfig
	txService        txtypes.ServiceClient
	tmService        tmservice.ServiceClient
	grpc             *grpc.ClientConn
	bridge           *thorclient.ThorchainBridge
	solvencyReporter SolvencyReporter

	// feeCache contains a rolling window of suggested gas fees which are computed as the
	// gas price paid in each observed transaction multiplied by the default GasLimit.
	// Fees are stored at 100x the values on the observed chain due to compensate for the
	// difference in base chain decimals (thorchain:1e8, terra:1e6).
	feeCache []ctypes.Uint
}

// NewCosmosBlockScanner create a new instance of BlockScan
func NewCosmosBlockScanner(cfg config.BlockScannerConfiguration,
	scanStorage blockscanner.ScannerStorage,
	bridge *thorclient.ThorchainBridge,
	m *metrics.Metrics,
	solvencyReporter SolvencyReporter,
) (*CosmosBlockScanner, error) {
	if scanStorage == nil {
		return nil, errors.New("scanStorage is nil")
	}
	if m == nil {
		return nil, errors.New("metrics is nil")
	}

	logger := log.Logger.With().Str("module", "blockscanner").Str("chain", common.TERRAChain.String()).Logger()

	host := strings.ReplaceAll(cfg.RPCHost, "http://", "")
	conn, err := grpc.Dial(host, grpc.WithInsecure())
	if err != nil {
		logger.Fatal().Err(err).Msg("fail to dial")
	}

	// Registry for decoding txs
	registry := bridge.GetContext().InterfaceRegistry
	registry.RegisterImplementations((*ctypes.Msg)(nil), &wasm.MsgExecuteContract{})

	btypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	// Registry for encoding txs
	marshaler := codec.NewProtoCodec(registry)
	txConfig := tx.NewTxConfig(marshaler, []signingtypes.SignMode{signingtypes.SignMode_SIGN_MODE_DIRECT})
	tmService := tmservice.NewServiceClient(conn)
	txService := txtypes.NewServiceClient(conn)

	return &CosmosBlockScanner{
		cfg:              cfg,
		logger:           logger,
		db:               scanStorage,
		cdc:              cdc,
		txConfig:         txConfig,
		txService:        txService,
		tmService:        tmService,
		feeCache:         make([]ctypes.Uint, 0),
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

func (c *CosmosBlockScanner) updateGasCache(tx ctypes.FeeTx) {
	fees := tx.GetFee()

	// only consider transactions that have a single fee
	if len(fees) != 1 {
		return
	}

	// only consider transactions with fee paid in uluna
	coin, err := fromCosmosToThorchain(fees[0])
	if err != nil || !coin.Asset.Equals(c.cfg.ChainID.GetGasAsset()) {
		return
	}

	// sanity check to ensure fee is non-zero
	err = coin.Valid()
	if err != nil {
		c.logger.Error().Err(err).Interface("fees", fees).Msg("transaction with zero fee")
		return
	}

	// add the fee to our cache
	amount := coin.Amount.Mul(ctypes.NewUint(GasPriceFactor)) // multiply to handle price < 1
	price := amount.Quo(ctypes.NewUint(tx.GetGas()))          // divide by gas to get the price
	fee := price.Mul(ctypes.NewUint(GasLimit))                // tx fee for default gas limit
	fee = fee.Quo(ctypes.NewUint(GasPriceFactor))             // unroll the multiple
	c.feeCache = append(c.feeCache, fee)

	// truncate gas prices older than our max cached transactions
	if len(c.feeCache) > GasCacheTransactions {
		c.feeCache = c.feeCache[(len(c.feeCache) - GasCacheTransactions):]
	}
}

func (c *CosmosBlockScanner) averageFee() ctypes.Uint {
	sum := ctypes.NewUint(0)
	for _, val := range c.feeCache {
		sum = sum.Add(val)
	}
	return sum.Quo(ctypes.NewUint(uint64(len(c.feeCache))))
}

func (c *CosmosBlockScanner) updateGasFees(height int64) error {
	// post the gas fee over every cache period when we have a full gas cache
	if height%GasUpdatePeriodBlocks == 0 && len(c.feeCache) == GasCacheTransactions {
		gasFee := c.averageFee()

		// sanity check the fee is not zero
		if gasFee.Equal(ctypes.NewUint(0)) {
			err := errors.New("suggested gas fee was zero")
			c.logger.Error().Err(err).Msg(err.Error())
			return err
		}

		// NOTE: We post the fee to the network instead of the transaction rate, and set the
		// transaction size 1 to ensure the MaxGas in the generated TxOut contains the
		// correct fee. We cannot pass the proper size and rate without a deeper change to
		// Thornode, as the rate on Cosmos chains is less than 1 and cannot be represented
		// by the uint. This follows the pattern set in the BNB chain client.
		feeTx, err := c.bridge.PostNetworkFee(height, common.TERRAChain, 1, gasFee.Uint64())
		if err != nil {
			return err
		}
		c.logger.Info().
			Str("tx", feeTx.String()).
			Uint64("fee", gasFee.Uint64()).
			Int64("height", height).
			Msg("sent network fee to THORChain")
	}

	return nil
}

func (c *CosmosBlockScanner) processTxs(height int64, rawTxs [][]byte) ([]types.TxInItem, error) {
	decoder := tx.DefaultTxDecoder(c.cdc)

	var possibleTxs []string
	for _, rawTx := range rawTxs {
		hash := hex.EncodeToString(tmhash.Sum(rawTx))
		tx, err := decoder(rawTx)
		if err != nil {
			if strings.Contains(err.Error(), "unable to resolve type URL") {
				// couldn't find msg type in the interface registry, probably not relevant
				if strings.Contains(err.Error(), "MsgSend") || strings.Contains(err.Error(), "MsgExecuteContract") {
					// double check to make sure MsgSend or MsgExecuteContract isn't mentioned
					c.logger.Error().Str("tx", string(rawTx)).Err(err).Msg("unable to decode msg")
				}
			}
			continue
		}

		containsMsgSend := false
		for _, msg := range tx.GetMsgs() {
			switch msg.(type) {
			case *btypes.MsgSend:
				containsMsgSend = true

			default:
				continue
			}
		}
		if containsMsgSend {
			possibleTxs = append(possibleTxs, hash)
		}
	}

	var txIn []types.TxInItem
	for _, txhash := range possibleTxs {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		getTxResponse, err := c.txService.GetTx(ctx, &txtypes.GetTxRequest{Hash: txhash})
		if err != nil {
			if strings.Contains(err.Error(), "marshaling error") || strings.Contains(err.Error(), "unknown field") {
				// we cannot intepret one of the messages in the tx response. this transaction cannot be verified, skip it...
				c.logger.Warn().Err(err).Str("txhash", txhash).Msg("marshaling error or unknown field")
				continue
			}
			return c.processTxs(height, rawTxs)
		}

		if getTxResponse == nil || getTxResponse.TxResponse == nil {
			c.logger.Warn().Str("txhash", txhash).Msg("inbound tx nil getTxResponse, ignoring...")
			// the tx response is invalid. this transaction cannot be verified, skip it...
			continue
		}

		if getTxResponse.TxResponse.Code != 0 {
			c.logger.Warn().Str("txhash", txhash).Msg("inbound tx has non-zero response code, ignoring...")
			continue
		}

		if len(getTxResponse.TxResponse.Logs) == 0 {
			c.logger.Warn().Str("txhash", txhash).Msg("inbound tx does not contain any logs, ignoring...")
			continue
		}

		txBz, err := getTxResponse.Tx.Marshal()
		if err != nil {
			c.logger.Error().Str("txhash", txhash).Msg("unable to marshal getTxResponse.Tx to bytes, ignoring...")
			continue
		}

		tx, err := decoder(txBz)
		if err != nil {
			c.logger.Error().Str("txhash", txhash).Msg("unable to decode getTxResponse bytes, ignoring...")
			continue
		}

		fees := tx.(ctypes.FeeTx).GetFee()
		c.updateGasCache(tx.(ctypes.FeeTx))

		for _, msg := range tx.GetMsgs() {
			switch msg := msg.(type) {
			case *btypes.MsgSend:
				coins := common.Coins{}
				for _, coin := range msg.Amount {
					cCoin, err := fromCosmosToThorchain(coin)
					if err != nil {
						c.logger.Debug().Err(err).Interface("coins", c).Msg("unable to convert coin, not whitelisted. skipping...")
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
						c.logger.Debug().Err(err).Interface("fees", fees).Msg("unable to convert coin, not whitelisted. skipping...")
						continue
					}
					gasFees = append(gasFees, cCoin)
				}

				txIn = append(txIn, types.TxInItem{
					Tx:          txhash,
					BlockHeight: height,
					Memo:        getTxResponse.Tx.Body.GetMemo(),
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
	return txIn, nil
}

func (c *CosmosBlockScanner) FetchTxs(height int64) (types.TxIn, error) {
	block, err := c.GetBlock(height)
	if err != nil {
		return types.TxIn{}, err
	}

	txs, err := c.processTxs(height, block.Data.Txs)
	if err != nil {
		c.logger.Err(err).Int64("height", height).Msg("failed to processTxs")
		return types.TxIn{}, err
	}

	txIn := types.TxIn{
		Count:    strconv.Itoa(len(txs)),
		Chain:    common.TERRAChain,
		TxArray:  txs,
		Filtered: false,
		MemPool:  false,
	}

	err = c.updateGasFees(height)
	if err != nil {
		c.logger.Err(err).Int64("height", height).Msg("unable to update network fee")
	}

	if err := c.solvencyReporter(height); err != nil {
		c.logger.Err(err).Msg("fail to send solvency to THORChain")
	}

	return txIn, nil
}
