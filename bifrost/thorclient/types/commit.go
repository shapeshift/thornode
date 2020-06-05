package types

import (
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/bytes"
	"gitlab.com/thorchain/thornode/common"
)

type BadCommit struct {
	Height string      `json:"height"`
	TxHash common.TxID `json:"txhash"`
	Code   int         `json:"code"`
	Log    string      `json:"raw_log"`
}

type Commit struct {
	Height string      `json:"height"`
	TxHash common.TxID `json:"txhash"`
	Logs   []struct {
		Success bool   `json:"success"`
		Log     string `json:"log"`
	} `json:"logs"`
}

type ResultBroadcastTxCommit struct {
	CheckTx   abci.ResponseCheckTx   `json:"check_tx"`
	DeliverTx abci.ResponseDeliverTx `json:"deliver_tx"`
	Hash      bytes.HexBytes         `json:"hash"`
	Height    string                 `json:"height"`
}

type BroadcastResult struct {
	JSONRPC string                  `json:"jsonrpc"`
	Result  ResultBroadcastTxCommit `json:"result"`
}
