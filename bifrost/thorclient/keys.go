package thorclient

import (
	"fmt"
	"os/user"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/client/keys"
	ckeys "github.com/cosmos/cosmos-sdk/crypto/keys"
	"github.com/tendermint/tendermint/crypto"
)

const (
	// folder name for thorchain thorcli
	thorchainCliFolderName = `.thorcli`
)

// Keys manages all the keys used by thorchain
type Keys struct {
	signerName string
	password   string // TODO this is a bad way , need to fix it
	signerInfo ckeys.Info
	kb         ckeys.Keybase
}

// NewKeysWithKeybase create a new instance of Keys
func NewKeysWithKeybase(kb ckeys.Keybase, signerInfo ckeys.Info, password string) *Keys {
	return &Keys{
		signerName: signerInfo.GetName(),
		password:   password,
		signerInfo: signerInfo,
		kb:         kb,
	}
}

// getKeybase will create an instance of Keybase
func GetKeybase(thorchainHome, signerName string) (ckeys.Keybase, ckeys.Info, error) {
	cliDir := thorchainHome
	if len(thorchainHome) == 0 {
		usr, err := user.Current()
		if err != nil {
			return nil, nil, fmt.Errorf("fail to get current user,err:%w", err)
		}
		cliDir = filepath.Join(usr.HomeDir, thorchainCliFolderName)
	}
	kb, err := keys.NewKeyBaseFromDir(cliDir)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to create keybase from folder")
	}
	info, err := kb.Get(signerName)
	if err != nil {
		return kb, nil, fmt.Errorf("fail to get signer info(%s): %w", signerName, err)
	}
	return kb, info, nil
}

// GetSignerInfo return signer info
func (k *Keys) GetSignerInfo() ckeys.Info {
	return k.signerInfo
}

// GetPrivateKey return the private key
func (k *Keys) GetPrivateKey() (crypto.PrivKey, error) {
	return k.kb.ExportPrivateKeyObject(k.signerName, k.password)
}

// GetKeybase return the keybase
func (k *Keys) GetKeybase() ckeys.Keybase {
	return k.kb
}
