package types

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"gitlab.com/thorchain/thornode/common"
)

type TxOutItem struct {
	Chain       common.Chain   `json:"chain"`
	ToAddress   common.Address `json:"to"`
	VaultPubKey common.PubKey  `json:"vault_pubkey"`
	SeqNo       uint64         `json:"seq_no"`
	Coins       common.Coins   `json:"coins"`
	Memo        string         `json:"memo"`
	MaxGas      common.Gas     `json:"max_gas"`
	GasRate     int64          `json:"gas_rate"`
	InHash      common.TxID    `json:"in_hash"`
	OutHash     common.TxID    `json:"out_hash"`
}

func (tx TxOutItem) Hash() string {
	str := fmt.Sprintf("%s|%s|%s|%s|%s|%s", tx.Chain, tx.ToAddress, tx.VaultPubKey, tx.Coins, tx.Memo, tx.InHash)
	return fmt.Sprintf("%X", sha256.Sum256([]byte(str)))
}

func (tx TxOutItem) Equals(tx2 TxOutItem) bool {
	if !tx.Chain.Equals(tx2.Chain) {
		return false
	}
	if !tx.VaultPubKey.Equals(tx2.VaultPubKey) {
		return false
	}
	if !tx.ToAddress.Equals(tx2.ToAddress) {
		return false
	}
	if !tx.Coins.Equals(tx2.Coins) {
		return false
	}
	if !tx.InHash.Equals(tx2.InHash) {
		return false
	}
	if !strings.EqualFold(tx.Memo, tx2.Memo) {
		return false
	}
	if tx.GasRate != tx2.GasRate {
		return false
	}
	return true
}

type TxArrayItem struct {
	Chain       common.Chain   `json:"chain"`
	ToAddress   common.Address `json:"to"`
	VaultPubKey common.PubKey  `json:"vault_pubkey"`
	Coin        common.Coin    `json:"coin"`
	Memo        string         `json:"memo"`
	MaxGas      common.Gas     `json:"max_gas"`
	GasRate     int64          `json:"gas_rate"`
	InHash      common.TxID    `json:"in_hash"`
	OutHash     common.TxID    `json:"out_hash"`
}

func (tx TxArrayItem) TxOutItem() TxOutItem {
	return TxOutItem{
		Chain:       tx.Chain,
		ToAddress:   tx.ToAddress,
		VaultPubKey: tx.VaultPubKey,
		Coins:       common.Coins{tx.Coin},
		Memo:        tx.Memo,
		MaxGas:      tx.MaxGas,
		GasRate:     tx.GasRate,
		InHash:      tx.InHash,
		OutHash:     tx.OutHash,
	}
}

type TxOut struct {
	Height  int64         `json:"height,string"`
	TxArray []TxArrayItem `json:"tx_array"`
}

// GetKey will return a key we can used it to save the infor to level db
func (tai TxArrayItem) GetKey(height int64) string {
	return fmt.Sprintf("%d-%s-%s-%s-%s-%s", height, tai.InHash, tai.VaultPubKey, tai.Memo, tai.Coin, tai.ToAddress)
}
