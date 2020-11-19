package tss

import (
	"github.com/binance-chain/go-sdk/keys"
	"github.com/binance-chain/go-sdk/types/tx"

	"gitlab.com/thorchain/thornode/common"
)

// ThorchainKeyManager it is a composite of binance chain keymanager
type ThorchainKeyManager interface {
	keys.KeyManager
	SignWithPool(msg tx.StdSignMsg, poolPubKey common.PubKey) ([]byte, error)
	RemoteSign(msg []byte, poolPubKey string) ([]byte, error)
}
