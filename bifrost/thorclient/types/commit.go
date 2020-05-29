package types

import (
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
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

type BroadcastResult struct {
	JSONRPC string                            `json:"jsonrpc"`
	Result  coretypes.ResultBroadcastTxCommit `json:"result"`
}
