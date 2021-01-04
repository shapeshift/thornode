package ethereum

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ecommon "github.com/ethereum/go-ethereum/common"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/thorchain/thornode/bifrost/blockscanner"
	btypes "gitlab.com/thorchain/thornode/bifrost/blockscanner/types"
	"gitlab.com/thorchain/thornode/bifrost/config"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/ethereum/types"
	"gitlab.com/thorchain/thornode/bifrost/pubkeymanager"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	stypes "gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

const (
	BlockCacheSize         = 100
	MaxContractGas         = 80000
	depositEvent           = "0xef519b7eb82aaf6ac376a6df2d793843ebfd593de5f1a0601d3cc6ab49ebb395"
	transferOutEvent       = "0xa9cd03aa3c1b4515114539cd53d22085129d495cb9e9f9af77864526240f1bf7"
	transferAllowanceEvent = "0x05b90458f953d3fcb2d7fb25616a2fddeca749d0c47cc5c9832d0266b5346eea"
	vaultTransferEvent     = "0x281daef48d91e5cd3d32db0784f6af69cd8d8d2e8c612a3568dca51ded51e08f"
	ethToken               = "0x0000000000000000000000000000000000000000"
	symbolMethod           = "symbol"
	decimalMethod          = "decimals"
	defaultDecimals        = 18 // on ETH , consolidate all decimals to 18, in Wei
)

// ETHScanner is a scanner that understand how to interact with ETH chain ,and scan block , parse smart contract etc
type ETHScanner struct {
	cfg                config.BlockScannerConfiguration
	logger             zerolog.Logger
	db                 blockscanner.ScannerStorage
	m                  *metrics.Metrics
	errCounter         *prometheus.CounterVec
	gasPriceChanged    bool
	gasPrice           *big.Int
	client             *ethclient.Client
	blockMetaAccessor  BlockMetaAccessor
	globalErrataQueue  chan<- stypes.ErrataBlock
	vaultABI           *abi.ABI
	erc20ABI           *abi.ABI
	tokens             *LevelDBTokenMeta
	bridge             *thorclient.ThorchainBridge
	pubkeyMgr          pubkeymanager.PubKeyValidator
	eipSigner          etypes.EIP155Signer
	currentBlockHeight int64
}

// NewETHScanner create a new instance of ETHScanner
func NewETHScanner(cfg config.BlockScannerConfiguration,
	storage blockscanner.ScannerStorage,
	chainID *big.Int,
	client *ethclient.Client,
	bridge *thorclient.ThorchainBridge,
	m *metrics.Metrics,
	pubkeyMgr pubkeymanager.PubKeyValidator) (*ETHScanner, error) {
	if storage == nil {
		return nil, errors.New("storage is nil")
	}
	if m == nil {
		return nil, errors.New("metrics manager is nil")
	}
	if client == nil {
		return nil, errors.New("ETH client is nil")
	}
	if pubkeyMgr == nil {
		return nil, errors.New("pubkey manager is nil")
	}
	blockMetaAccessor, err := NewLevelDBBlockMetaAccessor(storage.GetInternalDb())
	if err != nil {
		return nil, fmt.Errorf("fail to create block meta accessor: %w", err)
	}
	tokens, err := NewLevelDBTokenMeta(storage.GetInternalDb())
	if err != nil {
		return nil, fmt.Errorf("fail to create token meta db: %w", err)
	}
	err = tokens.SaveTokenMeta("ETH", ethToken, defaultDecimals)
	if err != nil {
		return nil, err
	}
	vaultABI, erc20ABI, err := getContractABI()
	if err != nil {
		return nil, fmt.Errorf("fail to create contract abi: %w", err)
	}
	return &ETHScanner{
		cfg:               cfg,
		logger:            log.Logger.With().Str("module", "block_scanner").Str("chain", common.ETHChain.String()).Logger(),
		errCounter:        m.GetCounterVec(metrics.BlockScanError(common.ETHChain)),
		client:            client,
		db:                storage,
		m:                 m,
		gasPrice:          big.NewInt(0),
		gasPriceChanged:   false,
		blockMetaAccessor: blockMetaAccessor,
		tokens:            tokens,
		bridge:            bridge,
		vaultABI:          vaultABI,
		erc20ABI:          erc20ABI,
		eipSigner:         etypes.NewEIP155Signer(chainID),
		pubkeyMgr:         pubkeyMgr,
	}, nil
}

// GetGasPrice returns current gas price
func (e *ETHScanner) GetGasPrice() *big.Int {
	return e.gasPrice
}

func (e *ETHScanner) getContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), e.cfg.HttpRequestTimeout)
}

// GetHeight return latest block height
func (e *ETHScanner) GetHeight() (int64, error) {
	ctx, cancel := e.getContext()
	defer cancel()
	block, err := e.client.BlockByNumber(ctx, nil)
	if err != nil {
		return -1, fmt.Errorf("fail to get block height: %w", err)
	}
	return block.Number().Int64(), nil
}

// FetchMemPool get tx from mempool
func (e *ETHScanner) FetchMemPool(height int64) (stypes.TxIn, error) {
	return stypes.TxIn{}, nil
}

// GetTokens return all the token meta data
func (e *ETHScanner) GetTokens() ([]*types.TokenMeta, error) {
	return e.tokens.GetTokens()
}

// FetchTxs query the ETH chain to get txs in the given block height
func (e *ETHScanner) FetchTxs(height int64) (stypes.TxIn, error) {
	block, err := e.getRPCBlock(height)
	if err != nil {
		return stypes.TxIn{}, err
	}

	txIn, err := e.processBlock(block)
	if err != nil {
		e.logger.Error().Err(err).Int64("height", height).Msg("fail to search tx in block")
		return stypes.TxIn{}, fmt.Errorf("fail to process block: %d, err:%w", height, err)
	}
	e.currentBlockHeight = height
	pruneHeight := height - BlockCacheSize
	if pruneHeight > 0 {
		defer func() {
			if err := e.blockMetaAccessor.PruneBlockMeta(pruneHeight); err != nil {
				e.logger.Err(err).Msgf("fail to prune block meta, height(%d)", pruneHeight)
			}
		}()
	}
	if e.gasPriceChanged {
		// only send the network fee to THORNode when the price get changed
		if _, err := e.bridge.PostNetworkFee(height, common.ETHChain, MaxContractGas, e.GetGasPrice().Uint64()); err != nil {
			e.logger.Err(err).Msg("fail to post ETH chain single transfer fee to THORNode")
		}
	}
	return txIn, nil
}

func (e *ETHScanner) updateGasPrice() {
	ctx, cancel := e.getContext()
	defer cancel()
	gasPrice, err := e.client.SuggestGasPrice(ctx)
	if err != nil {
		e.logger.Err(err).Msg("fail to get suggest gas price")
		return
	}
	if e.gasPrice.Cmp(gasPrice) == 0 {
		e.gasPriceChanged = false
		return
	}
	e.gasPriceChanged = true
	e.gasPrice = gasPrice
}

// vaultDepositEvent represent a vault deposit
type vaultDepositEvent struct {
	To    ecommon.Address
	Asset ecommon.Address
	Value *big.Int
	Memo  string
}

func (e *ETHScanner) parseDeposit(log etypes.Log) (vaultDepositEvent, error) {
	const DepositEventName = "Deposit"
	event := vaultDepositEvent{}
	if err := e.unpackVaultLog(&event, DepositEventName, log); err != nil {
		return event, fmt.Errorf("fail to unpack event: %w", err)
	}
	return event, nil
}

// RouterCoin represent the coins transfer between vault
type RouterCoin struct {
	Asset  ecommon.Address
	Amount *big.Int
}

type routerVaultTransfer struct {
	OldVault ecommon.Address
	NewVault ecommon.Address
	Coins    []RouterCoin
	Memo     string
}

func (e *ETHScanner) parseVaultTransfer(log etypes.Log) (routerVaultTransfer, error) {
	const vaultTransferEventName = "VaultTransfer"
	event := routerVaultTransfer{}
	if err := e.unpackVaultLog(&event, vaultTransferEventName, log); err != nil {
		return event, fmt.Errorf("fail to unpack event: %w", err)
	}
	return event, nil
}

func (e *ETHScanner) unpackVaultLog(out interface{}, event string, log etypes.Log) error {
	if len(log.Data) > 0 {
		if err := e.vaultABI.UnpackIntoInterface(out, event, log.Data); err != nil {
			return fmt.Errorf("fail to parse event: %w", err)
		}
	}
	var indexed abi.Arguments
	for _, arg := range e.vaultABI.Events[event].Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	return abi.ParseTopics(out, indexed, log.Topics[1:])
}

type vaultTransferOutEvent struct {
	Vault ecommon.Address
	To    ecommon.Address
	Asset ecommon.Address
	Value *big.Int
	Memo  string
}

func (e *ETHScanner) parseTransferOut(log etypes.Log) (vaultTransferOutEvent, error) {
	const TransferOutEventName = "TransferOut"
	event := vaultTransferOutEvent{}
	if err := e.unpackVaultLog(&event, TransferOutEventName, log); err != nil {
		return event, fmt.Errorf("fail to parse transfer out event")
	}
	return event, nil
}

type vaultTransferAllowanceEvent struct {
	OldVault ecommon.Address
	NewVault ecommon.Address
	Asset    ecommon.Address
	Value    *big.Int
	Memo     string
}

func (e *ETHScanner) parseTransferAllowanceEvent(log etypes.Log) (vaultTransferAllowanceEvent, error) {
	const TransferAllowanceEventName = "TransferAllowance"
	event := vaultTransferAllowanceEvent{}
	if err := e.unpackVaultLog(&event, TransferAllowanceEventName, log); err != nil {
		return event, fmt.Errorf("fail to parse transfer allowance event")
	}
	return event, nil
}

// processBlock extracts transactions from block
func (e *ETHScanner) processBlock(block *etypes.Block) (stypes.TxIn, error) {
	height := int64(block.NumberU64())

	// Update gas price
	e.updateGasPrice()
	if err := e.processReorg(block.Header()); err != nil {
		e.logger.Error().Err(err).Msgf("fail to process reorg for block %d", height)
		return stypes.TxIn{}, err
	}

	if block.Transactions().Len() == 0 {
		return stypes.TxIn{}, nil
	}
	txIn, err := e.extractTxs(block)
	if err != nil {
		return stypes.TxIn{}, err
	}

	blockMeta := types.NewBlockMeta(block.Header(), txIn)
	if err := e.blockMetaAccessor.SaveBlockMeta(blockMeta.Height, blockMeta); err != nil {
		e.logger.Err(err).Msgf("fail to save block meta of height: %d ", blockMeta.Height)
	}
	return txIn, nil
}

func (e *ETHScanner) extractTxs(block *etypes.Block) (stypes.TxIn, error) {
	txInbound := stypes.TxIn{
		Chain:    common.ETHChain,
		Filtered: false,
		MemPool:  false,
	}

	for _, tx := range block.Transactions() {
		if tx.To() == nil {
			continue
		}

		txInItem, err := e.fromTxToTxIn(tx)
		if err != nil {
			e.errCounter.WithLabelValues("fail_get_tx", "").Inc()
			e.logger.Error().Err(err).Str("hash", tx.Hash().Hex()).Msg("fail to get one tx from server")
			// if THORNode fail to get one tx hash from server, then THORNode should bail, because THORNode might miss tx
			// if THORNode bail here, then THORNode should retry later
			return stypes.TxIn{}, fmt.Errorf("fail to get one tx from server: %w", err)
		}
		if txInItem != nil {
			txInItem.BlockHeight = block.Number().Int64()
			txInbound.TxArray = append(txInbound.TxArray, *txInItem)
			e.logger.Info().Str("hash", tx.Hash().Hex()).Msgf("%s got %d tx", e.cfg.ChainID, 1)
		}
	}
	if len(txInbound.TxArray) == 0 {
		e.logger.Info().Int64("block", int64(block.NumberU64())).Msg("no tx need to be processed in this block")
		return stypes.TxIn{}, nil
	}
	txInbound.Count = strconv.Itoa(len(txInbound.TxArray))

	return txInbound, nil
}

func (e *ETHScanner) processReorg(block *etypes.Header) error {
	previousHeight := block.Number.Int64() - 1
	prevBlockMeta, err := e.blockMetaAccessor.GetBlockMeta(previousHeight)
	if err != nil {
		return fmt.Errorf("fail to get block meta of height(%d) : %w", previousHeight, err)
	}
	if prevBlockMeta == nil {
		return nil
	}
	// the block's previous hash need to be the same as the block hash chain client recorded in block meta
	// blockMetas[PreviousHeight].BlockHash == Block.PreviousHash
	if strings.EqualFold(prevBlockMeta.BlockHash, block.ParentHash.Hex()) {
		return nil
	}

	e.logger.Info().Msgf("re-org detected, current block height:%d ,previous block hash is : %s , however block meta at height: %d, block hash is %s", block.Number.Int64(), block.ParentHash.Hex(), prevBlockMeta.Height, prevBlockMeta.BlockHash)
	return e.reprocessTxs()
}

// reprocessTx will be kicked off only when chain client detected a re-org on ethereum chain
// it will read through all the block meta data from local storage, and go through all the txs.
// For each transaction, it will send a RPC request to ethereuem chain, double check whether the TX exist or not
// if the tx still exist, then it is all good, if a transaction previous we detected, however doesn't exist anymore, that means
// the transaction had been removed from chain, chain client should report to thorchain
func (e *ETHScanner) reprocessTxs() error {
	blockMetas, err := e.blockMetaAccessor.GetBlockMetas()
	if err != nil {
		return fmt.Errorf("fail to get block metas from local storage: %w", err)
	}

	for _, blockMeta := range blockMetas {
		metaTxs := make([]types.TransactionMeta, 0)
		var errataTxs []stypes.ErrataTx
		for _, tx := range blockMeta.Transactions {
			if e.checkTransaction(tx.Hash) {
				e.logger.Info().Msgf("block height: %d, tx: %s still exist", blockMeta.Height, tx.Hash)
				metaTxs = append(metaTxs, tx)
				continue
			}
			// this means the tx doesn't exist in chain ,thus should errata it
			errataTxs = append(errataTxs, stypes.ErrataTx{
				TxID:  common.TxID(tx.Hash),
				Chain: common.ETHChain,
			})
		}
		if len(errataTxs) == 0 {
			continue
		}
		e.globalErrataQueue <- stypes.ErrataBlock{
			Height: blockMeta.Height,
			Txs:    errataTxs,
		}
		// Let's get the block again to fix the block hash
		block, err := e.getHeader(blockMeta.Height)
		if err != nil {
			e.logger.Err(err).Msgf("fail to get block verbose tx result: %d", blockMeta.Height)
		}
		blockMeta.PreviousHash = block.ParentHash.Hex()
		blockMeta.BlockHash = block.Hash().Hex()
		blockMeta.Transactions = metaTxs
		if err := e.blockMetaAccessor.SaveBlockMeta(blockMeta.Height, blockMeta); err != nil {
			e.logger.Err(err).Msgf("fail to save block meta of height: %d ", blockMeta.Height)
		}
	}
	return nil
}

func (e *ETHScanner) checkTransaction(hash string) bool {
	ctx, cancel := e.getContext()
	defer cancel()
	receipt, err := e.client.TransactionReceipt(ctx, ecommon.HexToHash(hash))
	if err != nil || receipt == nil {
		return false
	}
	return true
}

func (e *ETHScanner) getReceipt(hash string) (*etypes.Receipt, error) {
	ctx, cancel := e.getContext()
	defer cancel()
	return e.client.TransactionReceipt(ctx, ecommon.HexToHash(hash))
}

func (e *ETHScanner) getHeader(height int64) (*etypes.Header, error) {
	ctx, cancel := e.getContext()
	defer cancel()
	return e.client.HeaderByNumber(ctx, big.NewInt(height))
}

func (e *ETHScanner) getBlock(height int64) (*etypes.Block, error) {
	ctx, cancel := e.getContext()
	defer cancel()
	return e.client.BlockByNumber(ctx, big.NewInt(height))
}

func (e *ETHScanner) getRPCBlock(height int64) (*etypes.Block, error) {
	block, err := e.getBlock(height)
	if err == ethereum.NotFound {
		return nil, btypes.UnavailableBlock
	}
	if err != nil {
		return nil, fmt.Errorf("fail to fetch block: %w", err)
	}
	return block, nil
}

func (e *ETHScanner) getDecimals(token string) (uint64, error) {
	if IsETH(token) {
		return defaultDecimals, nil
	}
	to := ecommon.HexToAddress(token)
	input, err := e.erc20ABI.Pack(decimalMethod)
	if err != nil {
		return defaultDecimals, fmt.Errorf("fail to pack decimal method: %w", err)
	}
	ctx, cancel := e.getContext()
	defer cancel()
	res, err := e.client.CallContract(ctx, ethereum.CallMsg{
		To:   &to,
		Data: input,
	}, nil)
	if err != nil {
		return defaultDecimals, fmt.Errorf("fail to call smart contract get decimals: %w", err)
	}
	output, err := e.erc20ABI.Unpack(decimalMethod, res)
	if err != nil {
		return defaultDecimals, fmt.Errorf("fail to unpack decimal method call result: %w", err)
	}
	switch output[0].(type) {
	case uint8:
		decimals := *abi.ConvertType(output[0], new(uint8)).(*uint8)
		return uint64(decimals), nil
	case *big.Int:
		decimals := *abi.ConvertType(output[0], new(*big.Int)).(**big.Int)
		return decimals.Uint64(), nil
	}
	return defaultDecimals, fmt.Errorf("%s is %T fail to parse it", output[0], output[0])
}

// replace the . in symbol to *, and replace the - in symbol to #
// because . and - had been reserved to use in THORChain symbol
var symbolReplacer = strings.NewReplacer(".", "*", "-", "#")

func sanitiseSymbol(symbol string) string {
	return symbolReplacer.Replace(symbol)
}

func (e *ETHScanner) getSymbol(token string) (string, error) {
	if IsETH(token) {
		return "ETH", nil
	}
	to := ecommon.HexToAddress(token)
	input, err := e.erc20ABI.Pack(symbolMethod)
	if err != nil {
		return "", nil
	}
	ctx, cancel := e.getContext()
	defer cancel()
	res, err := e.client.CallContract(ctx, ethereum.CallMsg{
		To:   &to,
		Data: input,
	}, nil)
	if err != nil {
		return "", fmt.Errorf("fail to call to smart contract and get symbol: %w", err)
	}
	output, err := e.erc20ABI.Unpack(symbolMethod, res)
	if err != nil {
		return "", fmt.Errorf("fail to unpack symbol method call: %w", err)
	}
	symbol := *abi.ConvertType(output[0], new(string)).(*string)
	return sanitiseSymbol(symbol), nil
}

func (e *ETHScanner) isToSmartContract(toAddr *ecommon.Address) bool {
	contractAddresses := e.pubkeyMgr.GetContracts(common.ETHChain)
	for _, item := range contractAddresses {
		if strings.EqualFold(item.String(), toAddr.String()) {
			return true
		}
	}
	return false
}

func (e *ETHScanner) getTokenMeta(token string) (types.TokenMeta, error) {
	tokenMeta, err := e.tokens.GetTokenMeta(token)
	if err != nil {
		return types.TokenMeta{}, fmt.Errorf("fail to get token meta: %w", err)
	}
	if tokenMeta.IsEmpty() {
		symbol, err := e.getSymbol(token)
		if err != nil {
			return types.TokenMeta{}, fmt.Errorf("fail to get symbol: %w", err)
		}
		decimals, err := e.getDecimals(token)
		if err != nil {
			e.logger.Err(err).Msgf("fail to get decimals from smart contract, default to: %d", defaultDecimals)
		}
		tokenMeta = types.NewTokenMeta(symbol, token, decimals)
		if err = e.tokens.SaveTokenMeta(symbol, token, decimals); err != nil {
			return types.TokenMeta{}, fmt.Errorf("fail to save token meta: %w", err)
		}
	}
	return tokenMeta, nil
}

func (e *ETHScanner) convertAmount(token string, amt *big.Int) cosmos.Uint {
	if IsETH(token) {
		return cosmos.NewUintFromBigInt(amt)
	}
	decimals := uint64(defaultDecimals)
	tokenMeta, err := e.getTokenMeta(token)
	if err != nil {
		e.logger.Err(err).Msgf("fail to get token meta for token address: %s", token)
	}
	if !tokenMeta.IsEmpty() {
		decimals = tokenMeta.Decimal
	}
	if decimals != defaultDecimals {
		var value big.Int
		amt = amt.Mul(amt, value.Exp(big.NewInt(10), big.NewInt(defaultDecimals), nil))
		amt = amt.Div(amt, value.Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
	}
	return cosmos.NewUintFromBigInt(amt)
}

func (e *ETHScanner) getAssetFromTokenAddress(token string) (common.Asset, error) {
	if IsETH(token) {
		return common.ETHAsset, nil
	}
	tokenMeta, err := e.getTokenMeta(token)
	if err != nil {
		return common.EmptyAsset, fmt.Errorf("fail to get token meta: %w", err)
	}
	asset := common.ETHAsset
	if tokenMeta.Symbol != common.ETHChain.String() {
		asset, err = common.NewAsset(fmt.Sprintf("ETH.%s-%s", tokenMeta.Symbol, strings.ToUpper(tokenMeta.Address)))
		if err != nil {
			return common.EmptyAsset, fmt.Errorf("fail to create asset: %w", err)
		}
	}
	return asset, nil
}

// getTxInFromSmartContract returns txInItem
func (e *ETHScanner) getTxInFromSmartContract(tx *etypes.Transaction) (*stypes.TxInItem, error) {
	e.logger.Info().Msg("parse tx from smart contract")
	txInItem := &stypes.TxInItem{
		Tx: tx.Hash().Hex()[2:],
	}
	sender, err := e.eipSigner.Sender(tx)
	if err != nil {
		return nil, fmt.Errorf("fail to get sender: %w", err)
	}
	txInItem.Sender = strings.ToLower(sender.String())
	receipt, err := e.getReceipt(tx.Hash().Hex())
	if err != nil {
		return nil, fmt.Errorf("fail to get transaction receipt: %w", err)
	}

	for _, item := range receipt.Logs {
		switch item.Topics[0].String() {
		case depositEvent:
			depositEvt, err := e.parseDeposit(*item)
			if err != nil {
				return nil, fmt.Errorf("fail to parse deposit event: %w", err)
			}
			e.logger.Info().Msgf("deposit:%+v", depositEvt)
			txInItem.To = depositEvt.To.String()
			txInItem.Memo = depositEvt.Memo
			asset, err := e.getAssetFromTokenAddress(depositEvt.Asset.String())
			if err != nil {
				return nil, fmt.Errorf("fail to get asset from token address: %w", err)
			}
			txInItem.Coins = append(txInItem.Coins, common.NewCoin(asset, e.convertAmount(depositEvt.Asset.String(), depositEvt.Value)))
		case transferOutEvent:
			transferOutEvt, err := e.parseTransferOut(*item)
			if err != nil {
				return nil, fmt.Errorf("fail to parse transfer out event: %w", err)
			}
			e.logger.Info().Msgf("transfer out: %+v", transferOutEvt)
			txInItem.Sender = transferOutEvt.Vault.String()
			txInItem.To = transferOutEvt.To.String()
			txInItem.Memo = transferOutEvt.Memo
			asset, err := e.getAssetFromTokenAddress(transferOutEvt.Asset.String())
			if err != nil {
				return nil, fmt.Errorf("fail to get asset from token address: %w", err)
			}
			txInItem.Coins = append(txInItem.Coins, common.NewCoin(asset, e.convertAmount(transferOutEvt.Asset.String(), transferOutEvt.Value)))
		case transferAllowanceEvent:
			transferAllowanceEvt, err := e.parseTransferAllowanceEvent(*item)
			if err != nil {
				return nil, fmt.Errorf("fail to parse transfer allowance event: %w", err)
			}
			e.logger.Info().Msgf("transfer allowance: %+v", transferAllowanceEvt)
			txInItem.Sender = transferAllowanceEvt.OldVault.String()
			txInItem.To = transferAllowanceEvt.NewVault.String()
			txInItem.Memo = transferAllowanceEvt.Memo
			asset, err := e.getAssetFromTokenAddress(transferAllowanceEvt.Asset.String())
			if err != nil {
				return nil, fmt.Errorf("fail to get asset from token address: %w", err)
			}
			txInItem.Coins = append(txInItem.Coins, common.NewCoin(asset, e.convertAmount(transferAllowanceEvt.Asset.String(), transferAllowanceEvt.Value)))
		case vaultTransferEvent:
			transferEvent, err := e.parseVaultTransfer(*item)
			if err != nil {
				return nil, fmt.Errorf("fail to parse vault transfer event: %w", err)
			}
			e.logger.Info().Msgf("vault transfer: %+v", transferEvent)
			txInItem.Sender = transferEvent.OldVault.String()
			txInItem.To = transferEvent.NewVault.String()
			txInItem.Memo = transferEvent.Memo
			for _, item := range transferEvent.Coins {
				asset, err := e.getAssetFromTokenAddress(item.Asset.String())
				if err != nil {
					return nil, fmt.Errorf("fail to get asset from token address: %w", err)
				}
				txInItem.Coins = append(txInItem.Coins, common.NewCoin(asset, e.convertAmount(item.Asset.String(), item.Amount)))
			}
			ethValue := cosmos.NewUintFromBigInt(tx.Value())
			if !ethValue.IsZero() {
				txInItem.Coins = append(txInItem.Coins, common.NewCoin(common.ETHAsset, ethValue))
			}
		}
	}
	e.logger.Info().Msgf("tx: %s, gas price: %s, gas used: %d", txInItem.Tx, tx.GasPrice().String(), receipt.GasUsed)
	txInItem.Gas = common.MakeETHGas(tx.GasPrice(), receipt.GasUsed)
	return txInItem, nil
}

func (e *ETHScanner) getTxInFromTransaction(tx *etypes.Transaction) (*stypes.TxInItem, error) {
	txInItem := &stypes.TxInItem{
		Tx: tx.Hash().Hex()[2:],
	}
	asset := common.ETHAsset
	sender, err := e.eipSigner.Sender(tx)
	if err != nil {
		return nil, fmt.Errorf("fail to get sender: %w", err)
	}
	txInItem.Sender = strings.ToLower(sender.String())
	txInItem.To = strings.ToLower(tx.To().String())
	// this is native , thus memo is data field
	data := tx.Data()
	if len(data) > 0 {
		memo, err := hex.DecodeString(string(data))
		if err != nil {
			txInItem.Memo = string(data)
		} else {
			txInItem.Memo = string(memo)
		}

	}
	txInItem.Coins = append(txInItem.Coins, common.NewCoin(asset, cosmos.NewUintFromBigInt(tx.Value())))
	txInItem.Gas = common.MakeETHGas(tx.GasPrice(), tx.Gas())
	return txInItem, nil
}

func (e *ETHScanner) fromTxToTxIn(tx *etypes.Transaction) (*stypes.TxInItem, error) {
	if tx == nil || tx.To() == nil {
		return nil, nil
	}
	smartContract := e.isToSmartContract(tx.To())
	if smartContract {
		return e.getTxInFromSmartContract(tx)
	}
	return e.getTxInFromTransaction(tx)
}
