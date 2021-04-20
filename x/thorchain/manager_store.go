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
	case 42:
		smgr.migrateStoreV42(ctx, version, constantAccessor)
	case 43:
		smgr.migrateStoreV43(ctx, version, constantAccessor)
	case 46:
		smgr.migrateStoreV46(ctx, version, constantAccessor)
	}

	smgr.keeper.SetStoreVersion(ctx, int64(i))
	return nil
}
func (smgr *StoreMgr) migrateStoreV46(ctx cosmos.Context, version semver.Version, constantAccessor constants.ConstantValues) {
	// housekeeping, deleting unused mimir settings
	_ = smgr.keeper.DeleteMimir(ctx, "SIGNINGTRANSACTIONPERIOD")
	_ = smgr.keeper.DeleteMimir(ctx, "MAXLIQUIDITYRUNE")

}
func (smgr *StoreMgr) migrateStoreV43(ctx cosmos.Context, version semver.Version, constantAccessor constants.ConstantValues) {
	// housekeeping, deleting unused mimir settings
	_ = smgr.keeper.DeleteMimir(ctx, "NEWPOOLCYCLE")
	_ = smgr.keeper.DeleteMimir(ctx, "ROTATEPERBLOCKHEIGHT")

}

func (smgr *StoreMgr) migrateStoreV42(ctx cosmos.Context, version semver.Version, constantAccessor constants.ConstantValues) {
	vaultsToRetire := []struct {
		PubKey     string
		ETHAmount  uint64
		USDTAmount uint64
	}{
		{
			PubKey:     "tthorpub1addwnpepqfrsx4rcnch7m3qd6n6g6y0mmuujjxpjy3nlts2kpzn5cazc5xmtxf54f3p", // 0xa8871c6377daD71e4B3f230Fa189cDCA5B5A6d92
			ETHAmount:  232967470,
			USDTAmount: 8741974000,
		},
		{
			PubKey:    "tthorpub1addwnpepqtgv8rnpv4nkevjcagmr9ky6v0kasj9h0gzvr4gfk2magqprvl37jnlzv9u", // 0xA839159a91f0952f527c3b4cDF3B1f771213093d
			ETHAmount: 1084097,
		},
		{
			PubKey:     "tthorpub1addwnpepqv3eslzyykkqzrf2zz5yp7pfepur2tyrvsvhd2lu72egdejpuezgcjyzfgp", // 0x6e3a059Fcbd7E3d06EA863f86223640d19F39D1f
			ETHAmount:  326616225,
			USDTAmount: 14441257100,
		},
	}
	for _, item := range vaultsToRetire {
		pkey, err := common.NewPubKey(item.PubKey)
		if err != nil {
			ctx.Logger().Error("fail to parse pubkey", "error", err, "pubkey", item.PubKey)
			continue
		}
		v, err := smgr.keeper.GetVault(ctx, pkey)
		if err != nil {
			ctx.Logger().Error("fail to retrieve vault", "error", err)
			continue
		}

		if v.IsEmpty() {
			continue
		}
		v.AddFunds(common.Coins{
			common.NewCoin(common.ETHAsset, cosmos.NewUint(item.ETHAmount)),
		})
		if item.USDTAmount > 0 {
			usdtAsset, err := common.NewAsset("ETH.USDT-0XA3910454BF2CB59B8B3A401589A3BACC5CA42306")
			if err != nil {
				ctx.Logger().Error("fail to parse USDT asset", "error", err)
				continue
			}
			v.AddFunds(common.Coins{
				common.NewCoin(usdtAsset, cosmos.NewUint(item.USDTAmount)),
			})
		}
		if v.Status == InactiveVault {
			v.UpdateStatus(RetiringVault, ctx.BlockHeight())
		}
		if err := smgr.keeper.SetVault(ctx, v); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
		}
	}

}
