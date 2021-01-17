package types

import (
	legacytypes "github.com/cosmos/cosmos-sdk/x/auth/legacy/legacytx"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type SetTx struct {
	Mode string `json:"mode"`
	Tx   struct {
		Msg        []cosmos.Msg               `json:"msg"`
		Fee        legacytypes.StdFee         `json:"fee"`
		Signatures []legacytypes.StdSignature `json:"signatures"`
		Memo       string                     `json:"memo"`
	} `json:"tx"`
}
