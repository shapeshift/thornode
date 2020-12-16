package types

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
)

// ChainContract is a structure define to represent the contract used by different chain
// At the moment for ETH , it need to have a smart contract address
type ChainContract struct {
	Chain    common.Chain   `json:"chain"`
	Contract common.Address `json:"address"`
}

// NewChainContract create a new instance of ChainContract
func NewChainContract(chain common.Chain, contract common.Address) ChainContract {
	return ChainContract{
		Chain:    chain,
		Contract: contract,
	}
}

// IsEmpty returns true when both chain and Contract address are empty
func (cc ChainContract) IsEmpty() bool {
	return cc.Chain.IsEmpty() || cc.Contract.IsEmpty()
}

// String implement fmt.Stringer, return a string representation of ChainContract
func (cc ChainContract) String() string {
	return fmt.Sprintf("%s-%s", cc.Chain, cc.Contract)
}
