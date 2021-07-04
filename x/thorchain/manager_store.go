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
	case 55:
		smgr.migrateStoreV55(ctx, version, constantAccessor)
	case 56:
		smgr.migrateStoreV56(ctx, version, constantAccessor)
	case 58:
		smgr.migrateStoreV58(ctx, version, constantAccessor)
	}
	smgr.keeper.SetStoreVersion(ctx, int64(i))
	return nil
}

func (smgr *StoreMgr) migrateStoreV56(ctx cosmos.Context, version semver.Version, constantAccessor constants.ConstantValues) {
	pools, err := smgr.keeper.GetPools(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get pools during migration", "error", err)
		return
	}
	runePool := cosmos.ZeroUint()
	for _, p := range pools {
		runePool = runePool.Add(p.BalanceRune)
	}

	runeMod := smgr.keeper.GetRuneBalanceOfModule(ctx, AsgardName)

	// sanity checks, ensure we don't have any zeroes. If we did, something
	// ain't right. Exit immediately and retry later
	if runePool.IsZero() || runeMod.IsZero() {
		ctx.Logger().Error("cannot migrate with a zero amount", "rune in pools", runePool.Uint64(), "rune in module", runeMod.Uint64())
		return
	}
	ctx.Logger().Info("Rune totals", "pool", runePool.Uint64(), "rune in module", runeMod.Uint64())

	if runeMod.GT(runePool) {
		toBurn := common.NewCoin(
			common.RuneAsset(),
			common.SafeSub(runeMod, runePool),
		)
		ctx.Logger().Info("Burning native rune for migration", "total", toBurn)

		// sanity check, ensure we are not burning more rune than the pools have
		if common.SafeSub(runeMod, toBurn.Amount).LT(runePool) {
			ctx.Logger().Error("an attempt to burn too much rune from the pool")
			return
		}

		// move rune to thorchain module, so it can be burned there (asgard
		// module cannot burn funds)
		if err := smgr.keeper.SendFromModuleToModule(ctx, AsgardName, ModuleName, common.NewCoins(toBurn)); err != nil {
			ctx.Logger().Error("fail to move funds to module during migration", "error", err)
			return
		}
		if err := smgr.keeper.BurnFromModule(ctx, ModuleName, toBurn); err != nil {
			ctx.Logger().Error("fail to burn funds during migration", "error", err)
			return
		}
	}
}

func (smgr *StoreMgr) migrateStoreV55(ctx cosmos.Context, version semver.Version, constantAccessor constants.ConstantValues) {
	assetToAdjust, err := common.NewAsset("BNB.USDT-6D8")
	if err != nil {
		ctx.Logger().Error("fail to parse asset", "error", err)
		return
	}
	pool, err := smgr.keeper.GetPool(ctx, assetToAdjust)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return
	}
	pool.BalanceAsset = pool.BalanceAsset.Add(cosmos.NewUint(900000000))
	if err := smgr.keeper.SetPool(ctx, pool); err != nil {
		ctx.Logger().Error("fail to save pool", "error", err)
	}

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

// migrateStoreV58 this method will update
func (smgr *StoreMgr) migrateStoreV58(ctx cosmos.Context, version semver.Version, constantAccessor constants.ConstantValues) {
	// retired vault
	pubKey, err := common.NewPubKey("tthorpub1addwnpepqg65km6vfflrlymsjhrnmn4w58d2d36h977pcu3aqp6dxee2yf88yg0z3v4")
	if err != nil {
		ctx.Logger().Error("fail to parse pubkey", "error", err)
		return
	}

	retiredVault, err := smgr.keeper.GetVault(ctx, pubKey)
	if err != nil {
		ctx.Logger().Error("fail to get vault", "error", err, "pubKey", pubKey.String())
		return
	}

	// There are the fund left on a retired vault https://ropsten.etherscan.io/address/0xf9143927522cd839fcbc6ac303efa5621fbe2c69
	inputs := []struct {
		assetName string
		amount    cosmos.Uint
	}{
		{"ETH.ETH", cosmos.NewUint(6526291057)},
		{"ETH.DAI-0XAD6D458402F60FD3BD25163575031ACDCE07538D", cosmos.NewUint(360050841539)},
		{"ETH.MARS-0X9465DC5A988957CB56BE398D1F05A66F65170361", cosmos.NewUint(5574518015)},
		{"ETH.REP-0X6FD34013CDD2905D8D27B0ADAD5B97B2345CF2B8", cosmos.NewUint(1465188833)},
		{"ETH.UNI-0X71D82EB6A5051CFF99582F4CDF2AE9CD402A4882", cosmos.NewUint(517167096562)},
		{"ETH.USDT-0XA3910454BF2CB59B8B3A401589A3BACC5CA42306", cosmos.NewUint(10062840700)},
		{"ETH.XEENUS-0X7E0480CA9FD50EB7A3855CF53C347A1B4D6A2FF5", cosmos.NewUint(24518594156)},
		{"ETH.XRUNE-0X0FE3ECD525D16FA09AA1FF177014DE5304C835E2", cosmos.NewUint(3343103629754577)},
		{"ETH.XRUNE-0X8626DB1A4F9F3E1002EEB9A4F3C6D391436FFC23", cosmos.NewUint(34863118271295)},
		{"ETH.ZRX-0XE4C6182EA459E63B8F1BE7C428381994CCC2D49C", cosmos.NewUint(6732474974)},
		{"ETH.XRUNE-0XD9B37D046C543EB0E5E3EC27B86609E31DA205D7", cosmos.NewUint(10000000000)},
	}
	var coinsToCredit common.Coins
	for _, item := range inputs {
		asset, err := common.NewAsset(item.assetName)
		if err != nil {
			ctx.Logger().Error("fail to parse asset", "error", err, "asset name", item.assetName)
			continue
		}
		coinsToCredit = append(coinsToCredit, common.NewCoin(asset, item.amount))
	}
	retiredVault.AddFunds(coinsToCredit)
	retiredVault.Status = RetiringVault
	if err := smgr.keeper.SetVault(ctx, retiredVault); err != nil {
		ctx.Logger().Error("fail to set vault back to retiring state", "error", err)
		return
	}

	retiringVaultPubKey, err := common.NewPubKey("tthorpub1addwnpepqfz98sx54jpv3f95qfg39zkx500avc6tr0d8ww0lv283yu3ucgq3g9y9njj")
	if err != nil {
		ctx.Logger().Error("fail to parse current active vault pubkey", "error", err)
		return
	}
	retiringVault, err := smgr.keeper.GetVault(ctx, retiringVaultPubKey)
	if err != nil {
		ctx.Logger().Error("fail to get current active vault", "error", err)
		return
	}
	// these are the funds in current active vault, https://ropsten.etherscan.io/address/0x8d1133a8cf23112fdb21f1efca340d727a98196e
	inputs = []struct {
		assetName string
		amount    cosmos.Uint
	}{
		{"ETH.ETH", cosmos.NewUint(2357498870)},
		{"ETH.DAI-0XAD6D458402F60FD3BD25163575031ACDCE07538D", cosmos.NewUint(380250359282)},
		{"ETH.MARS-0X9465DC5A988957CB56BE398D1F05A66F65170361", cosmos.NewUint(5747652585)},
		{"ETH.REP-0X6FD34013CDD2905D8D27B0ADAD5B97B2345CF2B8", cosmos.NewUint(1556246434)},
		{"ETH.UNI-0X71D82EB6A5051CFF99582F4CDF2AE9CD402A4882", cosmos.NewUint(548635445548)},
		{"ETH.USDT-0XA3910454BF2CB59B8B3A401589A3BACC5CA42306", cosmos.NewUint(10987963700)},
		{"ETH.XEENUS-0X7E0480CA9FD50EB7A3855CF53C347A1B4D6A2FF5", cosmos.NewUint(25626085749)},
		{"ETH.XRUNE-0X0FE3ECD525D16FA09AA1FF177014DE5304C835E2", cosmos.NewUint(3550865535196787)},
		{"ETH.XRUNE-0X8626DB1A4F9F3E1002EEB9A4F3C6D391436FFC23", cosmos.NewUint(16098175953548)},
		{"ETH.ZRX-0XE4C6182EA459E63B8F1BE7C428381994CCC2D49C", cosmos.NewUint(6732474974)},
		{"ETH.XRUNE-0XD9B37D046C543EB0E5E3EC27B86609E31DA205D7", cosmos.NewUint(10000000000)},
	}
	var coinsToSubtract common.Coins
	for _, item := range inputs {
		asset, err := common.NewAsset(item.assetName)
		if err != nil {
			ctx.Logger().Error("fail to parse asset", "error", err, "asset name", item.assetName)
			continue
		}
		coinsToSubtract = append(coinsToSubtract, common.NewCoin(asset, item.amount))
	}
	retiringVault.SubFunds(coinsToSubtract)
	if err := smgr.keeper.SetVault(ctx, retiringVault); err != nil {
		ctx.Logger().Error("fail to save retiring vault", "error", err)
		return
	}
}
