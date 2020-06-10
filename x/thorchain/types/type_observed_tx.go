package types

import (
	"errors"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type (
	status string
)

const (
	Incomplete status = "incomplete"
	Done       status = "done"
	Reverted   status = "reverted"
)

// Meant to track if THORNode have processed a specific tx
type ObservedTx struct {
	Tx             common.Tx           `json:"tx"`
	Status         status              `json:"status"`
	OutHashes      common.TxIDs        `json:"out_hashes"` // completed chain tx hash. This is a slice to track if we've "double spent" an input
	BlockHeight    int64               `json:"block_height"`
	Signers        []cosmos.AccAddress `json:"signers"` // node keys of node account saw this tx
	ObservedPubKey common.PubKey       `json:"observed_pub_key"`
}

type ObservedTxs []ObservedTx

func NewObservedTx(tx common.Tx, height int64, pk common.PubKey) ObservedTx {
	return ObservedTx{
		Tx:             tx,
		Status:         Incomplete,
		BlockHeight:    height,
		ObservedPubKey: pk,
	}
}

func (tx ObservedTx) Valid() error {
	if err := tx.Tx.IsValid(); err != nil {
		return err
	}

	// Memo should not be empty, but it can't be checked here, because a message failed validation will be rejected by THORNode.
	// Thus THORNode can't refund customer accordingly , which will result fund lost
	if tx.BlockHeight == 0 {
		return errors.New("block height can't be zero")
	}
	if tx.ObservedPubKey.IsEmpty() {
		return errors.New("observed pool pubkey is empty")
	}
	return nil
}

func (tx ObservedTx) IsEmpty() bool {
	return tx.Tx.IsEmpty()
}

func (tx ObservedTx) Equals(tx2 ObservedTx) bool {
	if !tx.Tx.Equals(tx2.Tx) {
		return false
	}
	if !tx.ObservedPubKey.Equals(tx2.ObservedPubKey) {
		return false
	}
	return true
}

func (tx ObservedTx) String() string {
	return tx.Tx.String()
}

// HasSigned - check if given address has signed
func (tx ObservedTx) HasSigned(signer cosmos.AccAddress) bool {
	for _, sign := range tx.Signers {
		if sign.Equals(signer) {
			return true
		}
	}
	return false
}

// Sign add the given node account to signers list
// if the given signer is already in the list, it will return false, otherwise true
func (tx *ObservedTx) Sign(signer cosmos.AccAddress) bool {
	if tx.HasSigned(signer) {
		return false
	}
	tx.Signers = append(tx.Signers, signer)
	return true
}

func (tx *ObservedTx) SetDone(hash common.TxID, numOuts int) {
	for _, done := range tx.OutHashes {
		if done.Equals(hash) {
			return
		}
	}
	tx.OutHashes = append(tx.OutHashes, hash)
	if tx.IsDone(numOuts) {
		tx.Status = Done
	}
}

func (tx *ObservedTx) IsDone(numOuts int) bool {
	if len(tx.OutHashes) >= numOuts {
		return true
	}
	return false
}

type ObservedTxVoter struct {
	TxID    common.TxID `json:"tx_id"`
	Tx      ObservedTx  `json:"tx"` // final consensus transaction
	Height  int64       `json:"height"`
	Txs     ObservedTxs `json:"in_tx"`   // copies of tx in by various observers.
	Actions []TxOutItem `json:"actions"` // outbound txs set to be sent
	OutTxs  common.Txs  `json:"out_txs"` // observed outbound transactions
}

type ObservedTxVoters []ObservedTxVoter

func NewObservedTxVoter(txID common.TxID, txs []ObservedTx) ObservedTxVoter {
	return ObservedTxVoter{
		TxID: txID,
		Txs:  txs,
	}
}

func (tx ObservedTxVoter) Valid() error {
	if tx.TxID.IsEmpty() {
		return errors.New("cannot have an empty tx id")
	}

	for _, in := range tx.Txs {
		if err := in.Valid(); err != nil {
			return err
		}
	}

	return nil
}

func (tx ObservedTxVoter) Key() common.TxID {
	return tx.TxID
}

// String implement fmt.Stringer
func (tx ObservedTxVoter) String() string {
	return tx.TxID.String()
}

// matchActionItem is to check the given outboundTx again the list of actions , return true of the outboundTx matched any of the actions
func (tx ObservedTxVoter) matchActionItem(outboundTx common.Tx) bool {
	for _, toi := range tx.Actions {
		// note: Coins.Contains will match amount as well
		if strings.EqualFold(toi.Memo, outboundTx.Memo) &&
			toi.ToAddress.Equals(outboundTx.ToAddress) &&
			toi.Chain.Equals(outboundTx.Chain) &&
			outboundTx.Coins.Contains(toi.Coin) {
			return true
		}
	}
	return false
}

// AddOutTx trying to add the outbound tx into OutTxs ,
// return value false indicate the given outbound tx doesn't match any of the actions items , node account should be slashed for a malicious tx
// true indicated the outbound tx matched an action item , and it has been added into internal OutTxs
func (tx *ObservedTxVoter) AddOutTx(in common.Tx) bool {
	if !tx.matchActionItem(in) {
		// no action item match the outbound tx
		return false
	}

	for _, t := range tx.OutTxs {
		if in.ID.Equals(t.ID) {
			return true
		}
	}
	tx.OutTxs = append(tx.OutTxs, in)
	for i := range tx.Txs {
		tx.Txs[i].SetDone(in.ID, len(tx.Actions))
	}
	if !tx.Tx.IsEmpty() {
		tx.Tx.SetDone(in.ID, len(tx.Actions))
	}
	return true
}

func (tx *ObservedTxVoter) IsDone() bool {
	return len(tx.Actions) <= len(tx.OutTxs)
}

// Add is trying to add the given observed tx into the voter , if the signer already sign , they will not add twice , it simply return false
func (tx *ObservedTxVoter) Add(observedTx ObservedTx, signer cosmos.AccAddress) bool {
	// check if this signer has already signed, no take backs allowed
	votedIdx := -1
	for idx, transaction := range tx.Txs {
		if !transaction.Equals(observedTx) {
			continue
		}
		votedIdx = idx
		// check whether the signer is already in the list
		for _, siggy := range transaction.Signers {
			if siggy.Equals(signer) {
				return false
			}
		}

	}
	if votedIdx != -1 {
		return tx.Txs[votedIdx].Sign(signer)
	}

	observedTx.Signers = []cosmos.AccAddress{signer}
	tx.Txs = append(tx.Txs, observedTx)
	return true
}

// HasConsensus is to check whether any of the tx in this ObservedTxVoter reach consensus
func (tx ObservedTxVoter) HasConsensus(nodeAccounts NodeAccounts) bool {
	for _, txIn := range tx.Txs {
		var count int
		for _, signer := range txIn.Signers {
			if nodeAccounts.IsNodeKeys(signer) {
				count += 1
			}
		}
		if HasSuperMajority(count, len(nodeAccounts)) {
			return true
		}
	}

	return false
}

// GetTx return the tx that has super majority
func (tx *ObservedTxVoter) GetTx(nodeAccounts NodeAccounts) ObservedTx {
	if !tx.Tx.IsEmpty() {
		return tx.Tx
	}
	for _, txIn := range tx.Txs {
		var count int
		for _, signer := range txIn.Signers {
			if nodeAccounts.IsNodeKeys(signer) {
				count += 1
			}
		}
		if HasSuperMajority(count, len(nodeAccounts)) {
			tx.Tx = txIn
			return txIn
		}
	}

	return ObservedTx{}
}
