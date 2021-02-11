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
	case 21:
		smgr.upgradeV21(ctx, constantAccessor)
		break
	}
	smgr.keeper.SetStoreVersion(ctx, int64(i))
	return nil
}

func (smgr *StoreMgr) upgradeV21(ctx cosmos.Context, constantAccessor constants.ConstantValues) {
	currentBlockHeight := common.BlockHeight(ctx)
	asset, err := common.NewAsset("ETH.USDT-0X62E273709DA575835C7F6AEF4A31140CA5B1D190")
	if err != nil {
		ctx.Logger().Error("fail to parse asset", "error", err)
		return
	}
	outboundHash := common.TxID("4557588CDB1A29FCCB54A82B0224F664F0B61C8A799A394F867DA085F9A7CF1D")
	// remove all tx out item
	signingPeriod := constantAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	startHeight := currentBlockHeight - signingPeriod
	if startHeight < 1 {
		startHeight = 1
	}

	for i := startHeight; i < currentBlockHeight; i++ {
		blockOut, err := smgr.keeper.GetTxOut(ctx, i)
		if err != nil {
			ctx.Logger().Error("fail to get block tx out", "error", err)
		}
		if blockOut == nil {
			continue
		}
		if blockOut.IsEmpty() {
			continue
		}
		for idx, item := range blockOut.TxArray {
			if !item.Coin.Asset.Equals(asset) {
				continue
			}
			// Mark the txout item as completed
			blockOut.TxArray[idx].OutHash = outboundHash
		}
		if err := smgr.keeper.SetTxOut(ctx, blockOut); err != nil {
			ctx.Logger().Error("fail to save block out", "error", err)
		}
	}
	// update node account back to Active
	addrs := []string{
		"tthor160vlf5an2shydcvw6xusrwnejvz9hf97q6y8tz",
		"tthor1e926rurxxv2kp66735pztwzjkrlv3eacf7ct65",
		"tthor1fjv3zv0rgvf5wlf2rddu3te0p6cltujxtaqzu7",
	}
	for _, item := range addrs {
		nodeAddr, err := cosmos.AccAddressFromBech32(item)
		if err != nil {
			ctx.Logger().Error("fail to decode address", "error", err, "address", item)
			continue
		}
		nodeAccount, err := smgr.keeper.GetNodeAccount(ctx, nodeAddr)
		if err != nil {
			ctx.Logger().Error("fail to get node account", "error", err, "address", item)
			continue
		}
		if nodeAccount.IsEmpty() {
			continue
		}
		nodeAccount.UpdateStatus(NodeActive, currentBlockHeight)
		nodeAccount.ForcedToLeave = false
		if err := smgr.keeper.SetNodeAccount(ctx, nodeAccount); err != nil {
			ctx.Logger().Error("fail to save node account", "error", err)
		}
	}
	// update pool
	pool, err := smgr.keeper.GetPool(ctx, asset)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return
	}
	if pool.IsEmpty() {
		return
	}
	pool.Decimals = 6
	pool.BalanceAsset = cosmos.RoundToDecimal(pool.BalanceAsset, pool.Decimals)
	if err := smgr.keeper.SetPool(ctx, pool); err != nil {
		ctx.Logger().Error("fail to save pool", "error", err)
		return
	}
	vaults, err := smgr.keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		ctx.Logger().Error("fail to get active asgard vaults", "error", err)
		return
	}
	for _, v := range vaults {
		for idx, c := range v.Coins {
			if c.Asset.Equals(asset) {
				v.Coins[idx].Amount = cosmos.RoundToDecimal(c.Amount, 6)
			}
		}
		if err := smgr.keeper.SetVault(ctx, v); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
			continue
		}
	}

}
