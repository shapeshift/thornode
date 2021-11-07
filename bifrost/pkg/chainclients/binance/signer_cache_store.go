package binance

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/syndtr/goleveldb/leveldb"
)

const (
	signedCachePrefix = "signed-v3-"
	txMapPrefix       = "tx-map-"
)

// SignerCacheStore is a store to save what tx out item has been signed by this client
type SignerCacheStore struct {
	logger zerolog.Logger
	db     *leveldb.DB
}

// NewSignerCacheStore create a new instance of SignerCacheStore
func NewSignerCacheStore(db *leveldb.DB) (*SignerCacheStore, error) {
	return &SignerCacheStore{
		logger: log.With().Str("module", "binance-signer-cache").Logger(),
		db:     db,
	}, nil
}

// SetSigned update key value store to set the given height and hash as signed
func (s *SignerCacheStore) SetSigned(hash string) error {
	key := s.getSignedKey(hash)
	s.logger.Debug().Msgf("key:%s set to signed", key)
	return s.db.Put([]byte(key), []byte{1}, nil)
}
func (s *SignerCacheStore) getSignedKey(hash string) string {
	return fmt.Sprintf("%s%s", signedCachePrefix, hash)
}
func (s *SignerCacheStore) getMapKey(txHash string) string {
	return fmt.Sprintf("%s%s", txMapPrefix, txHash)
}

// HasSigned check whether the given height and hash has been signed before or not
func (s *SignerCacheStore) HasSigned(hash string) bool {
	key := s.getSignedKey(hash)
	exist, _ := s.db.Has([]byte(key), nil)
	s.logger.Debug().Msgf("key:%s has signed: %t", key, exist)
	return exist
}

// RemoveSigned delete a hash from the signed cache
func (s *SignerCacheStore) RemoveSigned(transactionHash string) error {
	mapKey := s.getMapKey(transactionHash)
	value, err := s.db.Get([]byte(mapKey), nil)
	if err != nil {
		if !errors.Is(err, leveldb.ErrNotFound) {
			s.logger.Err(err).Msg("fail to check map key exist")
		}
		return err
	}
	key := s.getSignedKey(string(value))
	if err := s.db.Delete([]byte(key), nil); err != nil {
		s.logger.Error().Err(err).Msgf("fail to remove %s from signed cache", string(value))
		return fmt.Errorf("fail to remove signed cache, err: %w", err)
	}
	return nil
}

// SetTransactionHashMap map a transaction hash to a tx out item hash
func (s *SignerCacheStore) SetTransactionHashMap(txOutItemHash, transactionHash string) error {
	key := s.getMapKey(transactionHash)
	return s.db.Put([]byte(key), []byte(txOutItemHash), nil)
}

// Close underlying db
func (s *SignerCacheStore) Close() error {
	return s.db.Close()
}
