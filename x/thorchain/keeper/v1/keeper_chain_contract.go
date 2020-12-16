package keeperv1

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// SetChainContract - save chain contract address
func (k KVStore) SetChainContract(ctx cosmos.Context, cc ChainContract) {
	k.set(ctx, k.GetKey(ctx, prefixChainContract, cc.Chain.String()), cc)
}

// GetChainContract - gets chain contract
func (k KVStore) GetChainContract(ctx cosmos.Context, chain common.Chain) (ChainContract, error) {
	var record ChainContract
	_, err := k.get(ctx, k.GetKey(ctx, prefixChainContract, chain.String()), &record)
	return record, err
}

// GetChainContractIterator - get an iterator for chain contract
func (k KVStore) GetChainContractIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getIterator(ctx, prefixChainContract)
}
