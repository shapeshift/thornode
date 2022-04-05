package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// StoreManager define the method as the entry point for store upgrade
type StoreManager interface {
	Iterator(_ cosmos.Context) error
}

// StoreMgr implement StoreManager interface
type StoreMgr struct {
	mgr *Mgrs
}

// newStoreMgr create a new instance of StoreMgr
func newStoreMgr(mgr *Mgrs) *StoreMgr {
	return &StoreMgr{
		mgr: mgr,
	}
}

// Iterator implement StoreManager interface decide whether it need to upgrade store
func (smgr *StoreMgr) Iterator(ctx cosmos.Context) error {
	version := smgr.mgr.Keeper().GetLowestActiveVersion(ctx)

	if version.Major > constants.SWVersion.Major || version.Minor > constants.SWVersion.Minor {
		return fmt.Errorf("out of date software: have %s, network running %s", constants.SWVersion, version)
	}

	storeVer := smgr.mgr.Keeper().GetStoreVersion(ctx)
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
	// add the logic to migrate store here when it is needed

	switch i {
	case 84:
		migrateStoreV84(ctx, smgr.mgr)
	case 85:
		migrateStoreV85(ctx, smgr.mgr)
	case 86:
		migrateStoreV86(ctx, smgr.mgr)
	}

	smgr.mgr.Keeper().SetStoreVersion(ctx, int64(i))
	return nil
}

func migrateStoreV84(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v84", "error", err)
		}
	}()
	removeTransactions(ctx, mgr,
		"956AE0EDE6285E9125AE4AAC1ECB249FF327977DFE5792896FD866B1274F9BF8",
		"6D010D37AA436F48C06853F09E166DB74612DF02B532A775E813B6B20C1C3106")
}

func migrateStoreV85(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v84", "error", err)
		}
	}()
	removeTransactions(ctx, mgr,
		"DDE93247EAEF9B8DBC10605FA611AB2DC5E174C9099A319D6B0E6C7B2864CD5A")
}
