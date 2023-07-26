package utxo

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcutil"
	dogetxscript "gitlab.com/thorchain/bifrost/dogd-txscript"

	btypes "gitlab.com/thorchain/thornode/bifrost/blockscanner/types"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/utxo"
	"gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	mem "gitlab.com/thorchain/thornode/x/thorchain/memo"
)

////////////////////////////////////////////////////////////////////////////////////////
// Address Checks
////////////////////////////////////////////////////////////////////////////////////////

func (c *Client) getAsgardAddress() ([]common.Address, error) {
	if time.Since(c.lastAsgard) < constants.ThorchainBlockTime && c.asgardAddresses != nil {
		return c.asgardAddresses, nil
	}
	newAddresses, err := utxo.GetAsgardAddress(c.cfg.ChainID, MaxAsgardAddresses, c.bridge)
	if err != nil {
		return nil, fmt.Errorf("fail to get asgards: %w", err)
	}
	if len(newAddresses) > 0 { // ensure we don't overwrite with empty list
		c.asgardAddresses = newAddresses
	}
	c.lastAsgard = time.Now()
	return c.asgardAddresses, nil
}

func (c *Client) isAsgardAddress(addressToCheck string) bool {
	asgards, err := c.getAsgardAddress()
	if err != nil {
		c.log.Err(err).Msg("fail to get asgard addresses")
		return false
	}
	for _, addr := range asgards {
		if strings.EqualFold(addr.String(), addressToCheck) {
			return true
		}
	}
	return false
}

////////////////////////////////////////////////////////////////////////////////////////
// Reorg Handling
////////////////////////////////////////////////////////////////////////////////////////

func (c *Client) processReorg(block *btcjson.GetBlockVerboseTxResult) ([]types.TxIn, error) {
	previousHeight := block.Height - 1
	prevBlockMeta, err := c.temporalStorage.GetBlockMeta(previousHeight)
	if err != nil {
		return nil, fmt.Errorf("fail to get block meta of height(%d): %w", previousHeight, err)
	}
	if prevBlockMeta == nil {
		return nil, nil
	}
	// the block's previous hash need to be the same as the block hash chain client recorded in block meta
	// blockMetas[PreviousHeight].BlockHash == Block.PreviousHash
	if strings.EqualFold(prevBlockMeta.BlockHash, block.PreviousHash) {
		return nil, nil
	}

	c.log.Info().
		Int64("currentHeight", block.Height).
		Str("previousHash", block.PreviousHash).
		Int64("blockMetaHeight", prevBlockMeta.Height).
		Str("blockMetaHash", prevBlockMeta.BlockHash).
		Msg("re-org detected")

	blockHeights, err := c.reConfirmTx()
	if err != nil {
		c.log.Err(err).Msgf("fail to reprocess all txs")
	}
	var txIns []types.TxIn
	for _, height := range blockHeights {
		c.log.Info().Int64("height", height).Msg("rescanning block")
		b, err := c.getBlock(height)
		if err != nil {
			c.log.Err(err).Int64("height", height).Msg("fail to get block from RPC")
			continue
		}
		txIn, err := c.extractTxs(b)
		if err != nil {
			c.log.Err(err).Msgf("fail to extract txIn from block")
			continue
		}
		if len(txIn.TxArray) == 0 {
			continue
		}
		txIns = append(txIns, txIn)
	}
	return txIns, nil
}

// reConfirmTx is triggered on detection of a re-org. It will iterate all UTXOs in local
// storage, and check if the transaction still exists or not. If the transaction no
// longer exists on chain, then it will send an Errata transaction to Thorchain.
func (c *Client) reConfirmTx() ([]int64, error) {
	blockMetas, err := c.temporalStorage.GetBlockMetas()
	if err != nil {
		return nil, fmt.Errorf("fail to get block metas from local storage: %w", err)
	}
	var rescanBlockHeights []int64
	for _, blockMeta := range blockMetas {
		c.log.Info().Int64("height", blockMeta.Height).Msg("re-confirming transactions")
		var errataTxs []types.ErrataTx
		for _, tx := range blockMeta.CustomerTransactions {
			// check if the tx still exists in chain
			if c.confirmTx(tx) {
				c.log.Info().Int64("height", blockMeta.Height).Str("txid", tx).Msg("transaction still exists")
				continue
			}

			// otherwise add it to the errata txs
			c.log.Info().Int64("height", blockMeta.Height).Str("txid", tx).Msg("errata tx")
			errataTxs = append(errataTxs, types.ErrataTx{
				TxID:  common.TxID(tx),
				Chain: c.cfg.ChainID,
			})

			blockMeta.RemoveCustomerTransaction(tx)
		}

		if len(errataTxs) > 0 {
			c.globalErrataQueue <- types.ErrataBlock{
				Height: blockMeta.Height,
				Txs:    errataTxs,
			}
		}

		// retrieve the block hash again
		hash, err := c.rpc.GetBlockHash(blockMeta.Height)
		if !strings.EqualFold(blockMeta.BlockHash, hash) {
			rescanBlockHeights = append(rescanBlockHeights, blockMeta.Height)
		}
		if err != nil {
			c.log.Err(err).Int64("height", blockMeta.Height).Msg("fail to get block hash")
			continue
		}

		// update the stored block meta with the new block hash
		r, err := c.rpc.GetBlockVerbose(hash)
		if err != nil {
			c.log.Err(err).Int64("height", blockMeta.Height).Msg("fail to get block verbose result")
		}
		blockMeta.PreviousHash = r.PreviousHash
		blockMeta.BlockHash = r.Hash
		if err := c.temporalStorage.SaveBlockMeta(blockMeta.Height, blockMeta); err != nil {
			c.log.Err(err).Int64("height", blockMeta.Height).Msg("fail to save block meta of height")
		}
	}
	return rescanBlockHeights, nil
}

func (c *Client) confirmTx(txid string) bool {
	// since daemons are run with the tx index enabled, this covers block and mempool
	_, err := c.rpc.GetRawTransaction(txid, false)
	if err != nil {
		c.log.Err(err).Str("txid", txid).Msg("fail to get tx")
	}
	return err == nil
}

////////////////////////////////////////////////////////////////////////////////////////
// Mempool Cache
////////////////////////////////////////////////////////////////////////////////////////

func (c *Client) removeFromMemPoolCache(hash string) {
	if err := c.temporalStorage.UntrackMempoolTx(hash); err != nil {
		c.log.Err(err).Str("txid", hash).Msg("fail to remove from mempool cache")
	}
}

func (c *Client) tryAddToMemPoolCache(hash string) bool {
	exist, err := c.temporalStorage.TrackMempoolTx(hash)
	if err != nil {
		c.log.Err(err).Str("txid", hash).Msg("fail to add to mempool cache")
	}
	return exist
}

func (c *Client) canDeleteBlock(blockMeta *utxo.BlockMeta) bool {
	if blockMeta == nil {
		return true
	}
	for _, tx := range blockMeta.SelfTransactions {
		if result, err := c.rpc.GetMempoolEntry(tx); err == nil && result != nil {
			c.log.Info().Str("txid", tx).Msg("still in mempool, block cannot be deleted")
			return false
		}
	}
	return true
}

func (c *Client) updateNetworkInfo() {
	networkInfo, err := c.rpc.GetNetworkInfo()
	if err != nil {
		c.log.Err(err).Msg("fail to get network info")
		return
	}
	amt, err := btcutil.NewAmount(networkInfo.RelayFee)
	if err != nil {
		c.log.Err(err).Msg("fail to get minimum relay fee")
		return
	}
	c.minRelayFeeSats = uint64(amt.ToUnit(btcutil.AmountSatoshi))
}

// sendNetworkFeeFromBlock will send network fee to Thornode based on the block result,
// for chains like Dogecoin which do not support the getblockstats RPC.
func (c *Client) sendNetworkFeeFromBlock(blockResult *btcjson.GetBlockVerboseTxResult) error {
	height := blockResult.Height
	var total float64 // total coinbase value, block reward + all transaction fees in the block
	var totalVSize int32
	for _, tx := range blockResult.Tx {
		if len(tx.Vin) == 1 && tx.Vin[0].IsCoinBase() {
			for _, opt := range tx.Vout {
				total += opt.Value
			}
		} else {
			totalVSize += tx.Vsize
		}
	}

	// skip updating network fee if there are no utxos (except coinbase) in the block
	if totalVSize == 0 {
		return nil
	}
	amt, err := btcutil.NewAmount(total - c.cfg.ChainID.DefaultCoinbase())
	if err != nil {
		return fmt.Errorf("fail to parse total block fee amount, err: %w", err)
	}

	// average fee rate in sats/vbyte or default min relay fee
	feeRateSats := uint64(amt.ToUnit(btcutil.AmountSatoshi) / float64(totalVSize))
	if c.cfg.UTXO.DefaultMinRelayFeeSats > feeRateSats {
		feeRateSats = c.cfg.UTXO.DefaultMinRelayFeeSats
	}

	// round to prevent fee observation noise
	resolution := uint64(c.cfg.BlockScanner.GasPriceResolution)
	feeRateSats = ((feeRateSats / resolution) + 1) * resolution

	// skip fee if less than 1 resolution away from the last
	feeDelta := new(big.Int).Sub(big.NewInt(int64(feeRateSats)), big.NewInt(int64(c.lastFeeRate)))
	feeDelta.Abs(feeDelta)
	if c.lastFeeRate != 0 && feeDelta.Cmp(big.NewInt(c.cfg.BlockScanner.GasPriceResolution)) != 1 {
		return nil
	}

	c.log.Info().
		Int64("height", height).
		Uint64("lastFeeRate", c.lastFeeRate).
		Uint64("feeRateSats", feeRateSats).
		Msg("sendNetworkFee")

	_, err = c.bridge.PostNetworkFee(height, c.cfg.ChainID, c.cfg.UTXO.EstimatedAverageTxSize, feeRateSats)
	if err != nil {
		c.log.Error().Err(err).Msg("failed to post network fee to thornode")
		return fmt.Errorf("fail to post network fee to thornode: %w", err)
	}
	c.lastFeeRate = feeRateSats

	return nil
}

func (c *Client) getBlock(height int64) (*btcjson.GetBlockVerboseTxResult, error) {
	switch c.cfg.ChainID {
	case common.DOGEChain:
		return c.getBlockWithoutVerbose(height)
	default:
		c.log.Fatal().Msg("unsupported chain")
		return nil, nil
	}
}

// getBlockWithoutVerbose will get the block without verbose transaction details, and
// then make batch calls to populate them. This should only be used on chains that do not
// support verbosity level 2 for getblock (currently only dogecoin).
func (c *Client) getBlockWithoutVerbose(height int64) (*btcjson.GetBlockVerboseTxResult, error) {
	hash, err := c.rpc.GetBlockHash(height)
	if err != nil {
		return &btcjson.GetBlockVerboseTxResult{}, err
	}

	// get block without verbose transactions
	block, err := c.rpc.GetBlockVerbose(hash)
	if err != nil {
		return &btcjson.GetBlockVerboseTxResult{}, err
	}

	// copy block data to verbose result
	blockResult := btcjson.GetBlockVerboseTxResult{
		Hash:          block.Hash,
		Confirmations: block.Confirmations,
		StrippedSize:  block.StrippedSize,
		Size:          block.Size,
		Weight:        block.Weight,
		Height:        block.Height,
		Version:       block.Version,
		VersionHex:    block.VersionHex,
		MerkleRoot:    block.MerkleRoot,
		Time:          block.Time,
		Nonce:         block.Nonce,
		Bits:          block.Bits,
		Difficulty:    block.Difficulty,
		PreviousHash:  block.PreviousHash,
		NextHash:      block.NextHash,
	}

	// create our batches
	batches := [][]string{}
	batch := []string{}
	for _, txid := range block.Tx {
		batch = append(batch, txid)
		if len(batch) >= c.cfg.UTXO.TransactionBatchSize {
			batches = append(batches, batch)
			batch = []string{}
		}
	}
	if len(batch) > 0 {
		batches = append(batches, batch)
	}

	// process batch requests one at a time to avoid overloading the node
	retries := 0
	for i := 0; i < len(batches); i++ {
		results, errs, err := c.rpc.BatchGetRawTransaction(batches[i], true)

		// if there was no rpc error, check for any tx errors
		txErrCount := 0
		if err == nil {
			for _, txErr := range errs {
				if txErr != nil {
					err = txErr
				}
				txErrCount++
			}
		}

		// retry the batch a few times on any errors to avoid wasted work
		// TODO: implement partial retry
		if err != nil {
			if retries >= 3 {
				return &btcjson.GetBlockVerboseTxResult{}, err
			}

			c.log.Err(err).Int("txErrCount", txErrCount).Msgf("retrying block txs batch %d", i)
			time.Sleep(time.Second)
			retries++
			i-- // retry the same batch
			continue
		}

		// add transactions to block result
		for _, tx := range results {
			blockResult.Tx = append(blockResult.Tx, *tx)
		}
	}

	return &blockResult, nil
}

func (c *Client) isValidUTXO(hexPubKey string) bool {
	buf, err := hex.DecodeString(hexPubKey)
	if err != nil {
		c.log.Err(err).Msgf("fail to decode hex string, %s", hexPubKey)
		return false
	}

	switch c.cfg.ChainID {
	case common.DOGEChain:
		scriptType, addresses, requireSigs, err := dogetxscript.ExtractPkScriptAddrs(buf, c.getChainCfgDOGE())
		if err != nil {
			c.log.Err(err).Msg("fail to extract pub key script")
			return false
		}
		switch scriptType {
		case dogetxscript.MultiSigTy:
			return false
		default:
			return len(addresses) == 1 && requireSigs == 1
		}
	default:
		c.log.Fatal().Msg("unsupported chain")
		return false
	}
}

func (c *Client) isRBFEnabled(tx *btcjson.TxRawResult) bool {
	for _, vin := range tx.Vin {
		if vin.Sequence < (0xffffffff - 1) {
			return true
		}
	}
	return false
}

func (c *Client) getTxIn(tx *btcjson.TxRawResult, height int64, isMemPool bool) (types.TxInItem, error) {
	if c.ignoreTx(tx, height) {
		c.log.Debug().Int64("height", height).Str("txid", tx.Hash).Msg("ignore tx not matching format")
		return types.TxInItem{}, nil
	}
	// RBF enabled transaction will not be observed until committed to block
	if c.isRBFEnabled(tx) && isMemPool {
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
		return types.TxInItem{}, fmt.Errorf("memo (%s) longer than max allow length (%d)", memo, constants.MaxMemoSize)
	}
	m, err := mem.ParseMemo(common.LatestVersion, memo)
	if err != nil {
		c.log.Debug().Err(err).Str("memo", memo).Msg("fail to parse memo")
	}
	output, err := c.getOutput(sender, tx, m.IsType(mem.TxConsolidate))
	if err != nil {
		if errors.Is(err, btypes.ErrFailOutputMatchCriteria) {
			c.log.Debug().Int64("height", height).Str("txid", tx.Hash).Msg("ignore tx not matching format")
			return types.TxInItem{}, nil
		}
		return types.TxInItem{}, fmt.Errorf("fail to get output from tx: %w", err)
	}
	toAddr := output.ScriptPubKey.Addresses[0]
	if c.isAsgardAddress(toAddr) {
		// only inbound UTXO need to be validated against multi-sig
		if !c.isValidUTXO(output.ScriptPubKey.Hex) {
			return types.TxInItem{}, fmt.Errorf("invalid utxo")
		}
	}
	amount, err := btcutil.NewAmount(output.Value)
	if err != nil {
		return types.TxInItem{}, fmt.Errorf("fail to parse float64: %w", err)
	}
	amt := uint64(amount.ToUnit(btcutil.AmountSatoshi))

	gas, err := c.getGas(tx)
	if err != nil {
		return types.TxInItem{}, fmt.Errorf("fail to get gas from tx: %w", err)
	}
	return types.TxInItem{
		BlockHeight: height,
		Tx:          tx.Txid,
		Sender:      sender,
		To:          toAddr,
		Coins: common.Coins{
			common.NewCoin(c.cfg.ChainID.GetGasAsset(), cosmos.NewUint(amt)),
		},
		Memo: memo,
		Gas:  gas,
	}, nil
}

func (c *Client) extractTxs(block *btcjson.GetBlockVerboseTxResult) (types.TxIn, error) {
	txIn := types.TxIn{
		Chain:   c.GetChain(),
		MemPool: false,
	}
	var txItems []types.TxInItem
	for idx, tx := range block.Tx {
		// mempool transaction get committed to block , thus remove it from mempool cache
		c.removeFromMemPoolCache(tx.Hash)
		txInItem, err := c.getTxIn(&block.Tx[idx], block.Height, false)
		if err != nil {
			c.log.Debug().Err(err).Msg("fail to get TxInItem")
			continue
		}
		if txInItem.IsEmpty() {
			continue
		}
		if txInItem.Coins.IsEmpty() {
			continue
		}
		if txInItem.Coins[0].Amount.LT(c.cfg.ChainID.DustThreshold()) {
			continue
		}
		exist, err := c.temporalStorage.TrackObservedTx(txInItem.Tx)
		if err != nil {
			c.log.Err(err).Msgf("fail to determinate whether hash(%s) had been observed before", txInItem.Tx)
		}
		if !exist {
			c.log.Info().Msgf("tx: %s had been report before, ignore", txInItem.Tx)
			if err := c.temporalStorage.UntrackObservedTx(txInItem.Tx); err != nil {
				c.log.Err(err).Msgf("fail to remove observed tx from cache: %s", txInItem.Tx)
			}
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
// we expect array of "vout" to have this format
// OP_RETURN is mandatory only on inbound tx
// vout:0 is our vault
// vout:1 is any any change back to themselves
// vout:2 is OP_RETURN (first 80 bytes)
// vout:3 is OP_RETURN (next 80 bytes)
//
// Rules to ignore a tx are:
// - count vouts > 4
// - count vouts with coins (value) > 2
func (c *Client) ignoreTx(tx *btcjson.TxRawResult, height int64) bool {
	if len(tx.Vin) == 0 || len(tx.Vout) == 0 || len(tx.Vout) > 4 {
		return true
	}
	if tx.Vin[0].Txid == "" {
		return true
	}
	// LockTime <= current height doesn't affect spendability,
	// and most wallets for users doing Memoless Savers deposits automatically set LockTime to the current height.
	if tx.LockTime > uint32(height) {
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

	// none of the output has any value
	if countWithOutput == 0 {
		return true
	}
	// there are more than two output with value in it, not THORChain format
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
// an exception need to be made for consolidate tx , because consolidate tx will be send from asgard back asgard itself
func (c *Client) getOutput(sender string, tx *btcjson.TxRawResult, consolidate bool) (btcjson.Vout, error) {
	for _, vout := range tx.Vout {
		if strings.EqualFold(vout.ScriptPubKey.Type, "nulldata") {
			continue
		}
		if len(vout.ScriptPubKey.Addresses) != 1 {
			return btcjson.Vout{}, fmt.Errorf("no vout address available")
		}
		if vout.Value > 0 {
			if consolidate && vout.ScriptPubKey.Addresses[0] == sender {
				return vout, nil
			}
			if !consolidate && vout.ScriptPubKey.Addresses[0] != sender {
				return vout, nil
			}
		}
	}
	return btcjson.Vout{}, btypes.ErrFailOutputMatchCriteria
}

// getSender returns sender address for a btc tx, using vin:0
func (c *Client) getSender(tx *btcjson.TxRawResult) (string, error) {
	if len(tx.Vin) == 0 {
		return "", fmt.Errorf("no vin available in tx")
	}
	vinTx, err := c.rpc.GetRawTransaction(tx.Vin[0].Txid, true)
	if err != nil {
		return "", fmt.Errorf("fail to query raw tx")
	}
	vout := vinTx.Vout[tx.Vin[0].Vout]
	if len(vout.ScriptPubKey.Addresses) == 0 {
		return "", fmt.Errorf("no address available in vout")
	}
	return vout.ScriptPubKey.Addresses[0], nil
}

// getMemo returns memo for a btc tx, using vout OP_RETURN
func (c *Client) getMemo(tx *btcjson.TxRawResult) (string, error) {
	var opReturns string
	for _, vOut := range tx.Vout {
		if !strings.EqualFold(vOut.ScriptPubKey.Type, "nulldata") {
			continue
		}
		buf, err := hex.DecodeString(vOut.ScriptPubKey.Hex)
		if err != nil {
			c.log.Err(err).Msg("fail to hex decode scriptPubKey")
			continue
		}

		var asm string
		switch c.cfg.ChainID {
		case common.DOGEChain:
			asm, err = dogetxscript.DisasmString(buf)
		default:
			c.log.Fatal().Msg("unsupported chain")
		}

		if err != nil {
			c.log.Err(err).Msg("fail to disasm script pubkey")
			continue
		}
		opReturnFields := strings.Fields(asm)
		if len(opReturnFields) == 2 {
			decoded, err := hex.DecodeString(opReturnFields[1])
			if err != nil {
				c.log.Err(err).Msgf("fail to decode OP_RETURN string: %s", opReturnFields[1])
				continue
			}
			opReturns += string(decoded)
		}
	}

	return opReturns, nil
}

// getGas returns gas for a tx (sum vin - sum vout)
func (c *Client) getGas(tx *btcjson.TxRawResult) (common.Gas, error) {
	var sumVin uint64 = 0
	for _, vin := range tx.Vin {
		vinTx, err := c.rpc.GetRawTransaction(vin.Txid, true)
		if err != nil {
			return common.Gas{}, fmt.Errorf("fail to query raw tx from node")
		}

		amount, err := btcutil.NewAmount(vinTx.Vout[vin.Vout].Value)
		if err != nil {
			return nil, err
		}
		sumVin += uint64(amount.ToUnit(btcutil.AmountSatoshi))
	}
	var sumVout uint64 = 0
	for _, vout := range tx.Vout {
		amount, err := btcutil.NewAmount(vout.Value)
		if err != nil {
			return nil, err
		}
		sumVout += uint64(amount.ToUnit(btcutil.AmountSatoshi))
	}
	totalGas := sumVin - sumVout
	return common.Gas{
		common.NewCoin(c.cfg.ChainID.GetGasAsset(), cosmos.NewUint(totalGas)),
	}, nil
}

func (c *Client) getCoinbaseValue(blockHeight int64) (int64, error) {
	// TODO: this is inefficient, in particular for dogecoin, investigate coinbase cache
	result, err := c.getBlock(blockHeight)
	if err != nil {
		return 0, fmt.Errorf("fail to get block verbose tx: %w", err)
	}
	for _, tx := range result.Tx {
		if len(tx.Vin) == 1 && tx.Vin[0].IsCoinBase() {
			total := float64(0)
			for _, opt := range tx.Vout {
				total += opt.Value
			}
			amt, err := btcutil.NewAmount(total)
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
	totalTxValue := txIn.GetTotalTransactionValue(c.cfg.ChainID.GetGasAsset(), c.asgardAddresses)
	totalFeeAndSubsidy, err := c.getCoinbaseValue(height)
	if err != nil {
		c.log.Err(err).Msgf("fail to get coinbase value")
	}
	if totalFeeAndSubsidy == 0 {
		cbValue, err := btcutil.NewAmount(c.cfg.ChainID.DefaultCoinbase())
		if err != nil {
			return 0, fmt.Errorf("fail to get default coinbase value: %w", err)
		}
		totalFeeAndSubsidy = int64(cbValue)
	}
	confirm := totalTxValue.QuoUint64(uint64(totalFeeAndSubsidy)).Uint64()
	c.log.Info().Msgf("totalTxValue:%s, totalFeeAndSubsidy:%d, confirm:%d", totalTxValue, totalFeeAndSubsidy, confirm)
	return int64(confirm), nil
}

// getVaultSignerLock , with consolidate UTXO process add into bifrost , there are two entry points for SignTx , one is from signer , signing the outbound tx
// from state machine, the other one will be consolidate utxo process
// this keep a lock per vault pubkey , the goal is each vault we only have one key sign in flight at a time, however different vault can do key sign in parallel
// assume there are multiple asgards(A,B) , and local yggdrasil vault , when A is signing , B and local yggdrasil vault should be able to sign as well
// however if A already has a key sign in flight , bifrost should not kick off another key sign in parallel, otherwise we might double spend some UTXOs
func (c *Client) getVaultSignerLock(vaultPubKey string) *sync.Mutex {
	c.signerLock.Lock()
	defer c.signerLock.Unlock()
	l, ok := c.vaultSignerLocks[vaultPubKey]
	if !ok {
		newLock := &sync.Mutex{}
		c.vaultSignerLocks[vaultPubKey] = newLock
		return newLock
	}
	return l
}
