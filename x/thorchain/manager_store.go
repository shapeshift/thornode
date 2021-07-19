package thorchain

import (
	"fmt"

	"github.com/blang/semver"
	"gitlab.com/thorchain/thornode/common"
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

// NewStoreMgr create a new instance of StoreMgr
func NewStoreMgr(mgr *Mgrs) *StoreMgr {
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
		smgr.migrateStoreV58Refund(ctx, version, constantAccessor)
	case 59:
		smgr.migrateStoreV59(ctx, version, constantAccessor)
	case 60:
		smgr.migrateStoreV60(ctx, version, constantAccessor)
	}

	smgr.mgr.Keeper().SetStoreVersion(ctx, int64(i))
	return nil
}

func (smgr *StoreMgr) migrateStoreV56(ctx cosmos.Context, version semver.Version, constantAccessor constants.ConstantValues) {
	pools, err := smgr.mgr.Keeper().GetPools(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get pools during migration", "error", err)
		return
	}
	runePool := cosmos.ZeroUint()
	for _, p := range pools {
		runePool = runePool.Add(p.BalanceRune)
	}

	runeMod := smgr.mgr.Keeper().GetRuneBalanceOfModule(ctx, AsgardName)

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
		if err := smgr.mgr.Keeper().SendFromModuleToModule(ctx, AsgardName, ModuleName, common.NewCoins(toBurn)); err != nil {
			ctx.Logger().Error("fail to move funds to module during migration", "error", err)
			return
		}
		if err := smgr.mgr.Keeper().BurnFromModule(ctx, ModuleName, toBurn); err != nil {
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
	pool, err := smgr.mgr.Keeper().GetPool(ctx, assetToAdjust)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return
	}
	pool.BalanceAsset = pool.BalanceAsset.Add(cosmos.NewUint(900000000))
	if err := smgr.mgr.Keeper().SetPool(ctx, pool); err != nil {
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
	smgr.mgr.Keeper().SetLiquidityProvider(ctx, lp)
}
func (smgr *StoreMgr) migrateStoreV46(ctx cosmos.Context, version semver.Version, constantAccessor constants.ConstantValues) {
	// housekeeping, deleting unused mimir settings
	_ = smgr.mgr.Keeper().DeleteMimir(ctx, "SIGNINGTRANSACTIONPERIOD")
	_ = smgr.mgr.Keeper().DeleteMimir(ctx, "MAXLIQUIDITYRUNE")
	// retiring vault
	pkey, err := common.NewPubKey("tthorpub1addwnpepqdnujur3husklhltj3l0kmmsepn0u68sge0jxg5k550nvdpphxm9s0v7f3v")
	if err != nil {
		ctx.Logger().Error("fail to parse pubkey", "error", err, "pubkey", "tthorpub1addwnpepqdnujur3husklhltj3l0kmmsepn0u68sge0jxg5k550nvdpphxm9s0v7f3v")
		return
	}
	v, err := smgr.mgr.Keeper().GetVault(ctx, pkey)
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
	if err := smgr.mgr.Keeper().SetVault(ctx, v); err != nil {
		ctx.Logger().Error("fail to save vault", "error", err)
		return
	}
	// active vault
	activeVaultPubKey, err := common.NewPubKey("tthorpub1addwnpepqvs9feju7lhu53m79hmkz2dz20exa6lsj7cr867nhl6fuf7ja4hvv45fp0j")
	if err != nil {
		ctx.Logger().Error("fail to parse active vault pubkey", "error", err)
		return
	}
	activeVault, err := smgr.mgr.Keeper().GetVault(ctx, activeVaultPubKey)
	if err != nil {
		ctx.Logger().Error("fail to get active vault", "error", err)
		return
	}
	activeVault.AddFunds(coins)
	if err := smgr.mgr.Keeper().SetVault(ctx, activeVault); err != nil {
		ctx.Logger().Error("fail to save vault", "error", err)
	}
}
func (smgr *StoreMgr) migrateStoreV43(ctx cosmos.Context, version semver.Version, constantAccessor constants.ConstantValues) {
	// housekeeping, deleting unused mimir settings
	_ = smgr.mgr.Keeper().DeleteMimir(ctx, "NEWPOOLCYCLE")
	_ = smgr.mgr.Keeper().DeleteMimir(ctx, "ROTATEPERBLOCKHEIGHT")

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
		v, err := smgr.mgr.Keeper().GetVault(ctx, pkey)
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
		if err := smgr.mgr.Keeper().SetVault(ctx, v); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
		}
	}
}
func (smgr *StoreMgr) migrateStoreV58Refund(ctx cosmos.Context, version semver.Version, constantAccessor constants.ConstantValues) {
	if err := smgr.mgr.BeginBlock(ctx); err != nil {
		ctx.Logger().Error("fail to initialise block", "error", err)
		return
	}
	txOutStore := smgr.mgr.TxOutStore()
	mkrAddr, err := common.NewAddress("0x340b94e5369cEDe551a117960c75547eA84eAEdE")
	if err != nil {
		ctx.Logger().Error("fail to parse address", "error", err)
		return
	}
	mkrToken, err := common.NewAsset("ETH.MKR-0X9F8F72AA9304C8B593D555F12EF6589CC3A579A2")
	if err != nil {
		ctx.Logger().Error("fail to parse MKR asset", "error", err)
		return
	}
	refundMKRTxID, err := common.NewTxID("f8165c9d888c1abd51edddf8b3da9c8bcf6cde4cacdca15f3df2d176332dcdd7")
	if err != nil {
		ctx.Logger().Error("fail to parse MKR transaction id", "error", err)
		return
	}

	refundMkr := TxOutItem{
		Chain:     common.ETHChain,
		ToAddress: mkrAddr,
		Coin: common.Coin{
			Asset:  mkrToken,
			Amount: cosmos.NewUint(60000000),
		},
		Memo:   NewRefundMemo(refundMKRTxID).String(),
		InHash: refundMKRTxID,
	}
	_, err = txOutStore.TryAddTxOutItem(ctx, smgr.mgr, refundMkr)
	if err != nil {
		ctx.Logger().Error("fail to schedule refund MKR transaction", "error", err)
		return
	}

	// refund LINK
	linkAddr, err := common.NewAddress("0x0749405611B77f94311576C6e80FAe69CfcCa41A")
	if err != nil {
		ctx.Logger().Error("fail to parse address", "error", err)
		return
	}
	linkToken, err := common.NewAsset("ETH.LINK-0X514910771AF9CA656AF840DFF83E8264ECF986CA")
	if err != nil {
		ctx.Logger().Error("fail to parse LINK asset", "error", err)
		return
	}
	refundLINKTxID, err := common.NewTxID("b8489f8a5bdfd39c899cc1987eb32e81490580f2fb6426cd4bc710e45c20b721")
	if err != nil {
		ctx.Logger().Error("fail to parse LINK transaction id", "error", err)
		return
	}

	refundLINK := TxOutItem{
		Chain:     common.ETHChain,
		ToAddress: linkAddr,
		Coin: common.Coin{
			Asset:  linkToken,
			Amount: cosmos.NewUint(140112242412),
		},
		Memo:   NewRefundMemo(refundLINKTxID).String(),
		InHash: refundLINKTxID,
	}
	_, err = txOutStore.TryAddTxOutItem(ctx, smgr.mgr, refundLINK)
	if err != nil {
		ctx.Logger().Error("fail to schedule refund LINK transaction", "error", err)
		return
	}

	// inTxID 6232075D4C63A69CDC8B65157A1737CBC4C1DA979BAA7E6F8B6B9F20A38388CA
	// inTxID 05A7C2FD035FEC62BB957465BC80970177EBD80605138B7C4709333F118E7338
	for _, txID := range []string{
		"6232075D4C63A69CDC8B65157A1737CBC4C1DA979BAA7E6F8B6B9F20A38388CA",
		"05A7C2FD035FEC62BB957465BC80970177EBD80605138B7C4709333F118E7338",
	} {
		inTxID, err := common.NewTxID(txID)
		if err != nil {
			ctx.Logger().Error("fail to parse tx id", "error", err, "tx_id", inTxID)
			continue
		}
		voter, err := smgr.mgr.Keeper().GetObservedTxInVoter(ctx, inTxID)
		if err != nil {
			ctx.Logger().Error("fail to get observed tx voter", "error", err)
			continue
		}
		if !voter.Tx.IsEmpty() {
			voter.Tx.SetDone(common.BlankTxID, len(voter.Actions))
		}
		// set the tx outbound with a blank txid will mark it as down , and will be skipped in the reschedule logic
		for idx := range voter.Txs {
			voter.Txs[idx].SetDone(common.BlankTxID, len(voter.Actions))
		}
		smgr.mgr.Keeper().SetObservedTxInVoter(ctx, voter)
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

	retiredVault, err := smgr.mgr.Keeper().GetVault(ctx, pubKey)
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
	if err := smgr.mgr.Keeper().SetVault(ctx, retiredVault); err != nil {
		ctx.Logger().Error("fail to set vault back to retiring state", "error", err)
		return
	}

	retiringVaultPubKey, err := common.NewPubKey("tthorpub1addwnpepqfz98sx54jpv3f95qfg39zkx500avc6tr0d8ww0lv283yu3ucgq3g9y9njj")
	if err != nil {
		ctx.Logger().Error("fail to parse current active vault pubkey", "error", err)
		return
	}
	retiringVault, err := smgr.mgr.Keeper().GetVault(ctx, retiringVaultPubKey)
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
	if err := smgr.mgr.Keeper().SetVault(ctx, retiringVault); err != nil {
		ctx.Logger().Error("fail to save retiring vault", "error", err)
		return
	}
}

func (smgr *StoreMgr) migrateStoreV59(ctx cosmos.Context, version semver.Version, constantAccessor constants.ConstantValues) {
	// remove transaction FD9BFC2D42F7E8B20777CA1CEEC39395827A4FF22D1A83641343E1CDFB850135
	for _, txID := range []string{
		"FD9BFC2D42F7E8B20777CA1CEEC39395827A4FF22D1A83641343E1CDFB850135",
	} {
		inTxID, err := common.NewTxID(txID)
		if err != nil {
			ctx.Logger().Error("fail to parse tx id", "error", err, "tx_id", inTxID)
			continue
		}
		voter, err := smgr.mgr.Keeper().GetObservedTxInVoter(ctx, inTxID)
		if err != nil {
			ctx.Logger().Error("fail to get observed tx voter", "error", err)
			continue
		}
		if !voter.Tx.IsEmpty() {
			voter.Tx.SetDone(common.BlankTxID, len(voter.Actions))
		}
		// set the tx outbound with a blank txid will mark it as down , and will be skipped in the reschedule logic
		for idx := range voter.Txs {
			voter.Txs[idx].SetDone(common.BlankTxID, len(voter.Actions))
		}
		smgr.mgr.Keeper().SetObservedTxInVoter(ctx, voter)
	}

	xRUNEAsset, err := common.NewAsset("ETH.XRUNE-0X69FA0FEE221AD11012BAB0FDB45D444D3D2CE71C")
	if err != nil {
		ctx.Logger().Error("fail to parse XRUNE asset", "error", err)
		return
	}
	vaultIter := smgr.mgr.Keeper().GetVaultIterator(ctx)
	defer vaultIter.Close()
	for ; vaultIter.Valid(); vaultIter.Next() {
		var vault Vault
		if err := smgr.mgr.Keeper().Cdc().UnmarshalBinaryBare(vaultIter.Value(), &vault); err != nil {
			ctx.Logger().Error("fail to unmarshal vault", "error", err)
			continue
		}
		// vault is empty , ignore
		if vault.IsEmpty() {
			continue
		}

		for idx, c := range vault.Coins {
			if c.Asset.Equals(xRUNEAsset) {
				vault.Coins[idx].Amount = cosmos.ZeroUint()
				if err := smgr.mgr.Keeper().SetVault(ctx, vault); err != nil {
					ctx.Logger().Error("fail to save vault", "error", err)
				}
			}
		}
	}
}
func (smgr *StoreMgr) migrateStoreV60(ctx cosmos.Context, version semver.Version, constantAccessor constants.ConstantValues) {
	// remove all the outbound transactions that caused by the attack transaction
	smgr.removeTransactions(ctx,
		"BC68035CE2C8A2C549604FF7DB59E07931F39040758B138190338FA697338DB3",
		"5DA19C1C5C2F6BBDF9D4FB0E6FF16A0DF6D6D7FE1F8E95CA755E5B3C6AADA458",
		"D58D1EF6D6E49EB99D0524128C16115893396FD05877EF4856FCE474B5BA09A7",
		"347926A5AF7C8434F27DDFCFD14A42AC1525B0FCB3E6803203A83C89276660E3",
		"4F61A1590C4AA2A30FFAB1735F5273B72BC1FBB63F90D5FAF1B380312913BB55",
		"91CD37D7E59F2C6A96E002059D03218045EDA4E2B487C1E70619162826E57AFA",
		"E30F53E7F8D153C50C53574B7055098BA5949948C33674EC779D19FEE73170DC",
		"5B5A971DCF5E8A2B551E2E9CF3EACF3C30D49A3289AFC4EADF1A1D3EF5AF87D4",
		"9D60D243C01C735E2DB88C044E77142D5D1D6D3CB21A0BD16F496A6A2327C70B",
		"FE4B3FE42EF4EDC9A33791DC415D7BF22BF0C4A77B270A46B06F2EC4F5664313",
		"03C90BA802F5CC14FB21912E8ECD3501C09D60F7335293A1D4B2B9F4500D29C5",
		"8FEF62A0837C9E41429DE427E7C8692DF66004C74CF25CB70127B36DF1B44036",
		"B23BEEB4C72755230CE655D1C45CBA1C662E55E4AA1F47A4A3B0EF4A3CF4362E",
		"3C0377B41D3B0D709CF5FE15767422B91FAAB724A12816C4650833E551406E36",
	)
	smgr.creditAssetBackToVaultAndPool(ctx)
}

func (smgr *StoreMgr) removeTransactions(ctx cosmos.Context, hashes ...string) {
	for _, txID := range hashes {
		inTxID, err := common.NewTxID(txID)
		if err != nil {
			ctx.Logger().Error("fail to parse tx id", "error", err, "tx_id", inTxID)
			continue
		}
		voter, err := smgr.mgr.Keeper().GetObservedTxInVoter(ctx, inTxID)
		if err != nil {
			ctx.Logger().Error("fail to get observed tx voter", "error", err)
			continue
		}
		// all outbound action get removed
		voter.Actions = []TxOutItem{}
		if voter.Tx.IsEmpty() {
			continue
		}
		voter.Tx.SetDone(common.BlankTxID, 0)
		// set the tx outbound with a blank txid will mark it as down , and will be skipped in the reschedule logic
		for idx := range voter.Txs {
			voter.Txs[idx].SetDone(common.BlankTxID, 0)
		}
		smgr.mgr.Keeper().SetObservedTxInVoter(ctx, voter)
	}
}

func (smgr *StoreMgr) migrateStoreV61(ctx cosmos.Context, version semver.Version, constantAccessor constants.ConstantValues) {
	smgr.correctAsgardVaultBalanceV61(ctx)
	smgr.purgeETHOutboundQueue(ctx, constantAccessor)
	// the following two inbound from a binance address which has memo flag set , thus these outbound will not able to sent out
	smgr.removeTransactions(ctx, "BB3A3E34783C11BE58C616F2C4D22C785D20697E793B54588035A2B0EE7B603A", "83C25D7AB3EBEBD462C95AF7D63E54DA010715541A1C73C0347E04191E78867D")
}
