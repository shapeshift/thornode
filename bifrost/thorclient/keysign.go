package thorclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	btypes "gitlab.com/thorchain/thornode/bifrost/blockscanner/types"
	"gitlab.com/thorchain/thornode/bifrost/thorclient/types"
)

var ErrNotFound error = fmt.Errorf("not found")

type QueryKeysign struct {
	Keysign   types.TxOut `json:"keysign"`
	Signature string      `json:"signature"`
}

// GetKeysign retrieves txout from this block height from thorchain
func (b *ThorchainBridge) GetKeysign(blockHeight int64, pk string) (types.TxOut, error) {
	path := fmt.Sprintf("%s/%d/%s", KeysignEndpoint, blockHeight, pk)
	body, status, err := b.getWithPath(path)
	if err != nil {
		b.errCounter.WithLabelValues("fail_get_tx_out", strconv.FormatInt(blockHeight, 10)).Inc()
		if status == http.StatusNotFound {
			return types.TxOut{}, btypes.UnavailableBlock
		}
		return types.TxOut{}, fmt.Errorf("failed to get tx from a block height: %w", err)
	}
	var query QueryKeysign
	if err := json.Unmarshal(body, &query); err != nil {
		b.errCounter.WithLabelValues("fail_unmarshal_tx_out", strconv.FormatInt(blockHeight, 10)).Inc()
		return types.TxOut{}, fmt.Errorf("failed to unmarshal TxOut: %w", err)
	}
	return query.Keysign, nil
}
