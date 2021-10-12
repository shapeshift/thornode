package cosmos

// This file is largely a copy from https://github.com/binance-chain/go-sdk/blob/515ede99ef1b6c7b5eaf27c67fa7984d98be58e3/keys/keys.go.
// Needed a manual way to set `privKey` which the original source doesn't give a means to do so

import (
	"encoding/hex"
	"fmt"

	ctypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/legacy/legacytx"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type keyManager struct {
	privKey  ctypes.PrivKey
	addr     types.AccAddress
	pubkey   common.PubKey
	mnemonic string
}

func (m *keyManager) Pubkey() common.PubKey {
	return m.pubkey
}

func (m *keyManager) ExportAsMnemonic() (string, error) {
	if m.mnemonic == "" {
		return "", fmt.Errorf("this key manager is not recover from mnemonic or auto generated ")
	}
	return m.mnemonic, nil
}

func (m *keyManager) ExportAsPrivateKey() (string, error) {
	return hex.EncodeToString(m.privKey.Bytes()), nil
}

func (m *keyManager) Sign(msg legacytx.StdSignMsg) ([]byte, error) {
	return []byte(""), nil
}

func (m *keyManager) GetPrivKey() ctypes.PrivKey {
	return m.privKey
}

func (m *keyManager) GetAddr() cosmos.AccAddress {
	return m.addr
}
