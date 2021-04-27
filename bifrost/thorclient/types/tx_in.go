package types

import (
	"fmt"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	memo "gitlab.com/thorchain/thornode/x/thorchain/memo"
)

type TxIn struct {
	Count                string       `json:"count"`
	Chain                common.Chain `json:"chain"`
	TxArray              []TxInItem   `json:"txArray"`
	Filtered             bool         `json:"filtered"`
	MemPool              bool         `json:"mem_pool"`          // indicate whether this item is in the mempool or not
	SentUnFinalised      bool         `json:"sent_un_finalised"` // indicate whehter unfinalised tx had been sent to THORChain
	Finalised            bool         `json:"finalised"`
	ConfirmationRequired int64        `json:"confirmation_required"`
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

var ErrPanicParseMemo = fmt.Errorf("panic while parse memo")

func (t TxInItem) GetAddressToCheck() (addr common.Address, err error) {
	defer func() {
		if recoverErr := recover(); recoverErr != nil {
			addr = common.NoAddress
			err = fmt.Errorf("fail to parse memo(%s),err:%s,%w", t.Memo, recoverErr, ErrPanicParseMemo)
		}
	}()
	addr = common.NoAddress
	err = nil
	m, parseErr := memo.ParseMemo(t.Memo)
	if parseErr != nil {
		err = parseErr
		return
	}
	addr = m.GetDestination()
	return
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

// GetTotalTransactionValue return the total value of the requested asset
func (t TxIn) GetTotalTransactionValue(asset common.Asset, excludeFrom []common.Address) cosmos.Uint {
	total := cosmos.ZeroUint()
	if len(t.TxArray) == 0 {
		return total
	}
	for _, item := range t.TxArray {
		fromAsgard := false
		for _, fromAddress := range excludeFrom {
			if strings.EqualFold(fromAddress.String(), item.Sender) {
				fromAsgard = true
			}
		}
		if fromAsgard {
			continue
		}
		// skip confirmation counting if it is internal tx
		m, err := memo.ParseMemo(item.Memo)
		if err == nil && m.IsInternal() {
			continue
		}
		c := item.Coins.GetCoin(asset)
		if c.IsEmpty() {
			continue
		}
		total = total.Add(c.Amount)
	}
	return total
}

// GetTotalGas return the total gas
func (t TxIn) GetTotalGas() cosmos.Uint {
	total := cosmos.ZeroUint()
	if len(t.TxArray) == 0 {
		return total
	}
	for _, item := range t.TxArray {
		if item.Gas == nil {
			continue
		}
		if err := item.Gas.Valid(); err != nil {
			continue
		}
		total = total.Add(item.Gas[0].Amount)
	}
	return total
}
