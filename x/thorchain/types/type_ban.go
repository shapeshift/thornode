package types

import (
	"errors"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type BanVoter struct {
	NodeAddress cosmos.AccAddress   `json:"node_address"`
	BlockHeight int64               `json:"block_height"`
	Signers     []cosmos.AccAddress `json:"signers"` // node keys of node account saw this tx
}

func NewBanVoter(addr cosmos.AccAddress) BanVoter {
	return BanVoter{
		NodeAddress: addr,
	}
}

func (b BanVoter) IsValid() error {
	if b.NodeAddress.Empty() {
		return errors.New("node address is empty")
	}
	return nil
}

func (b BanVoter) IsEmpty() bool {
	return b.NodeAddress.Empty()
}

func (b BanVoter) String() string {
	return b.NodeAddress.String()
}

// HasSigned - check if given address has signed
func (b BanVoter) HasSigned(signer cosmos.AccAddress) bool {
	for _, sign := range b.Signers {
		if sign.Equals(signer) {
			return true
		}
	}
	return false
}

func (b *BanVoter) Sign(signer cosmos.AccAddress) {
	if !b.HasSigned(signer) {
		b.Signers = append(b.Signers, signer)
	}
}

func (b BanVoter) HasConsensus(nodeAccounts NodeAccounts) bool {
	var count int
	for _, signer := range b.Signers {
		if nodeAccounts.IsNodeKeys(signer) {
			count += 1
		}
	}
	if HasSuperMajority(count, len(nodeAccounts)) {
		return true
	}

	return false
}
