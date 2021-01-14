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

// GetChainContracts return a list of chain contracts , which match the requested chains
func (k KVStore) GetChainContracts(ctx cosmos.Context, chains common.Chains) []ChainContract {
	contracts := make([]ChainContract, 0, len(chains))
	for _, item := range chains {
		cc, err := k.GetChainContract(ctx, item)
		if err != nil {
			ctx.Logger().Error("fail to get chain contract", "err", err, "chain", item.String())
			continue
		}
		if cc.IsEmpty() {
			continue
		}
		contracts = append(contracts, cc)
	}
	return contracts
}
