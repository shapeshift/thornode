package thorchain

import (
	"fmt"

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
			if err := smgr.migrate(ctx, i, constantAccessor); err != nil {
				return err
			}
		}
	} else {
		ctx.Logger().Debug("No store migration needed")
	}
	return nil
}

func (smgr *StoreMgr) migrate(ctx cosmos.Context, i uint64, constantAccessor constants.ConstantValues) error {
	ctx.Logger().Info("Migrating store to new version", "version", i)
	// add the logic to migrate store here when it is needed
	switch i {
	case 30:
		smgr.migrateVersion30(ctx, constantAccessor)
	}
	smgr.keeper.SetStoreVersion(ctx, int64(i))
	return nil
}

func (smgr *StoreMgr) migrateVersion30(ctx cosmos.Context, constAccessor constants.ConstantValues) {
	// go through the last 300 blocks , mark current stucked refund tx to be finished
	signingTransPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	if common.BlockHeight(ctx) < signingTransPeriod {
		return
	}
	startHeight := common.BlockHeight(ctx) - signingTransPeriod
	for i := startHeight; i < common.BlockHeight(ctx); i++ {
		txs, err := smgr.keeper.GetTxOut(ctx, i)
		if err != nil {
			ctx.Logger().Error(fmt.Sprintf("fail to get txout from block height(%d): %s", i, err))
			continue
		}
		if txs == nil {
			continue
		}
		if txs.IsEmpty() {
			continue
		}
		found := false
		for idx, item := range txs.TxArray {
			if !item.OutHash.IsEmpty() {
				continue
			}
			if !item.Coin.Asset.IsRune() {
				continue
			}
			if !item.Chain.Equals(common.ETHChain) {
				continue
			}
			ctx.Logger().Info("found tx out item , and mark it as completed")
			txs.TxArray[idx].OutHash = common.BlankTxID
			found = true
			break
		}
		if found {
			if err := smgr.keeper.SetTxOut(ctx, txs); err != nil {
				ctx.Logger().Error("fail to save tx out", "error", err)
			}
		}
	}
}
