package bitcoin

import (
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

// BlockMeta is a structure to store the blocks bifrost scanned
type BlockMeta struct {
	PreviousHash         string           `json:"previous_hash"`
	Height               int64            `json:"height"`
	BlockHash            string           `json:"block_hash"`
	SelfTransactions     []chainhash.Hash `json:"self_transactions"`     // keep the transactions that broadcast by itself
	CustomerTransactions []chainhash.Hash `json:"customer_transactions"` // keep the transactions that from customer
}

// NewBlockMeta create a new instance of BlockMeta
func NewBlockMeta(previousHash string, height int64, blockHash string) *BlockMeta {
	return &BlockMeta{
		PreviousHash: previousHash,
		Height:       height,
		BlockHash:    blockHash,
	}
}

// AddSelfTransaction add the given Transaction into block meta
func (b *BlockMeta) AddSelfTransaction(txID chainhash.Hash) {
	b.SelfTransactions = addTransaction(b.SelfTransactions, txID)
}

func addTransaction(hashes []chainhash.Hash, txID chainhash.Hash) []chainhash.Hash {
	var exist bool
	for _, tx := range hashes {
		if tx.String() == txID.String() {
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
func (b *BlockMeta) AddCustomerTransaction(txID chainhash.Hash) {
	for _, tx := range b.SelfTransactions {
		if tx.String() == txID.String() {
			return
		}
	}
	b.CustomerTransactions = addTransaction(b.CustomerTransactions, txID)
}

func removeTransaction(hashes []chainhash.Hash, txID chainhash.Hash) []chainhash.Hash {
	idx := 0
	for i, tx := range hashes {
		if tx.String() == txID.String() {
			idx = i
			break
		}
	}
	hashes = append(hashes[:idx], hashes[idx+1:]...)
	return hashes
}

// RemoveCustomerTransaction remove the given transaction from the block
func (b *BlockMeta) RemoveCustomerTransaction(txID chainhash.Hash) {
	b.CustomerTransactions = removeTransaction(b.CustomerTransactions, txID)
}
