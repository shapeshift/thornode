package thorchain

import (
	"github.com/blang/semver"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type VersionedTxOutStore interface {
	GetTxOutStore(ctx cosmos.Context, keeper Keeper, version semver.Version) (TxOutStore, error)
}

type TxOutStore interface {
	NewBlock(height int64, constAccessor constants.ConstantValues)
	GetBlockOut(ctx cosmos.Context) (*TxOut, error)
	ClearOutboundItems(ctx cosmos.Context)
	GetOutboundItems(ctx cosmos.Context) ([]*TxOutItem, error)
	TryAddTxOutItem(ctx cosmos.Context, toi *TxOutItem) (bool, error)
	UnSafeAddTxOutItem(ctx cosmos.Context, toi *TxOutItem) error
}

type VersionedTxOutStorage struct {
	txOutStorage          TxOutStore
	versionedEventManager VersionedEventManager
}

// NewVersionedTxOutStore create a new instance of VersionedTxOutStorage
func NewVersionedTxOutStore(versionedEventManager VersionedEventManager) *VersionedTxOutStorage {
	return &VersionedTxOutStorage{
		versionedEventManager: versionedEventManager,
	}
}

// GetTxOutStore will return an implementation of the txout store that
func (s *VersionedTxOutStorage) GetTxOutStore(ctx cosmos.Context, keeper Keeper, version semver.Version) (TxOutStore, error) {
	if version.GTE(semver.MustParse("0.1.0")) {
		if s.txOutStorage == nil {
			eventMgr, err := s.versionedEventManager.GetEventManager(ctx, version)
			if err != nil {
				return nil, errFailGetEventManager
			}
			s.txOutStorage = NewTxOutStorageV1(keeper, eventMgr)
		}
		return s.txOutStorage, nil
	}
	return nil, errInvalidVersion
}
