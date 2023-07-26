package utxo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"

	"github.com/btcsuite/btcd/mempool"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	dogewire "github.com/eager7/dogd/wire"
	"github.com/eager7/dogutil"

	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/utxo"
	stypes "gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

////////////////////////////////////////////////////////////////////////////////////////
// Client - Signing
////////////////////////////////////////////////////////////////////////////////////////

// SignTx builds and signs the outbound transaction. Returns the signed transaction, a
// serialized checkpoint on error, and an error.
func (c *Client) SignTx(tx stypes.TxOutItem, thorchainHeight int64) ([]byte, []byte, *stypes.TxInItem, error) {
	if !tx.Chain.Equals(c.cfg.ChainID) {
		return nil, nil, nil, errors.New("wrong chain")
	}

	// skip outbounds without coins
	if tx.Coins.IsEmpty() {
		return nil, nil, nil, nil
	}

	// skip outbounds that have been signed
	if c.signerCacheManager.HasSigned(tx.CacheHash()) {
		c.log.Info().Msgf("ignoring already signed transaction: (%+v)", tx)
		return nil, nil, nil, nil
	}

	// only one keysign per chain at a time
	vaultSignerLock := c.getVaultSignerLock(tx.VaultPubKey.String())
	if vaultSignerLock == nil {
		c.log.Error().Msgf("fail to get signer lock for vault pub key: %s", tx.VaultPubKey.String())
		return nil, nil, nil, fmt.Errorf("fail to get signer lock")
	}
	vaultSignerLock.Lock()
	defer vaultSignerLock.Unlock()

	sourceScript, err := c.getSourceScript(tx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fail to get source pay to address script: %w", err)
	}

	// get chain specific address type
	var outputAddr interface{}
	var outputAddrStr string
	switch c.cfg.ChainID {
	case common.DOGEChain:
		outputAddr, err = dogutil.DecodeAddress(tx.ToAddress.String(), c.getChainCfgDOGE())
		if err != nil {
			return nil, nil, nil, fmt.Errorf("fail to decode next address: %w", err)
		}
		outputAddrStr = outputAddr.(dogutil.Address).String() // trunk-ignore(golangci-lint/forcetypeassert)
	default:
		c.log.Fatal().Msg("unsupported chain")
	}

	// verify address
	if !strings.EqualFold(outputAddrStr, tx.ToAddress.String()) {
		c.log.Info().Msgf("output address: %s, to address: %s can't roundtrip", outputAddrStr, tx.ToAddress.String())
		return nil, nil, nil, nil
	}
	switch outputAddr.(type) {
	case *dogutil.AddressPubKey:
		c.log.Info().Msgf("address: %s is address pubkey type, should not be used", outputAddrStr)
		return nil, nil, nil, nil
	default: // keep lint happy
	}

	// load from checkpoint if it exists
	checkpoint := utxo.SignCheckpoint{}
	redeemTx := &wire.MsgTx{}
	if tx.Checkpoint != nil {
		if err := json.Unmarshal(tx.Checkpoint, &checkpoint); err != nil {
			return nil, nil, nil, fmt.Errorf("fail to unmarshal checkpoint: %w", err)
		}
		if err := redeemTx.Deserialize(bytes.NewReader(checkpoint.UnsignedTx)); err != nil {
			return nil, nil, nil, fmt.Errorf("fail to deserialize tx: %w", err)
		}
	} else {
		redeemTx, checkpoint.IndividualAmounts, err = c.buildTx(tx, sourceScript)
		if err != nil {
			return nil, nil, nil, err
		}
		buf := bytes.NewBuffer([]byte{})
		err = redeemTx.Serialize(buf)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("fail to serialize tx: %w", err)
		}
		checkpoint.UnsignedTx = buf.Bytes()
	}

	// serialize the checkpoint for later
	checkpointBytes, err := json.Marshal(checkpoint)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fail to marshal checkpoint: %w", err)
	}

	// create the list of signing requests
	c.log.Info().Msgf("UTXOs to sign: %d", len(redeemTx.TxIn))
	signings := []struct{ idx, amount int64 }{}
	totalAmount := int64(0)
	for idx, txIn := range redeemTx.TxIn {
		key := fmt.Sprintf("%s-%d", txIn.PreviousOutPoint.Hash, txIn.PreviousOutPoint.Index)
		outputAmount := checkpoint.IndividualAmounts[key]
		totalAmount += outputAmount
		signings = append(signings, struct{ idx, amount int64 }{int64(idx), outputAmount})
	}

	// convert the wire tx to the chain specific tx for signing
	var stx interface{}
	switch c.cfg.ChainID {
	case common.DOGEChain:
		stx = wireToDOGE(redeemTx)
	default:
		c.log.Fatal().Msg("unsupported chain")
	}

	// sign the tx
	wg := &sync.WaitGroup{}
	wg.Add(len(signings))
	mu := &sync.Mutex{}
	var utxoErr error
	for _, signing := range signings {
		go func(i int, amount int64) {
			defer wg.Done()
			var err error

			// chain specific signing
			switch c.cfg.ChainID {
			case common.DOGEChain:
				// trunk-ignore(golangci-lint/forcetypeassert)
				err = c.signUTXODOGE(stx.(*dogewire.MsgTx), tx, amount, sourceScript, i, thorchainHeight)
			default:
				c.log.Fatal().Msg("unsupported chain")
			}

			if err != nil {
				mu.Lock()
				utxoErr = multierror.Append(utxoErr, err)
				mu.Unlock()
			}
		}(int(signing.idx), signing.amount)
	}
	wg.Wait()
	if utxoErr != nil {
		err = utxo.PostKeysignFailure(c.bridge, tx, c.log, thorchainHeight, utxoErr)
		return nil, checkpointBytes, nil, fmt.Errorf("fail to sign the message: %w", err)
	}

	// convert back to wire tx
	switch c.cfg.ChainID {
	case common.DOGEChain:
		redeemTx = dogeToWire(stx.(*dogewire.MsgTx)) // trunk-ignore(golangci-lint/forcetypeassert)
	default:
		c.log.Fatal().Msg("unsupported chain")
	}

	// calculate the final transaction size
	finalSize := redeemTx.SerializeSize()
	finalVBytes := mempool.GetTxVirtualSize(btcutil.NewTx(redeemTx))
	c.log.Info().Msgf("final size: %d, final vbyte: %d", finalSize, finalVBytes)
	var signedTx bytes.Buffer
	if err := redeemTx.Serialize(&signedTx); err != nil {
		return nil, nil, nil, fmt.Errorf("fail to serialize tx to bytes: %w", err)
	}

	// create the observation to be sent by the signer before broadcast
	chainHeight, err := c.rpc.GetBlockCount()
	if err != nil { // fall back to the scanner height, thornode voter does not use height
		chainHeight = c.currentBlockHeight.Load()
	}
	amt := redeemTx.TxOut[0].Value // the first output is the outbound amount
	gas := totalAmount
	for _, txOut := range redeemTx.TxOut { // subtract all vouts to from vins to get the gas
		gas -= txOut.Value
	}
	var txIn *stypes.TxInItem
	sender, err := tx.VaultPubKey.GetAddress(tx.Chain)
	if err == nil {
		txIn = stypes.NewTxInItem(
			chainHeight+1,
			redeemTx.TxHash().String(),
			tx.Memo,
			sender.String(),
			tx.ToAddress.String(),
			common.NewCoins(
				common.NewCoin(c.cfg.ChainID.GetGasAsset(), cosmos.NewUint(uint64(amt))),
			),
			common.Gas(common.NewCoins(
				common.NewCoin(c.cfg.ChainID.GetGasAsset(), cosmos.NewUint(uint64(gas))),
			)),
			tx.VaultPubKey,
			"",
			"",
			nil,
		)
	}

	return signedTx.Bytes(), nil, txIn, nil
}

////////////////////////////////////////////////////////////////////////////////////////
// Client - Broadcast
////////////////////////////////////////////////////////////////////////////////////////

// BroadcastTx will broadcast the given payload.
func (c *Client) BroadcastTx(txOut stypes.TxOutItem, payload []byte) (string, error) {
	height, err := c.rpc.GetBlockCount()
	if err != nil {
		return "", fmt.Errorf("fail to get block height: %w", err)
	}
	bm, err := c.temporalStorage.GetBlockMeta(height)
	if err != nil {
		c.log.Err(err).Int64("height", height).Msg("fail to get blockmeta")
	}
	if bm == nil {
		bm = utxo.NewBlockMeta("", height, "")
	}
	defer func() {
		if err := c.temporalStorage.SaveBlockMeta(height, bm); err != nil {
			c.log.Err(err).Msg("fail to save block metadata")
		}
	}()

	redeemTx := wire.NewMsgTx(wire.TxVersion)
	buf := bytes.NewBuffer(payload)
	if err := redeemTx.Deserialize(buf); err != nil {
		return "", fmt.Errorf("fail to deserialize payload: %w", err)
	}

	// broadcast tx
	txid, err := c.rpc.SendRawTransaction(redeemTx)
	if txid != "" {
		bm.AddSelfTransaction(txid)
	}
	if err != nil {
		if strings.Contains(err.Error(), "already in block chain") {
			c.log.Info().Str("hash", redeemTx.TxHash().String()).Msg("broadcasted by another node")
			return redeemTx.TxHash().String(), nil
		}
		return "", fmt.Errorf("fail to broadcast transaction to chain: %w", err)
	}

	// save tx id to block meta in case we need to errata later
	if err := c.signerCacheManager.SetSigned(txOut.CacheHash(), txid); err != nil {
		c.log.Err(err).Msgf("fail to mark tx out item (%+v) as signed", txOut)
	}

	return txid, nil
}
