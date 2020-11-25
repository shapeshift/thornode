package types

import (
	"errors"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type status string

const (
	Incomplete status = "incomplete"
	Done       status = "done"
	Reverted   status = "reverted"
)

// ObservedTx is design to track if THORNode have processed a specific tx
type ObservedTx struct {
	// Tx observed tx information
	Tx common.Tx `json:"tx"`

	// Status of this TX
	Status status `json:"status"`

	// OutHashes completed chain tx hash. This is a slice to track if we've "double spent" an input
	OutHashes common.TxIDs `json:"out_hashes"`

	// BlockHeight which block height this tx was observed
	BlockHeight int64 `json:"block_height"`

	// Signers  node account address saw this tx
	Signers []cosmos.AccAddress `json:"signers"`

	// ObservedPubKey the vault public key that observed this tx
	ObservedPubKey common.PubKey `json:"observed_pub_key"`

	// KeysingMs this field will only have value for outbound tx, indicate how many milliseconds THORChain TSS spent to send it out
	KeysignMs int64 `json:"keysign_ms"` // this field is optional

	// FinaliseHeight indicate when this tx will be finalised
	// When FinaliseHeight == BlockHeight means the ObserveTx is final
	FinaliseHeight int64 `json:"finalise_height"`
}

// ObservedTxs a list of ObservedTx
type ObservedTxs []ObservedTx

// NewObservedTx create a new instance of ObservedTx
func NewObservedTx(tx common.Tx, height int64, pk common.PubKey, finalisedHeight int64) ObservedTx {
	return ObservedTx{
		Tx:             tx,
		Status:         Incomplete,
		BlockHeight:    height,
		ObservedPubKey: pk,
		FinaliseHeight: finalisedHeight,
	}
}

// Valid check whether the observed tx represent valid information
func (tx ObservedTx) Valid() error {
	if err := tx.Tx.Valid(); err != nil {
		return err
	}
	// Memo should not be empty, but it can't be checked here, because a
	// message failed validation will be rejected by THORNode.
	// Thus THORNode can't refund customer accordingly , which will result fund lost
	if tx.BlockHeight <= 0 {
		return errors.New("block height can't be zero")
	}
	if tx.ObservedPubKey.IsEmpty() {
		return errors.New("observed pool pubkey is empty")
	}
	if tx.FinaliseHeight <= 0 {
		return errors.New("finalise block height can't be zero")
	}
	return nil
}

// IsEmpty check whether the Tx is empty
func (tx ObservedTx) IsEmpty() bool {
	return tx.Tx.IsEmpty()
}

// Equals compare two ObservedTx
func (tx ObservedTx) Equals(tx2 ObservedTx) bool {
	if !tx.Tx.Equals(tx2.Tx) {
		return false
	}
	if !tx.ObservedPubKey.Equals(tx2.ObservedPubKey) {
		return false
	}
	if tx.BlockHeight != tx2.BlockHeight {
		return false
	}
	if tx.FinaliseHeight != tx2.FinaliseHeight {
		return false
	}

	return true
}

// IsFinal indcate whether ObserveTx is final
func (tx ObservedTx) IsFinal() bool {
	return tx.FinaliseHeight == tx.BlockHeight
}

// String implement fmt.Stringer
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

// SetDone check the ObservedTx status, update it's status to done if the outbound tx had been processed
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

// IsDone will only return true when the number of out hashes is larger or equals the input number
func (tx *ObservedTx) IsDone(numOuts int) bool {
	if len(tx.OutHashes) >= numOuts {
		return true
	}
	return false
}

// ObservedTxVoter tracks who voted for an observed tx
// only tx reach super majority will be processed by THORChain
// as THORChain start to support PoW like BTC , tx need to do confirmation counting , a tx need to meet confirmation
// counting requirement before it can be processed
type ObservedTxVoter struct {
	TxID            common.TxID `json:"tx_id"`            // Tx Hash
	Tx              ObservedTx  `json:"tx"`               // final consensus transaction
	Height          int64       `json:"height"`           // Block height
	Txs             ObservedTxs `json:"in_tx"`            // copies of tx in by various observers.
	Actions         []TxOutItem `json:"actions"`          // outbound txs set to be sent
	OutTxs          common.Txs  `json:"out_txs"`          // observed outbound transactions
	FinalisedHeight int64       `json:"finalised_height"` // block height the finalised Tx get consensus
	UpdatedVault    bool        `json:"updated_vault"`    // has the fund in this tx been credit to vault
	Reverted        bool        `json:"reverted"`         // reverted using Errata
}

// ObservedTxVoters a list of observed tx voter
type ObservedTxVoters []ObservedTxVoter

// NewObservedTxVoter create a new instance of ObservedTxVoter
func NewObservedTxVoter(txID common.TxID, txs []ObservedTx) ObservedTxVoter {
	observedTxVoter := ObservedTxVoter{
		TxID: txID,
		Txs:  txs,
	}
	return observedTxVoter
}

// Valid check whether the tx is valid , if it is not , then an error will be returned
func (tx ObservedTxVoter) Valid() error {
	if tx.TxID.IsEmpty() {
		return errors.New("cannot have an empty tx id")
	}

	// check all other normal tx
	for _, in := range tx.Txs {
		if err := in.Valid(); err != nil {
			return err
		}
	}

	return nil
}

// Key is to get the txid
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
		matchCoin := outboundTx.Coins.Contains(toi.Coin)
		if !matchCoin && toi.Coin.Asset.Equals(toi.Chain.GetGasAsset()) {
			asset := toi.Chain.GetGasAsset()
			intendToSpend := toi.Coin.Amount.Add(toi.MaxGas.ToCoins().GetCoin(asset).Amount)
			actualSpend := outboundTx.Coins.GetCoin(asset).Amount.Add(outboundTx.Gas.ToCoins().GetCoin(asset).Amount)
			if intendToSpend.Equal(actualSpend) {
				matchCoin = true
			}

		}
		if strings.EqualFold(toi.Memo, outboundTx.Memo) &&
			toi.ToAddress.Equals(outboundTx.ToAddress) &&
			toi.Chain.Equals(outboundTx.Chain) &&
			matchCoin {
			return true
		}
	}
	return false
}

// AddOutTx trying to add the outbound tx into OutTxs ,
// return value false indicate the given outbound tx doesn't match any of the
// actions items , node account should be slashed for a malicious tx
// true indicated the outbound tx matched an action item , and it has been
// added into internal OutTxs
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

// IsDone check whether THORChain finished process the tx, all outbound tx had
// been sent and observed
func (tx *ObservedTxVoter) IsDone() bool {
	return len(tx.Actions) <= len(tx.OutTxs)
}

// Add is trying to add the given observed tx into the voter , if the signer
// already sign , they will not add twice , it simply return false
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

// HasConsensus is to check whether the tx with finalise = false in this ObservedTxVoter reach consensus
// if ObservedTxVoter HasFinalised , then this function will return true as well
func (tx ObservedTxVoter) HasConsensus(nodeAccounts NodeAccounts) bool {
	consensusTx := tx.GetTx(nodeAccounts)
	return !consensusTx.IsEmpty()
}

// HasFinalised is to check whether the tx with finalise = true  reach super majority
func (tx ObservedTxVoter) HasFinalised(nodeAccounts NodeAccounts) bool {
	finalTx := tx.GetTx(nodeAccounts)
	if finalTx.IsEmpty() {
		return false
	}
	return finalTx.IsFinal()
}

// GetTx return the tx that has super majority
func (tx *ObservedTxVoter) GetTx(nodeAccounts NodeAccounts) ObservedTx {
	if !tx.Tx.IsEmpty() && tx.Tx.IsFinal() {
		return tx.Tx
	}
	finalTx := tx.getConsensusTx(nodeAccounts, true)
	if !finalTx.IsEmpty() {
		tx.Tx = finalTx
	} else {
		discoverTx := tx.getConsensusTx(nodeAccounts, false)
		if !discoverTx.IsEmpty() {
			tx.Tx = discoverTx
		}
	}
	return tx.Tx
}

func (tx *ObservedTxVoter) getConsensusTx(accounts NodeAccounts, final bool) ObservedTx {
	for _, txIn := range tx.Txs {
		var count int
		if txIn.IsFinal() != final {
			continue
		}
		for _, signer := range txIn.Signers {
			if accounts.IsNodeKeys(signer) {
				count++
			}
		}
		if HasSuperMajority(count, len(accounts)) {
			return txIn
		}
	}
	return ObservedTx{}
}

func (tx *ObservedTxVoter) SetReverted() {
	for _, item := range tx.Txs {
		item.Status = Reverted
	}

	tx.Reverted = true
	if !tx.Tx.IsEmpty() {
		tx.Tx.Status = Reverted
	}
}
