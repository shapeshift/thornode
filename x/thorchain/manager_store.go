package thorchain

import (
	"fmt"

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
	case 42:
		migrateStoreV42(ctx, smgr.mgr)
	case 43:
		migrateStoreV43(ctx, smgr.mgr)
	case 46:
		migrateStoreV46(ctx, smgr.mgr)
	case 49:
		migrateStoreV49(ctx, smgr.mgr)
	case 55:
		migrateStoreV55(ctx, smgr.mgr)
	case 56:
		migrateStoreV56(ctx, smgr.mgr)
	case 58:
		migrateStoreV58(ctx, smgr.mgr)
		migrateStoreV58Refund(ctx, smgr.mgr)
	case 59:
		migrateStoreV59(ctx, smgr.mgr)
	case 60:
		migrateStoreV60(ctx, smgr.mgr)
	case 62:
		migrateStoreV62(ctx, smgr.mgr)
	case 69:
		migrateStoreV69(ctx, smgr.mgr)
	case 70:
		migrateStoreV70Pools(ctx, smgr.mgr)
		refundBNBTransactions(ctx, smgr.mgr)
		cancelOutboundTxs(ctx, smgr.mgr)
	case 71:
		refundBNBTransactionsV71(ctx, smgr.mgr)
		correctBurnedBEP2Rune(ctx, smgr.mgr)
	case 75:
		migrateStoreV75(ctx, smgr.mgr)
		migrateStoreV75CorrectVaultAndRefund(ctx, smgr.mgr)
		migrateStoreV75UnMarkValidators(ctx, smgr.mgr)
	case 77:
		migrateStoreV77(ctx, smgr.mgr)
	case 79:
		migrateStoreV79(ctx, smgr.mgr)
	case 80:
		migrateStoreV80(ctx, smgr.mgr)
	}

	smgr.mgr.Keeper().SetStoreVersion(ctx, int64(i))
	return nil
}

func migrateStoreV56(ctx cosmos.Context, mgr Manager) {
	pools, err := mgr.Keeper().GetPools(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get pools during migration", "error", err)
		return
	}
	runePool := cosmos.ZeroUint()
	for _, p := range pools {
		runePool = runePool.Add(p.BalanceRune)
	}

	runeMod := mgr.Keeper().GetRuneBalanceOfModule(ctx, AsgardName)

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
		if err := mgr.Keeper().SendFromModuleToModule(ctx, AsgardName, ModuleName, common.NewCoins(toBurn)); err != nil {
			ctx.Logger().Error("fail to move funds to module during migration", "error", err)
			return
		}
		if err := mgr.Keeper().BurnFromModule(ctx, ModuleName, toBurn); err != nil {
			ctx.Logger().Error("fail to burn funds during migration", "error", err)
			return
		}
	}
}

func migrateStoreV55(ctx cosmos.Context, mgr Manager) {
	assetToAdjust, err := common.NewAsset("BNB.USDT-6D8")
	if err != nil {
		ctx.Logger().Error("fail to parse asset", "error", err)
		return
	}
	pool, err := mgr.Keeper().GetPool(ctx, assetToAdjust)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return
	}
	pool.BalanceAsset = pool.BalanceAsset.Add(cosmos.NewUint(900000000))
	if err := mgr.Keeper().SetPool(ctx, pool); err != nil {
		ctx.Logger().Error("fail to save pool", "error", err)
	}
}

func migrateStoreV49(ctx cosmos.Context, mgr Manager) {
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
	mgr.Keeper().SetLiquidityProvider(ctx, lp)
}

func migrateStoreV46(ctx cosmos.Context, mgr Manager) {
	// housekeeping, deleting unused mimir settings
	_ = mgr.Keeper().DeleteMimir(ctx, "SIGNINGTRANSACTIONPERIOD")
	_ = mgr.Keeper().DeleteMimir(ctx, "MAXLIQUIDITYRUNE")
	// retiring vault
	pkey, err := common.NewPubKey("tthorpub1addwnpepqdnujur3husklhltj3l0kmmsepn0u68sge0jxg5k550nvdpphxm9s0v7f3v")
	if err != nil {
		ctx.Logger().Error("fail to parse pubkey", "error", err, "pubkey", "tthorpub1addwnpepqdnujur3husklhltj3l0kmmsepn0u68sge0jxg5k550nvdpphxm9s0v7f3v")
		return
	}
	v, err := mgr.Keeper().GetVault(ctx, pkey)
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
	if err := mgr.Keeper().SetVault(ctx, v); err != nil {
		ctx.Logger().Error("fail to save vault", "error", err)
		return
	}
	// active vault
	activeVaultPubKey, err := common.NewPubKey("tthorpub1addwnpepqvs9feju7lhu53m79hmkz2dz20exa6lsj7cr867nhl6fuf7ja4hvv45fp0j")
	if err != nil {
		ctx.Logger().Error("fail to parse active vault pubkey", "error", err)
		return
	}
	activeVault, err := mgr.Keeper().GetVault(ctx, activeVaultPubKey)
	if err != nil {
		ctx.Logger().Error("fail to get active vault", "error", err)
		return
	}
	activeVault.AddFunds(coins)
	if err := mgr.Keeper().SetVault(ctx, activeVault); err != nil {
		ctx.Logger().Error("fail to save vault", "error", err)
	}
}

func migrateStoreV43(ctx cosmos.Context, mgr Manager) {
	// housekeeping, deleting unused mimir settings
	_ = mgr.Keeper().DeleteMimir(ctx, "NEWPOOLCYCLE")
	_ = mgr.Keeper().DeleteMimir(ctx, "ROTATEPERBLOCKHEIGHT")
}

func migrateStoreV42(ctx cosmos.Context, mgr Manager) {
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
		v, err := mgr.Keeper().GetVault(ctx, pkey)
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
		if err := mgr.Keeper().SetVault(ctx, v); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
		}
	}
}

func migrateStoreV58Refund(ctx cosmos.Context, mgr *Mgrs) {
	if err := mgr.BeginBlock(ctx); err != nil {
		ctx.Logger().Error("fail to initialise block", "error", err)
		return
	}
	txOutStore := mgr.TxOutStore()
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
	_, err = txOutStore.TryAddTxOutItem(ctx, mgr, refundMkr)
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
	_, err = txOutStore.TryAddTxOutItem(ctx, mgr, refundLINK)
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
		voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, inTxID)
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
		mgr.Keeper().SetObservedTxInVoter(ctx, voter)
	}
}

// migrateStoreV58 this method will update
func migrateStoreV58(ctx cosmos.Context, mgr Manager) {
	// retired vault
	pubKey, err := common.NewPubKey("tthorpub1addwnpepqg65km6vfflrlymsjhrnmn4w58d2d36h977pcu3aqp6dxee2yf88yg0z3v4")
	if err != nil {
		ctx.Logger().Error("fail to parse pubkey", "error", err)
		return
	}

	retiredVault, err := mgr.Keeper().GetVault(ctx, pubKey)
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
	if err := mgr.Keeper().SetVault(ctx, retiredVault); err != nil {
		ctx.Logger().Error("fail to set vault back to retiring state", "error", err)
		return
	}

	retiringVaultPubKey, err := common.NewPubKey("tthorpub1addwnpepqfz98sx54jpv3f95qfg39zkx500avc6tr0d8ww0lv283yu3ucgq3g9y9njj")
	if err != nil {
		ctx.Logger().Error("fail to parse current active vault pubkey", "error", err)
		return
	}
	retiringVault, err := mgr.Keeper().GetVault(ctx, retiringVaultPubKey)
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
	if err := mgr.Keeper().SetVault(ctx, retiringVault); err != nil {
		ctx.Logger().Error("fail to save retiring vault", "error", err)
		return
	}
}

func migrateStoreV59(ctx cosmos.Context, mgr Manager) {
	// remove transaction FD9BFC2D42F7E8B20777CA1CEEC39395827A4FF22D1A83641343E1CDFB850135
	for _, txID := range []string{
		"FD9BFC2D42F7E8B20777CA1CEEC39395827A4FF22D1A83641343E1CDFB850135",
	} {
		inTxID, err := common.NewTxID(txID)
		if err != nil {
			ctx.Logger().Error("fail to parse tx id", "error", err, "tx_id", inTxID)
			continue
		}
		voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, inTxID)
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
		mgr.Keeper().SetObservedTxInVoter(ctx, voter)
	}

	xRUNEAsset, err := common.NewAsset("ETH.XRUNE-0X69FA0FEE221AD11012BAB0FDB45D444D3D2CE71C")
	if err != nil {
		ctx.Logger().Error("fail to parse XRUNE asset", "error", err)
		return
	}
	vaultIter := mgr.Keeper().GetVaultIterator(ctx)
	defer vaultIter.Close()
	for ; vaultIter.Valid(); vaultIter.Next() {
		var vault Vault
		if err := mgr.Keeper().Cdc().Unmarshal(vaultIter.Value(), &vault); err != nil {
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
				if err := mgr.Keeper().SetVault(ctx, vault); err != nil {
					ctx.Logger().Error("fail to save vault", "error", err)
				}
			}
		}
	}
}

func migrateStoreV60(ctx cosmos.Context, mgr Manager) {
	// remove all the outbound transactions that caused by the attack transaction
	removeTransactions(ctx, mgr,
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
	creditAssetBackToVaultAndPool(ctx, mgr)
}

func migrateStoreV62(ctx cosmos.Context, mgr Manager) {
	// use the specific vault pub key to find the asgard we need to update
	const asgardPubKey = `thorpub1addwnpepqdr4386mnkqyqzpqlydtat0k82f8xvkfwzh4xtjc84cuaqmwx5vjvgnf6v5`
	targetPubKey, err := common.NewPubKey(asgardPubKey)
	if err != nil {
		ctx.Logger().Error("fail to parse pub key", "error", err)
		return
	}
	correctAsgardVaultBalanceV61(ctx, mgr, targetPubKey)
	purgeETHOutboundQueue(ctx, mgr)
	// the following two inbound from a binance address which has memo flag set , thus these outbound will not able to sent out
	removeTransactions(ctx, mgr, "BB3A3E34783C11BE58C616F2C4D22C785D20697E793B54588035A2B0EE7B603A", "83C25D7AB3EBEBD462C95AF7D63E54DA010715541A1C73C0347E04191E78867D")
}

func migrateStoreV69(ctx cosmos.Context, mgr Manager) {
	// Note: there was a panic in this function when the network upgrade to 0.69.0
	// The network continues , however it caused some nodes that didn't update in time to get into consensus failure.
	// In order to make sure that this function will not be run when the network upgrade to 0.70.0 , thus add the
	// following condition
	if common.BlockHeight(ctx) > 2464736 {
		return
	}
	// the following is to adjust the asgard vault, removing coins that were
	// exploited out of the network a couple of months ago. The treasury is
	// giving back via `DONATE` memos of asset and rune
	alcx, _ := common.NewAsset("ETH.ALCX-0XDBDB4D16EDA451D0503B854CF79D55697F90C8DF")
	sushi, _ := common.NewAsset("ETH.SUSHI-0X6B3595068778DD592E39A122F4F5A5CF09C90FE2")
	usdc, _ := common.NewAsset("ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48")
	yfi, _ := common.NewAsset("ETH.YFI-0X0BC529C00C6401AEF6D220BE8C6EA1667F6AD93E")
	usdt, _ := common.NewAsset("ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7")
	xrune, _ := common.NewAsset("ETH.XRUNE-0X69FA0FEE221AD11012BAB0FDB45D444D3D2CE71C")
	ethCoins := common.NewCoins(
		common.NewCoin(alcx, cosmos.NewUint(96688561785)),
		common.NewCoin(sushi, cosmos.NewUint(5612449913455)),
		common.NewCoin(usdc, cosmos.NewUint(167288429419600)),
		common.NewCoin(yfi, cosmos.NewUint(691738219)),
		common.NewCoin(usdt, cosmos.NewUint(99023689717400)),
		common.NewCoin(xrune, cosmos.NewUint(2035726619258494)),
	)

	// due to outage, and binance daemons getting reset, some transaction were
	// dropped by the network. This is an attempt to recover funds and manually
	// refund users

	const asgardPk = `thorpub1addwnpepqdr4386mnkqyqzpqlydtat0k82f8xvkfwzh4xtjc84cuaqmwx5vjvgnf6v5`
	asgardPubKey, err := common.NewPubKey(asgardPk)
	if err != nil {
		ctx.Logger().Error("fail to parse pub key", "error", err)
		return
	}

	gaps := []struct {
		name   string
		amount cosmos.Uint
	}{
		{"BNB.RUNE-B1A", cosmos.NewUint(47717.64492281 * 1e8)},
		{"BNB.BNB", cosmos.NewUint(10.61276535 * 1e8)},
		{"BNB.BUSD-BD1", cosmos.NewUint(6357.09856987 * 1e8)},
		{"BNB.ETH-1C9", cosmos.NewUint(1.08972069 * 1e8)},
		{"BNB.CAS-167", cosmos.NewUint(10973.75396994 * 1e8)},
	}
	var coins common.Coins
	for _, item := range gaps {
		asset, err := common.NewAsset(item.name)
		if err != nil {
			ctx.Logger().Error("fail to parse asset", "asset", item.name, "error", err)
			continue
		}
		coins = append(coins, common.NewCoin(asset, item.amount))
	}

	asgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		ctx.Logger().Error("fail to get active asgard", "error", err)
		return
	}

	for _, v := range asgards {
		if !v.PubKey.Equals(asgardPubKey) {
			continue
		}
		v.AddFunds(coins)    // add dropped BEP2 coins
		v.SubFunds(ethCoins) // subtract ETH coins
		if err := mgr.Keeper().SetVault(ctx, v); err != nil {
			ctx.Logger().Error("fail to save asgard", "error", err)
			continue
		}
	}

	refunds := []struct {
		hash   string
		addr   string
		amount float64
		asset  string
	}{
		{"54E4DE4A93CC4FEBDE35EB723B51F4490923958FA32BA474901C02ECBFF047B9", "bnb1h05y4pe6heq2g8u6955kchrdst7s8gxspg3u2s", 1, "RUNE-B1A"},
		{"7F6EC99B8F82F36501DCCDDBF54E9BBEAA51A8604CD39E9635027711967B2DF9", "bnb1h05y4pe6heq2g8u6955kchrdst7s8gxspg3u2s", 628, "RUNE-B1A"},
		{"2EF95165FDAABBF72CA4EE10DBAEF825EE06C280B8B2098AB55A89B922105C57", "bnb1uresxjchzzffvdvjgkf2su7kcupx5uycc58ygs", 34.166, "RUNE-B1A"},
		{"1D8DE0EC576D7C9A485ECA74401560CBA65870F9E07DF814257B24C0EC8165FC", "bnb1kcay57v22f6gn5kf5vsnstxv8dfnaz2gecckeg", 47.477, "RUNE-B1A"},
		{"24A7EF13DDDC77815EB861E98BD8EA4B0E393F1D9BD1CB3345FD46CA9CB520F4", "bnb1dulgfyr9f3jnyu40f2hnaarrm8apkhdfsdvpcv", 0.976, "RUNE-B1A"},
		{"EA6627C95264B150DB729D2BFBD9C52B91300A5118C404F519244C07E9A0A4C1", "bnb1zxtn3xt5prfd7w3st53jjnkeuexkyn7uh8a24c", 126.10821, "RUNE-B1A"},
		{"402DF369568560B79F90B6B0800C75802858800814AA196B911039E524357EC6", "bnb1t68m836ztz7jvcp77uf2djsl4mhfqxa8h5jdl0", 0.002, "BNB"},
		{"6FFFF126170C6B4A144121DD5E5E9A195B9355F083C5494C4C9BF2DA855B9739", "bnb1kcay57v22f6gn5kf5vsnstxv8dfnaz2gecckeg", 251.082, "RUNE-B1A"},
		{"57CE7BD54ADCC89061B31B9EEB1823E475337E1B4061A3377D4FB2B2071EC65C", "bnb153jd3vzzlhusmt4gvv6pq8nsewktcpf0799wdf", 44.1178, "RUNE-B1A"},
		{"37E64B6DF40541E9C77107C29BBE3A03BD051EA072ED8CBDC2CC51CF3AD2BF59", "bnb1rvzk2wkn6f8k447s63tf6yuk52tef4m0t79vn0", 50.84107, "RUNE-B1A"},
		{"B1D6CC11BC34E680362E26CDE8628444222D5AEA30520EF1A38BF3B1BB63C246", "bnb1km5j6s27w5zr9677p3nydv7gq8xemh53g6keeq", 100, "RUNE-B1A"},
		{"0F5115DBB611B658A576084ACCA1C7573CDAB3389E3CA015A6406195BA00AD75", "bnb1uw5jgsm904e5hczpm7lhrlmy9lq05hjjrv4xqx", 3, "RUNE-B1A"},
		{"F46B5F083B587E29E8A11C75E12F7C8E81562E06AB5466A877B308ED34ED6AA2", "bnb1t68m836ztz7jvcp77uf2djsl4mhfqxa8h5jdl0", 0.001, "BNB"},
		{"B57C28C425EFA75560C32CA6EBB9DC2BE86EE118837B33840008023808CD3583", "bnb1ljkguffu227zwvz29quzn7aj6a3wdf3nc4rffz", 641.09822, "RUNE-B1A"},
		{"87E08B731347CE4C57A9F622FD7BCEE092462F48EC36AF312CE5AA5B80717942", "bnb1t68m836ztz7jvcp77uf2djsl4mhfqxa8h5jdl0", 0.00569604, "BNB"},
		{"4030559AA45D78422296581D6C352FA90B51F913DF216E953B0B044F5107A17E", "bnb1zptk7ftjmessmw06q2qg4te2d2cd54973wthye", 2.846, "RUNE-B1A"},
		{"2C6FA678407506A4CC8F8C8CC1EC07511C6228120B963A2FDDD4A22E67611AA6", "bnb1zptk7ftjmessmw06q2qg4te2d2cd54973wthye", 0.106, "RUNE-B1A"},
		{"A77AE682E0244D3203A31BF2AAD9DE317742B0D38BED40162E093EA5B3411233", "bnb1n63mqfexeqj5rdc08ww4jyu6ys5k3s2h8n95tj", 0.04983093, "BNB"},
		{"189BD06D18EE322D5662BF6AD5F18AF44286FD71985EE8408B91726EF59F29A0", "bnb1q3wkg6p2plf88342qtxe943qsnrhacp7f2f364", 498.446, "RUNE-B1A"},
		{"EAF6B55E6718BA0E0E81F425DB34E4CA6E9C36D3CE16562362DF2FC57FD3E65E", "bnb1mfz43ul3njltlgfhn976l88a2fd0uvcm2xyl6p", 1, "BUSD-BD1"},
		{"920E66FF45FF39EE469D00B60CC739A574C0F9F7774820BF6E92D822E3FA7A7E", "bnb1n7ne3kpjqm7almtx65aff2pfps6duc0wpumts0", 2.404065, "BNB"},
		{"9D3C914F4049101410A86298E471E5C3C113DF45568087B362A2376C7575663B", "bnb1ukfhdgy9c0ws42eqt786q82ept9ht6fag88t6u", 0.01, "BNB"},
		{"55C3F13B1C07B178F71DE62CD249673F042FBC6BC6742DA84AAD620604349D6F", "bnb1afdvrfpls66n2hm58lx6je0yqmv5namftuwdrm", 0.41, "BUSD-BD1"},
		{"CCB5E301893EBDE7D498E0EBFBB6582B5230FB2F203C27D1A37B127380BD1E3C", "bnb1afdvrfpls66n2hm58lx6je0yqmv5namftuwdrm", 0.5, "BUSD-BD1"},
		{"5025E6CFB2112A15B9F399A12F318B84E975ED9D9E82F2DE5E3DE30A991D462F", "bnb1afdvrfpls66n2hm58lx6je0yqmv5namftuwdrm", 0.0001, "BUSD-BD1"},
		{"41DFA55BDF79BDB4393C15CD86D971307E1E9825BB33B68A7709991213940B15", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9969772, "BUSD-BD1"},
		{"09970D09FF884612C5A09F8DA81733977545B810F8B8DDEFFBDEAF3BFF27879E", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9993278, "BUSD-BD1"},
		{"2243A8F3938031C485506955D1F5170EB3D7C6226444AE37F122D29194CB9208", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9875878, "BUSD-BD1"},
		{"35CFE9093F532ACAE2B7DD79C30D34777C6929A0CD6A5AD02B335B5DC69906B9", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 129.9999985, "BUSD-BD1"},
		{"3D693B525AABDD3FFFE6F88082BC4EDA69F50A9E8336BD3B58DDC916E35B1AAE", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 119.9999999, "BUSD-BD1"},
		{"1D783C114889B42A58091CB5097DF7877ACA4852BB84AD989C4102692B5CDA80", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9969772, "BUSD-BD1"},
		{"E4546065A5E1F7D0D69951E3990FE9333CCFB34AA14FFAE4A6387F080C15F7E4", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9993278, "BUSD-BD1"},
		{"E3A23F59F02AF1F2FA312A92565C28DCB468133A164719DF573B7CA6D8791138", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 129.9999985, "BUSD-BD1"},
		{"C9C027D0AF581A8555D6D728B19C1933E232AC45994A3E3BB526B89DAB50137F", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 119.9999999, "BUSD-BD1"},
		{"B094713F6AA70DDDF58E50ABCAD4A4526502A3BB76DA6586042CC1368FC319C4", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9969772, "BUSD-BD1"},
		{"61537E93C783E38E1F66A9C830F25D38414CC5A82DEAD6AB353F9F724C7E3C6F", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9875878, "BUSD-BD1"},
		{"EC733CA438027B4B816A66C63BB36DD97F7928D41D3FD1FF8EE10A8E395A90FD", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 129.9999985, "BUSD-BD1"},
		{"62DC14CB8617786AC9A67E4C0BAD9322883AF4CEFF4568458AC3577D46CD01C2", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 119.9999999, "BUSD-BD1"},
		{"05FADCD5E58368F242F11A664527B85E439381F851E2E4977C94455502E3B785", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9993278, "BUSD-BD1"},
		{"A43D760B722A4720DE79941A0F22163687FA059FC0BA513C4C299999FEDE8118", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 129.9999985, "BUSD-BD1"},
		{"0FF1F83A32FDCEF50C04388A414830DB42E98AE0B75B503210BEC71B69B212E2", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 119.9999999, "BUSD-BD1"},
		{"5001D020A8A2215E265CB0CFA80EB0BBD23143C01380906C19715C818FD11036", "bnb10nqkqtw06s45x52qu5m27gqamfwz3pnvehgct9", 113.328, "RUNE-B1A"},
		{"166EAA50B2F2F92138B6C8D80A86001D937BAE736AD8E267EF0C1CC049725FFF", "bnb1q3wkg6p2plf88342qtxe943qsnrhacp7f2f364", 498.954, "RUNE-B1A"},
		{"76D38C831622F356E93893A736D9F68B55BE81A2A1C323D0A044F7AB8C86F6B3", "bnb1afdvrfpls66n2hm58lx6je0yqmv5namftuwdrm", 0.5, "BUSD-BD1"},
		{"8F7FFA2DC9BC7876F6770265C0AF55616FCC43245CF2C8D8E13DC02542D2E0BE", "bnb1afdvrfpls66n2hm58lx6je0yqmv5namftuwdrm", 0.5, "BUSD-BD1"},
		{"55ADF8DDADCFFD871BAC4CDA45C244CD3E682675E6FB79B541FD32C378425B71", "bnb1afdvrfpls66n2hm58lx6je0yqmv5namftuwdrm", 0.5, "BUSD-BD1"},
		{"C0D1EBADC2B104D62DE3D0C6E34691C3B163FC806736D2D7C03F1BB47ACEFE58", "bnb1afdvrfpls66n2hm58lx6je0yqmv5namftuwdrm", 0.5, "BUSD-BD1"},
		{"76325C9563DB3436BB2EA1ECE1E702E0EF375DB91491A2D42F3950484BEC8443", "bnb1m0j9gkmwzs7l2dec5jmuehn047k43q2l5feuqm", 1.08972069, "ETH-1C9"},
		{"AD320230E8CA73922A589EF7DB80EAAB55F784E2B26697F0EFD8AFE63D7AA360", "bnb1q3wkg6p2plf88342qtxe943qsnrhacp7f2f364", 0.977, "RUNE-B1A"},
		{"C45D3FB30E234CF7EBA662CD5FDC6A653BD7201467072447BEE74D563421EA52", "bnb127nls3lt2lrdjgr32l93kelpp8euc30kc9cqfx", 85.75213, "RUNE-B1A"},
		{"F1ADEDDEFC52659FBE5A473A008E1D4D796278B9B08A1B6EFFC16DB504E031F3", "bnb1e0ynwm4qyvpqzdv7hx20trssjx3cx6jpcv9rm8", 0.10178468, "BNB"},
		{"9F0B4AB193DF32F737338ECC1341349CF7B1566EB1BDD33F7D8C10B643B4AF44", "bnb1d78r2u65mdrrez70tvrt3a7t0pp88yk8eawx49", 79.0559, "RUNE-B1A"},
		{"641526261704400C907520721471EE6D58F2B909F1112107FF206127FD11395B", "bnb1fc3yvr0yt2mztv6m377a5jh9m677880d9m25cx", 2.47, "RUNE-B1A"},
		{"DBDC036F4AB177FCB5C578C09A8C1A1BAA50645E15AC8BB983EDFBE97C89C244", "bnb1gq9w99x7k53tyxnlx02f4ymvfv935ls3063hjl", 99.974, "RUNE-B1A"},
		{"E72C7D0E789164558B19372D60C4E75314E5BF6BB7A0515F9F63837E09AD2211", "bnb12d0p6s47l5tr7xs0qedu7v300dc0glc3vcstx7", 0.2991625, "BNB"},
		{"8EA505016F51A3784246E625D782121CF7B1CDFD6D9761CCF257A16C7E469386", "bnb1s9hznv329sa44hd3fqc2tkt34mwn9lsfydvl3l", 29.7763598, "RUNE-B1A"},
		{"52EE48D2AB26E3019D94468B05D55093FA067CC8DAE161621C8F7C16DA1F7125", "bnb1ps98jc7ahv26ystenqhuh24fmxyv7t9zzc85xh", 17.5, "RUNE-B1A"},
		{"9495BBC4D8F341F05BB1DCBE75C09774DD4C75FA41E4E7DD3B8989AFCCBBCC1C", "bnb1ysmzc8047v2z9828skad5jk252e49t7d5dn564", 125.926, "RUNE-B1A"},
		{"E2B18B54AD30B0EF66DF430E3F640585EE6006BBC04461AA787D9F8FA3776DD5", "bnb12ahp8vpwhe8yw86n33clsvy5z2vgq8fl0pgcfy", 24.4632, "RUNE-B1A"},
		{"03679A63C0AB42470900319B7B1B99993100AA6EA73ED5A900FEF083F382DA0E", "bnb1vrugu3yq4s5zal29v6sn9pavpfjwzwp3sezjp4", 7.06, "RUNE-B1A"},
		{"117B04B214A4F97CF4C911B0150B01A8AF0AF56FA2EF919700A556ACD2C20523", "bnb1vukqle2pzckhd5jpxm8vp3yw8l2wzz9rwg9st4", 59.95595267, "BUSD-BD1"},
		{"2C86AAE999A3A7400126C6B148C7E9EB39EDCCBE85C83EB035B832C6EDC0EA8C", "bnb164jc0pxcyu8gnkd2u022z3mtst2e5cxfpt76sm", 3.628, "RUNE-B1A"},
		{"047C6698FB19A0CCD4404AD29CB074B787418B64C0502C8BCE210AF8ADD51213", "bnb19w5yazzc9cmfcjf7fm35u5l7t5npgu673936ht", 156.1715529, "RUNE-B1A"},
		{"55830B94869D5F7CBE11BE28C7B4A7443E3C3F58DEAE8108789AF9BF69570F94", "bnb1gxlwervwwuptckljxjel0lucmcwpkfg32an06k", 150, "BUSD-BD1"},
		{"7CF656F2C1944E4B8A32573FED02F65A5A055B09BFB353A8555189C11E56ADF2", "bnb164jc0pxcyu8gnkd2u022z3mtst2e5cxfpt76sm", 15.76, "RUNE-B1A"},
		{"26FB54195FDA0D4200A3510AD7939FF8DE02DAFDB7F4FB250B51BA3A773CE48D", "bnb1afwzv54gff0e62t7wx9hl9eng4qk0ajqfgh62g", 5609.330992, "RUNE-B1A"},
		{"C4B9586FCC72E1969146E4F4FCB1BF9EB744AAEA70C9B5D01F568ABBA8B6D0E5", "bnb1e37dllsfm9p0h883xusz6hkxle54am6k2kwdkc", 1, "RUNE-B1A"},
		{"30B5E211A1C6DBC1B7C939490B2A19605192472BB71C3C21CE7EA960C40808DD", "bnb1n8v8grqlq22h70s87g2rc9dhp3mddughpx8c6j", 0.1498125, "BNB"},
		{"0F8157A253B9231C860DFB4F71EEE0D3553C7A2DFA4D4C9D89AC5F33EE78B65C", "bnb1n8v8grqlq22h70s87g2rc9dhp3mddughpx8c6j", 514.2599, "RUNE-B1A"},
		{"952976E7AF4F9017CEBC2FC0C37F2F815A85CE5B67D1E485405D89197EA4F760", "bnb14a3sppfhumq64fvfveatfd5gqfg6ggpjr9q9xm", 0.108, "BNB"},
		{"0779BD5785C0E27EA0FBC39965C21182A57697F5091CC14E50E6FA3FF418E5FC", "bnb1524d9an2cm8zhts5zrs9auty8dnke4cnc2x72s", 14.266, "RUNE-B1A"},
		{"1A52CD381E1873C96BFF09411E88A8354CB2700A97EF1FF918EA92C02F9313C4", "bnb1rr62n6ut4njflm2dkk3f0p9zy59cuzl9w74ldh", 10.663, "RUNE-B1A"},
		{"A0C98DE72DE296257E0384AF5A166AFF830C8A020649991CD40E350F48D57F0F", "bnb1zn8c782lncq527zflyghh3jf3anqgf8rwmwtv4", 43300, "CAS-167"},
		{"138493B9E8240C134C3FBE4FE2C2DB3334990E88145B7BEC7E91B8887011F750", "bnb12d0p6s47l5tr7xs0qedu7v300dc0glc3vcstx7", 3.9490056, "RUNE-B1A"},
		{"331A747A30AFDC051E9A7251F4C68B9D95DC6BF5942D68E1562E03A17F0F173B", "bnb12d0p6s47l5tr7xs0qedu7v300dc0glc3vcstx7", 6, "RUNE-B1A"},
		{"1F42A9EF95CBF6D2DD454C071EB31855D586794B214954D38AA04C0282022CE9", "bnb12d0p6s47l5tr7xs0qedu7v300dc0glc3vcstx7", 99.966, "RUNE-B1A"},
		{"889AFC8FC0E4B4B843261B66F8463EB9FAE9C95C722D569C44ACF6ACA7BF4750", "bnb153w67dcy7znda5c0xz9yzkxwzprqac4thu9swz", 252.252403, "RUNE-B1A"},
		{"4A4C867E21F8041B920BE0B8C18F3A7981EE4C4B5813936A3831E4FFA0DD25AE", "bnb1chv7uxr79hs7nuj48s2dfh3eeyvw9gnp7qpqdv", 80, "RUNE-B1A"},
		{"57F4E20E2A0AA6548DB10988B2EA68EBFD4384B7D3EC99F9A17E2B20E666598F", "bnb1en9lzcakhnudhdqpk5t7dqh3atncfu36mj4g7z", 100.03134, "RUNE-B1A"},
		{"933818BA5A4F27CA3309D53B22CA5AD26277AE11EC135773CD4AF1F3BB4F2FA9", "bnb1agx9jgj54e3f3cyfun2wlv9yty0hxdm89gad7v", 252.6842, "RUNE-B1A"},
		{"78BFBDB747233E25FAF1D59CA542457560BB81C4D96B5B3E2C2056CD664437A9", "bnb1dht3y9gm200n9h7pjlynl74t3kfdqg55qtux5r", 11.2, "RUNE-B1A"},
		{"9CA45BFC2E42A24B2779A626574481A5127157DBC81D417AC8F4FD024817976D", "bnb1dht3y9gm200n9h7pjlynl74t3kfdqg55qtux5r", 11.1, "RUNE-B1A"},
		{"686EDE578BA46FFA9EFEAC9E1667DB95F535E31D14CE0BAFBEA798442672A103", "bnb1jw620r89kk8ueg0wpfmra2lhh3c3473jjuz6x2", 173.21, "RUNE-B1A"},
		{"8120830E7D38079F73EB20E03DFA17638999F50A4C1B068D2A07657C1E77A553", "bnb1u0u2hj8seq4prgcz7dk87alrlwq5kwpad3s83v", 1120.5272, "RUNE-B1A"},
		{"18C65B412960B2482F38196390080AD87F9236ECA85A67C59A7ED211590544CE", "bnb1dyysfz2ecs20p33spvmk36fuyhqje9dac04299", 5.957, "RUNE-B1A"},
		{"6B8FD3FCCE475476B7FEA886BDE950DF107A12A9A27B6F32AA338F52782AF2F2", "bnb1ejn27zjq8zehjpvqs0856845xk00ypu62g5tuy", 400.526055, "RUNE-B1A"},
		{"476E5C238329AD6EFA346799A6B4DCD4DCC01BD4EB55BEC54A5D2DF84CDBF0C5", "bnb1fuxxzeun6me55fk4phm2nqqfztsd82etvztsn2", 1.00572, "RUNE-B1A"},
		{"DA0C266AE094F1F75142057633EBEE2A99389351DE9A647780C7E96FC10C2898", "bnb1qseefdrdskdy0lhgd9jns9tt0l280xtftrgvpg", 36.002, "RUNE-B1A"},
		{"FF0D24AA6E58BA3A53C2BB14D6260734F4D6E38DD7579EFFAEA5F586897309EB", "bnb1eqxsxe0yqnrq68k6quunqfaalqwqyg52yykut8", 100.5736732, "RUNE-B1A"},
		{"ABD677489F289A18A800AD5071FC4B730069A49653D3CE626ADEE5D7889F7D39", "bnb1746dz6vvxqljh60x0zy65k2degpal7rxk5qlfp", 366.4234204, "RUNE-B1A"},
		{"B7E2C03A79A159148BFAB454AED45020FB30A63529EA63E501CD4ADA8A1EE4F4", "bnb1z0tnnws7ap37npn274n684luvyzxd72sl4xepg", 10.02191076, "RUNE-B1A"},
		{"48D5DCCCE9743306DAD1BE66E2A5E87D315F4E4F760225E775E4F40F841C8667", "bnb1cxx3l6j50pkcmse4rxeg7lu5rtyjn0flfl7da9", 15.702, "RUNE-B1A"},
		{"48242D1F96344ECF05F47B438C70A62ECBFD69E91FA188A54E410E757BF61CFD", "bnb1f8nwzzr9c97z668v4qz90ssmnk37txl5mlhczw", 5, "RUNE-B1A"},
		{"0CBB1DED208A7C0EDFDD14E2EE950B636C497DF68C8D4AAFD71BB7E635D8DF3D", "bnb1752gtqckrng6uxr6wdmhlxzpwkhffxseg08u22", 24.5, "RUNE-B1A"},
		{"24B6EDE1841115D2BE287AE2A7756D9C406593A555BB7CD29B6D1EE456003B48", "bnb1eqfyhkumpzxplsdjjudj302nktj320jj4x0fcd", 760.861, "RUNE-B1A"},
		{"E3D464210A41A874DEEC525FAE791C0AD22326124CFD1B9118F768AD93B98B80", "bnb1dp409mu2kc5frgkytnn7lxrzc5855e9hdmucun", 0.5, "RUNE-B1A"},
		{"28F3B39ACFD2AD0DDFCFA8C53F193515F7B0E3C9DC9B12974D0588A54DC3DF19", "bnb1vrugu3yq4s5zal29v6sn9pavpfjwzwp3sezjp4", 30001.73045, "RUNE-B1A"},
		{"805D99B5876E7CCECEF3C70FB8FF71CC8B6E888DACF311D35CE8A75CED87827D", "bnb1pt38g0xddfrkufv554dexuq9a98f843t580cnr", 16.553353, "RUNE-B1A"},
		{"CDC8888E7142BC08FB5FAB120008C73486A55AC38008B098BBCDF84F72C0273A", "bnb13slj5utg460mdzmnccp23wjq8a64ju9l4u02dd", 59.18399431, "RUNE-B1A"},
		{"2EF665B8A35FBF224431CFFC2D5E454262F9B30B78F63A7D92E9AB5302B6E902", "bnb1gufh8sp5kaqsetmwg9j6x5rfp6gzxkk6hngzdp", 543.0727504, "RUNE-B1A"},
		{"34C91917D01158E335736C37A8944A56B8FFD7F222C416F3A4A175369CA98268", "bnb1pqmlr4mewzl72w6r26mmnhqfzgwhk0cu3se6la", 1096.223315, "RUNE-B1A"},
		{"0169142021C2A82F608872404C02F04513AA61BE4F422D641ADEC70D47384AF9", "bnb129f9dvzkla80v90vgt7e5r84gvea26c9cz5pyd", 2.9, "RUNE-B1A"},
		{"BF8A81481F56A0C95009B6CB021A52B8F4F19AF6FBE9CC3EA5CE0F6E600C2DA1", "bnb1chnqh4vvf0hqyymkcfcrf4grq88f9ks3yr9v2h", 1, "BUSD-BD1"},
		{"7AA0CAE1E1B73513AEAD98DDD59C0B64C39C2CD159EBCBD431D203039BF395C8", "bnb1jggavnaxrt669tr6czrfelkpqm0q42y7jkmnh8", 5.183, "RUNE-B1A"},
		{"4E5EDBED7DD3515065E92842AD8E89121AB05F77F57B275C2AE49E96B0EB7815", "bnb19424lc6n639lspmshjcqa3e4g3xcdfhlmvtyhv", 99.9, "RUNE-B1A"},
		{"906B60C8BD5ECF2578438C3165130DA90BD240D638E45278C211A9B0DAD39101", "bnb1h7rdk6pe9l2w0mt64m5axjvtylrave4f87g9nr", 8.4433734, "RUNE-B1A"},
		{"AA12CB4BC2D0DAF928C220E9E9AC614FFA1A5DD6AF958CBD16269E8E891B8E23", "bnb145yyrd783ye42c699qhp0l9ke7vq6n4j7kcgvd", 1441.785562, "RUNE-B1A"},
		{"1DF73EDF6CF201800E62A7C05249F3FCAB4109C9C9468397F071DC904C8E20C4", "bnb1qrltc54faytdq4p80kraat7p83av6txqt4klmj", 284.4659987, "RUNE-B1A"},
		{"E6F49096971DAA96791FC1101333B4F0DF887D023F410ECE411CF1DB6FE321C1", "bnb1c8vl0r5ue53awz408ems6r3pvk04t7uenwt48w", 10, "RUNE-B1A"},
		{"E3A0F6E465B793EA4144FAE42A5985FAEBFBD17780A4CF147C24B2FDC2D9E1F5", "bnb13xrnj2e7hpae4tnksm9yxqznk9m95ujk0pv6gn", 3, "RUNE-B1A"},
		{"B139AB86B9B67EB4F729C6300EB136E22B6CFF92D1F0CF98284D0E8569E28034", "bnb176heu734unjw5hjw84tqulmha2fpuxaxlynclu", 76.20612, "RUNE-B1A"},
		{"D2EA3A93323546ACC71632393DF6AC986119752B52FFE422207060FC001BB226", "bnb1a8vapazge7g6z3fgh7f6m384n7p3ak6snln9kh", 0.83965422, "ETH-1C9"},
		{"032DB75290EDEE1B96EFCEADB3C32F9C0E97E059123782DD6AF98CDBA9F2C7BC", "bnb1yvj0p2j994u7qet9qd2fzgv0smr8098m6e5f0w", 40.665, "AVA-645"},
		{"B5BA7BE324095F4B079313175D10AD7822885CE3EBF4D81C883B214B2DD01E2B", "bnb164jc0pxcyu8gnkd2u022z3mtst2e5cxfpt76sm", 14, "RUNE-B1A"},
		{"EA593992C9E54DAD028B3E9F8799C506454B9DFCE69356829219C4C2B6E834F5", "bnb1zn8c782lncq527zflyghh3jf3anqgf8rwmwtv4", 15340, "CAS-167"},
		{"F536F680C72BFCFD339FC43442F2F23445BA9C2201CB9B1A176BF006E02C8B5D", "bnb1xehlqphf90r7cca8s28kmnzvfkl4vsq2aeg4h7", 61.499, "RUNE-B1A"},
		{"BC7FC7CD884A2A4E46A8064D98213D348598504C637FA65D109072D1884CC572", "bnb1weh8yy8ywrhuc9nf9h2dymr5h059wlua2a97vy", 0.20983857, "ETH-1C9"},
		{"2FE1F5B67877B59A6607391146A92BFB7CFABF183BFCC57A7A10FE5A7995792E", "bnb15qx8wpupc9tx69uruszqhvv6hxhyp6w4h0tp6g", 0.0048125, "BNB"},
		{"571052F9C10FE4ED55BCA05F5EF0D5F8115790156EB82E6C676CD2465ACB6B0F", "bnb1jynhd850akjzzw8g9aefx3km66vshe38yhfhkt", 1032.193937, "BUSD-BD1"},
		{"A906A663E7357CF0250845117D0665D1F1E2239071C92CA215B2D2907DD00573", "bnb1p9zneqlme5tmatkyzxg3tczer4rxwxhgqnyhmf", 14.249292, "RUNE-B1A"},
		{"CDA2C4DC7DE5124DE69407784A87B09814042F170C04056EDBE6E18EF0128BF1", "bnb1y86p3erdsuukhdelpf8vq3auzjseuzfen348cl", 0.10047994, "BNB"},
		{"BA99E1FEA1627924651CCEF154C82D03F4FD62E1A8BEB9DC757478456A534D6E", "bnb1tk05fcy0zsg3atxtp24s70cpjnps0h7vrk9kg2", 319.9599277, "BUSD-BD1"},
		{"A56A8353F1DE78CC0626BAC40C6E224AB576E8B434C623F7474F7E19AA7042E5", "bnb1wfghslmn9zax6rg2fa2qv2hqahxvgscrna7883", 0.0001, "ETH-1C9"},
		{"4B3EC98C9BA3B0D7CB7B68862DB32A138930ED157736C5B006D9A1E5B581E3D9", "bnb1ydu26ssyx48x258qawtrrgwhpj28mhqhake57p", 1, "BNB"},
		{"3D3D1C370CE5BE090DA5EEAAD1D6B51C4DABB1B1FCC26E62CDC1AE48AC17E1F2", "bnb1y3pp87yegy8l38amv4dvsmznl0cpzllvqc3t25", 3440.418072, "RUNE-B1A"},
		{"9A457583D2BBC0B75BDDECB0CFD2C0DE5BFFB19C2415AF28631735B4A65D5C08", "bnb1u2rd2vkpdr24lw9mddkw7k2v0nna94vnfsuvfs", 35.824113, "RUNE-B1A"},
		{"1210AB1E86AA55FD634188B7F9BA9AB8342C58868F7A6964E62162FCE4EB66B5", "bnb16zhl9f5gtclneyy6mcczlkxwrwcupqenknryd8", 0.027, "RUNE-B1A"},
		{"73F5D7C41FFE885CCC345FEA33F42DCF935C8004E9BB3942C67971548389C2DA", "bnb17uy5a2faxvddnled85am3d7m78gw303d9qgp7k", 0.0198875, "BNB"},
		{"2793CD3D0DA1220AB568F70CD88808F3DC0AE2B91A816D66CC6FC55D636701A3", "bnb1f8ht8rp7dk0uxshx8ckjpxn9zgsphxlklwluxc", 1, "BNB"},
		{"2BA7E53B2C6D3D3165F2F6087924A1E7B44CDAF383CB81D61281CBEF4C7758C9", "bnb1m9vh6wrtvd900fllze6pqzgk82a0s0vmeacjs5", 0.09020911, "BNB"},
		{"F0D1DA17AC3AE17B546B39F2D368A1C0EACAD68D3FCE51D3087FDC25945D9618", "bnb1zevt9n9x0n3k398ghkugh3wzwmufdrrhhnc6ct", 36.74990723, "BUSD-BD1"},
		{"157FDF2EB71403A6E9D535834BC95223C8D9C43639657831315D0A42428B0158", "bnb14e2e2jn7cjxca6ys63w9jtachf2c7xx50jsnw8", 5, "RUNE-B1A"},
		{"88E52E21C9A5ECF2AF7663B6CC827EE31CD4B7C3F201A38EA3E843A660686165", "bnb14e2e2jn7cjxca6ys63w9jtachf2c7xx50jsnw8", 5, "RUNE-B1A"},
		{"899B355C8051370F669FD4FAA3F921DE2CE10362CE6C8598248A8FC0687187BE", "bnb159wrhm5gt94fgkzldp8zmfgnlxa365k03zzsfm", 1.0911, "BNB"},
		{"E3555213FA7B8A3ECE24B77266E4757A7137B007EE5C7CAC7794580846D611CA", "bnb15ktsdv6w5lp973vduxukgpg7zdtg8r2hvws00c", 8.852112, "RUNE-B1A"},
		{"7C00E88FC775A0D5356767D89FBBC2888F4A07237B17BC4524012532117817E1", "bnb1vf3ljyga82sewznuc9lemvm2t4335wtm2xrxn5", 1143.246025, "RUNE-B1A"},
		{"B63C5591635CD671B7D442A5FD202A74DDDB0428096AEEAC1D8476E652788D22", "bnb1panzh7z479dfwj7j5dxkfa6hpze3j3gd8wykcj", 4000, "BUSD-BD1"},
		{"87B7544A4B8521BF2183BAB99AE68F01E4A4AFDEE5172B15B1D4D2C9E377F679", "bnb1amyth8xm9lr3l7h20zaxl5xw0taxz8d5y2zxlv", 218.315439, "RUNE-B1A"},
		{"C465E3EAF174BECB1E88402017E540FD0001B503ABA4C0606ABBE2EE03200AEB", "bnb18hpy44al65skdkenuh4ns47u8nf24fw2dtk8uq", 0.05113845, "BNB"},
		{"A2F9102F98BAB1A936E63CA4E7792B840487A6812A80E551E0F7BE5D82DE64E4", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 7606.5, "BUSD-BD1"},
		{"E5FB928D24028185FCF94777B312A2B1D9EB85241C116BFE43626D3541747ABB", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 4757.85, "BUSD-BD1"},
		{"70D0C9206E47C7A2FA3C5AD1E2BBC1B3F0A90BC535028FF32A5AA9EE71047734", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 31170.8404, "BUSD-BD1"},
		{"76367C6A9A580BDDAC2D9C6CCA07564A1CDA741BC4A11B8A34FD965EB3E6913C", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 7734, "BUSD-BD1"},
		{"BA182566510AB2A7799075AA6078992058ECFFEE599BB4AFC7032CDBEEF64EA7", "bnb1f0kxq029tgac6zhx83q4gppsn857rqvwdzsw76", 1000, "BUSD-BD1"},
		{"72E92F8BB2FE9B26F4FAF4751D69FBEE76782F200149A805AC00DDC0843935F6", "bnb1m8wuq0kqq5cju704fhp94eth436v6zpvfl0ylw", 500, "BUSD-BD1"},
		{"68A009E3CCAB8A8725E15A36E308D972A74DB785ED4B8D966C0FFC445CF827E8", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 1019.749, "BUSD-BD1"},
		{"62F46CEF75ED79C78350E4905AE22361FB80CDC1DEFB047F91E6CA242C84174A", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 4935, "BUSD-BD1"},
		{"9F4A8A9942AAA81E829D95FD6D6043E352762DFCA6B59A5059D7E49C68885E12", "bnb17f84u3u85av9r94mkc6rfe6jeqhkk3sj67fx3l", 1796.2116, "RUNE-B1A"},
		{"79AD53B82B4A6939E357DBFB23C9AC3479BD52FA7EF9EF9D5E54C734154D158C", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 4729.65, "BUSD-BD1"},
		{"5B2354EB21D4ABEEAC7C8CB35CFC2F304147B2B7A1C581DF10D64927A4DA46C8", "bnb1dx3yxj37e7jku4azgehztr2vf2uwkjjlxq8xfh", 370, "RUNE-B1A"},
		{"5126B3858B6296E810FD061D40F9D5A3D3915D46D7757E2120AADFF4B3EDD349", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 8077.5, "BUSD-BD1"},
		{"9586D02514B8C7951205A0A2E028A59B90BD678A35236FCADF912F629F3B6E9D", "bnb1m8wuq0kqq5cju704fhp94eth436v6zpvfl0ylw", 35, "BNB"},
		{"ADF763E3A0075C3E272CF8E0B32BD77F21C528C3996E0FBF085DB9D03F555421", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 4719.15, "BUSD-BD1"},
		{"C90A60BE87EFB7E340C2446A64A0702933525CCE0F5084E1A2C0323D98D9B84A", "bnb1rgwulut04q6ef85tcvk76ey5yld52slaplwwa9", 54.06594774, "BUSD-BD1"},
		{"BA9E117D6E40B9ADB257FF6A4B06B467DF82F47FF93D55191E0BDCB0004C5309", "bnb1x4xuxzeyckpctkhp5g425w3wf7y8m3w7z57s0f", 388.373211, "RUNE-B1A"},
		{"0E6E05E73D7DD5A74FE9F125748B2D24908154B75661FC4248B3B3901DB423C4", "bnb12xufpsysr6r0tankhqdaams9zee0tzxy006qns", 5, "RUNE-B1A"},
		{"E71642CAC325B733A3581F31F1DA64A52AA5AC590980E5630EF80BE1D906AECF", "bnb137xvx6ceqpxe34xr46y3ax350fr2q4zymw5l94", 2500, "BUSD-BD1"},
		{"A4F17152DDB1473AE3341311F256171A532558711B70467AB606040E570A6F46", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 30376.57296, "BUSD-BD1"},
	}

	for _, item := range refunds {
		hash, err := common.NewTxID(item.hash)
		if err != nil {
			ctx.Logger().Error("fail to parse hash", "hash", item.hash, "error", err)
			continue
		}
		addr, err := common.NewAddress(item.addr)
		if err != nil {
			ctx.Logger().Error("fail to parse address", "address", item.addr, "error", err)
			continue
		}
		asset, err := common.NewAsset(fmt.Sprintf("BNB.%s", item.asset))
		if err != nil {
			ctx.Logger().Error("fail to parse asset", "asset", item.asset, "error", err)
			continue
		}
		coin := common.NewCoin(asset, cosmos.NewUint(uint64(item.amount*common.One)))
		maxGas, err := mgr.GasMgr().GetMaxGas(ctx, coin.Asset.GetChain())
		if err != nil {
			ctx.Logger().Error("fail to get max gas", "error", err)
			continue
		}
		toi := TxOutItem{
			Chain:       coin.Asset.GetChain(),
			InHash:      hash,
			ToAddress:   addr,
			Coin:        coin,
			Memo:        NewRefundMemo(hash).String(),
			MaxGas:      common.Gas{maxGas},
			GasRate:     int64(mgr.GasMgr().GetGasRate(ctx, coin.Asset.GetChain()).Uint64()),
			VaultPubKey: asgardPubKey,
		}

		voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, toi.InHash)
		if err != nil {
			ctx.Logger().Error("fail to get observe tx in voter", "error", err)
			continue
		}
		voter.OutboundHeight = common.BlockHeight(ctx)
		mgr.Keeper().SetObservedTxInVoter(ctx, voter)

		if err := mgr.TxOutStore().UnSafeAddTxOutItem(ctx, mgr, toi); err != nil {
			ctx.Logger().Error("fail to send manual refund", "address", item.addr, "error", err)
		}
	}
}

// migrateStoreV70Pools update the pool balance
// the balance change is based on the balance of pool , total asset in wallet , and the amount of asset treasury is going to top up
// https://docs.google.com/spreadsheets/d/1xXInEE3bbx1YTvoffvrMJE_BO6WYI0hgD46wyK5KBtY/edit#gid=0
// the pool asset balance adjust is required to make pool vs vault solvent
func migrateStoreV70Pools(ctx cosmos.Context, mgr Manager) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v70", "error", err)
		}
	}()
	inputs := []struct {
		asset          string
		subtractAmount cosmos.Uint
	}{
		{"ETH.ALCX-0XDBDB4D16EDA451D0503B854CF79D55697F90C8DF", cosmos.NewUint(96688561785)},
		{"ETH.SUSHI-0X6B3595068778DD592E39A122F4F5A5CF09C90FE2", cosmos.NewUint(5159511094095)},
		{"ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48 ", cosmos.NewUint(161845558262800)},
		{"ETH.YFI-0X0BC529C00C6401AEF6D220BE8C6EA1667F6AD93E", cosmos.NewUint(727860649)},
		{"ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7", cosmos.NewUint(99023689717400)},
		{"ETH.XRUNE-0X69FA0FEE221AD11012BAB0FDB45D444D3D2CE71C", cosmos.NewUint(2081880169421610)},
	}
	for _, item := range inputs {
		asset, err := common.NewAsset(item.asset)
		if err != nil {
			ctx.Logger().Error("fail to parse asset", "asset", item.asset, "error", err)
			continue
		}
		p, err := mgr.Keeper().GetPool(ctx, asset)
		if err != nil {
			ctx.Logger().Error("fail to get pool", "error", err, "asset", item.asset)
			continue
		}
		if p.IsEmpty() {
			continue
		}
		p.BalanceAsset = common.SafeSub(p.BalanceAsset, item.subtractAmount)
		if err := mgr.Keeper().SetPool(ctx, p); err != nil {
			ctx.Logger().Error("fail to save pool", "error", err, "asset", item.asset)
		}
	}
}

func refundBNBTransactions(ctx cosmos.Context, mgr *Mgrs) {
	// due to outage, and binance daemons getting reset, some transaction were
	// dropped by the network. This is an attempt to recover funds and manually
	// refund users
	transactionToRefund := []adhocRefundTx{
		{"54E4DE4A93CC4FEBDE35EB723B51F4490923958FA32BA474901C02ECBFF047B9", "bnb1h05y4pe6heq2g8u6955kchrdst7s8gxspg3u2s", 1, "BNB.RUNE-B1A"},
		{"7F6EC99B8F82F36501DCCDDBF54E9BBEAA51A8604CD39E9635027711967B2DF9", "bnb1h05y4pe6heq2g8u6955kchrdst7s8gxspg3u2s", 628, "BNB.RUNE-B1A"},
		{"2EF95165FDAABBF72CA4EE10DBAEF825EE06C280B8B2098AB55A89B922105C57", "bnb1uresxjchzzffvdvjgkf2su7kcupx5uycc58ygs", 34.166, "BNB.RUNE-B1A"},
		{"1D8DE0EC576D7C9A485ECA74401560CBA65870F9E07DF814257B24C0EC8165FC", "bnb1kcay57v22f6gn5kf5vsnstxv8dfnaz2gecckeg", 47.477, "BNB.RUNE-B1A"},
		{"24A7EF13DDDC77815EB861E98BD8EA4B0E393F1D9BD1CB3345FD46CA9CB520F4", "bnb1dulgfyr9f3jnyu40f2hnaarrm8apkhdfsdvpcv", 0.976, "BNB.RUNE-B1A"},
		{"EA6627C95264B150DB729D2BFBD9C52B91300A5118C404F519244C07E9A0A4C1", "bnb1zxtn3xt5prfd7w3st53jjnkeuexkyn7uh8a24c", 126.10821, "BNB.RUNE-B1A"},
		{"402DF369568560B79F90B6B0800C75802858800814AA196B911039E524357EC6", "bnb1t68m836ztz7jvcp77uf2djsl4mhfqxa8h5jdl0", 0.002, "BNB.BNB"},
		{"6FFFF126170C6B4A144121DD5E5E9A195B9355F083C5494C4C9BF2DA855B9739", "bnb1kcay57v22f6gn5kf5vsnstxv8dfnaz2gecckeg", 251.082, "BNB.RUNE-B1A"},
		{"57CE7BD54ADCC89061B31B9EEB1823E475337E1B4061A3377D4FB2B2071EC65C", "bnb153jd3vzzlhusmt4gvv6pq8nsewktcpf0799wdf", 44.1178, "BNB.RUNE-B1A"},
		{"37E64B6DF40541E9C77107C29BBE3A03BD051EA072ED8CBDC2CC51CF3AD2BF59", "bnb1rvzk2wkn6f8k447s63tf6yuk52tef4m0t79vn0", 50.84107, "BNB.RUNE-B1A"},
		{"B1D6CC11BC34E680362E26CDE8628444222D5AEA30520EF1A38BF3B1BB63C246", "bnb1km5j6s27w5zr9677p3nydv7gq8xemh53g6keeq", 100, "BNB.RUNE-B1A"},
		{"0F5115DBB611B658A576084ACCA1C7573CDAB3389E3CA015A6406195BA00AD75", "bnb1uw5jgsm904e5hczpm7lhrlmy9lq05hjjrv4xqx", 3, "BNB.RUNE-B1A"},
		{"F46B5F083B587E29E8A11C75E12F7C8E81562E06AB5466A877B308ED34ED6AA2", "bnb1t68m836ztz7jvcp77uf2djsl4mhfqxa8h5jdl0", 0.001, "BNB.BNB"},
		{"B57C28C425EFA75560C32CA6EBB9DC2BE86EE118837B33840008023808CD3583", "bnb1ljkguffu227zwvz29quzn7aj6a3wdf3nc4rffz", 641.09822, "BNB.RUNE-B1A"},
		{"87E08B731347CE4C57A9F622FD7BCEE092462F48EC36AF312CE5AA5B80717942", "bnb1t68m836ztz7jvcp77uf2djsl4mhfqxa8h5jdl0", 0.00569604, "BNB.BNB"},
		{"4030559AA45D78422296581D6C352FA90B51F913DF216E953B0B044F5107A17E", "bnb1zptk7ftjmessmw06q2qg4te2d2cd54973wthye", 2.846, "BNB.RUNE-B1A"},
		{"2C6FA678407506A4CC8F8C8CC1EC07511C6228120B963A2FDDD4A22E67611AA6", "bnb1zptk7ftjmessmw06q2qg4te2d2cd54973wthye", 0.106, "BNB.RUNE-B1A"},
		{"A77AE682E0244D3203A31BF2AAD9DE317742B0D38BED40162E093EA5B3411233", "bnb1n63mqfexeqj5rdc08ww4jyu6ys5k3s2h8n95tj", 0.04983093, "BNB.BNB"},
		{"189BD06D18EE322D5662BF6AD5F18AF44286FD71985EE8408B91726EF59F29A0", "bnb1q3wkg6p2plf88342qtxe943qsnrhacp7f2f364", 498.446, "BNB.RUNE-B1A"},
		{"EAF6B55E6718BA0E0E81F425DB34E4CA6E9C36D3CE16562362DF2FC57FD3E65E", "bnb1mfz43ul3njltlgfhn976l88a2fd0uvcm2xyl6p", 1, "BNB.BUSD-BD1"},
		{"920E66FF45FF39EE469D00B60CC739A574C0F9F7774820BF6E92D822E3FA7A7E", "bnb1n7ne3kpjqm7almtx65aff2pfps6duc0wpumts0", 2.404065, "BNB.BNB"},
		{"9D3C914F4049101410A86298E471E5C3C113DF45568087B362A2376C7575663B", "bnb1ukfhdgy9c0ws42eqt786q82ept9ht6fag88t6u", 0.01, "BNB.BNB"},
		{"55C3F13B1C07B178F71DE62CD249673F042FBC6BC6742DA84AAD620604349D6F", "bnb1afdvrfpls66n2hm58lx6je0yqmv5namftuwdrm", 0.41, "BNB.BUSD-BD1"},
		{"CCB5E301893EBDE7D498E0EBFBB6582B5230FB2F203C27D1A37B127380BD1E3C", "bnb1afdvrfpls66n2hm58lx6je0yqmv5namftuwdrm", 0.5, "BNB.BUSD-BD1"},
		{"5025E6CFB2112A15B9F399A12F318B84E975ED9D9E82F2DE5E3DE30A991D462F", "bnb1afdvrfpls66n2hm58lx6je0yqmv5namftuwdrm", 0.0001, "BNB.BUSD-BD1"},
		{"41DFA55BDF79BDB4393C15CD86D971307E1E9825BB33B68A7709991213940B15", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9969772, "BNB.BUSD-BD1"},
		{"09970D09FF884612C5A09F8DA81733977545B810F8B8DDEFFBDEAF3BFF27879E", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9993278, "BNB.BUSD-BD1"},
		{"2243A8F3938031C485506955D1F5170EB3D7C6226444AE37F122D29194CB9208", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9875878, "BNB.BUSD-BD1"},
		{"35CFE9093F532ACAE2B7DD79C30D34777C6929A0CD6A5AD02B335B5DC69906B9", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 129.9999985, "BNB.BUSD-BD1"},
		{"3D693B525AABDD3FFFE6F88082BC4EDA69F50A9E8336BD3B58DDC916E35B1AAE", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 119.9999999, "BNB.BUSD-BD1"},
		{"1D783C114889B42A58091CB5097DF7877ACA4852BB84AD989C4102692B5CDA80", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9969772, "BNB.BUSD-BD1"},
		{"E4546065A5E1F7D0D69951E3990FE9333CCFB34AA14FFAE4A6387F080C15F7E4", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9993278, "BNB.BUSD-BD1"},
		{"E3A23F59F02AF1F2FA312A92565C28DCB468133A164719DF573B7CA6D8791138", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 129.9999985, "BNB.BUSD-BD1"},
		{"C9C027D0AF581A8555D6D728B19C1933E232AC45994A3E3BB526B89DAB50137F", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 119.9999999, "BNB.BUSD-BD1"},
		{"B094713F6AA70DDDF58E50ABCAD4A4526502A3BB76DA6586042CC1368FC319C4", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9969772, "BNB.BUSD-BD1"},
		{"61537E93C783E38E1F66A9C830F25D38414CC5A82DEAD6AB353F9F724C7E3C6F", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9875878, "BNB.BUSD-BD1"},
		{"EC733CA438027B4B816A66C63BB36DD97F7928D41D3FD1FF8EE10A8E395A90FD", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 129.9999985, "BNB.BUSD-BD1"},
		{"62DC14CB8617786AC9A67E4C0BAD9322883AF4CEFF4568458AC3577D46CD01C2", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 119.9999999, "BNB.BUSD-BD1"},
		{"05FADCD5E58368F242F11A664527B85E439381F851E2E4977C94455502E3B785", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9993278, "BNB.BUSD-BD1"},
		{"A43D760B722A4720DE79941A0F22163687FA059FC0BA513C4C299999FEDE8118", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 129.9999985, "BNB.BUSD-BD1"},
		{"0FF1F83A32FDCEF50C04388A414830DB42E98AE0B75B503210BEC71B69B212E2", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 119.9999999, "BNB.BUSD-BD1"},
		{"5001D020A8A2215E265CB0CFA80EB0BBD23143C01380906C19715C818FD11036", "bnb10nqkqtw06s45x52qu5m27gqamfwz3pnvehgct9", 113.328, "BNB.RUNE-B1A"},
		{"166EAA50B2F2F92138B6C8D80A86001D937BAE736AD8E267EF0C1CC049725FFF", "bnb1q3wkg6p2plf88342qtxe943qsnrhacp7f2f364", 498.954, "BNB.RUNE-B1A"},
		{"76D38C831622F356E93893A736D9F68B55BE81A2A1C323D0A044F7AB8C86F6B3", "bnb1afdvrfpls66n2hm58lx6je0yqmv5namftuwdrm", 0.5, "BNB.BUSD-BD1"},
		{"8F7FFA2DC9BC7876F6770265C0AF55616FCC43245CF2C8D8E13DC02542D2E0BE", "bnb1afdvrfpls66n2hm58lx6je0yqmv5namftuwdrm", 0.5, "BNB.BUSD-BD1"},
		{"55ADF8DDADCFFD871BAC4CDA45C244CD3E682675E6FB79B541FD32C378425B71", "bnb1afdvrfpls66n2hm58lx6je0yqmv5namftuwdrm", 0.5, "BNB.BUSD-BD1"},
		{"C0D1EBADC2B104D62DE3D0C6E34691C3B163FC806736D2D7C03F1BB47ACEFE58", "bnb1afdvrfpls66n2hm58lx6je0yqmv5namftuwdrm", 0.5, "BNB.BUSD-BD1"},
		{"76325C9563DB3436BB2EA1ECE1E702E0EF375DB91491A2D42F3950484BEC8443", "bnb1m0j9gkmwzs7l2dec5jmuehn047k43q2l5feuqm", 1.08972069, "BNB.ETH-1C9"},
		{"AD320230E8CA73922A589EF7DB80EAAB55F784E2B26697F0EFD8AFE63D7AA360", "bnb1q3wkg6p2plf88342qtxe943qsnrhacp7f2f364", 0.977, "BNB.RUNE-B1A"},
		{"C45D3FB30E234CF7EBA662CD5FDC6A653BD7201467072447BEE74D563421EA52", "bnb127nls3lt2lrdjgr32l93kelpp8euc30kc9cqfx", 85.75213, "BNB.RUNE-B1A"},
		{"F1ADEDDEFC52659FBE5A473A008E1D4D796278B9B08A1B6EFFC16DB504E031F3", "bnb1e0ynwm4qyvpqzdv7hx20trssjx3cx6jpcv9rm8", 0.10178468, "BNB.BNB"},
		{"9F0B4AB193DF32F737338ECC1341349CF7B1566EB1BDD33F7D8C10B643B4AF44", "bnb1d78r2u65mdrrez70tvrt3a7t0pp88yk8eawx49", 79.0559, "BNB.RUNE-B1A"},
		{"641526261704400C907520721471EE6D58F2B909F1112107FF206127FD11395B", "bnb1fc3yvr0yt2mztv6m377a5jh9m677880d9m25cx", 2.47, "BNB.RUNE-B1A"},
		{"DBDC036F4AB177FCB5C578C09A8C1A1BAA50645E15AC8BB983EDFBE97C89C244", "bnb1gq9w99x7k53tyxnlx02f4ymvfv935ls3063hjl", 99.974, "BNB.RUNE-B1A"},
		{"E72C7D0E789164558B19372D60C4E75314E5BF6BB7A0515F9F63837E09AD2211", "bnb12d0p6s47l5tr7xs0qedu7v300dc0glc3vcstx7", 0.2991625, "BNB.BNB"},
		{"8EA505016F51A3784246E625D782121CF7B1CDFD6D9761CCF257A16C7E469386", "bnb1s9hznv329sa44hd3fqc2tkt34mwn9lsfydvl3l", 29.7763598, "BNB.RUNE-B1A"},
		{"52EE48D2AB26E3019D94468B05D55093FA067CC8DAE161621C8F7C16DA1F7125", "bnb1ps98jc7ahv26ystenqhuh24fmxyv7t9zzc85xh", 17.5, "BNB.RUNE-B1A"},
		{"9495BBC4D8F341F05BB1DCBE75C09774DD4C75FA41E4E7DD3B8989AFCCBBCC1C", "bnb1ysmzc8047v2z9828skad5jk252e49t7d5dn564", 125.926, "BNB.RUNE-B1A"},
		{"E2B18B54AD30B0EF66DF430E3F640585EE6006BBC04461AA787D9F8FA3776DD5", "bnb12ahp8vpwhe8yw86n33clsvy5z2vgq8fl0pgcfy", 24.4632, "BNB.RUNE-B1A"},
		{"03679A63C0AB42470900319B7B1B99993100AA6EA73ED5A900FEF083F382DA0E", "bnb1vrugu3yq4s5zal29v6sn9pavpfjwzwp3sezjp4", 7.06, "BNB.RUNE-B1A"},
		{"117B04B214A4F97CF4C911B0150B01A8AF0AF56FA2EF919700A556ACD2C20523", "bnb1vukqle2pzckhd5jpxm8vp3yw8l2wzz9rwg9st4", 59.95595267, "BNB.BUSD-BD1"},
		{"2C86AAE999A3A7400126C6B148C7E9EB39EDCCBE85C83EB035B832C6EDC0EA8C", "bnb164jc0pxcyu8gnkd2u022z3mtst2e5cxfpt76sm", 3.628, "BNB.RUNE-B1A"},
		{"047C6698FB19A0CCD4404AD29CB074B787418B64C0502C8BCE210AF8ADD51213", "bnb19w5yazzc9cmfcjf7fm35u5l7t5npgu673936ht", 156.1715529, "BNB.RUNE-B1A"},
		{"55830B94869D5F7CBE11BE28C7B4A7443E3C3F58DEAE8108789AF9BF69570F94", "bnb1gxlwervwwuptckljxjel0lucmcwpkfg32an06k", 150, "BNB.BUSD-BD1"},
		{"7CF656F2C1944E4B8A32573FED02F65A5A055B09BFB353A8555189C11E56ADF2", "bnb164jc0pxcyu8gnkd2u022z3mtst2e5cxfpt76sm", 15.76, "BNB.RUNE-B1A"},
		{"26FB54195FDA0D4200A3510AD7939FF8DE02DAFDB7F4FB250B51BA3A773CE48D", "bnb1afwzv54gff0e62t7wx9hl9eng4qk0ajqfgh62g", 5609.330992, "BNB.RUNE-B1A"},
		{"C4B9586FCC72E1969146E4F4FCB1BF9EB744AAEA70C9B5D01F568ABBA8B6D0E5", "bnb1e37dllsfm9p0h883xusz6hkxle54am6k2kwdkc", 1, "BNB.RUNE-B1A"},
		{"30B5E211A1C6DBC1B7C939490B2A19605192472BB71C3C21CE7EA960C40808DD", "bnb1n8v8grqlq22h70s87g2rc9dhp3mddughpx8c6j", 0.1498125, "BNB.BNB"},
		{"0F8157A253B9231C860DFB4F71EEE0D3553C7A2DFA4D4C9D89AC5F33EE78B65C", "bnb1n8v8grqlq22h70s87g2rc9dhp3mddughpx8c6j", 514.2599, "BNB.RUNE-B1A"},
		{"952976E7AF4F9017CEBC2FC0C37F2F815A85CE5B67D1E485405D89197EA4F760", "bnb14a3sppfhumq64fvfveatfd5gqfg6ggpjr9q9xm", 0.108, "BNB.BNB"},
		{"0779BD5785C0E27EA0FBC39965C21182A57697F5091CC14E50E6FA3FF418E5FC", "bnb1524d9an2cm8zhts5zrs9auty8dnke4cnc2x72s", 14.266, "BNB.RUNE-B1A"},
		{"1A52CD381E1873C96BFF09411E88A8354CB2700A97EF1FF918EA92C02F9313C4", "bnb1rr62n6ut4njflm2dkk3f0p9zy59cuzl9w74ldh", 10.663, "BNB.RUNE-B1A"},
		{"A0C98DE72DE296257E0384AF5A166AFF830C8A020649991CD40E350F48D57F0F", "bnb1zn8c782lncq527zflyghh3jf3anqgf8rwmwtv4", 43300, "BNB.CAS-167"},
		{"138493B9E8240C134C3FBE4FE2C2DB3334990E88145B7BEC7E91B8887011F750", "bnb12d0p6s47l5tr7xs0qedu7v300dc0glc3vcstx7", 3.9490056, "BNB.RUNE-B1A"},
		{"331A747A30AFDC051E9A7251F4C68B9D95DC6BF5942D68E1562E03A17F0F173B", "bnb12d0p6s47l5tr7xs0qedu7v300dc0glc3vcstx7", 6, "BNB.RUNE-B1A"},
		{"1F42A9EF95CBF6D2DD454C071EB31855D586794B214954D38AA04C0282022CE9", "bnb12d0p6s47l5tr7xs0qedu7v300dc0glc3vcstx7", 99.966, "BNB.RUNE-B1A"},
		{"889AFC8FC0E4B4B843261B66F8463EB9FAE9C95C722D569C44ACF6ACA7BF4750", "bnb153w67dcy7znda5c0xz9yzkxwzprqac4thu9swz", 252.252403, "BNB.RUNE-B1A"},
		{"4A4C867E21F8041B920BE0B8C18F3A7981EE4C4B5813936A3831E4FFA0DD25AE", "bnb1chv7uxr79hs7nuj48s2dfh3eeyvw9gnp7qpqdv", 80, "BNB.RUNE-B1A"},
		{"57F4E20E2A0AA6548DB10988B2EA68EBFD4384B7D3EC99F9A17E2B20E666598F", "bnb1en9lzcakhnudhdqpk5t7dqh3atncfu36mj4g7z", 100.03134, "BNB.RUNE-B1A"},
		{"933818BA5A4F27CA3309D53B22CA5AD26277AE11EC135773CD4AF1F3BB4F2FA9", "bnb1agx9jgj54e3f3cyfun2wlv9yty0hxdm89gad7v", 252.6842, "BNB.RUNE-B1A"},
		{"78BFBDB747233E25FAF1D59CA542457560BB81C4D96B5B3E2C2056CD664437A9", "bnb1dht3y9gm200n9h7pjlynl74t3kfdqg55qtux5r", 11.2, "BNB.RUNE-B1A"},
		{"9CA45BFC2E42A24B2779A626574481A5127157DBC81D417AC8F4FD024817976D", "bnb1dht3y9gm200n9h7pjlynl74t3kfdqg55qtux5r", 11.1, "BNB.RUNE-B1A"},
		{"686EDE578BA46FFA9EFEAC9E1667DB95F535E31D14CE0BAFBEA798442672A103", "bnb1jw620r89kk8ueg0wpfmra2lhh3c3473jjuz6x2", 173.21, "BNB.RUNE-B1A"},
		{"8120830E7D38079F73EB20E03DFA17638999F50A4C1B068D2A07657C1E77A553", "bnb1u0u2hj8seq4prgcz7dk87alrlwq5kwpad3s83v", 1120.5272, "BNB.RUNE-B1A"},
		{"18C65B412960B2482F38196390080AD87F9236ECA85A67C59A7ED211590544CE", "bnb1dyysfz2ecs20p33spvmk36fuyhqje9dac04299", 5.957, "BNB.RUNE-B1A"},
		{"6B8FD3FCCE475476B7FEA886BDE950DF107A12A9A27B6F32AA338F52782AF2F2", "bnb1ejn27zjq8zehjpvqs0856845xk00ypu62g5tuy", 400.526055, "BNB.RUNE-B1A"},
		{"476E5C238329AD6EFA346799A6B4DCD4DCC01BD4EB55BEC54A5D2DF84CDBF0C5", "bnb1fuxxzeun6me55fk4phm2nqqfztsd82etvztsn2", 1.00572, "BNB.RUNE-B1A"},
		{"DA0C266AE094F1F75142057633EBEE2A99389351DE9A647780C7E96FC10C2898", "bnb1qseefdrdskdy0lhgd9jns9tt0l280xtftrgvpg", 36.002, "BNB.RUNE-B1A"},
		{"FF0D24AA6E58BA3A53C2BB14D6260734F4D6E38DD7579EFFAEA5F586897309EB", "bnb1eqxsxe0yqnrq68k6quunqfaalqwqyg52yykut8", 100.5736732, "BNB.RUNE-B1A"},
		{"ABD677489F289A18A800AD5071FC4B730069A49653D3CE626ADEE5D7889F7D39", "bnb1746dz6vvxqljh60x0zy65k2degpal7rxk5qlfp", 366.4234204, "BNB.RUNE-B1A"},
		{"B7E2C03A79A159148BFAB454AED45020FB30A63529EA63E501CD4ADA8A1EE4F4", "bnb1z0tnnws7ap37npn274n684luvyzxd72sl4xepg", 10.02191076, "BNB.RUNE-B1A"},
		{"48D5DCCCE9743306DAD1BE66E2A5E87D315F4E4F760225E775E4F40F841C8667", "bnb1cxx3l6j50pkcmse4rxeg7lu5rtyjn0flfl7da9", 15.702, "BNB.RUNE-B1A"},
		{"48242D1F96344ECF05F47B438C70A62ECBFD69E91FA188A54E410E757BF61CFD", "bnb1f8nwzzr9c97z668v4qz90ssmnk37txl5mlhczw", 5, "BNB.RUNE-B1A"},
		{"0CBB1DED208A7C0EDFDD14E2EE950B636C497DF68C8D4AAFD71BB7E635D8DF3D", "bnb1752gtqckrng6uxr6wdmhlxzpwkhffxseg08u22", 24.5, "BNB.RUNE-B1A"},
		{"24B6EDE1841115D2BE287AE2A7756D9C406593A555BB7CD29B6D1EE456003B48", "bnb1eqfyhkumpzxplsdjjudj302nktj320jj4x0fcd", 760.861, "BNB.RUNE-B1A"},
		{"E3D464210A41A874DEEC525FAE791C0AD22326124CFD1B9118F768AD93B98B80", "bnb1dp409mu2kc5frgkytnn7lxrzc5855e9hdmucun", 0.5, "BNB.RUNE-B1A"},
		{"28F3B39ACFD2AD0DDFCFA8C53F193515F7B0E3C9DC9B12974D0588A54DC3DF19", "bnb1vrugu3yq4s5zal29v6sn9pavpfjwzwp3sezjp4", 30001.73045, "BNB.RUNE-B1A"},
		{"805D99B5876E7CCECEF3C70FB8FF71CC8B6E888DACF311D35CE8A75CED87827D", "bnb1pt38g0xddfrkufv554dexuq9a98f843t580cnr", 16.553353, "BNB.RUNE-B1A"},
		{"CDC8888E7142BC08FB5FAB120008C73486A55AC38008B098BBCDF84F72C0273A", "bnb13slj5utg460mdzmnccp23wjq8a64ju9l4u02dd", 59.18399431, "BNB.RUNE-B1A"},
		{"2EF665B8A35FBF224431CFFC2D5E454262F9B30B78F63A7D92E9AB5302B6E902", "bnb1gufh8sp5kaqsetmwg9j6x5rfp6gzxkk6hngzdp", 543.0727504, "BNB.RUNE-B1A"},
		{"34C91917D01158E335736C37A8944A56B8FFD7F222C416F3A4A175369CA98268", "bnb1pqmlr4mewzl72w6r26mmnhqfzgwhk0cu3se6la", 1096.223315, "BNB.RUNE-B1A"},
		{"0169142021C2A82F608872404C02F04513AA61BE4F422D641ADEC70D47384AF9", "bnb129f9dvzkla80v90vgt7e5r84gvea26c9cz5pyd", 2.9, "BNB.RUNE-B1A"},
		{"BF8A81481F56A0C95009B6CB021A52B8F4F19AF6FBE9CC3EA5CE0F6E600C2DA1", "bnb1chnqh4vvf0hqyymkcfcrf4grq88f9ks3yr9v2h", 1, "BNB.BUSD-BD1"},
		{"7AA0CAE1E1B73513AEAD98DDD59C0B64C39C2CD159EBCBD431D203039BF395C8", "bnb1jggavnaxrt669tr6czrfelkpqm0q42y7jkmnh8", 5.183, "BNB.RUNE-B1A"},
		{"4E5EDBED7DD3515065E92842AD8E89121AB05F77F57B275C2AE49E96B0EB7815", "bnb19424lc6n639lspmshjcqa3e4g3xcdfhlmvtyhv", 99.9, "BNB.RUNE-B1A"},
		{"906B60C8BD5ECF2578438C3165130DA90BD240D638E45278C211A9B0DAD39101", "bnb1h7rdk6pe9l2w0mt64m5axjvtylrave4f87g9nr", 8.4433734, "BNB.RUNE-B1A"},
		{"AA12CB4BC2D0DAF928C220E9E9AC614FFA1A5DD6AF958CBD16269E8E891B8E23", "bnb145yyrd783ye42c699qhp0l9ke7vq6n4j7kcgvd", 1441.785562, "BNB.RUNE-B1A"},
		{"1DF73EDF6CF201800E62A7C05249F3FCAB4109C9C9468397F071DC904C8E20C4", "bnb1qrltc54faytdq4p80kraat7p83av6txqt4klmj", 284.4659987, "BNB.RUNE-B1A"},
		{"E6F49096971DAA96791FC1101333B4F0DF887D023F410ECE411CF1DB6FE321C1", "bnb1c8vl0r5ue53awz408ems6r3pvk04t7uenwt48w", 10, "BNB.RUNE-B1A"},
		{"E3A0F6E465B793EA4144FAE42A5985FAEBFBD17780A4CF147C24B2FDC2D9E1F5", "bnb13xrnj2e7hpae4tnksm9yxqznk9m95ujk0pv6gn", 3, "BNB.RUNE-B1A"},
		{"B139AB86B9B67EB4F729C6300EB136E22B6CFF92D1F0CF98284D0E8569E28034", "bnb176heu734unjw5hjw84tqulmha2fpuxaxlynclu", 76.20612, "BNB.RUNE-B1A"},
		{"D2EA3A93323546ACC71632393DF6AC986119752B52FFE422207060FC001BB226", "bnb1a8vapazge7g6z3fgh7f6m384n7p3ak6snln9kh", 0.83965422, "BNB.ETH-1C9"},
		{"032DB75290EDEE1B96EFCEADB3C32F9C0E97E059123782DD6AF98CDBA9F2C7BC", "bnb1yvj0p2j994u7qet9qd2fzgv0smr8098m6e5f0w", 40.665, "BNB.AVA-645"},
		{"B5BA7BE324095F4B079313175D10AD7822885CE3EBF4D81C883B214B2DD01E2B", "bnb164jc0pxcyu8gnkd2u022z3mtst2e5cxfpt76sm", 14, "BNB.RUNE-B1A"},
		{"EA593992C9E54DAD028B3E9F8799C506454B9DFCE69356829219C4C2B6E834F5", "bnb1zn8c782lncq527zflyghh3jf3anqgf8rwmwtv4", 15340, "BNB.CAS-167"},
		{"F536F680C72BFCFD339FC43442F2F23445BA9C2201CB9B1A176BF006E02C8B5D", "bnb1xehlqphf90r7cca8s28kmnzvfkl4vsq2aeg4h7", 61.499, "BNB.RUNE-B1A"},
		{"BC7FC7CD884A2A4E46A8064D98213D348598504C637FA65D109072D1884CC572", "bnb1weh8yy8ywrhuc9nf9h2dymr5h059wlua2a97vy", 0.20983857, "BNB.ETH-1C9"},
		{"2FE1F5B67877B59A6607391146A92BFB7CFABF183BFCC57A7A10FE5A7995792E", "bnb15qx8wpupc9tx69uruszqhvv6hxhyp6w4h0tp6g", 0.0048125, "BNB.BNB"},
		{"571052F9C10FE4ED55BCA05F5EF0D5F8115790156EB82E6C676CD2465ACB6B0F", "bnb1jynhd850akjzzw8g9aefx3km66vshe38yhfhkt", 1032.193937, "BNB.BUSD-BD1"},
		{"A906A663E7357CF0250845117D0665D1F1E2239071C92CA215B2D2907DD00573", "bnb1p9zneqlme5tmatkyzxg3tczer4rxwxhgqnyhmf", 14.249292, "BNB.RUNE-B1A"},
		{"CDA2C4DC7DE5124DE69407784A87B09814042F170C04056EDBE6E18EF0128BF1", "bnb1y86p3erdsuukhdelpf8vq3auzjseuzfen348cl", 0.10047994, "BNB.BNB"},
		{"BA99E1FEA1627924651CCEF154C82D03F4FD62E1A8BEB9DC757478456A534D6E", "bnb1tk05fcy0zsg3atxtp24s70cpjnps0h7vrk9kg2", 319.9599277, "BNB.BUSD-BD1"},
		{"A56A8353F1DE78CC0626BAC40C6E224AB576E8B434C623F7474F7E19AA7042E5", "bnb1wfghslmn9zax6rg2fa2qv2hqahxvgscrna7883", 0.0001, "BNB.ETH-1C9"},
		{"4B3EC98C9BA3B0D7CB7B68862DB32A138930ED157736C5B006D9A1E5B581E3D9", "bnb1ydu26ssyx48x258qawtrrgwhpj28mhqhake57p", 1, "BNB.BNB"},
		{"3D3D1C370CE5BE090DA5EEAAD1D6B51C4DABB1B1FCC26E62CDC1AE48AC17E1F2", "bnb1y3pp87yegy8l38amv4dvsmznl0cpzllvqc3t25", 3440.418072, "BNB.RUNE-B1A"},
		{"9A457583D2BBC0B75BDDECB0CFD2C0DE5BFFB19C2415AF28631735B4A65D5C08", "bnb1u2rd2vkpdr24lw9mddkw7k2v0nna94vnfsuvfs", 35.824113, "BNB.RUNE-B1A"},
		{"1210AB1E86AA55FD634188B7F9BA9AB8342C58868F7A6964E62162FCE4EB66B5", "bnb16zhl9f5gtclneyy6mcczlkxwrwcupqenknryd8", 0.027, "BNB.RUNE-B1A"},
		{"73F5D7C41FFE885CCC345FEA33F42DCF935C8004E9BB3942C67971548389C2DA", "bnb17uy5a2faxvddnled85am3d7m78gw303d9qgp7k", 0.0198875, "BNB.BNB"},
		{"2793CD3D0DA1220AB568F70CD88808F3DC0AE2B91A816D66CC6FC55D636701A3", "bnb1f8ht8rp7dk0uxshx8ckjpxn9zgsphxlklwluxc", 1, "BNB.BNB"},
		{"2BA7E53B2C6D3D3165F2F6087924A1E7B44CDAF383CB81D61281CBEF4C7758C9", "bnb1m9vh6wrtvd900fllze6pqzgk82a0s0vmeacjs5", 0.09020911, "BNB.BNB"},
		{"F0D1DA17AC3AE17B546B39F2D368A1C0EACAD68D3FCE51D3087FDC25945D9618", "bnb1zevt9n9x0n3k398ghkugh3wzwmufdrrhhnc6ct", 36.74990723, "BNB.BUSD-BD1"},
		{"157FDF2EB71403A6E9D535834BC95223C8D9C43639657831315D0A42428B0158", "bnb14e2e2jn7cjxca6ys63w9jtachf2c7xx50jsnw8", 5, "BNB.RUNE-B1A"},
		{"88E52E21C9A5ECF2AF7663B6CC827EE31CD4B7C3F201A38EA3E843A660686165", "bnb14e2e2jn7cjxca6ys63w9jtachf2c7xx50jsnw8", 5, "BNB.RUNE-B1A"},
		{"899B355C8051370F669FD4FAA3F921DE2CE10362CE6C8598248A8FC0687187BE", "bnb159wrhm5gt94fgkzldp8zmfgnlxa365k03zzsfm", 1.0911, "BNB.BNB"},
		{"E3555213FA7B8A3ECE24B77266E4757A7137B007EE5C7CAC7794580846D611CA", "bnb15ktsdv6w5lp973vduxukgpg7zdtg8r2hvws00c", 8.852112, "BNB.RUNE-B1A"},
		{"7C00E88FC775A0D5356767D89FBBC2888F4A07237B17BC4524012532117817E1", "bnb1vf3ljyga82sewznuc9lemvm2t4335wtm2xrxn5", 1143.246025, "BNB.RUNE-B1A"},
		{"B63C5591635CD671B7D442A5FD202A74DDDB0428096AEEAC1D8476E652788D22", "bnb1panzh7z479dfwj7j5dxkfa6hpze3j3gd8wykcj", 4000, "BNB.BUSD-BD1"},
		{"87B7544A4B8521BF2183BAB99AE68F01E4A4AFDEE5172B15B1D4D2C9E377F679", "bnb1amyth8xm9lr3l7h20zaxl5xw0taxz8d5y2zxlv", 218.315439, "BNB.RUNE-B1A"},
		{"C465E3EAF174BECB1E88402017E540FD0001B503ABA4C0606ABBE2EE03200AEB", "bnb18hpy44al65skdkenuh4ns47u8nf24fw2dtk8uq", 0.05113845, "BNB.BNB"},
		{"A2F9102F98BAB1A936E63CA4E7792B840487A6812A80E551E0F7BE5D82DE64E4", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 7606.5, "BNB.BUSD-BD1"},
		{"E5FB928D24028185FCF94777B312A2B1D9EB85241C116BFE43626D3541747ABB", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 4757.85, "BNB.BUSD-BD1"},
		{"70D0C9206E47C7A2FA3C5AD1E2BBC1B3F0A90BC535028FF32A5AA9EE71047734", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 31170.8404, "BNB.BUSD-BD1"},
		{"76367C6A9A580BDDAC2D9C6CCA07564A1CDA741BC4A11B8A34FD965EB3E6913C", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 7734, "BNB.BUSD-BD1"},
		{"BA182566510AB2A7799075AA6078992058ECFFEE599BB4AFC7032CDBEEF64EA7", "bnb1f0kxq029tgac6zhx83q4gppsn857rqvwdzsw76", 1000, "BNB.BUSD-BD1"},
		{"72E92F8BB2FE9B26F4FAF4751D69FBEE76782F200149A805AC00DDC0843935F6", "bnb1m8wuq0kqq5cju704fhp94eth436v6zpvfl0ylw", 500, "BNB.BUSD-BD1"},
		{"68A009E3CCAB8A8725E15A36E308D972A74DB785ED4B8D966C0FFC445CF827E8", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 1019.749, "BNB.BUSD-BD1"},
		{"62F46CEF75ED79C78350E4905AE22361FB80CDC1DEFB047F91E6CA242C84174A", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 4935, "BNB.BUSD-BD1"},
		{"9F4A8A9942AAA81E829D95FD6D6043E352762DFCA6B59A5059D7E49C68885E12", "bnb17f84u3u85av9r94mkc6rfe6jeqhkk3sj67fx3l", 1796.2116, "BNB.RUNE-B1A"},
		{"79AD53B82B4A6939E357DBFB23C9AC3479BD52FA7EF9EF9D5E54C734154D158C", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 4729.65, "BNB.BUSD-BD1"},
		{"5B2354EB21D4ABEEAC7C8CB35CFC2F304147B2B7A1C581DF10D64927A4DA46C8", "bnb1dx3yxj37e7jku4azgehztr2vf2uwkjjlxq8xfh", 370, "BNB.RUNE-B1A"},
		{"5126B3858B6296E810FD061D40F9D5A3D3915D46D7757E2120AADFF4B3EDD349", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 8077.5, "BNB.BUSD-BD1"},
		{"9586D02514B8C7951205A0A2E028A59B90BD678A35236FCADF912F629F3B6E9D", "bnb1m8wuq0kqq5cju704fhp94eth436v6zpvfl0ylw", 35, "BNB.BNB"},
		{"ADF763E3A0075C3E272CF8E0B32BD77F21C528C3996E0FBF085DB9D03F555421", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 4719.15, "BNB.BUSD-BD1"},
		{"C90A60BE87EFB7E340C2446A64A0702933525CCE0F5084E1A2C0323D98D9B84A", "bnb1rgwulut04q6ef85tcvk76ey5yld52slaplwwa9", 54.06594774, "BNB.BUSD-BD1"},
		{"BA9E117D6E40B9ADB257FF6A4B06B467DF82F47FF93D55191E0BDCB0004C5309", "bnb1x4xuxzeyckpctkhp5g425w3wf7y8m3w7z57s0f", 388.373211, "BNB.RUNE-B1A"},
		{"0E6E05E73D7DD5A74FE9F125748B2D24908154B75661FC4248B3B3901DB423C4", "bnb12xufpsysr6r0tankhqdaams9zee0tzxy006qns", 5, "BNB.RUNE-B1A"},
		{"E71642CAC325B733A3581F31F1DA64A52AA5AC590980E5630EF80BE1D906AECF", "bnb137xvx6ceqpxe34xr46y3ax350fr2q4zymw5l94", 2500, "BNB.BUSD-BD1"},
		{"A4F17152DDB1473AE3341311F256171A532558711B70467AB606040E570A6F46", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 30376.57296, "BNB.BUSD-BD1"},
	}
	const asgardPk = `thorpub1addwnpepqdr4386mnkqyqzpqlydtat0k82f8xvkfwzh4xtjc84cuaqmwx5vjvgnf6v5`
	refundTransactions(ctx, mgr, asgardPk, transactionToRefund...)
}

func cancelOutboundTxs(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to schedule refund BNB transactions", "error", err)
		}
	}()
	// Chaosnet experience TSS issues during 2021-10-07~2021-10-10
	// during this time , yggdrasil vault get drained , which cause some outbounds get split
	// The following transactions have a few residual outbound transactions , which are a few cents , the network will not
	// send it out , thus marked all these to be done
	removeTransactions(ctx, mgr,
		"850C14C0F71EA02DD85AB185812C5DF67ADFF73DF894F117F524B0944E07D756",
		"53B6EA81FC37DDE00EFDE6CF86C1DB09E87EB52B01867DDEA0F23FC73C31F5C6",
		"7CEF5A3A1446D00AE7CC786A4B95BD03165D72D8559E83E099D197E609FF4703",
		"81E5441F90A2426A2F22D9DA8D85AEE5BF4BBD0F7AA98A99F4D1D1FDC835D500",
		"5B4E911F853283FFC0DE182AF44E2CC34E6B4197DCEFA3BBBDC57F2B4ED056AF",
		"525993ACCDB9DAB806C674DDC46DA9407AFA3334F4377B3F6E80C39FD09F7048",
		"7783842727E430964EAE00E92E09E73DB62D7B8CCA79EDBBD48A52E9D986D70C",
		"78BDB611B68FDD8F2B36A5D0663C6D0E69EAB49E72B193E80ECDE6508ABBE566",
		"47A7C4D813C194D43299EF8C127C2E4790CBDEC049E2D3314752A72611CF2BF8",
		"CFB7748A64A5854BAF40003312FEC8DA0C02AA724EDDC5FB3812475C958BDAE3")
}

func refundBNBTransactionsV71(ctx cosmos.Context, mgr *Mgrs) {
	// Some transactions were scheduled and processed at v70, however not all of them , the rest fail to reschedule
	transactionToRefund := []adhocRefundTx{
		{"7F6EC99B8F82F36501DCCDDBF54E9BBEAA51A8604CD39E9635027711967B2DF9", "bnb1h05y4pe6heq2g8u6955kchrdst7s8gxspg3u2s", 628, "BNB.RUNE-B1A"},
		{"2EF95165FDAABBF72CA4EE10DBAEF825EE06C280B8B2098AB55A89B922105C57", "bnb1uresxjchzzffvdvjgkf2su7kcupx5uycc58ygs", 34.166, "BNB.RUNE-B1A"},
		{"1D8DE0EC576D7C9A485ECA74401560CBA65870F9E07DF814257B24C0EC8165FC", "bnb1kcay57v22f6gn5kf5vsnstxv8dfnaz2gecckeg", 47.477, "BNB.RUNE-B1A"},
		{"EA6627C95264B150DB729D2BFBD9C52B91300A5118C404F519244C07E9A0A4C1", "bnb1zxtn3xt5prfd7w3st53jjnkeuexkyn7uh8a24c", 126.10821, "BNB.RUNE-B1A"},
		{"6FFFF126170C6B4A144121DD5E5E9A195B9355F083C5494C4C9BF2DA855B9739", "bnb1kcay57v22f6gn5kf5vsnstxv8dfnaz2gecckeg", 251.082, "BNB.RUNE-B1A"},
		{"37E64B6DF40541E9C77107C29BBE3A03BD051EA072ED8CBDC2CC51CF3AD2BF59", "bnb1rvzk2wkn6f8k447s63tf6yuk52tef4m0t79vn0", 50.84107, "BNB.RUNE-B1A"},
		{"B1D6CC11BC34E680362E26CDE8628444222D5AEA30520EF1A38BF3B1BB63C246", "bnb1km5j6s27w5zr9677p3nydv7gq8xemh53g6keeq", 100, "BNB.RUNE-B1A"},
		{"B57C28C425EFA75560C32CA6EBB9DC2BE86EE118837B33840008023808CD3583", "bnb1ljkguffu227zwvz29quzn7aj6a3wdf3nc4rffz", 641.09822, "BNB.RUNE-B1A"},
		{"4030559AA45D78422296581D6C352FA90B51F913DF216E953B0B044F5107A17E", "bnb1zptk7ftjmessmw06q2qg4te2d2cd54973wthye", 2.846, "BNB.RUNE-B1A"},
		{"2C6FA678407506A4CC8F8C8CC1EC07511C6228120B963A2FDDD4A22E67611AA6", "bnb1zptk7ftjmessmw06q2qg4te2d2cd54973wthye", 0.106, "BNB.RUNE-B1A"},
		{"A77AE682E0244D3203A31BF2AAD9DE317742B0D38BED40162E093EA5B3411233", "bnb1n63mqfexeqj5rdc08ww4jyu6ys5k3s2h8n95tj", 0.04983093, "BNB.BNB"},
		{"189BD06D18EE322D5662BF6AD5F18AF44286FD71985EE8408B91726EF59F29A0", "bnb1q3wkg6p2plf88342qtxe943qsnrhacp7f2f364", 498.446, "BNB.RUNE-B1A"},
		{"EAF6B55E6718BA0E0E81F425DB34E4CA6E9C36D3CE16562362DF2FC57FD3E65E", "bnb1mfz43ul3njltlgfhn976l88a2fd0uvcm2xyl6p", 1, "BNB.BUSD-BD1"},
		{"9D3C914F4049101410A86298E471E5C3C113DF45568087B362A2376C7575663B", "bnb1ukfhdgy9c0ws42eqt786q82ept9ht6fag88t6u", 0.01, "BNB.BNB"},
		{"2243A8F3938031C485506955D1F5170EB3D7C6226444AE37F122D29194CB9208", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9875878, "BNB.BUSD-BD1"},
		{"E4546065A5E1F7D0D69951E3990FE9333CCFB34AA14FFAE4A6387F080C15F7E4", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9993278, "BNB.BUSD-BD1"},
		{"E3A23F59F02AF1F2FA312A92565C28DCB468133A164719DF573B7CA6D8791138", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 129.9999985, "BNB.BUSD-BD1"},
		{"61537E93C783E38E1F66A9C830F25D38414CC5A82DEAD6AB353F9F724C7E3C6F", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9875878, "BNB.BUSD-BD1"},
		{"62DC14CB8617786AC9A67E4C0BAD9322883AF4CEFF4568458AC3577D46CD01C2", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 119.9999999, "BNB.BUSD-BD1"},
		{"05FADCD5E58368F242F11A664527B85E439381F851E2E4977C94455502E3B785", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9993278, "BNB.BUSD-BD1"},
		{"F1ADEDDEFC52659FBE5A473A008E1D4D796278B9B08A1B6EFFC16DB504E031F3", "bnb1e0ynwm4qyvpqzdv7hx20trssjx3cx6jpcv9rm8", 0.10178468, "BNB.BNB"},
		{"E72C7D0E789164558B19372D60C4E75314E5BF6BB7A0515F9F63837E09AD2211", "bnb12d0p6s47l5tr7xs0qedu7v300dc0glc3vcstx7", 0.2991625, "BNB.BNB"},
		{"8EA505016F51A3784246E625D782121CF7B1CDFD6D9761CCF257A16C7E469386", "bnb1s9hznv329sa44hd3fqc2tkt34mwn9lsfydvl3l", 29.7763598, "BNB.RUNE-B1A"},
		{"52EE48D2AB26E3019D94468B05D55093FA067CC8DAE161621C8F7C16DA1F7125", "bnb1ps98jc7ahv26ystenqhuh24fmxyv7t9zzc85xh", 17.5, "BNB.RUNE-B1A"},
		{"E2B18B54AD30B0EF66DF430E3F640585EE6006BBC04461AA787D9F8FA3776DD5", "bnb12ahp8vpwhe8yw86n33clsvy5z2vgq8fl0pgcfy", 24.4632, "BNB.RUNE-B1A"},
		{"117B04B214A4F97CF4C911B0150B01A8AF0AF56FA2EF919700A556ACD2C20523", "bnb1vukqle2pzckhd5jpxm8vp3yw8l2wzz9rwg9st4", 59.95595267, "BNB.BUSD-BD1"},
		{"047C6698FB19A0CCD4404AD29CB074B787418B64C0502C8BCE210AF8ADD51213", "bnb19w5yazzc9cmfcjf7fm35u5l7t5npgu673936ht", 156.1715529, "BNB.RUNE-B1A"},
		{"26FB54195FDA0D4200A3510AD7939FF8DE02DAFDB7F4FB250B51BA3A773CE48D", "bnb1afwzv54gff0e62t7wx9hl9eng4qk0ajqfgh62g", 5609.330992, "BNB.RUNE-B1A"},
		{"30B5E211A1C6DBC1B7C939490B2A19605192472BB71C3C21CE7EA960C40808DD", "bnb1n8v8grqlq22h70s87g2rc9dhp3mddughpx8c6j", 0.1498125, "BNB.BNB"},
		{"0F8157A253B9231C860DFB4F71EEE0D3553C7A2DFA4D4C9D89AC5F33EE78B65C", "bnb1n8v8grqlq22h70s87g2rc9dhp3mddughpx8c6j", 514.2599, "BNB.RUNE-B1A"},
		{"952976E7AF4F9017CEBC2FC0C37F2F815A85CE5B67D1E485405D89197EA4F760", "bnb14a3sppfhumq64fvfveatfd5gqfg6ggpjr9q9xm", 0.108, "BNB.BNB"},
		{"0779BD5785C0E27EA0FBC39965C21182A57697F5091CC14E50E6FA3FF418E5FC", "bnb1524d9an2cm8zhts5zrs9auty8dnke4cnc2x72s", 14.266, "BNB.RUNE-B1A"},
		{"138493B9E8240C134C3FBE4FE2C2DB3334990E88145B7BEC7E91B8887011F750", "bnb12d0p6s47l5tr7xs0qedu7v300dc0glc3vcstx7", 3.9490056, "BNB.RUNE-B1A"},
		{"4A4C867E21F8041B920BE0B8C18F3A7981EE4C4B5813936A3831E4FFA0DD25AE", "bnb1chv7uxr79hs7nuj48s2dfh3eeyvw9gnp7qpqdv", 80, "BNB.RUNE-B1A"},
		{"57F4E20E2A0AA6548DB10988B2EA68EBFD4384B7D3EC99F9A17E2B20E666598F", "bnb1en9lzcakhnudhdqpk5t7dqh3atncfu36mj4g7z", 100.03134, "BNB.RUNE-B1A"},
		{"933818BA5A4F27CA3309D53B22CA5AD26277AE11EC135773CD4AF1F3BB4F2FA9", "bnb1agx9jgj54e3f3cyfun2wlv9yty0hxdm89gad7v", 252.6842, "BNB.RUNE-B1A"},
		{"9CA45BFC2E42A24B2779A626574481A5127157DBC81D417AC8F4FD024817976D", "bnb1dht3y9gm200n9h7pjlynl74t3kfdqg55qtux5r", 11.1, "BNB.RUNE-B1A"},
		{"686EDE578BA46FFA9EFEAC9E1667DB95F535E31D14CE0BAFBEA798442672A103", "bnb1jw620r89kk8ueg0wpfmra2lhh3c3473jjuz6x2", 173.21, "BNB.RUNE-B1A"},
		{"18C65B412960B2482F38196390080AD87F9236ECA85A67C59A7ED211590544CE", "bnb1dyysfz2ecs20p33spvmk36fuyhqje9dac04299", 5.957, "BNB.RUNE-B1A"},
		{"6B8FD3FCCE475476B7FEA886BDE950DF107A12A9A27B6F32AA338F52782AF2F2", "bnb1ejn27zjq8zehjpvqs0856845xk00ypu62g5tuy", 400.526055, "BNB.RUNE-B1A"},
		{"DA0C266AE094F1F75142057633EBEE2A99389351DE9A647780C7E96FC10C2898", "bnb1qseefdrdskdy0lhgd9jns9tt0l280xtftrgvpg", 36.002, "BNB.RUNE-B1A"},
		{"FF0D24AA6E58BA3A53C2BB14D6260734F4D6E38DD7579EFFAEA5F586897309EB", "bnb1eqxsxe0yqnrq68k6quunqfaalqwqyg52yykut8", 100.5736732, "BNB.RUNE-B1A"},
		{"48D5DCCCE9743306DAD1BE66E2A5E87D315F4E4F760225E775E4F40F841C8667", "bnb1cxx3l6j50pkcmse4rxeg7lu5rtyjn0flfl7da9", 15.702, "BNB.RUNE-B1A"},
		{"24B6EDE1841115D2BE287AE2A7756D9C406593A555BB7CD29B6D1EE456003B48", "bnb1eqfyhkumpzxplsdjjudj302nktj320jj4x0fcd", 760.861, "BNB.RUNE-B1A"},
		{"E3D464210A41A874DEEC525FAE791C0AD22326124CFD1B9118F768AD93B98B80", "bnb1dp409mu2kc5frgkytnn7lxrzc5855e9hdmucun", 0.5, "BNB.RUNE-B1A"},
		{"28F3B39ACFD2AD0DDFCFA8C53F193515F7B0E3C9DC9B12974D0588A54DC3DF19", "bnb1vrugu3yq4s5zal29v6sn9pavpfjwzwp3sezjp4", 30001.73045, "BNB.RUNE-B1A"},
		{"805D99B5876E7CCECEF3C70FB8FF71CC8B6E888DACF311D35CE8A75CED87827D", "bnb1pt38g0xddfrkufv554dexuq9a98f843t580cnr", 16.553353, "BNB.RUNE-B1A"},
		{"2EF665B8A35FBF224431CFFC2D5E454262F9B30B78F63A7D92E9AB5302B6E902", "bnb1gufh8sp5kaqsetmwg9j6x5rfp6gzxkk6hngzdp", 543.0727504, "BNB.RUNE-B1A"},
		{"34C91917D01158E335736C37A8944A56B8FFD7F222C416F3A4A175369CA98268", "bnb1pqmlr4mewzl72w6r26mmnhqfzgwhk0cu3se6la", 1096.223315, "BNB.RUNE-B1A"},
		{"0169142021C2A82F608872404C02F04513AA61BE4F422D641ADEC70D47384AF9", "bnb129f9dvzkla80v90vgt7e5r84gvea26c9cz5pyd", 2.9, "BNB.RUNE-B1A"},
		{"BF8A81481F56A0C95009B6CB021A52B8F4F19AF6FBE9CC3EA5CE0F6E600C2DA1", "bnb1chnqh4vvf0hqyymkcfcrf4grq88f9ks3yr9v2h", 1, "BNB.BUSD-BD1"},
		{"4E5EDBED7DD3515065E92842AD8E89121AB05F77F57B275C2AE49E96B0EB7815", "bnb19424lc6n639lspmshjcqa3e4g3xcdfhlmvtyhv", 99.9, "BNB.RUNE-B1A"},
		{"906B60C8BD5ECF2578438C3165130DA90BD240D638E45278C211A9B0DAD39101", "bnb1h7rdk6pe9l2w0mt64m5axjvtylrave4f87g9nr", 8.4433734, "BNB.RUNE-B1A"},
		{"AA12CB4BC2D0DAF928C220E9E9AC614FFA1A5DD6AF958CBD16269E8E891B8E23", "bnb145yyrd783ye42c699qhp0l9ke7vq6n4j7kcgvd", 1441.785562, "BNB.RUNE-B1A"},
		{"E6F49096971DAA96791FC1101333B4F0DF887D023F410ECE411CF1DB6FE321C1", "bnb1c8vl0r5ue53awz408ems6r3pvk04t7uenwt48w", 10, "BNB.RUNE-B1A"},
		{"E3A0F6E465B793EA4144FAE42A5985FAEBFBD17780A4CF147C24B2FDC2D9E1F5", "bnb13xrnj2e7hpae4tnksm9yxqznk9m95ujk0pv6gn", 3, "BNB.RUNE-B1A"},
		{"B139AB86B9B67EB4F729C6300EB136E22B6CFF92D1F0CF98284D0E8569E28034", "bnb176heu734unjw5hjw84tqulmha2fpuxaxlynclu", 76.20612, "BNB.RUNE-B1A"},
		{"D2EA3A93323546ACC71632393DF6AC986119752B52FFE422207060FC001BB226", "bnb1a8vapazge7g6z3fgh7f6m384n7p3ak6snln9kh", 0.83965422, "BNB.ETH-1C9"},
		{"032DB75290EDEE1B96EFCEADB3C32F9C0E97E059123782DD6AF98CDBA9F2C7BC", "bnb1yvj0p2j994u7qet9qd2fzgv0smr8098m6e5f0w", 40.665, "BNB.AVA-645"},
		{"B5BA7BE324095F4B079313175D10AD7822885CE3EBF4D81C883B214B2DD01E2B", "bnb164jc0pxcyu8gnkd2u022z3mtst2e5cxfpt76sm", 14, "BNB.RUNE-B1A"},
		{"F536F680C72BFCFD339FC43442F2F23445BA9C2201CB9B1A176BF006E02C8B5D", "bnb1xehlqphf90r7cca8s28kmnzvfkl4vsq2aeg4h7", 61.499, "BNB.RUNE-B1A"},
		{"BC7FC7CD884A2A4E46A8064D98213D348598504C637FA65D109072D1884CC572", "bnb1weh8yy8ywrhuc9nf9h2dymr5h059wlua2a97vy", 0.20983857, "BNB.ETH-1C9"},
		{"A906A663E7357CF0250845117D0665D1F1E2239071C92CA215B2D2907DD00573", "bnb1p9zneqlme5tmatkyzxg3tczer4rxwxhgqnyhmf", 14.249292, "BNB.RUNE-B1A"},
		{"CDA2C4DC7DE5124DE69407784A87B09814042F170C04056EDBE6E18EF0128BF1", "bnb1y86p3erdsuukhdelpf8vq3auzjseuzfen348cl", 0.10047994, "BNB.BNB"},
		{"BA99E1FEA1627924651CCEF154C82D03F4FD62E1A8BEB9DC757478456A534D6E", "bnb1tk05fcy0zsg3atxtp24s70cpjnps0h7vrk9kg2", 319.9599277, "BNB.BUSD-BD1"},
		{"4B3EC98C9BA3B0D7CB7B68862DB32A138930ED157736C5B006D9A1E5B581E3D9", "bnb1ydu26ssyx48x258qawtrrgwhpj28mhqhake57p", 1, "BNB.BNB"},
		{"9A457583D2BBC0B75BDDECB0CFD2C0DE5BFFB19C2415AF28631735B4A65D5C08", "bnb1u2rd2vkpdr24lw9mddkw7k2v0nna94vnfsuvfs", 35.824113, "BNB.RUNE-B1A"},
		{"2793CD3D0DA1220AB568F70CD88808F3DC0AE2B91A816D66CC6FC55D636701A3", "bnb1f8ht8rp7dk0uxshx8ckjpxn9zgsphxlklwluxc", 1, "BNB.BNB"},
		{"88E52E21C9A5ECF2AF7663B6CC827EE31CD4B7C3F201A38EA3E843A660686165", "bnb14e2e2jn7cjxca6ys63w9jtachf2c7xx50jsnw8", 5, "BNB.RUNE-B1A"},
		{"899B355C8051370F669FD4FAA3F921DE2CE10362CE6C8598248A8FC0687187BE", "bnb159wrhm5gt94fgkzldp8zmfgnlxa365k03zzsfm", 1.0911, "BNB.BNB"},
		{"E3555213FA7B8A3ECE24B77266E4757A7137B007EE5C7CAC7794580846D611CA", "bnb15ktsdv6w5lp973vduxukgpg7zdtg8r2hvws00c", 8.852112, "BNB.RUNE-B1A"},
		{"7C00E88FC775A0D5356767D89FBBC2888F4A07237B17BC4524012532117817E1", "bnb1vf3ljyga82sewznuc9lemvm2t4335wtm2xrxn5", 1143.246025, "BNB.RUNE-B1A"},
		{"B63C5591635CD671B7D442A5FD202A74DDDB0428096AEEAC1D8476E652788D22", "bnb1panzh7z479dfwj7j5dxkfa6hpze3j3gd8wykcj", 4000, "BNB.BUSD-BD1"},
		{"87B7544A4B8521BF2183BAB99AE68F01E4A4AFDEE5172B15B1D4D2C9E377F679", "bnb1amyth8xm9lr3l7h20zaxl5xw0taxz8d5y2zxlv", 218.315439, "BNB.RUNE-B1A"},
		{"C465E3EAF174BECB1E88402017E540FD0001B503ABA4C0606ABBE2EE03200AEB", "bnb18hpy44al65skdkenuh4ns47u8nf24fw2dtk8uq", 0.05113845, "BNB.BNB"},
		{"A2F9102F98BAB1A936E63CA4E7792B840487A6812A80E551E0F7BE5D82DE64E4", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 7606.5, "BNB.BUSD-BD1"},
		{"E5FB928D24028185FCF94777B312A2B1D9EB85241C116BFE43626D3541747ABB", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 4757.85, "BNB.BUSD-BD1"},
		{"70D0C9206E47C7A2FA3C5AD1E2BBC1B3F0A90BC535028FF32A5AA9EE71047734", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 31170.8404, "BNB.BUSD-BD1"},
		{"BA182566510AB2A7799075AA6078992058ECFFEE599BB4AFC7032CDBEEF64EA7", "bnb1f0kxq029tgac6zhx83q4gppsn857rqvwdzsw76", 1000, "BNB.BUSD-BD1"},
		{"72E92F8BB2FE9B26F4FAF4751D69FBEE76782F200149A805AC00DDC0843935F6", "bnb1m8wuq0kqq5cju704fhp94eth436v6zpvfl0ylw", 500, "BNB.BUSD-BD1"},
		{"68A009E3CCAB8A8725E15A36E308D972A74DB785ED4B8D966C0FFC445CF827E8", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 1019.749, "BNB.BUSD-BD1"},
		{"62F46CEF75ED79C78350E4905AE22361FB80CDC1DEFB047F91E6CA242C84174A", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 4935, "BNB.BUSD-BD1"},
		{"5126B3858B6296E810FD061D40F9D5A3D3915D46D7757E2120AADFF4B3EDD349", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 8077.5, "BNB.BUSD-BD1"},
		{"ADF763E3A0075C3E272CF8E0B32BD77F21C528C3996E0FBF085DB9D03F555421", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 4719.15, "BNB.BUSD-BD1"},
		{"C90A60BE87EFB7E340C2446A64A0702933525CCE0F5084E1A2C0323D98D9B84A", "bnb1rgwulut04q6ef85tcvk76ey5yld52slaplwwa9", 54.06594774, "BNB.BUSD-BD1"},
		{"BA9E117D6E40B9ADB257FF6A4B06B467DF82F47FF93D55191E0BDCB0004C5309", "bnb1x4xuxzeyckpctkhp5g425w3wf7y8m3w7z57s0f", 388.373211, "BNB.RUNE-B1A"},
		{"0E6E05E73D7DD5A74FE9F125748B2D24908154B75661FC4248B3B3901DB423C4", "bnb12xufpsysr6r0tankhqdaams9zee0tzxy006qns", 5, "BNB.RUNE-B1A"},
		{"E71642CAC325B733A3581F31F1DA64A52AA5AC590980E5630EF80BE1D906AECF", "bnb137xvx6ceqpxe34xr46y3ax350fr2q4zymw5l94", 2500, "BNB.BUSD-BD1"},
		{"A4F17152DDB1473AE3341311F256171A532558711B70467AB606040E570A6F46", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 30376.57296, "BNB.BUSD-BD1"},
	}
	const asgardPk = `thorpub1addwnpepqdr4386mnkqyqzpqlydtat0k82f8xvkfwzh4xtjc84cuaqmwx5vjvgnf6v5`
	refundTransactionsV71(ctx, mgr, asgardPk, transactionToRefund...)
}

func correctBurnedBEP2Rune(ctx cosmos.Context, mgr Manager) {
	network, err := mgr.Keeper().GetNetwork(ctx)
	if err != nil {
		ctx.Logger().Error("failed to get network", "error", err)
		return
	}

	network.BurnedBep2Rune = common.SafeSub(network.BurnedBep2Rune, cosmos.NewUint(19450465409520))
	if err := mgr.Keeper().SetNetwork(ctx, network); err != nil {
		ctx.Logger().Error("failed to set network", "error", err)
	}
}

func migrateStoreV75(ctx cosmos.Context, mgr Manager) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v75", "error", err)
		}
	}()
	inputs := []struct {
		asset             string
		address           string
		depositAssetValue cosmos.Uint
		depositRuneValue  cosmos.Uint
	}{
		{
			asset:             "ETH.THOR-0XA5F2211B9B8170F694421F2046281775E8468044",
			address:           "thor1kgdmqje08dk8f8x3v880jnhggejmf8f03tjd6x",
			depositAssetValue: cosmos.NewUint(440896413157093),
			depositRuneValue:  cosmos.NewUint(81219749756642),
		},
		{
			asset:             "ETH.XRUNE-0X69FA0FEE221AD11012BAB0FDB45D444D3D2CE71C",
			address:           "thor1e38n9c9l3h56gwfwazu6xds32u9ajdapukv65w",
			depositAssetValue: cosmos.NewUint(1123947590415172),
			depositRuneValue:  cosmos.NewUint(36941596233762),
		},
	}
	for _, item := range inputs {
		asset, err := common.NewAsset(item.asset)
		if err != nil {
			ctx.Logger().Error("fail to parse THOR asset", "error", err)
			continue
		}
		addr, err := common.NewAddress(item.address)
		if err != nil {
			ctx.Logger().Error("fail to parse address", "error", err)
			continue
		}
		lp, err := mgr.Keeper().GetLiquidityProvider(ctx, asset, addr)
		if err != nil {
			ctx.Logger().Error("fail to get liquidity provider", "error", err)
			continue
		}
		lp.AssetDepositValue = item.depositAssetValue
		lp.RuneDepositValue = item.depositRuneValue
		mgr.Keeper().SetLiquidityProvider(ctx, lp)
	}
	// the following transaction is swap to a binance exchange address, which has memo flag set,
	// thus the network won't be able to send tx to the address , cancel it
	removeTransactions(ctx, mgr,
		"AC0826366836EE2F6337BBC0B16B25A6B2269A490E6E7B92C4E9328663F6B3CE",
		"8C79D2F64F46AAA2658E08D78F7AAFAA5E4366A04EC167BEC2C527D5D1E434BC",
		"89D9EC3BD2E2E7372D793D58BBBE2E063367BFB663E5173EF50B5371DEF0B182",
		"0F2EE2CFA8AB695C7A789C93C3D91596A29EE7793CFF8FEFDF8784949A6F05E0",
		"55FE00CE035B89258C532D87BBAAFCA69155057897105A0C7346F8C347EEB731",
		"0BF8054DE2271FEC9C983AE73D1F7103B645723856C8C4715E150B92A073E0D2",
		"EC24A6F6597683F402298C5EA9AFD6CFAF65E1860F8B0DFE911CC17C9525B376",
	)
}

func migrateStoreV75CorrectVaultAndRefund(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to correct asgard vault and refund BNB transactions", "error", err)
		}
	}()
	const asgardVaultPubKey = `thorpub1addwnpepqwk8cx4x6jjlsrq6305zs68xtpfcyk3l2k7gp37yqcz0q4g8ar2sz9ms9ur`
	pubKey, err := common.NewPubKey(asgardVaultPubKey)
	if err != nil {
		ctx.Logger().Error("fail to parse asgard pub key", "error", err)
		return
	}
	vault, err := mgr.Keeper().GetVault(ctx, pubKey)
	if err != nil {
		ctx.Logger().Error("fail to get vault", "error", err)
		return
	}
	vault.AddFunds(common.Coins{
		common.NewCoin(common.RuneB1AAsset, cosmos.NewUint(116518754214)),
	})
	if err := mgr.Keeper().SetVault(ctx, vault); err != nil {
		ctx.Logger().Error("fail to save vault", "error", err)
		return
	}
	transactionToRefund := []adhocRefundTx{
		{"8633E86EDE33C7F7D15EFB52E5AC5ED456C5454836D6EBE30179DEAF80E27A16", "bnb1frp33cj2qxsj82d3z8lny7g0epx9dyvuc03zus", 151.748, "BNB.RUNE-B1A"},
		{"4C0D3D448FC7949DA01A285D66D091BDC3446DB83EEB2B05D3A39EED5143DFF7", "bnb1mesfny6ulxnxeqaca6m7wm3j52nud0xq5cgvfv", 0.79816, "BNB.RUNE-B1A"},
		{"A23E773F0AC426F79BC941AFE0D1C10C948413D279E93F4FA838F2A0F1A3B613", "bnb1x59vkrsd3zzy0t4wlx7cmsmka37mk8xfrdru5d", 20, "BNB.RUNE-B1A"},
		{"CA5E861D5B53A477BD68C4E88B45EB8492A34301DC3D2807829ED795056CA589", "bnb1x59vkrsd3zzy0t4wlx7cmsmka37mk8xfrdru5d", 10, "BNB.RUNE-B1A"},
		{"78CA3A5E1F17AB912CCB53B47E7B03276708BFCCB77ED52BEE76D2DB533A40B1", "bnb1xmwvx6un9z6h7fl32gl0g27gu2ypeja5h84pmr", 89.974, "BNB.RUNE-B1A"},
		{"7B8FB0CAA6E47B19D89739F94F9FED7C4E3AF6D6768775C46AFE4436B2CA69C8", "bnb1ateuyzw78yl4umyrs8swqgckwd724jzzqhzwky", 430, "BNB.RUNE-B1A"},
		{"B490833A9A89646B3B2D8956BE14E035BC0B3718A766EECF71A5D004B7536F9A", "bnb1n764f3plr9lww0k3w5lrrs5fpmkfavp0e3jucr", 240.5, "BNB.RUNE-B1A"},
		{"A29E132084D3736B1EAF5807F1660852714C454931B2BCDCF596532096A1380E", "bnb1g4zxac7sl46652pghmk3ahxnr3z27eehsnnchz", 4.304, "BNB.RUNE-B1A"},
		{"7900BF172A5491666823999D0FB85BE1D044BE97FAC43909D89F1F8110E7222B", "bnb1hunrxwy4xgplrtgwmc5ww8cr8e45acxla45j8f", 2.15333449, "BNB.RUNE-B1A"},
		{"75173D6BDAAF462B7E5FE946396117D13C40ADF197ACA6647867E9F9FF6D3ED2", "bnb1hunrxwy4xgplrtgwmc5ww8cr8e45acxla45j8f", 2.17508535, "BNB.RUNE-B1A"},
		{"F2B6E64678C5B2C7A8E6F2CC5885883D2226F7F4E778B8448603C8E1B14BA417", "bnb1e8e4jczs04pp8253f0pw22468qfahlz2h0027g", 0.049086, "BNB.BNB"},
		{"E89A18699C41E7F9688620AE8E15F1F8CF04EFE7A6DF569DC9C2CDF299D6640D", "bnb1kjy7h48us2we2lpgg4er5lc0gg9cpxdaxa7cv9", 82.58787547, "BNB.RUNE-B1A"},
		{"69F45243A08E0EE475ACC82AED432748D971C7802065A345488293743CE749C9", "bnb19l00xc9r9syuhdxtwl2fc5la3ezlsdcavsw7dn", 13.028, "BNB.RUNE-B1A"},
		{"A9126599A8619BA03F5437F72AFA2E1A9ABBD1BC78AEF1BC93942FC981837C66", "bnb1kz03p8zttlfanz4r7rxle5xtdh0lmh0pzyev6h", 102.9710868, "BNB.RUNE-B1A"},
		{"D9033D729087019BA9AB6308B8AB854C517CE949B3A3AB35D33608CBC1724078", "bnb1e9mpcq0p66xvjaglj7mhy76506y2s7wset5zl2", 9.974, "BNB.RUNE-B1A"},
		{"8F5E8B4D66DFDD9E852F06B1FA8BF760019EAC52A06D8FB71C59CE9569EC7E53", "bnb1q4fml0g6wdsuuy9yajpq76rpdwuycxgkan3jdm", 4.974, "BNB.RUNE-B1A"},
	}
	refundTransactionsV71(ctx, mgr, asgardVaultPubKey, transactionToRefund...)
}

func migrateStoreV75UnMarkValidators(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to unmark validators", "error", err)
		}
	}()
	nodeAccounts, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		ctx.Logger().Error("fail to list active validators", "error", err)
		return
	}
	for _, na := range nodeAccounts {
		if na.RequestedToLeave {
			continue
		}
		if na.ForcedToLeave {
			continue
		}
		na.LeaveScore = 0
		if err := mgr.Keeper().SetNodeAccount(ctx, na); err != nil {
			ctx.Logger().Error("fail to save node account", "error", err)
			continue
		}
	}
}

func migrateStoreV77(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v77", "error", err)
		}
	}()
	const asgardVaultPk = `thorpub1addwnpepq0myn4whrj7qfrzc647dju7rgtjc5punucxwvfut56mghuzxakq37e8ev4y`
	pk, err := common.NewPubKey(asgardVaultPk)
	if err != nil {
		ctx.Logger().Error("fail to parse asgard vault public key", "error", err)
		return
	}
	asgard, err := mgr.Keeper().GetVault(ctx, pk)
	if err != nil {
		ctx.Logger().Error("fail to get asgard vault", "error", err)
		return
	}
	donations := []struct {
		asset  string
		amount cosmos.Uint
	}{
		{asset: "ETH.YFI-0X0BC529C00C6401AEF6D220BE8C6EA1667F6AD93E", amount: cosmos.NewUint(10015767)},
		{asset: "ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7", amount: cosmos.NewUint(555652034700)},
		{asset: "ETH.XRUNE-0X69FA0FEE221AD11012BAB0FDB45D444D3D2CE71C", amount: cosmos.NewUint(43906122817485)},
		{asset: "ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48", amount: cosmos.NewUint(2456761120600)},
		{asset: "ETH.ALCX-0XDBDB4D16EDA451D0503B854CF79D55697F90C8DF", amount: cosmos.NewUint(3290908800)},
		{asset: "ETH.CREAM-0X2BA592F78DB6436527729929AAF6C908497CB200", amount: cosmos.NewUint(1238645952)},
		{asset: "ETH.SNX-0XC011A73EE8576FB46F5E1C5751CA3B9FE0AF2A6F", amount: cosmos.NewUint(21225979435)},
		{asset: "ETH.DODO-0X43DFC4159D86F3A37A5A4B3D4580B888AD7D4DDD", amount: cosmos.NewUint(201067634531)},
		{asset: "ETH.PERP-0XBC396689893D065F41BC2C6ECBEE5E0085233447", amount: cosmos.NewUint(43419172120)},
		{asset: "ETH.SUSHI-0X6B3595068778DD592E39A122F4F5A5CF09C90FE2", amount: cosmos.NewUint(74548758317)},
		{asset: "ETH.RAZE-0X5EAA69B29F99C84FE5DE8200340B4E9B4AB38EAC", amount: cosmos.NewUint(722328715369)},
		{asset: "ETH.KYL-0X67B6D479C7BB412C54E03DCA8E1BC6740CE6B99C", amount: cosmos.NewUint(4450083484193)},
		{asset: "ETH.ALPHA-0XA1FAA113CBE53436DF28FF0AEE54275C13B40975", amount: cosmos.NewUint(318113978753)},
		{asset: "ETH.HOT-0X6C6EE5E31D828DE241282B9606C8E98EA48526E2", amount: cosmos.NewUint(60699215576820)},
		{asset: "ETH.HEGIC-0X584BC13C7D411C00C01A62E8019472DE68768430", amount: cosmos.NewUint(1694523495579)},
		{asset: "ETH.AAVE-0X7FC66500C84A76AD7E9C93437BFC5AC33E2DDAE9", amount: cosmos.NewUint(1073207831)},
	}
	for _, item := range donations {
		asset, err := common.NewAsset(item.asset)
		if err != nil {
			ctx.Logger().Error("fail to parse asset", "error", err)
			continue
		}
		p, err := mgr.Keeper().GetPool(ctx, asset)
		if err != nil {
			ctx.Logger().Error("fail to get pool", "error", err, "asset", asset)
			continue
		}
		if p.IsEmpty() {
			continue
		}
		p.BalanceAsset = p.BalanceAsset.Add(item.amount)
		if err := mgr.Keeper().SetPool(ctx, p); err != nil {
			ctx.Logger().Error("fail to save pool", "error", err, "asset", asset)
			continue
		}
		// add pool balance change event , so as midgard will be able to handle the change correctly
		emitPoolBalanceChangedEvent(ctx, NewPoolMod(asset, cosmos.ZeroUint(), false, item.amount, true), "donation", mgr)
		asgard.AddFunds(common.NewCoins(common.NewCoin(asset, item.amount)))
	}
	if err := mgr.Keeper().SetVault(ctx, asgard); err != nil {
		ctx.Logger().Error("fail to save asgard vault", "error", err)
	}
	removeTransactions(ctx, mgr,
		"613B05512D468D3EE5A6C60BA45F085698823AABA85B2BA0F35736E7C674DED6",
		"7C11E57236DDFC87C18665A3D929027B60ECFF69A95C15C9063412386D0FF93E")
}

func migrateStoreV79(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v79", "error", err)
		}
	}()
	removeTransactions(ctx, mgr,
		"16401243F851F97BB5C4D20C0319614DCC890137821417C0E948B55D7E4BD078",
		"7F79C8A600B8064B47C33CDF3388571CE6AF08F1FAC4F23AE62C9EEF0322CBB6",
		"33D521205BB3915C5D1D8D39EEAAC0E1B1CDE31B0A5318A1796D8D86D55DD67A",
		"F3901C5206F79BDA26E94EB84384D9CEB91DDBF473368E51AF88A356DB9AB407")
}
