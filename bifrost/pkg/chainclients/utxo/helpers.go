package utxo

import (
	"github.com/btcsuite/btcd/btcec"
	"github.com/tendermint/tendermint/crypto/secp256k1"

	"gitlab.com/thorchain/thornode/common"
)

func bech32AccountPubKey(key *btcec.PrivateKey) (common.PubKey, error) {
	buf := key.PubKey().SerializeCompressed()
	pk := secp256k1.PubKey(buf)
	return common.NewPubKeyFromCrypto(pk)
}
