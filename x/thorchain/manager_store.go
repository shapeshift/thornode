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
	case 49:
		smgr.migrateStoreV49(ctx, version, constantAccessor)
	}

	smgr.keeper.SetStoreVersion(ctx, int64(i))
	return nil
}

func (smgr *StoreMgr) migrateStoreV49(ctx cosmos.Context, version semver.Version, constantAccessor constants.ConstantValues) {
	// due to a withdrawal bug, this user lost their BTC. Recuperate a user's
	// lost pending asset
	lp := LiquidityProvider{
		Asset:              common.BTCAsset,
		RuneAddress:        common.Address("thor1anzpcqcanagcplxq7ppc7cveueg097d0594k9g"),
		AssetAddress:       common.Address("bc1qqjr5twftctxf5u77wzvdks07p9gujql6dn97qz"),
		LastAddHeight:      0,
		LastWithdrawHeight: 0,
		Units:              cosmos.ZeroUint(),
		PendingRune:        cosmos.ZeroUint(),
		PendingAsset:       cosmos.NewUint(40181088),
		PendingTxID:        common.TxID("AE5F42C21F22CB36DDCCE8D0971C8F258783EE92244048BBE106845900C17136"),
		RuneDepositValue:   cosmos.ZeroUint(),
		AssetDepositValue:  cosmos.ZeroUint(),
	}
	smgr.keeper.SetLiquidityProvider(ctx, lp)
}

func (smgr *StoreMgr) migrateStoreV46(ctx cosmos.Context, version semver.Version, constantAccessor constants.ConstantValues) {
	// housekeeping, deleting unused mimir settings
	_ = smgr.keeper.DeleteMimir(ctx, "SIGNINGTRANSACTIONPERIOD")
	_ = smgr.keeper.DeleteMimir(ctx, "MAXLIQUIDITYRUNE")
	// retiring vault
	pkey, err := common.NewPubKey("tthorpub1addwnpepqdnujur3husklhltj3l0kmmsepn0u68sge0jxg5k550nvdpphxm9s0v7f3v")
	if err != nil {
		ctx.Logger().Error("fail to parse pubkey", "error", err, "pubkey", "tthorpub1addwnpepqdnujur3husklhltj3l0kmmsepn0u68sge0jxg5k550nvdpphxm9s0v7f3v")
		return
	}
	v, err := smgr.keeper.GetVault(ctx, pkey)
	if err != nil {
		ctx.Logger().Error("fail to retrieve vault", "error", err)
		return
	}
	usdt, err := common.NewAsset("ETH.USDT-0XA3910454BF2CB59B8B3A401589A3BACC5CA42306")
	if err != nil {
		ctx.Logger().Error("fail to parse USDT asset", "error", err)
		return
	}
	daiAsset, err := common.NewAsset("ETH.DAI-0XAD6D458402F60FD3BD25163575031ACDCE07538D")
	if err != nil {
		ctx.Logger().Error("fail to parse dai asset", "error", err)
		return
	}
	xruneAsset, err := common.NewAsset("ETH.XRUNE-0X8626DB1A4F9F3E1002EEB9A4F3C6D391436FFC23")
	if err != nil {
		ctx.Logger().Error("fail to parse xrune asset", "error", err)
		return
	}
	coins := common.Coins{
		common.NewCoin(common.ETHAsset, cosmos.NewUint(1416158522)), // https://ropsten.etherscan.io/tx/0x6f23a03b6c9701faedc144321eb48b8b817ceed5abdc872ede71f2362ba5346f
		common.NewCoin(usdt, cosmos.NewUint(5309903900)),            // https://ropsten.etherscan.io/tx/0x12c915ffca980562aaf3ac1757455ba26b90caa8dc8a8d8d5fb582789524f2e6
		common.NewCoin(daiAsset, cosmos.NewUint(178297108)),         // https://ropsten.etherscan.io/tx/0x41bfb56cfeb1f8215a8aa0996396d3b55fb9aeef59b5fc63b08f14016aba721c
		common.NewCoin(xruneAsset, cosmos.NewUint(13439238115298)),  // https://ropsten.etherscan.io/tx/0x8a2915a5eb5831680851884394e702c41b9a35ce0d79c99a85f807fdf8ef306e
	}
	v.SubFunds(coins)
	if err := smgr.keeper.SetVault(ctx, v); err != nil {
		ctx.Logger().Error("fail to save vault", "error", err)
		return
	}
	// active vault
	activeVaultPubKey, err := common.NewPubKey("tthorpub1addwnpepqvs9feju7lhu53m79hmkz2dz20exa6lsj7cr867nhl6fuf7ja4hvv45fp0j")
	if err != nil {
		ctx.Logger().Error("fail to parse active vault pubkey", "error", err)
		return
	}
	activeVault, err := smgr.keeper.GetVault(ctx, activeVaultPubKey)
	if err != nil {
		ctx.Logger().Error("fail to get active vault", "error", err)
		return
	}
	activeVault.AddFunds(coins)
	if err := smgr.keeper.SetVault(ctx, activeVault); err != nil {
		ctx.Logger().Error("fail to save vault", "error", err)
	}
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
