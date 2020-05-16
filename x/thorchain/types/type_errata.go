package types

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type ErrataTxVoter struct {
	TxID        common.TxID         `json:"tx_id"`
	Chain       common.Chain        `json:"chain"`
	BlockHeight int64               `json:"block_height"`
	Signers     []cosmos.AccAddress `json:"signers"`
}

func NewErrataTxVoter(txID common.TxID, chain common.Chain) ErrataTxVoter {
	return ErrataTxVoter{
		TxID:  txID,
		Chain: chain,
	}
}

// HasSigned - check if given address has signed
func (errata ErrataTxVoter) HasSigned(signer cosmos.AccAddress) bool {
	for _, sign := range errata.Signers {
		if sign.Equals(signer) {
			return true
		}
	}
	return false
}

// Sign this voter with given signer address
func (errata *ErrataTxVoter) Sign(signer cosmos.AccAddress) bool {
	if errata.HasSigned(signer) {
		return false
	}
	errata.Signers = append(errata.Signers, signer)
	return true
}

// Determine if this errata has enough signers
func (errata *ErrataTxVoter) HasConsensus(nas NodeAccounts) bool {
	var count int
	for _, signer := range errata.Signers {
		if nas.IsNodeKeys(signer) {
			count += 1
		}
	}
	if HasSuperMajority(count, len(nas)) {
		return true
	}

	return false
}

func (errata *ErrataTxVoter) Empty() bool {
	if errata.TxID.IsEmpty() || errata.Chain.IsEmpty() {
		return true
	}
	return false
}

func (errata *ErrataTxVoter) String() string {
	return fmt.Sprintf("%s-%s", errata.Chain.String(), errata.TxID.String())
}
