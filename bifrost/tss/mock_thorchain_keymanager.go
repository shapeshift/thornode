package tss

import (
	ctypes "github.com/binance-chain/go-sdk/common/types"
	"github.com/binance-chain/go-sdk/keys"
	"github.com/binance-chain/go-sdk/types/tx"
	"github.com/tendermint/tendermint/crypto"

	"gitlab.com/thorchain/thornode/common"
)

// MockThorchainKeymanager is to mock the TSS , so as we could test it
type MockThorchainKeyManager struct{}

func (k *MockThorchainKeyManager) Sign(tx.StdSignMsg) ([]byte, error) {
	return nil, nil
}

func (k *MockThorchainKeyManager) GetPrivKey() crypto.PrivKey {
	return nil
}

func (k *MockThorchainKeyManager) GetAddr() ctypes.AccAddress {
	return nil
}

func (k *MockThorchainKeyManager) ExportAsMnemonic() (string, error) {
	return "", nil
}

func (k *MockThorchainKeyManager) ExportAsPrivateKey() (string, error) {
	return "", nil
}

func (k *MockThorchainKeyManager) ExportAsKeyStore(password string) (*keys.EncryptedKeyJSON, error) {
	return nil, nil
}

func (k *MockThorchainKeyManager) SignWithPool(msg tx.StdSignMsg, poolPubKey common.PubKey) ([]byte, error) {
	return nil, nil
}

func (k *MockThorchainKeyManager) RemoteSign(msg []byte, poolPubKey string) ([]byte, error) {
	// this is the key we are using to test TSS keysign result in BTC chain
	// tthorpub1addwnpepqwm9wsafv26hzqurtjvuuj3xk4j3jyc9yj2uastnmuuqjney9ep3clzt622
	if poolPubKey == "tthorpub1addwnpepqwm9wsafv26hzqurtjvuuj3xk4j3jyc9yj2uastnmuuqjney9ep3clzt622" {
		return getSignature("VqAlcVM+9ciiCL+/VBVNjekbLUjB5/NXI6ui0ZdTRZM=", "ENP93vjudq9s+UQu87nFPDZ1LKNurzRTo/hMIqetAb4=")
	}
	return nil, nil
}
