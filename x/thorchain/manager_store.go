package thorchain

import (
	"fmt"

	"github.com/blang/semver"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// StoreManager define the method as the entry point for store upgrade
type StoreManager interface {
	Iterator(_ cosmos.Context) error
}

// StoreMgr implement StoreManager interface
type StoreMgr struct {
	keeper keeper.Keeper
}

// NewStoreMgr create a new instance of StoreMgr
func NewStoreMgr(keeper keeper.Keeper) *StoreMgr {
	return &StoreMgr{keeper: keeper}
}

// Iterator implement StoreManager interface decide whether it need to upgrade store
func (smgr *StoreMgr) Iterator(ctx cosmos.Context) error {
	version := smgr.keeper.GetLowestActiveVersion(ctx)

	if version.Major > constants.SWVersion.Major || version.Minor > constants.SWVersion.Minor {
		return fmt.Errorf("out of date software: have %s, network running %s", constants.SWVersion, version)
	}

	storeVer := smgr.keeper.GetStoreVersion(ctx)
	if storeVer < 0 {
		return fmt.Errorf("unable to get store version: %d", storeVer)
	}
	constantAccessor := constants.GetConstantValues(version)
	if uint64(storeVer) < version.Minor {
		for i := uint64(storeVer + 1); i <= version.Minor; i++ {
			if err := smgr.migrate(ctx, i, constantAccessor, version); err != nil {
				return err
			}
		}
	} else {
		ctx.Logger().Debug("No store migration needed")
	}
	return nil
}

func (smgr *StoreMgr) migrate(ctx cosmos.Context, i uint64, constantAccessor constants.ConstantValues, version semver.Version) error {
	ctx.Logger().Info("Migrating store to new version", "version", i)
	// add the logic to migrate store here when it is needed
	switch i {
	case 36:
		smgr.migrateStoreV36(ctx, version, constantAccessor)
	}
	smgr.keeper.SetStoreVersion(ctx, int64(i))
	return nil
}

// migrateStoreV36 this is just let the state machine to process one tx , so it will be added to the pool correctly
func (smgr *StoreMgr) migrateStoreV36(ctx cosmos.Context, version semver.Version, constantAccessor constants.ConstantValues) {
	// tx fce2585aedeaec263bada44fe1a68124b4dd33110758915ad46fdda31ed797ca
	txID, err := common.NewTxID("fce2585aedeaec263bada44fe1a68124b4dd33110758915ad46fdda31ed797ca")
	if err != nil {
		ctx.Logger().Error("fail to parse tx id", "error", err)
		return
	}
	txVoter, err := smgr.keeper.GetObservedTxInVoter(ctx, txID)
	if err != nil {
		ctx.Logger().Error("fail to get tx in voter", "error", err)
		return
	}
	mgr := NewManagers(smgr.keeper)
	if err := mgr.BeginBlock(ctx); err != nil {
		ctx.Logger().Error("fail to create manager", "error", err)
		return
	}
	h := NewAddLiquidityHandler(smgr.keeper, mgr)
	tx := txVoter.Tx.Tx
	memo, err := ParseMemo(tx.Memo)
	if err != nil {
		ctx.Logger().Error("fail to parse memo", "error", err)
		return
	}
	switch m := memo.(type) {
	case AddLiquidityMemo:
		msg, err := getMsgAddLiquidityFromMemo(ctx, m, txVoter.Tx, txVoter.Tx.GetSigners()[0])
		if err != nil {
			ctx.Logger().Error("fail to get add liquidity msg", "error", err)
			return
		}
		_, err = h.Run(ctx, msg, version, constantAccessor)
		if err != nil {
			ctx.Logger().Error("fail to process add liquidity request", "error", err)
			return
		}
	}
}
