package types

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
)

// NewChainContract create a new instance of ChainContract
func NewChainContract(chain common.Chain, contract common.Address) ChainContract {
	return ChainContract{
		Chain:    chain,
		Contract: contract,
	}
}

// IsEmpty returns true when both chain and Contract address are empty
func (m *ChainContract) IsEmpty() bool {
	return m.Chain.IsEmpty() || m.Contract.IsEmpty()
}

// String implement fmt.Stringer, return a string representation of ChainContract
func (m *ChainContract) String() string {
	return fmt.Sprintf("%s-%s", m.Chain, m.Contract)
}
