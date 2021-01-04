package tss

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"time"

	ctypes "github.com/binance-chain/go-sdk/common/types"
	"github.com/binance-chain/go-sdk/keys"
	"github.com/binance-chain/go-sdk/types/tx"
	"github.com/blang/semver"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tendermint/btcd/btcec"
	"github.com/tendermint/tendermint/crypto"
	"gitlab.com/thorchain/tss/go-tss/keysign"
	"gitlab.com/thorchain/tss/go-tss/tss"

	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// KeySign is a proxy between signer and TSS
type KeySign struct {
	logger         zerolog.Logger
	server         *tss.TssServer
	bridge         *thorclient.ThorchainBridge
	currentVersion semver.Version
	lastCheck      time.Time
}

// NewKeySign create a new instance of KeySign
func NewKeySign(server *tss.TssServer, bridge *thorclient.ThorchainBridge) (*KeySign, error) {
	return &KeySign{
		server: server,
		bridge: bridge,
		logger: log.With().Str("module", "tss_signer").Logger(),
	}, nil
}

// GetPrivKey THORNode don't actually have any private key , but just return something
func (s *KeySign) GetPrivKey() crypto.PrivKey {
	return nil
}

func (s *KeySign) GetAddr() ctypes.AccAddress {
	return nil
}

// ExportAsMnemonic THORNode don't need this function for TSS, just keep it to fulfill KeyManager interface
func (s *KeySign) ExportAsMnemonic() (string, error) {
	return "", nil
}

// ExportAsPrivateKey THORNode don't need this function for TSS, just keep it to fulfill KeyManager interface
func (s *KeySign) ExportAsPrivateKey() (string, error) {
	return "", nil
}

// ExportAsKeyStore THORNode don't need this function for TSS, just keep it to fulfill KeyManager interface
func (s *KeySign) ExportAsKeyStore(password string) (*keys.EncryptedKeyJSON, error) {
	return nil, nil
}

func (s *KeySign) makeSignature(msg tx.StdSignMsg, poolPubKey string) (sig tx.StdSignature, err error) {
	var stdSignature tx.StdSignature
	pk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeAccPub, poolPubKey)
	if err != nil {
		return stdSignature, fmt.Errorf("fail to get pub key: %w", err)
	}
	hashedMsg := crypto.Sha256(msg.Bytes())
	signPack, _, err := s.RemoteSign(hashedMsg, poolPubKey)
	if err != nil {
		return stdSignature, fmt.Errorf("fail to TSS sign: %w", err)
	}

	if signPack == nil {
		return stdSignature, nil
	}
	if pk.VerifyBytes(msg.Bytes(), signPack) {
		s.logger.Info().Msg("we can successfully verify the bytes")
	} else {
		s.logger.Error().Msg("Oops! we cannot verify the bytes")
	}

	return tx.StdSignature{
		AccountNumber: msg.AccountNumber,
		Sequence:      msg.Sequence,
		PubKey:        pk,
		Signature:     signPack,
	}, nil
}

func (s *KeySign) Sign(msg tx.StdSignMsg) ([]byte, error) {
	return nil, nil
}

func (s *KeySign) SignWithPool(msg tx.StdSignMsg, poolPubKey common.PubKey) ([]byte, error) {
	sig, err := s.makeSignature(msg, poolPubKey.String())
	if err != nil {
		return nil, err
	}
	if len(sig.Signature) == 0 {
		return nil, errors.New("fail to make signature")
	}
	newTx := tx.NewStdTx(msg.Msgs, []tx.StdSignature{sig}, msg.Memo, msg.Source, msg.Data)
	bz, err := tx.Cdc.MarshalBinaryLengthPrefixed(&newTx)
	if err != nil {
		return nil, err
	}
	return bz, nil
}

func (s *KeySign) RemoteSign(msg []byte, poolPubKey string) ([]byte, []byte, error) {
	if len(msg) == 0 {
		return nil, nil, nil
	}

	encodedMsg := base64.StdEncoding.EncodeToString(msg)
	rResult, sResult, recoveryId, err := s.toLocalTSSSigner(poolPubKey, encodedMsg)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to tss sign: %w", err)
	}

	if len(rResult) == 0 && len(sResult) == 0 {
		// this means the node tried to do keygen , however this node has not been chosen to take part in the keysign committee
		return nil, nil, nil
	}
	s.logger.Debug().Str("R", rResult).Str("S", sResult).Str("recovery", recoveryId).Msg("tss result")
	data, err := getSignature(rResult, sResult)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to decode tss signature: %w", err)
	}
	bRecoveryId, err := base64.StdEncoding.DecodeString(recoveryId)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to decode recovery id: %w", err)
	}
	return data, bRecoveryId, nil
}

func getSignature(r, s string) ([]byte, error) {
	rBytes, err := base64.StdEncoding.DecodeString(r)
	if err != nil {
		return nil, err
	}
	sBytes, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}

	R := new(big.Int).SetBytes(rBytes)
	S := new(big.Int).SetBytes(sBytes)
	N := btcec.S256().N
	halfOrder := new(big.Int).Rsh(N, 1)
	// see: https://github.com/ethereum/go-ethereum/blob/f9401ae011ddf7f8d2d95020b7446c17f8d98dc1/crypto/signature_nocgo.go#L90-L93
	if S.Cmp(halfOrder) == 1 {
		S.Sub(N, S)
	}

	// Serialize signature to R || S.
	// R, S are padded to 32 bytes respectively.
	rBytes = R.Bytes()
	sBytes = S.Bytes()

	sigBytes := make([]byte, 64)
	// 0 pad the byte arrays from the left if they aren't big enough.
	copy(sigBytes[32-len(rBytes):32], rBytes)
	copy(sigBytes[64-len(sBytes):64], sBytes)
	return sigBytes, nil
}

func (s *KeySign) getVersion() semver.Version {
	requestTime := time.Now()
	if !s.currentVersion.Equals(semver.Version{}) && requestTime.Sub(s.lastCheck).Seconds() < constants.ThorchainBlockTime.Seconds() {
		return s.currentVersion
	}
	version, err := s.bridge.GetThorchainVersion()
	if err != nil {
		s.logger.Err(err).Msg("fail to get current thorchain version")
		return s.currentVersion
	}
	s.currentVersion = version
	s.lastCheck = requestTime
	return s.currentVersion
}

// toLocalTSSSigner will send the request to local signer
func (s *KeySign) toLocalTSSSigner(poolPubKey, msgToSign string) (string, string, string, error) {
	tssMsg := keysign.Request{
		PoolPubKey: poolPubKey,
		Message:    msgToSign,
	}
	currentVersion := s.getVersion()
	tssMsg.Version = currentVersion.String()
	s.logger.Info().Msg("new TSS join party")
	// get current thorchain block height
	blockHeight, err := s.bridge.GetBlockHeight()
	if err != nil {
		return "", "", "", fmt.Errorf("fail to get current thorchain block height: %w", err)
	}
	// this is just round the block height to the nearest 10
	tssMsg.BlockHeight = blockHeight / 10 * 10

	s.logger.Debug().Str("payload", fmt.Sprintf("PoolPubKey: %s, Message: %s, Signers: %+v", tssMsg.PoolPubKey, tssMsg.Message, tssMsg.SignerPubKeys)).Msg("msgToSign to tss Local node")

	ch := make(chan bool, 1)
	defer close(ch)
	timer := time.NewTimer(5 * time.Minute)
	defer timer.Stop()

	var keySignResp keysign.Response
	go func() {
		keySignResp, err = s.server.KeySign(tssMsg)
		ch <- true
	}()

	select {
	case <-ch:
		// do nothing
	case <-timer.C:
		panic("tss signer timeout")
	}

	if err != nil {
		return "", "", "", fmt.Errorf("fail to send request to local TSS node: %w", err)
	}

	// 1 means success,2 means fail , 0 means NA
	if keySignResp.Status == 1 && keySignResp.Blame.IsEmpty() {
		return keySignResp.R, keySignResp.S, keySignResp.RecoveryID, nil
	}

	// Blame need to be passed back to thorchain , so as thorchain can use the information to slash relevant node account
	return "", "", "", NewKeysignError(keySignResp.Blame)
}
