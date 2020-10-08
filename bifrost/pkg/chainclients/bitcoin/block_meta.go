package bitcoin

import (
	"strings"
)

// BlockMeta is a structure to store the blocks bifrost scanned
type BlockMeta struct {
	PreviousHash         string   `json:"previous_hash"`
	Height               int64    `json:"height"`
	BlockHash            string   `json:"block_hash"`
	SelfTransactions     []string `json:"self_transactions,omitempty"`     // keep the transactions that broadcast by itself
	CustomerTransactions []string `json:"customer_transactions,omitempty"` // keep the transactions that from customer
}

// NewBlockMeta create a new instance of BlockMeta
func NewBlockMeta(previousHash string, height int64, blockHash string) *BlockMeta {
	return &BlockMeta{
		PreviousHash: previousHash,
		Height:       height,
		BlockHash:    blockHash,
	}
}

// TransactionHashExist check whether the given traction hash exist in the block meta
func (b *BlockMeta) TransactionHashExist(hash string) bool {
	for _, item := range b.CustomerTransactions {
		if strings.EqualFold(item, hash) {
			return true
		}
	}
	for _, item := range b.SelfTransactions {
		if strings.EqualFold(item, hash) {
			return true
		}
	}
	return false
}

// AddSelfTransaction add the given Transaction into block meta
func (b *BlockMeta) AddSelfTransaction(txID string) {
	b.SelfTransactions = addTransaction(b.SelfTransactions, txID)
}

func addTransaction(hashes []string, txID string) []string {
	var exist bool
	for _, tx := range hashes {
		if strings.EqualFold(tx, txID) {
			exist = true
			break
		}
	}
	if !exist {
		hashes = append(hashes, txID)
	}
	return hashes
}

// AddCustomerTransaction add the given Transaction into block meta
func (b *BlockMeta) AddCustomerTransaction(txID string) {
	for _, tx := range b.SelfTransactions {
		if strings.EqualFold(tx, txID) {
			return
		}
	}
	b.CustomerTransactions = addTransaction(b.CustomerTransactions, txID)
}

func removeTransaction(hashes []string, txID string) []string {
	idx := 0
	toDelete := false
	for i, tx := range hashes {
		if strings.EqualFold(tx, txID) {
			idx = i
			toDelete = true
			break
		}
	}
	if toDelete {
		hashes = append(hashes[:idx], hashes[idx+1:]...)
	}
	return hashes
}

// RemoveCustomerTransaction remove the given transaction from the block
func (b *BlockMeta) RemoveCustomerTransaction(txID string) {
	b.CustomerTransactions = removeTransaction(b.CustomerTransactions, txID)
}
