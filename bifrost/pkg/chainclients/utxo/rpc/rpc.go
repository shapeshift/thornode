package rpc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/ethereum/go-ethereum/rpc"
)

////////////////////////////////////////////////////////////////////////////////////////
// Interfaces
////////////////////////////////////////////////////////////////////////////////////////

type SerializableTx interface {
	SerializeSize() int
	Serialize(io.Writer) error
}

////////////////////////////////////////////////////////////////////////////////////////
// Client
////////////////////////////////////////////////////////////////////////////////////////

// Client represents a client connection to a UTXO daemon. Internally this uses the
// Ethereum JSON-RPC 2.0 implementation, which allows batching and connection reuse.
type Client struct {
	c       *rpc.Client
	version int
}

// NewClient returns a client connection to a UTXO daemon.
func NewClient(host, user, password string, version int) (*Client, error) {
	authFn := func(h http.Header) error {
		auth := base64.StdEncoding.EncodeToString([]byte(user + ":" + password))
		h.Set("Authorization", fmt.Sprintf("Basic %s", auth))
		return nil
	}

	// default to http if no scheme is specified
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}

	c, err := rpc.DialOptions(context.Background(), host, rpc.WithHTTPAuth(authFn))
	if err != nil {
		return nil, err
	}

	return &Client{
		c:       c,
		version: version,
	}, nil
}

// GetBlockCount returns the number of blocks in the longest block chain.
func (c *Client) GetBlockCount() (int64, error) {
	var count int64
	err := c.c.Call(&count, "getblockcount")
	return count, extractBTCError(err)
}

// SendRawTransaction serializes and sends the transaction.
func (c *Client) SendRawTransaction(tx SerializableTx) (string, error) {
	txHex := ""
	if tx != nil {
		buf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSize()))
		if err := tx.Serialize(buf); err != nil {
			return "", err
		}
		txHex = hex.EncodeToString(buf.Bytes())
	}

	args := []interface{}{txHex}
	if c.version > 19 {
		args = append(args, 0)
	} else {
		args = append(args, false)
	}

	var txid string
	err := c.c.Call(&txid, "sendrawtransaction", args...)
	return txid, extractBTCError(err)
}

// GetBlockHash returns the hash of the block in best-block-chain at the given height.
func (c *Client) GetBlockHash(height int64) (string, error) {
	var hash string
	err := c.c.Call(&hash, "getblockhash", height)
	return hash, extractBTCError(err)
}

// GetBlockVerbose returns information about the block with verbosity 2.
func (c *Client) GetBlockVerboseTxs(hash string, verbosity int) (*btcjson.GetBlockVerboseTxResult, error) {
	var block btcjson.GetBlockVerboseTxResult
	err := c.c.Call(&block, "getblock", hash, 2)
	return &block, extractBTCError(err)
}

// GetBlockVerbose returns information about the block with verbosity 1.
func (c *Client) GetBlockVerbose(hash string) (*btcjson.GetBlockVerboseResult, error) {
	var block btcjson.GetBlockVerboseResult

	args := []interface{}{hash}
	if c.version >= 15 {
		args = append(args, 1)
	} else {
		args = append(args, true)
	}

	err := c.c.Call(&block, "getblock", args...)
	return &block, extractBTCError(err)
}

// GetBlockStats returns statistics about the block at the given height.
func (c *Client) GetBlockStats(height int64) (*btcjson.GetBlockStatsResult, error) {
	var stats btcjson.GetBlockStatsResult
	err := c.c.Call(&stats, "getblockstats", height)
	return &stats, extractBTCError(err)
}

// GetMempoolEntry returns mempool data for the given transaction.
func (c *Client) GetMempoolEntry(txid string) (*btcjson.GetMempoolEntryResult, error) {
	var entry btcjson.GetMempoolEntryResult
	err := c.c.Call(&entry, "getmempoolentry", txid)
	return &entry, extractBTCError(err)
}

// BatchGetMempoolEntry returns mempool data for the given transactions.
func (c *Client) BatchGetMempoolEntry(txids []string) ([]*btcjson.GetMempoolEntryResult, []error, error) {
	// create batch request
	batch := []rpc.BatchElem{}
	for _, txid := range txids {
		batch = append(batch, rpc.BatchElem{
			Method: "getmempoolentry",
			Args:   []interface{}{txid},
			Result: &btcjson.GetMempoolEntryResult{},
		})
	}

	// call batch request
	err := c.c.BatchCall(batch)
	if err != nil {
		return nil, nil, extractBTCError(err)
	}

	// collect results
	errs := make([]error, len(txids))
	results := make([]*btcjson.GetMempoolEntryResult, len(txids))
	var ok bool
	for i, b := range batch {
		results[i], ok = b.Result.(*btcjson.GetMempoolEntryResult)
		if !ok {
			return nil, nil, fmt.Errorf("unexpected type returned from batch call: %T", b.Result)
		}
		errs[i] = extractBTCError(b.Error)
	}

	return results, errs, nil
}

// GetRawMempool returns all transaction ids in the mempool.
func (c *Client) GetRawMempool() ([]string, error) {
	var txids []string
	err := c.c.Call(&txids, "getrawmempool")
	return txids, extractBTCError(err)
}

// GetRawTransaction returns raw transaction representation for given transaction id.
func (c *Client) GetRawTransaction(txid string, verbose bool) (*btcjson.TxRawResult, error) {
	var tx btcjson.TxRawResult
	args := []interface{}{txid}
	if verbose {
		args = append(args, 1)
	}
	err := c.c.Call(&tx, "getrawtransaction", args...)
	return &tx, extractBTCError(err)
}

// BatchGetRawTransaction returns raw transaction representation for given transaction ids.
func (c *Client) BatchGetRawTransaction(txids []string, verbose bool) ([]*btcjson.TxRawResult, []error, error) {
	// create batch request
	batch := make([]rpc.BatchElem, 0, len(txids))
	for _, txid := range txids {
		args := []interface{}{txid}
		if verbose {
			args = append(args, 1)
		}

		batch = append(batch, rpc.BatchElem{
			Method: "getrawtransaction",
			Args:   args,
			Result: &btcjson.TxRawResult{},
		})
	}

	// call batch request
	err := c.c.BatchCall(batch)
	if err != nil {
		return nil, nil, extractBTCError(err)
	}

	// collect results
	errs := make([]error, len(txids))
	results := make([]*btcjson.TxRawResult, len(txids))
	var ok bool
	for i, b := range batch {
		results[i], ok = b.Result.(*btcjson.TxRawResult)
		if !ok {
			return nil, nil, fmt.Errorf("unexpected type returned from batch call: %T", b.Result)
		}
		errs[i] = extractBTCError(b.Error)
	}

	return results, errs, nil
}

// ImportAddress imports the address with no rescan.
func (c *Client) ImportAddress(address string) error {
	err := c.c.Call(nil, "importaddress", address, "", false)
	return extractBTCError(err)
}

// CreateWallet creates a new wallet.
func (c *Client) CreateWallet(name string) error {
	err := c.c.Call(nil, "createwallet", name)
	err = extractBTCError(err)

	// ignore code -4 (wallet already exists)
	if rpcErr, ok := err.(*btcjson.RPCError); ok && rpcErr.Code == btcjson.ErrRPCWallet {
		return nil
	}

	return err
}

// ListUnspent returns all unspent outputs with between min and max confirmations.
func (c *Client) ListUnspent(address string) ([]btcjson.ListUnspentResult, error) {
	var unspent []btcjson.ListUnspentResult
	const minConfirm = 0
	const maxConfirm = 9999999
	err := c.c.Call(&unspent, "listunspent", minConfirm, maxConfirm, []string{address})
	return unspent, extractBTCError(err)
}

func (c *Client) GetNetworkInfo() (*btcjson.GetNetworkInfoResult, error) {
	var info btcjson.GetNetworkInfoResult
	err := c.c.Call(&info, "getnetworkinfo")
	return &info, extractBTCError(err)
}

////////////////////////////////////////////////////////////////////////////////////////
// Helpers
////////////////////////////////////////////////////////////////////////////////////////

// Ethereum RPC returns an error with the response appended to the HTTP status like:
// 404 Not Found: {"error":{"code":-32601,"message":"Method not found"},"id":1}
//
// This makes best effort to extract and return the error as a btcjson.RPCError.
func extractBTCError(err error) error {
	if err == nil {
		return nil
	}

	// split the error into the HTTP status and the JSON response
	parts := strings.SplitN(err.Error(), ": ", 2)
	if len(parts) != 2 {
		return err
	}

	// parse the JSON response
	var response struct {
		Error struct {
			Code    btcjson.RPCErrorCode `json:"code"`
			Message string               `json:"message"`
		} `json:"error"`
	}
	if jsonErr := json.Unmarshal([]byte(parts[1]), &response); jsonErr != nil {
		return err
	}

	// return the error message
	return btcjson.NewRPCError(response.Error.Code, response.Error.Message)
}
