package thorchain

import (
	"fmt"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type StoreManger interface {
	Iterator(_ cosmos.Context) error
}

type StoreMgr struct {
	keeper Keeper
}

func NewStoreMgr(keeper Keeper) *StoreMgr {
	return &StoreMgr{
		keeper: keeper,
	}
}

func (smgr *StoreMgr) Iterator(ctx cosmos.Context) error {
	version := smgr.keeper.GetLowestActiveVersion(ctx)

	storeVer := smgr.keeper.GetStoreVersion(ctx)
	if storeVer < 0 {
		return fmt.Errorf("unable to get store version: %d", storeVer)
	}

	if uint64(storeVer) < version.Minor {
		for i := uint64(storeVer + 1); i <= version.Minor; i++ {
			if err := smgr.migrate(ctx, i); err != nil {
				return err
			}
		}
	} else {
		ctx.Logger().Debug("No store migration needed")
	}
	return nil
}

func (smgr *StoreMgr) migrate(ctx cosmos.Context, i uint64) error {
	ctx.Logger().Info("Migrating store to new version", "version", i)
	switch i {
	case uint64(2):
		return migrateStoreV2(ctx, smgr.keeper)
	default:
		return fmt.Errorf("unsupported store version: %d", i)
	}
}
