package bitcoincash

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/rpcclient"
	"github.com/gcash/bchutil"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	tssp "gitlab.com/thorchain/tss/go-tss/tss"

	"gitlab.com/thorchain/thornode/bifrost/blockscanner"
	btypes "gitlab.com/thorchain/thornode/bifrost/blockscanner/types"
	"gitlab.com/thorchain/thornode/bifrost/config"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/bifrost/tss"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// BlockCacheSize the number of block meta that get store in storage.
const (
	BlockCacheSize      = 144
	MaximumConfirmation = 99999999
	MaxAsgardAddresses  = 100
	// EstimateAverageTxSize for THORChain the estimate tx size is hard code to 250 here , as most of time it will spend 1 input, have 3 output
	EstimateAverageTxSize = 1500
	DefaultCoinbaseValue  = 6.25
)

// Client observes bitcoin cash chain and allows to sign and broadcast tx
type Client struct {
	logger             zerolog.Logger
	cfg                config.ChainConfiguration
	client             *rpcclient.Client
	chain              common.Chain
	privateKey         *bchec.PrivateKey
	blockScanner       *blockscanner.BlockScanner
	blockMetaAccessor  BlockMetaAccessor
	ksWrapper          *KeySignWrapper
	bridge             *thorclient.ThorchainBridge
	globalErrataQueue  chan<- types.ErrataBlock
	nodePubKey         common.PubKey
	memPoolLock        *sync.Mutex
	processedMemPool   map[string]bool
	lastMemPoolScan    time.Time
	currentBlockHeight int64
	asgardAddresses    []common.Address
	lastAsgard         time.Time
	minRelayFeeSats    uint64
	tssKeySigner       *tss.KeySign
}

// NewClient generates a new Client
func NewClient(thorKeys *thorclient.Keys, cfg config.ChainConfiguration, server *tssp.TssServer, bridge *thorclient.ThorchainBridge, m *metrics.Metrics) (*Client, error) {
	client, err := rpcclient.New(&rpcclient.ConnConfig{
		Host:         cfg.RPCHost,
		User:         cfg.UserName,
		Pass:         cfg.Password,
		DisableTLS:   cfg.DisableTLS,
		HTTPPostMode: cfg.HTTPostMode,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("fail to create bitcoin cash rpc client: %w", err)
	}
	tssKm, err := tss.NewKeySign(server, bridge)
	if err != nil {
		return nil, fmt.Errorf("fail to create tss signer: %w", err)
	}
	thorPrivateKey, err := thorKeys.GetPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("fail to get THORChain private key: %w", err)
	}

	bchPrivateKey, err := getBCHPrivateKey(thorPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("fail to convert private key for BCH: %w", err)
	}
	ksWrapper, err := NewKeySignWrapper(bchPrivateKey, tssKm)
	if err != nil {
		return nil, fmt.Errorf("fail to create keysign wrapper: %w", err)
	}

	temp, err := codec.ToTmPubKeyInterface(thorPrivateKey.PubKey())
	if err != nil {
		return nil, fmt.Errorf("fail to get tm pub key: %w", err)
	}
	nodePubKey, err := common.NewPubKeyFromCrypto(temp)
	if err != nil {
		return nil, fmt.Errorf("fail to get the node pubkey: %w", err)
	}

	c := &Client{
		logger:           log.Logger.With().Str("module", "bitcoincash").Logger(),
		cfg:              cfg,
		chain:            cfg.ChainID,
		client:           client,
		privateKey:       bchPrivateKey,
		ksWrapper:        ksWrapper,
		bridge:           bridge,
		nodePubKey:       nodePubKey,
		memPoolLock:      &sync.Mutex{},
		processedMemPool: make(map[string]bool),
		minRelayFeeSats:  1000, // 1000 sats is the default minimal relay fee
		tssKeySigner:     tssKm,
	}

	var path string // if not set later, will in memory storage
	if len(c.cfg.BlockScanner.DBPath) > 0 {
		path = fmt.Sprintf("%s/%s", c.cfg.BlockScanner.DBPath, c.cfg.BlockScanner.ChainID)
	}
	storage, err := blockscanner.NewBlockScannerStorage(path)
	if err != nil {
		return c, fmt.Errorf("fail to create blockscanner storage: %w", err)
	}

	c.blockScanner, err = blockscanner.NewBlockScanner(c.cfg.BlockScanner, storage, m, bridge, c)
	if err != nil {
		return c, fmt.Errorf("fail to create block scanner: %w", err)
	}

	dbAccessor, err := NewLevelDBBlockMetaAccessor(storage.GetInternalDb())
	if err != nil {
		return c, fmt.Errorf("fail to create utxo accessor: %w", err)
	}
	c.blockMetaAccessor = dbAccessor

	if err := c.registerAddressInWalletAsWatch(c.nodePubKey); err != nil {
		return nil, fmt.Errorf("fail to register (%s): %w", c.nodePubKey, err)
	}
	c.updateNetworkInfo()
	return c, nil
}

// Start starts the block scanner
func (c *Client) Start(globalTxsQueue chan types.TxIn, globalErrataQueue chan types.ErrataBlock) {
	c.tssKeySigner.Start()
	c.blockScanner.Start(globalTxsQueue)
	c.globalErrataQueue = globalErrataQueue
}

// Stop stops the block scanner
func (c *Client) Stop() {
	c.tssKeySigner.Stop()
	c.blockScanner.Stop()
}

// GetConfig - get the chain configuration
func (c *Client) GetConfig() config.ChainConfiguration {
	return c.cfg
}

func (c *Client) IsBlockScannerHealthy() bool {
	return c.blockScanner.IsHealthy()
}

// GetChain returns BCH Chain
func (c *Client) GetChain() common.Chain {
	return common.BCHChain
}

// GetHeight returns current block height
func (c *Client) GetHeight() (int64, error) {
	return c.client.GetBlockCount()
}

// GetAddress returns address from pubkey
func (c *Client) GetAddress(poolPubKey common.PubKey) string {
	addr, err := poolPubKey.GetAddress(common.BCHChain)
	if err != nil {
		c.logger.Error().Err(err).Str("pool_pub_key", poolPubKey.String()).Msg("fail to get pool address")
		return ""
	}
	return addr.String()
}

// getUTXOs send a request to bitcond RPC endpoint to query all the UTXO
func (c *Client) getUTXOs(minConfirm, MaximumConfirm int, pkey common.PubKey) ([]btcjson.ListUnspentResult, error) {
	bchAddress, err := pkey.GetAddress(common.BCHChain)
	if err != nil {
		return nil, fmt.Errorf("fail to get BCH Address for pubkey(%s): %w", pkey, err)
	}
	addr, err := bchutil.DecodeAddress(bchAddress.String(), c.getChainCfg())
	if err != nil {
		return nil, fmt.Errorf("fail to decode BCH address(%s): %w", bchAddress.String(), err)
	}
	return c.client.ListUnspentMinMaxAddresses(minConfirm, MaximumConfirm, []bchutil.Address{
		addr,
	})
}

// GetAccount returns account with balance for an address
func (c *Client) GetAccount(pkey common.PubKey) (common.Account, error) {
	acct := common.Account{}
	if pkey.IsEmpty() {
		return acct, errors.New("pubkey can't be empty")
	}
	utxos, err := c.getUTXOs(0, MaximumConfirmation, pkey)
	if err != nil {
		return acct, fmt.Errorf("fail to get UTXOs: %w", err)
	}
	total := 0.0
	for _, item := range utxos {
		total += item.Amount
	}
	totalAmt, err := bchutil.NewAmount(total)
	if err != nil {
		return acct, fmt.Errorf("fail to convert total amount: %w", err)
	}
	return common.NewAccount(0, 0, common.Coins{
		common.NewCoin(common.BCHAsset, cosmos.NewUint(uint64(totalAmt))),
	}, false), nil
}

func (c *Client) GetAccountByAddress(string) (common.Account, error) {
	return common.Account{}, nil
}

func (c *Client) getAsgardAddress() ([]common.Address, error) {
	if time.Now().Sub(c.lastAsgard) < constants.ThorchainBlockTime && c.asgardAddresses != nil {
		return c.asgardAddresses, nil
	}
	vaults, err := c.bridge.GetAsgards()
	if err != nil {
		return nil, fmt.Errorf("fail to get asgards : %w", err)
	}

	for _, v := range vaults {
		addr, err := v.PubKey.GetAddress(common.BCHChain)
		if err != nil {
			c.logger.Err(err).Msg("fail to get address")
			continue
		}
		found := false
		for _, item := range c.asgardAddresses {
			if item.Equals(addr) {
				found = true
				break
			}
		}
		if !found {
			c.asgardAddresses = append(c.asgardAddresses, addr)
		}

	}
	if len(c.asgardAddresses) > MaxAsgardAddresses {
		startIdx := len(c.asgardAddresses) - MaxAsgardAddresses
		c.asgardAddresses = c.asgardAddresses[startIdx:]
	}
	c.lastAsgard = time.Now()
	return c.asgardAddresses, nil
}

func (c *Client) isFromAsgard(txIn types.TxInItem) bool {
	asgards, err := c.getAsgardAddress()
	if err != nil {
		c.logger.Err(err).Msg("fail to get asgard addresses")
		return false
	}
	isFromAsgard := false
	for _, addr := range asgards {
		if addr.String() == txIn.Sender {
			isFromAsgard = true
			break
		}
	}
	return isFromAsgard
}

// OnObservedTxIn gets called from observer when we have a valid observation
// For bitcoin cash chain client we want to save the utxo we can spend later to sign
func (c *Client) OnObservedTxIn(txIn types.TxInItem, blockHeight int64) {
	hash, err := chainhash.NewHashFromStr(txIn.Tx)
	if err != nil {
		c.logger.Error().Err(err).Str("txID", txIn.Tx).Msg("fail to add spendable utxo to storage")
		return
	}
	blockMeta, err := c.blockMetaAccessor.GetBlockMeta(blockHeight)
	if err != nil {
		c.logger.Err(err).Msgf("fail to get block meta on block height(%d)", blockHeight)
		return
	}
	if blockMeta == nil {
		blockMeta = NewBlockMeta("", blockHeight, "")
	}

	if c.isFromAsgard(txIn) {
		c.logger.Debug().Msgf("add hash %s as self transaction,block height:%d", hash.String(), blockHeight)
		blockMeta.AddSelfTransaction(hash.String())
	} else {
		// add the transaction to block meta
		blockMeta.AddCustomerTransaction(hash.String())
	}
	if err := c.blockMetaAccessor.SaveBlockMeta(blockHeight, blockMeta); err != nil {
		c.logger.Err(err).Msgf("fail to save block meta to storage,block height(%d)", blockHeight)
	}
}

func (c *Client) processReorg(block *btcjson.GetBlockVerboseTxResult) ([]types.TxIn, error) {
	previousHeight := block.Height - 1
	prevBlockMeta, err := c.blockMetaAccessor.GetBlockMeta(previousHeight)
	if err != nil {
		return nil, fmt.Errorf("fail to get block meta of height(%d) : %w", previousHeight, err)
	}
	if prevBlockMeta == nil {
		return nil, nil
	}
	// the block's previous hash need to be the same as the block hash chain client recorded in block meta
	// blockMetas[PreviousHeight].BlockHash == Block.PreviousHash
	if strings.EqualFold(prevBlockMeta.BlockHash, block.PreviousHash) {
		return nil, nil
	}

	c.logger.Info().Msgf("re-org detected, current block height:%d ,previous block hash is : %s , however block meta at height: %d, block hash is %s", block.Height, block.PreviousHash, prevBlockMeta.Height, prevBlockMeta.BlockHash)
	blockHeights, err := c.reConfirmTx()
	if err != nil {
		c.logger.Err(err).Msgf("fail to reprocess all txs")
	}
	var txIns []types.TxIn
	for _, item := range blockHeights {
		c.logger.Info().Msgf("rescan block height: %d", item)
		b, err := c.getBlock(item)
		if err != nil {
			c.logger.Err(err).Msgf("fail to get block from RPC for height:%d", item)
			continue
		}
		txIn, err := c.extractTxs(b)
		if err != nil {
			c.logger.Err(err).Msgf("fail to extract txIn from block")
			continue
		}

		if len(txIn.TxArray) == 0 {
			continue
		}
		txIns = append(txIns, txIn)
	}
	return txIns, nil
}

// reConfirmTx will be kicked off only when chain client detected a re-org on bitcoin cash chain
// it will read through all the block meta data from local storage , and go through all the UTXOs.
// For each UTXO , it will send a RPC request to bitcoin cash chain , double check whether the TX exist or not
// if the tx still exist , then it is all good, if a transaction previous we detected , however doesn't exist anymore , that means
// the transaction had been removed from chain,  chain client should report to thorchain
func (c *Client) reConfirmTx() ([]int64, error) {
	blockMetas, err := c.blockMetaAccessor.GetBlockMetas()
	if err != nil {
		return nil, fmt.Errorf("fail to get block metas from local storage: %w", err)
	}
	var rescanBlockHeights []int64
	for _, blockMeta := range blockMetas {
		var errataTxs []types.ErrataTx
		for _, tx := range blockMeta.CustomerTransactions {
			h, err := chainhash.NewHashFromStr(tx)
			if err != nil {
				c.logger.Info().Msgf("%s invalid transaction hash", tx)
				continue
			}
			if c.confirmTx(h) {
				c.logger.Info().Msgf("block height: %d, tx: %s still exist", blockMeta.Height, tx)
				continue
			}
			// this means the tx doesn't exist in chain ,thus should errata it
			errataTxs = append(errataTxs, types.ErrataTx{
				TxID:  common.TxID(tx),
				Chain: common.BCHChain,
			})
			blockMeta.RemoveCustomerTransaction(tx)
		}
		if len(errataTxs) > 0 {
			c.globalErrataQueue <- types.ErrataBlock{
				Height: blockMeta.Height,
				Txs:    errataTxs,
			}
		}
		// Let's get the block again to fix the block hash
		r, err := c.getBlock(blockMeta.Height)
		if err != nil {
			c.logger.Err(err).Msgf("fail to get block verbose tx result: %d", blockMeta.Height)
		}
		if !strings.EqualFold(blockMeta.BlockHash, r.Hash) {
			rescanBlockHeights = append(rescanBlockHeights, blockMeta.Height)
		}
		blockMeta.PreviousHash = r.PreviousHash
		blockMeta.BlockHash = r.Hash
		if err := c.blockMetaAccessor.SaveBlockMeta(blockMeta.Height, blockMeta); err != nil {
			c.logger.Err(err).Msgf("fail to save block meta of height: %d ", blockMeta.Height)
		}
	}
	return rescanBlockHeights, nil
}

// confirmTx check a tx is valid on chain post reorg
func (c *Client) confirmTx(txHash *chainhash.Hash) bool {
	// GetRawTransaction, it should check transaction in mempool as well
	_, err := c.client.GetRawTransaction(txHash)
	if err == nil {
		// exist , all good
		return true
	}
	c.logger.Err(err).Msgf("fail to get tx (%s) from chain", txHash)
	// double check mempool
	_, err = c.client.GetMempoolEntry(txHash.String())
	if err != nil {
		c.logger.Err(err).Msgf("fail to get tx(%s) from mempool", txHash)
		return false
	}
	return true
}

func (c *Client) removeFromMemPoolCache(hash string) {
	c.memPoolLock.Lock()
	defer c.memPoolLock.Unlock()
	delete(c.processedMemPool, hash)
}

func (c *Client) tryAddToMemPoolCache(hash string) bool {
	if c.processedMemPool[hash] {
		return false
	}
	c.memPoolLock.Lock()
	defer c.memPoolLock.Unlock()
	c.processedMemPool[hash] = true
	return true
}

func (c *Client) getMemPool(height int64) (types.TxIn, error) {
	hashes, err := c.client.GetRawMempool()
	if err != nil {
		return types.TxIn{}, fmt.Errorf("fail to get tx hashes from mempool: %w", err)
	}
	txIn := types.TxIn{
		Chain:   c.GetChain(),
		MemPool: true,
	}
	for _, h := range hashes {
		// this hash had been processed before , ignore it
		if !c.tryAddToMemPoolCache(h.String()) {
			c.logger.Debug().Msgf("%s had been processed , ignore", h.String())
			continue
		}

		c.logger.Debug().Msgf("process hash %s", h.String())
		result, err := c.client.GetRawTransactionVerbose(h)
		if err != nil {
			return types.TxIn{}, fmt.Errorf("fail to get raw transaction verbose with hash(%s): %w", h.String(), err)
		}
		txInItem, err := c.getTxIn(result, height)
		if err != nil {
			c.logger.Error().Err(err).Msg("fail to get TxInItem")
			continue
		}
		if txInItem.IsEmpty() {
			continue
		}
		if txInItem.Coins.IsEmpty() {
			continue
		}
		txIn.TxArray = append(txIn.TxArray, txInItem)
	}
	txIn.Count = strconv.Itoa(len(txIn.TxArray))
	return txIn, nil
}

// FetchMemPool retrieves txs from mempool
func (c *Client) FetchMemPool(height int64) (types.TxIn, error) {
	// make sure client doesn't scan mempool too much
	diff := time.Now().Sub(c.lastMemPoolScan)
	if diff < constants.ThorchainBlockTime {
		return types.TxIn{}, nil
	}
	c.lastMemPoolScan = time.Now()
	return c.getMemPool(height)
}

// FetchTxs retrieves txs for a block height
func (c *Client) FetchTxs(height int64) (types.TxIn, error) {
	txIn := types.TxIn{
		Chain:   common.BCHChain,
		TxArray: nil,
	}
	block, err := c.getBlock(height)
	if err != nil {
		if rpcErr, ok := err.(*btcjson.RPCError); ok && rpcErr.Code == btcjson.ErrRPCInvalidParameter {
			// this means the tx had been broadcast to chain, it must be another signer finished quicker then us
			return txIn, btypes.UnavailableBlock
		}
		return txIn, fmt.Errorf("fail to get block: %w", err)
	}

	// if somehow the block is not valid
	if block.Hash == "" && block.PreviousHash == "" {
		return txIn, fmt.Errorf("fail to get block: %w", err)
	}
	c.currentBlockHeight = height
	reScannedTxs, err := c.processReorg(block)
	if err != nil {
		c.logger.Err(err).Msg("fail to process bitcoin cash re-org")
	}
	if len(reScannedTxs) > 0 {
		for _, item := range reScannedTxs {
			if len(item.TxArray) == 0 {
				continue
			}
			txIn.TxArray = append(txIn.TxArray, item.TxArray...)
		}
	}

	blockMeta, err := c.blockMetaAccessor.GetBlockMeta(block.Height)
	if err != nil {
		return types.TxIn{}, fmt.Errorf("fail to get block meta from storage: %w", err)
	}
	if blockMeta == nil {
		blockMeta = NewBlockMeta(block.PreviousHash, block.Height, block.Hash)
	} else {
		blockMeta.PreviousHash = block.PreviousHash
		blockMeta.BlockHash = block.Hash
	}

	if err := c.blockMetaAccessor.SaveBlockMeta(block.Height, blockMeta); err != nil {
		return types.TxIn{}, fmt.Errorf("fail to save block meta into storage: %w", err)
	}
	pruneHeight := height - BlockCacheSize
	if pruneHeight > 0 {
		defer func() {
			if err := c.blockMetaAccessor.PruneBlockMeta(pruneHeight); err != nil {
				c.logger.Err(err).Msgf("fail to prune block meta, height(%d)", pruneHeight)
			}
		}()
	}

	txInBlock, err := c.extractTxs(block)
	if err != nil {
		return types.TxIn{}, fmt.Errorf("fail to extract txs from block: %w", err)
	}
	if len(txInBlock.TxArray) > 0 {
		txIn.TxArray = append(txIn.TxArray, txInBlock.TxArray...)
	}
	c.updateNetworkInfo()
	if err := c.sendNetworkFee(height); err != nil {
		c.logger.Err(err).Msg("fail to send network fee")
	}
	txIn.Count = strconv.Itoa(len(txIn.TxArray))
	return txIn, nil
}

func (c *Client) updateNetworkInfo() {
	infoResult, err := c.client.RawRequest("getnetworkinfo", []json.RawMessage{})
	if err != nil {
		c.logger.Err(err).Msg("fail to get network info")
		return
	}
	var networkInfo struct {
		RelayFee float64 `json:"relayfee"`
	}
	err = json.Unmarshal(infoResult, &networkInfo)
	if err != nil {
		c.logger.Err(err).Msg("fail to get network info")
		return
	}
	amt, err := bchutil.NewAmount(networkInfo.RelayFee)
	if err != nil {
		c.logger.Err(err).Msg("fail to get minimum relay fee")
		return
	}
	c.minRelayFeeSats = uint64(amt.ToUnit(bchutil.AmountSatoshi))
}

func (c *Client) sendNetworkFee(height int64) error {
	heightJSON, err := json.Marshal(height)
	if err != nil {
		return fmt.Errorf("fail to get block stats: %w", err)
	}
	rawResult, err := c.client.RawRequest("getblockstats", []json.RawMessage{heightJSON})
	if err != nil {
		return fmt.Errorf("fail to get block stats: %w", err)
	}
	var result struct {
		AverageFeeRate float64 `json:"avgfeerate"`
	}
	err = json.Unmarshal(rawResult, &result)
	if err != nil {
		return fmt.Errorf("fail to get block stats: %w", err)
	}
	// fee rate and tx size should not be 0
	if result.AverageFeeRate == 0 {
		return nil
	}
	feeRate := uint64(result.AverageFeeRate * common.One)
	if EstimateAverageTxSize*feeRate < c.minRelayFeeSats {
		feeRate = c.minRelayFeeSats / EstimateAverageTxSize
		if feeRate*EstimateAverageTxSize < c.minRelayFeeSats {
			feeRate++
		}
	}

	txid, err := c.bridge.PostNetworkFee(height, common.BCHChain, uint64(EstimateAverageTxSize), feeRate)
	if err != nil {
		return fmt.Errorf("fail to post network fee to thornode: %w", err)
	}
	c.logger.Debug().Str("txid", txid.String()).Msg("send network fee to THORNode successfully")
	return nil
}

// getBlock retrieves block from chain for a block height
func (c *Client) getBlock(height int64) (*btcjson.GetBlockVerboseTxResult, error) {
	hash, err := c.client.GetBlockHash(height)
	if err != nil {
		return &btcjson.GetBlockVerboseTxResult{}, err
	}
	return c.client.GetBlockVerboseTx(hash)
}

func (c *Client) getTxIn(tx *btcjson.TxRawResult, height int64) (types.TxInItem, error) {
	if c.ignoreTx(tx) {
		c.logger.Debug().Msgf("ignore tx (%s) at height %d, not correct format", tx.Txid, height)
		return types.TxInItem{}, nil
	}

	sender, err := c.getSender(tx)
	if err != nil {
		return types.TxInItem{}, fmt.Errorf("fail to get sender from tx: %w", err)
	}
	memo, err := c.getMemo(tx)
	if err != nil {
		return types.TxInItem{}, fmt.Errorf("fail to get memo from tx: %w", err)
	}
	if len([]byte(memo)) > constants.MaxMemoSize {
		return types.TxInItem{}, fmt.Errorf("memo (%s) longer than max allow length(%d)", memo, constants.MaxMemoSize)
	}
	output, err := c.getOutput(sender, tx)
	if err != nil {
		return types.TxInItem{}, fmt.Errorf("fail to get output from tx: %w", err)
	}
	amount, err := bchutil.NewAmount(output.Value)
	if err != nil {
		return types.TxInItem{}, fmt.Errorf("fail to parse float64: %w", err)
	}
	amt := uint64(amount.ToUnit(bchutil.AmountSatoshi))

	gas, err := c.getGas(tx)
	if err != nil {
		return types.TxInItem{}, fmt.Errorf("fail to get gas from tx: %w", err)
	}

	to := c.stripAddress(output.ScriptPubKey.Addresses[0])

	return types.TxInItem{
		BlockHeight: height,
		Tx:          tx.Txid,
		Sender:      sender,
		To:          to,
		Coins: common.Coins{
			common.NewCoin(common.BCHAsset, cosmos.NewUint(amt)),
		},
		Memo: memo,
		Gas:  gas,
	}, nil
}

// stripAddress removes prefix on bch addresses
func (c *Client) stripAddress(addr string) string {
	split := strings.Split(addr, ":")
	if len(split) > 1 {
		return split[1]
	}
	return split[0]
}

// extractTxs extracts txs from a block to type TxIn
func (c *Client) extractTxs(block *btcjson.GetBlockVerboseTxResult) (types.TxIn, error) {
	txIn := types.TxIn{
		Chain:   c.GetChain(),
		MemPool: false,
	}
	var txItems []types.TxInItem
	for _, tx := range block.Tx {
		// mempool transaction get committed to block , thus remove it from mempool cache
		c.removeFromMemPoolCache(tx.Hash)
		txInItem, err := c.getTxIn(&tx, block.Height)
		if err != nil {
			c.logger.Err(err).Msg("fail to get TxInItem")
			continue
		}
		if txInItem.IsEmpty() {
			continue
		}
		if txInItem.Coins.IsEmpty() {
			continue
		}
		if txInItem.Coins[0].Amount.LTE(cosmos.NewUint(minSpendableUTXOAmountSats)) {
			continue
		}
		txItems = append(txItems, txInItem)
	}
	txIn.TxArray = txItems
	txIn.Count = strconv.Itoa(len(txItems))
	return txIn, nil
}

// ignoreTx checks if we can already ignore a tx according to preset rules
//
// we expect array of "vout" for a BCH to have this format
// OP_RETURN is mandatory only on inbound tx
// vout:0 is our vault
// vout:1 is any any change back to themselves
// vout:2 is OP_RETURN (first 80 bytes)
// vout:3 is OP_RETURN (next 80 bytes)
//
// Rules to ignore a tx are:
// - vout:0 doesn't have coins (value)
// - vout:0 doesn't have address
// - count vouts > 4
// - count vouts with coins (value) > 2
//
func (c *Client) ignoreTx(tx *btcjson.TxRawResult) bool {
	if len(tx.Vin) == 0 || len(tx.Vout) == 0 || len(tx.Vout) > 4 {
		return true
	}
	if tx.Vout[0].Value == 0 || tx.Vin[0].Txid == "" {
		return true
	}
	// TODO check what we do if get multiple addresses
	if len(tx.Vout[0].ScriptPubKey.Addresses) != 1 {
		return true
	}
	countWithOutput := 0
	for idx, vout := range tx.Vout {
		if vout.Value > 0 {
			countWithOutput++
		}
		// check we have one address on the first 2 outputs
		// TODO check what we do if get multiple addresses
		if idx < 2 && vout.ScriptPubKey.Type != "nulldata" && len(vout.ScriptPubKey.Addresses) != 1 {
			return true
		}
	}

	if countWithOutput > 2 {
		return true
	}
	return false
}

// getOutput retrieve the correct output for both inbound
// outbound tx.
// logic is if FROM == TO then its an outbound change output
// back to the vault and we need to select the other output
// as Bifrost already filtered the txs to only have here
// txs with max 2 outputs with values
func (c *Client) getOutput(sender string, tx *btcjson.TxRawResult) (btcjson.Vout, error) {
	for _, vout := range tx.Vout {
		if len(vout.ScriptPubKey.Addresses) != 1 {
			return btcjson.Vout{}, fmt.Errorf("no vout address available")
		}
		if vout.Value > 0 && vout.ScriptPubKey.Addresses[0] != sender {
			return vout, nil
		}
	}
	return btcjson.Vout{}, fmt.Errorf("fail to get output matching criteria")
}

// getSender returns sender address for a btc tx, using vin:0
func (c *Client) getSender(tx *btcjson.TxRawResult) (string, error) {
	if len(tx.Vin) == 0 {
		return "", fmt.Errorf("no vin available in tx")
	}
	txHash, err := chainhash.NewHashFromStr(tx.Vin[0].Txid)
	if err != nil {
		return "", fmt.Errorf("fail to get tx hash from tx id string")
	}
	vinTx, err := c.client.GetRawTransactionVerbose(txHash)
	if err != nil {
		return "", fmt.Errorf("fail to query raw tx from btcd")
	}
	vout := vinTx.Vout[tx.Vin[0].Vout]
	if len(vout.ScriptPubKey.Addresses) == 0 {
		return "", fmt.Errorf("no address available in vout")
	}
	return c.stripAddress(vout.ScriptPubKey.Addresses[0]), nil
}

// getMemo returns memo for a btc tx, using vout OP_RETURN
func (c *Client) getMemo(tx *btcjson.TxRawResult) (string, error) {
	var opreturns string
	for _, vout := range tx.Vout {
		if strings.EqualFold(vout.ScriptPubKey.Type, "nulldata") {
			opreturn := strings.Fields(vout.ScriptPubKey.Asm)
			if len(opreturn) == 2 {
				opreturns += opreturn[1]
			}
		}
	}
	decoded, err := hex.DecodeString(opreturns)
	if err != nil {
		return "", fmt.Errorf("fail to decode OP_RETURN string: %s", opreturns)
	}
	return string(decoded), nil
}

// getGas returns gas for a btc tx (sum vin - sum vout)
func (c *Client) getGas(tx *btcjson.TxRawResult) (common.Gas, error) {
	var sumVin uint64 = 0
	for _, vin := range tx.Vin {
		txHash, err := chainhash.NewHashFromStr(vin.Txid)
		if err != nil {
			return common.Gas{}, fmt.Errorf("fail to get tx hash from tx id string")
		}
		vinTx, err := c.client.GetRawTransactionVerbose(txHash)
		if err != nil {
			return common.Gas{}, fmt.Errorf("fail to query raw tx from bitcoin cash node")
		}

		amount, err := bchutil.NewAmount(vinTx.Vout[vin.Vout].Value)
		if err != nil {
			return nil, err
		}
		sumVin += uint64(amount.ToUnit(bchutil.AmountSatoshi))
	}
	var sumVout uint64 = 0
	for _, vout := range tx.Vout {
		amount, err := bchutil.NewAmount(vout.Value)
		if err != nil {
			return nil, err
		}
		sumVout += uint64(amount.ToUnit(bchutil.AmountSatoshi))
	}
	totalGas := sumVin - sumVout
	return common.Gas{
		common.NewCoin(common.BCHAsset, cosmos.NewUint(totalGas)),
	}, nil
}

// registerAddressInWalletAsWatch make a RPC call to import the address relevant to the given pubkey
// in wallet as watch only , so as when bifrost call ListUnspent , it will return appropriate result
func (c *Client) registerAddressInWalletAsWatch(pkey common.PubKey) error {
	addr, err := pkey.GetAddress(common.BCHChain)
	if err != nil {
		return fmt.Errorf("fail to get BCH address from pubkey(%s): %w", pkey, err)
	}
	c.logger.Info().Msgf("import address: %s", addr.String())
	return c.client.ImportAddressRescan(addr.String(), "", false)
}

// RegisterPublicKey register the given pubkey to bitcoin cash wallet
func (c *Client) RegisterPublicKey(pkey common.PubKey) error {
	return c.registerAddressInWalletAsWatch(pkey)
}

func (c *Client) getCoinbaseValue(blockHeight int64) (int64, error) {
	hash, err := c.client.GetBlockHash(blockHeight)
	if err != nil {
		return 0, fmt.Errorf("fail to get block hash:%w", err)
	}
	result, err := c.client.GetBlockVerboseTx(hash)
	if err != nil {
		return 0, fmt.Errorf("fail to get block verbose tx: %w", err)
	}
	for _, tx := range result.Tx {
		if len(tx.Vin) == 1 && tx.Vin[0].IsCoinBase() {
			total := float64(0)
			for _, opt := range tx.Vout {
				total += opt.Value
			}
			amt, err := bchutil.NewAmount(total)
			if err != nil {
				return 0, fmt.Errorf("fail to parse amount: %w", err)
			}
			return int64(amt), nil
		}
	}
	return 0, fmt.Errorf("fail to get coinbase value")
}

// getBlockRequiredConfirmation find out how many confirmation the given txIn need to have before it can be send to THORChain
func (c *Client) getBlockRequiredConfirmation(txIn types.TxIn, height int64) (int64, error) {
	totalTxValue := txIn.GetTotalTransactionValue(common.BCHAsset, c.asgardAddresses)
	totalFeeAndSubsidy, err := c.getCoinbaseValue(height)
	if err != nil {
		c.logger.Err(err).Msg("fail to get coinbase value")
	}
	if totalFeeAndSubsidy == 0 {
		cbValue, err := bchutil.NewAmount(DefaultCoinbaseValue)
		if err != nil {
			return 0, fmt.Errorf("fail to get default coinbase value: %w", err)
		}
		totalFeeAndSubsidy = int64(cbValue)
	}
	confirm := totalTxValue.MulUint64(2).QuoUint64(uint64(totalFeeAndSubsidy)).Uint64()
	c.logger.Info().Msgf("totalTxValue:%s,total fee and Subsidy:%d,confirmation:%d", totalTxValue, totalFeeAndSubsidy, confirm)
	return int64(confirm), nil
}

func (c *Client) GetConfirmationCount(txIn types.TxIn) int64 {
	if len(txIn.TxArray) == 0 {
		return 0
	}
	// MemPool items doesn't need confirmation
	if txIn.MemPool {
		return 0
	}
	blockHeight := txIn.TxArray[0].BlockHeight
	confirm, err := c.getBlockRequiredConfirmation(txIn, blockHeight)
	c.logger.Info().Msgf("confirmation required: %d", confirm)
	if err != nil {
		c.logger.Err(err).Msg("fail to get block confirmation ")
		return 0
	}
	return confirm
}

// ConfirmationCountReady will be called by observer before send the txIn to thorchain
// confirmation counting is on block level , refer to https://medium.com/coinmonks/1confvalue-a-simple-pow-confirmation-rule-of-thumb-a8d9c6c483dd for detail
func (c *Client) ConfirmationCountReady(txIn types.TxIn) bool {
	if len(txIn.TxArray) == 0 {
		return true
	}
	// MemPool items doesn't need confirmation
	if txIn.MemPool {
		return true
	}
	blockHeight := txIn.TxArray[0].BlockHeight
	confirm := txIn.ConfirmationRequired
	c.logger.Info().Msgf("confirmation required: %d", confirm)
	// every tx in txIn already have at least 1 confirmation
	return (c.currentBlockHeight - blockHeight) >= confirm
}
