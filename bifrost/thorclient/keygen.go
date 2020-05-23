package thorclient

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	btypes "gitlab.com/thorchain/thornode/bifrost/blockscanner/types"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

// GetKeygen retrieves keygen request for the given block height from thorchain
func (b *ThorchainBridge) GetKeygenBlock(blockHeight int64, pk string) (types.KeygenBlock, error) {
	path := fmt.Sprintf("%s/%d/%s", KeygenEndpoint, blockHeight, pk)
	body, status, err := b.getWithPath(path)
	if err != nil {
		if status == http.StatusNotFound {
			return types.KeygenBlock{}, btypes.UnavailableBlock
		}
		b.errCounter.WithLabelValues("fail_get_keygen", strconv.FormatInt(blockHeight, 10)).Inc()
		return types.KeygenBlock{}, fmt.Errorf("failed to get keygen for a block height: %w", err)
	}
	var query types.QueryKeygenBlock
	if err := b.cdc.UnmarshalJSON(body, &query); err != nil {
		b.errCounter.WithLabelValues("fail_unmarshal_keygen", strconv.FormatInt(blockHeight, 10)).Inc()
		return types.KeygenBlock{}, fmt.Errorf("failed to unmarshal Keygen: %w", err)
	}

	buf, err := b.cdc.MarshalBinaryBare(query.KeygenBlock)
	if err != nil {
		return types.KeygenBlock{}, fmt.Errorf("fail to marshal keygen block to json: %w", err)
	}
	sig, _, err := b.keys.kb.Sign(b.keys.signerName, b.keys.password, buf)
	if err != nil {
		return types.KeygenBlock{}, fmt.Errorf("fail to marshal sign keygen: %w", err)
	}

	if base64.StdEncoding.EncodeToString(sig) != query.Signature || query.Signature == "" {
		return types.KeygenBlock{}, errors.New("invalid keygen signature")
	}

	return query.KeygenBlock, nil
}
