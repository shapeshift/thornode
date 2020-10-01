package types

import (
	"gitlab.com/thorchain/thornode/common"
	memo "gitlab.com/thorchain/thornode/x/thorchain/memo"
)

type TxIn struct {
	Count   string       `json:"count"`
	Chain   common.Chain `json:"chain"`
	TxArray []TxInItem   `json:"txArray"`
}

type TxInItem struct {
	BlockHeight         int64         `json:"block_height"`
	Tx                  string        `json:"tx"`
	Memo                string        `json:"memo"`
	Sender              string        `json:"sender"`
	To                  string        `json:"to"` // to adddress
	Coins               common.Coins  `json:"coins"`
	Gas                 common.Gas    `json:"gas"`
	ObservedVaultPubKey common.PubKey `json:"observed_vault_pub_key"`
	MemPool             bool          `json:"mem_pool"` // indicate whether this item is in the mempool or not
}
type TxInStatus byte

const (
	Processing TxInStatus = iota
	Failed
)

// TxInStatusItem represent the TxIn item status
type TxInStatusItem struct {
	TxIn   TxIn       `json:"tx_in"`
	Status TxInStatus `json:"status"`
}

func (t TxInItem) GetAddressToCheck() common.Address {
	m, err := memo.ParseMemo(t.Memo)
	if err != nil {
		return common.NoAddress
	}
	return m.GetDestination()
}

// IsEmpty return true only when every field in TxInItem is empty
func (t TxInItem) IsEmpty() bool {
	if t.BlockHeight == 0 &&
		t.Tx == "" &&
		t.Memo == "" &&
		t.Sender == "" &&
		t.To == "" &&
		t.Coins.IsEmpty() &&
		t.Gas.IsEmpty() &&
		t.ObservedVaultPubKey.IsEmpty() {
		return true
	}
	return false
}
