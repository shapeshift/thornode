package thorclient

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

// GetLastObservedInHeight returns the lastobservedin value for the chain past in
func (b *ThorchainBridge) GetLastObservedInHeight(chain common.Chain) (int64, error) {
	lastblock, err := b.getLastBlock(chain)
	if err != nil {
		return 0, fmt.Errorf("failed to GetLastObservedInHeight: %w", err)
	}
	for _, item := range lastblock {
		if item.Chain == chain {
			return item.LastChainHeight, nil
		}
	}
	return 0, fmt.Errorf("fail to GetLastObservedInHeight,chain(%s)", chain)
}

// GetLastSignedOutHeight returns the lastsignedout value for thorchain
func (b *ThorchainBridge) GetLastSignedOutHeight(chain common.Chain) (int64, error) {
	lastblock, err := b.getLastBlock(chain)
	if err != nil {
		return 0, fmt.Errorf("failed to GetLastSignedOutHeight: %w", err)
	}
	for _, item := range lastblock {
		if item.Chain == chain {
			return item.LastSignedHeight, nil
		}
	}
	return 0, fmt.Errorf("fail to GetLastSignedOutHeight,chain(%s)", chain)
}

// GetBlockHeight returns the current height for thorchain blocks
func (b *ThorchainBridge) GetBlockHeight() (int64, error) {
	lastblock, err := b.getLastBlock(common.EmptyChain)
	if err != nil {
		return 0, fmt.Errorf("failed to GetThorchainHeight: %w", err)
	}
	for _, item := range lastblock {
		return item.Thorchain, nil
	}
	return 0, fmt.Errorf("failed to GetThorchainHeight")
}

// getLastBlock calls the /lastblock/{chain} endpoint and Unmarshal's into the QueryResLastBlockHeights type
func (b *ThorchainBridge) getLastBlock(chain common.Chain) ([]types.QueryResLastBlockHeights, error) {
	path := LastBlockEndpoint
	if !chain.IsEmpty() {
		path = fmt.Sprintf("%s/%s", path, chain.String())
	}
	buf, _, err := b.getWithPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get lastblock: %w", err)
	}
	var lastBlock []types.QueryResLastBlockHeights
	if err := b.cdc.UnmarshalJSON(buf, &lastBlock); err != nil {
		return nil, fmt.Errorf("failed to unmarshal last block: %w", err)
	}
	return lastBlock, nil
}
