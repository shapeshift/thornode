package types

import "gitlab.com/thorchain/thornode/common"

type RagnarokWithdrawPosition struct {
	Number int64        `json:"number"`
	Pool   common.Asset `json:"pool"`
}

func (r RagnarokWithdrawPosition) IsEmpty() bool {
	return r.Number < 0 || r.Pool.IsEmpty()
}
