package thorchain

import (
	"errors"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
	types2 "gitlab.com/thorchain/thornode/x/thorchain/types"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type SlashingV88Suite struct{}

var _ = Suite(&SlashingV88Suite{})

func (s *SlashingV88Suite) SetUpSuite(_ *C) {
	SetupConfigForTest()
}

type TestSlashingLackKeeper struct {
	keeper.KVStoreDummy
	txOut                      *TxOut
	na                         NodeAccount
	vaults                     Vaults
	voter                      ObservedTxVoter
	failGetTxOut               bool
	failGetVault               bool
	failGetNodeAccountByPubKey bool
	failSetNodeAccount         bool
	failGetAsgardByStatus      bool
	failGetObservedTxVoter     bool
	failSetTxOut               bool
	slashPts                   map[string]int64
}

func (k *TestSlashingLackKeeper) PoolExist(ctx cosmos.Context, asset common.Asset) bool {
	return true
}

func (k *TestSlashingLackKeeper) GetObservedTxInVoter(_ cosmos.Context, _ common.TxID) (ObservedTxVoter, error) {
	if k.failGetObservedTxVoter {
		return ObservedTxVoter{}, errKaboom
	}
	return k.voter, nil
}

func (k *TestSlashingLackKeeper) SetObservedTxInVoter(_ cosmos.Context, voter ObservedTxVoter) {
	k.voter = voter
}

func (k *TestSlashingLackKeeper) GetVault(_ cosmos.Context, pk common.PubKey) (Vault, error) {
	if k.failGetVault {
		return Vault{}, errKaboom
	}
	return k.vaults[0], nil
}

func (k *TestSlashingLackKeeper) GetAsgardVaultsByStatus(_ cosmos.Context, _ VaultStatus) (Vaults, error) {
	if k.failGetAsgardByStatus {
		return nil, errKaboom
	}
	return k.vaults, nil
}

func (k *TestSlashingLackKeeper) GetTxOut(_ cosmos.Context, _ int64) (*TxOut, error) {
	if k.failGetTxOut {
		return nil, errKaboom
	}
	return k.txOut, nil
}

func (k *TestSlashingLackKeeper) SetTxOut(_ cosmos.Context, tx *TxOut) error {
	if k.failSetTxOut {
		return errKaboom
	}
	k.txOut = tx
	return nil
}

func (k *TestSlashingLackKeeper) IncNodeAccountSlashPoints(_ cosmos.Context, addr cosmos.AccAddress, pts int64) error {
	if _, ok := k.slashPts[addr.String()]; !ok {
		k.slashPts[addr.String()] = 0
	}
	k.slashPts[addr.String()] += pts
	return nil
}

func (k *TestSlashingLackKeeper) GetNodeAccountByPubKey(_ cosmos.Context, _ common.PubKey) (NodeAccount, error) {
	if k.failGetNodeAccountByPubKey {
		return NodeAccount{}, errKaboom
	}
	return k.na, nil
}

func (k *TestSlashingLackKeeper) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	if k.failSetNodeAccount {
		return errKaboom
	}
	k.na = na
	return nil
}

type TestSlashObservingKeeper struct {
	keeper.KVStoreDummy
	addrs                     []cosmos.AccAddress
	nas                       NodeAccounts
	failGetObservingAddress   bool
	failListActiveNodeAccount bool
	failSetNodeAccount        bool
	slashPts                  map[string]int64
}

func (k *TestSlashObservingKeeper) GetObservingAddresses(_ cosmos.Context) ([]cosmos.AccAddress, error) {
	if k.failGetObservingAddress {
		return nil, errKaboom
	}
	return k.addrs, nil
}

func (k *TestSlashObservingKeeper) ClearObservingAddresses(_ cosmos.Context) {
	k.addrs = nil
}

func (k *TestSlashObservingKeeper) IncNodeAccountSlashPoints(_ cosmos.Context, addr cosmos.AccAddress, pts int64) error {
	if _, ok := k.slashPts[addr.String()]; !ok {
		k.slashPts[addr.String()] = 0
	}
	k.slashPts[addr.String()] += pts
	return nil
}

func (k *TestSlashObservingKeeper) ListActiveValidators(_ cosmos.Context) (NodeAccounts, error) {
	if k.failListActiveNodeAccount {
		return nil, errKaboom
	}
	return k.nas, nil
}

func (k *TestSlashObservingKeeper) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	if k.failSetNodeAccount {
		return errKaboom
	}
	for i := range k.nas {
		if k.nas[i].NodeAddress.Equals(na.NodeAddress) {
			k.nas[i] = na
			return nil
		}
	}
	return errors.New("node account not found")
}

type TestDoubleSlashKeeper struct {
	keeper.KVStoreDummy
	na          NodeAccount
	network     Network
	slashPoints map[string]int64
	modules     map[string]int64
}

func (k *TestDoubleSlashKeeper) SendFromModuleToModule(_ cosmos.Context, from, to string, coins common.Coins) error {
	k.modules[from] -= int64(coins[0].Amount.Uint64())
	k.modules[to] += int64(coins[0].Amount.Uint64())
	return nil
}

func (k *TestDoubleSlashKeeper) ListActiveValidators(ctx cosmos.Context) (NodeAccounts, error) {
	return NodeAccounts{k.na}, nil
}

func (k *TestDoubleSlashKeeper) SetNodeAccount(ctx cosmos.Context, na NodeAccount) error {
	k.na = na
	return nil
}

func (k *TestDoubleSlashKeeper) GetNetwork(ctx cosmos.Context) (Network, error) {
	return k.network, nil
}

func (k *TestDoubleSlashKeeper) SetNetwork(ctx cosmos.Context, data Network) error {
	k.network = data
	return nil
}

func (k *TestDoubleSlashKeeper) IncNodeAccountSlashPoints(ctx cosmos.Context, addr cosmos.AccAddress, pts int64) error {
	k.slashPoints[addr.String()] += pts
	return nil
}

func (k *TestDoubleSlashKeeper) DecNodeAccountSlashPoints(ctx cosmos.Context, addr cosmos.AccAddress, pts int64) error {
	k.slashPoints[addr.String()] -= pts
	return nil
}

func (s *SlashingV88Suite) TestObservingSlashing(c *C) {
	var err error
	ctx, k := setupKeeperForTest(c)
	naActiveAfterTx := GetRandomValidatorNode(NodeActive)
	naActiveAfterTx.ActiveBlockHeight = 1030
	nas := NodeAccounts{
		GetRandomValidatorNode(NodeActive),
		GetRandomValidatorNode(NodeActive),
		GetRandomValidatorNode(NodeStandby),
		naActiveAfterTx,
	}
	for _, item := range nas {
		c.Assert(k.SetNodeAccount(ctx, item), IsNil)
	}
	height := int64(1024)
	txOut := NewTxOut(height)
	txHash := GetRandomTxHash()
	observedTx := GetRandomObservedTx()
	txVoter := NewObservedTxVoter(txHash, []ObservedTx{
		observedTx,
	})
	txVoter.FinalisedHeight = 1024
	txVoter.Add(observedTx, nas[0].NodeAddress)
	txVoter.Tx = txVoter.Txs[0]
	k.SetObservedTxInVoter(ctx, txVoter)

	txOut.TxArray = append(txOut.TxArray, TxOutItem{
		Chain:       common.BNBChain,
		InHash:      txHash,
		ToAddress:   GetRandomBNBAddress(),
		VaultPubKey: GetRandomPubKey(),
		Coin:        common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
		Memo:        "whatever",
	})

	c.Assert(k.SetTxOut(ctx, txOut), IsNil)

	ctx = ctx.WithBlockHeight(height + 300)
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)

	slasher := newSlasherV88(k, NewDummyEventMgr())
	// should slash na2 only
	lackOfObservationPenalty := constAccessor.GetInt64Value(constants.LackOfObservationPenalty)
	err = slasher.LackObserving(ctx, constAccessor)
	c.Assert(err, IsNil)
	slashPoint, err := k.GetNodeAccountSlashPoints(ctx, nas[0].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, int64(0))

	slashPoint, err = k.GetNodeAccountSlashPoints(ctx, nas[1].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, lackOfObservationPenalty)

	// standby node should not be slashed
	slashPoint, err = k.GetNodeAccountSlashPoints(ctx, nas[2].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, int64(0))

	// if node is active after the tx get observed , it should not be slashed
	slashPoint, err = k.GetNodeAccountSlashPoints(ctx, nas[3].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, int64(0))

	ctx = ctx.WithBlockHeight(height + 301)
	err = slasher.LackObserving(ctx, constAccessor)

	c.Assert(err, IsNil)
	slashPoint, err = k.GetNodeAccountSlashPoints(ctx, nas[0].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, int64(0))

	slashPoint, err = k.GetNodeAccountSlashPoints(ctx, nas[1].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, lackOfObservationPenalty)
}

func (s *SlashingV88Suite) TestLackObservingErrors(c *C) {
	ctx, _ := setupKeeperForTest(c)

	nas := NodeAccounts{
		GetRandomValidatorNode(NodeActive),
		GetRandomValidatorNode(NodeActive),
	}
	keeper := &TestSlashObservingKeeper{
		nas:      nas,
		addrs:    []cosmos.AccAddress{nas[0].NodeAddress},
		slashPts: make(map[string]int64),
	}
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	slasher := newSlasherV88(keeper, NewDummyEventMgr())
	err := slasher.LackObserving(ctx, constAccessor)
	c.Assert(err, IsNil)
}

func (s *SlashingV88Suite) TestNodeSignSlashErrors(c *C) {
	testCases := []struct {
		name        string
		condition   func(keeper *TestSlashingLackKeeper)
		shouldError bool
	}{
		{
			name: "fail to get tx out should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failGetTxOut = true
			},
			shouldError: true,
		},
		{
			name: "fail to get vault should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failGetVault = true
			},
			shouldError: false,
		},
		{
			name: "fail to get node account by pub key should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failGetNodeAccountByPubKey = true
			},
			shouldError: false,
		},
		{
			name: "fail to get asgard vault by status should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failGetAsgardByStatus = true
			},
			shouldError: true,
		},
		{
			name: "fail to get observed tx voter should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failGetObservedTxVoter = true
			},
			shouldError: true,
		},
		{
			name: "fail to set tx out should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failSetTxOut = true
			},
			shouldError: true,
		},
	}
	for _, item := range testCases {
		c.Logf("name:%s", item.name)
		ctx, _ := setupKeeperForTest(c)
		ctx = ctx.WithBlockHeight(201) // set blockheight
		ver := GetCurrentVersion()
		constAccessor := constants.GetConstantValues(ver)
		na := GetRandomValidatorNode(NodeActive)
		inTx := common.NewTx(
			GetRandomTxHash(),
			GetRandomBNBAddress(),
			GetRandomBNBAddress(),
			common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(320000000)),
				common.NewCoin(common.RuneAsset(), cosmos.NewUint(420000000)),
			},
			nil,
			"SWAP:BNB.BNB",
		)

		txOutItem := TxOutItem{
			Chain:       common.BNBChain,
			InHash:      inTx.ID,
			VaultPubKey: na.PubKeySet.Secp256k1,
			ToAddress:   GetRandomBNBAddress(),
			Coin: common.NewCoin(
				common.BNBAsset, cosmos.NewUint(3980500*common.One),
			),
		}
		txOut := NewTxOut(3)
		txOut.TxArray = append(txOut.TxArray, txOutItem)

		ygg := GetRandomVault()
		ygg.Type = YggdrasilVault
		keeper := &TestSlashingLackKeeper{
			txOut:  txOut,
			na:     na,
			vaults: Vaults{ygg},
			voter: ObservedTxVoter{
				Actions: []TxOutItem{txOutItem},
			},
			slashPts: make(map[string]int64),
		}
		signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
		ctx = ctx.WithBlockHeight(3 + signingTransactionPeriod)
		slasher := newSlasherV88(keeper, NewDummyEventMgr())
		item.condition(keeper)
		if item.shouldError {
			c.Assert(slasher.LackSigning(ctx, constAccessor, NewDummyMgr()), NotNil)
		} else {
			c.Assert(slasher.LackSigning(ctx, constAccessor, NewDummyMgr()), IsNil)
		}
	}
}

func (s *SlashingV88Suite) TestNotSigningSlash(c *C) {
	ctx, _ := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(201) // set blockheight
	txOutStore := NewTxStoreDummy()
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	na := GetRandomValidatorNode(NodeActive)
	inTx := common.NewTx(
		GetRandomTxHash(),
		GetRandomBNBAddress(),
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(320000000)),
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(420000000)),
		},
		nil,
		"SWAP:BNB.BNB",
	)

	txOutItem := TxOutItem{
		Chain:       common.BNBChain,
		InHash:      inTx.ID,
		VaultPubKey: na.PubKeySet.Secp256k1,
		ToAddress:   GetRandomBNBAddress(),
		Coin: common.NewCoin(
			common.BNBAsset, cosmos.NewUint(3980500*common.One),
		),
	}
	txOut := NewTxOut(3)
	txOut.TxArray = append(txOut.TxArray, txOutItem)

	ygg := GetRandomVault()
	ygg.Type = YggdrasilVault
	ygg.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(5000000*common.One)),
	}
	keeper := &TestSlashingLackKeeper{
		txOut:  txOut,
		na:     na,
		vaults: Vaults{ygg},
		voter: ObservedTxVoter{
			Actions: []TxOutItem{txOutItem},
		},
		slashPts: make(map[string]int64),
	}
	signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	ctx = ctx.WithBlockHeight(3 + signingTransactionPeriod)
	mgr := NewDummyMgr()
	mgr.txOutStore = txOutStore
	slasher := newSlasherV88(keeper, NewDummyEventMgr())
	c.Assert(slasher.LackSigning(ctx, constAccessor, mgr), IsNil)

	c.Check(keeper.slashPts[na.NodeAddress.String()], Equals, int64(600), Commentf("%+v\n", na))

	outItems, err := txOutStore.GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(outItems, HasLen, 1)
	c.Assert(outItems[0].VaultPubKey.Equals(keeper.vaults[0].PubKey), Equals, true)
	c.Assert(outItems[0].Memo, Equals, "")
	c.Assert(keeper.voter.Actions, HasLen, 1)
	// ensure we've updated our action item
	c.Assert(keeper.voter.Actions[0].VaultPubKey.Equals(outItems[0].VaultPubKey), Equals, true)
	c.Assert(keeper.txOut.TxArray[0].OutHash.IsEmpty(), Equals, false)
}

func (s *SlashingV88Suite) TestNewSlasher(c *C) {
	nas := NodeAccounts{
		GetRandomValidatorNode(NodeActive),
		GetRandomValidatorNode(NodeActive),
	}
	keeper := &TestSlashObservingKeeper{
		nas:      nas,
		addrs:    []cosmos.AccAddress{nas[0].NodeAddress},
		slashPts: make(map[string]int64),
	}
	slasher := newSlasherV88(keeper, NewDummyEventMgr())
	c.Assert(slasher, NotNil)
}

func (s *SlashingV88Suite) TestDoubleSign(c *C) {
	ctx, _ := setupKeeperForTest(c)
	constAccessor := constants.GetConstantValues(GetCurrentVersion())

	na := GetRandomValidatorNode(NodeActive)
	na.Bond = cosmos.NewUint(100 * common.One)

	keeper := &TestDoubleSlashKeeper{
		na:      na,
		network: NewNetwork(),
		modules: make(map[string]int64),
	}
	slasher := newSlasherV88(keeper, NewDummyEventMgr())

	pk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeConsPub, na.ValidatorConsPubKey)
	c.Assert(err, IsNil)
	err = slasher.HandleDoubleSign(ctx, pk.Address(), 0, constAccessor)
	c.Assert(err, IsNil)

	c.Check(keeper.na.Bond.Equal(cosmos.NewUint(9995000000)), Equals, true, Commentf("%d", keeper.na.Bond.Uint64()))
	c.Check(keeper.modules[ReserveName], Equals, int64(5000000))
}

func (s *SlashingV88Suite) TestIncreaseDecreaseSlashPoints(c *C) {
	ctx, _ := setupKeeperForTest(c)

	na := GetRandomValidatorNode(NodeActive)
	na.Bond = cosmos.NewUint(100 * common.One)

	keeper := &TestDoubleSlashKeeper{
		na:          na,
		network:     NewNetwork(),
		slashPoints: make(map[string]int64),
	}
	slasher := newSlasherV88(keeper, NewDummyEventMgr())
	addr := GetRandomBech32Addr()
	slasher.IncSlashPoints(ctx, 1, addr)
	slasher.DecSlashPoints(ctx, 1, addr)
	c.Assert(keeper.slashPoints[addr.String()], Equals, int64(0))
}

func (s *SlashingV88Suite) TestSlashVault(c *C) {
	ctx, mgr := setupManagerForTest(c)
	slasher := newSlasherV88(mgr.Keeper(), mgr.EventMgr())
	// when coins are empty , it should return nil
	c.Assert(slasher.SlashVault(ctx, GetRandomPubKey(), common.NewCoins(), mgr), IsNil)

	// when vault is not available , it should return an error
	err := slasher.SlashVault(ctx, GetRandomPubKey(), common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One))), mgr)
	c.Assert(err, NotNil)
	c.Assert(errors.Is(err, types.ErrVaultNotFound), Equals, true)

	// create a node
	node := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, node), IsNil)
	FundModule(c, ctx, mgr.Keeper(), BondName, node.Bond.Uint64())
	vault := GetRandomVault()
	vault.Type = YggdrasilVault
	vault.Status = types2.VaultStatus_ActiveVault
	vault.PubKey = node.PubKeySet.Secp256k1
	vault.Membership = []string{
		node.PubKeySet.Secp256k1.String(),
	}
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)
	// when pool doesn't exist , node can't be slashed , because no price
	err = slasher.SlashVault(ctx, vault.PubKey, common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One))), mgr)
	c.Assert(err, IsNil)
	nodeTemp, err := mgr.Keeper().GetNodeAccountByPubKey(ctx, vault.PubKey)
	c.Assert(err, IsNil)
	c.Assert(nodeTemp.Bond.Equal(cosmos.NewUint(1000*common.One)), Equals, true)

	pool := NewPool()
	pool.Asset = common.BTCAsset
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.Status = PoolAvailable

	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	asgardBeforeSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, AsgardName)
	bondBeforeSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, BondName)
	reserveBeforeSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, ReserveName)
	poolBeforeSlash := pool.BalanceRune

	err = slasher.SlashVault(ctx, vault.PubKey, common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One))), mgr)
	c.Assert(err, IsNil)
	nodeTemp, err = mgr.Keeper().GetNodeAccountByPubKey(ctx, vault.PubKey)
	c.Assert(err, IsNil)
	expectedBond := cosmos.NewUint(99848484849)
	c.Assert(nodeTemp.Bond.Equal(expectedBond), Equals, true, Commentf("%d", nodeTemp.Bond.Uint64()))

	asgardAfterSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, AsgardName)
	bondAfterSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, BondName)
	reserveAfterSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, ReserveName)

	pool, err = mgr.Keeper().GetPool(ctx, pool.Asset)
	c.Assert(err, IsNil)
	poolAfterSlash := pool.BalanceRune

	// ensure pool's change is in sync with asgard's change
	c.Assert(asgardAfterSlash.Sub(asgardBeforeSlash).Uint64(), Equals, poolAfterSlash.Sub(poolBeforeSlash).Uint64(), Commentf("%d", "pool/asgard rune mismatch"))

	c.Assert(asgardAfterSlash.Sub(asgardBeforeSlash).Uint64(), Equals, uint64(101010101), Commentf("%d", asgardAfterSlash.Sub(asgardBeforeSlash).Uint64()))
	c.Assert(asgardAfterSlash.Sub(asgardBeforeSlash).Uint64(), Equals, uint64(101010101), Commentf("%d", asgardAfterSlash.Sub(asgardBeforeSlash).Uint64()))
	c.Assert(bondBeforeSlash.Sub(bondAfterSlash).Uint64(), Equals, uint64(151515151), Commentf("%d", bondBeforeSlash.Sub(bondAfterSlash).Uint64()))
	c.Assert(reserveAfterSlash.Sub(reserveBeforeSlash).Uint64(), Equals, uint64(50505050), Commentf("%d", reserveAfterSlash.Sub(reserveBeforeSlash).Uint64()))

	// add one more node , slash asgard
	node1 := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, node1), IsNil)
	FundModule(c, ctx, mgr.Keeper(), BondName, node1.Bond.Uint64())

	vault1 := GetRandomVault()
	vault1.Type = AsgardVault
	vault1.Status = types2.VaultStatus_ActiveVault
	vault1.PubKey = GetRandomPubKey()
	vault1.Membership = []string{
		node.PubKeySet.Secp256k1.String(),
		node1.PubKeySet.Secp256k1.String(),
	}
	c.Assert(mgr.Keeper().SetVault(ctx, vault1), IsNil)
	nodeBeforeSlash, err := mgr.Keeper().GetNodeAccount(ctx, node.NodeAddress)
	c.Assert(err, IsNil)
	nodeBondBeforeSlash := nodeBeforeSlash.Bond
	node1BondBeforeSlash := node1.Bond
	mgr.Keeper().SetMimir(ctx, "PauseOnSlashThreshold", 1)
	err = slasher.SlashVault(ctx, vault1.PubKey, common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One))), mgr)
	c.Assert(err, IsNil)

	nodeAfterSlash, err := mgr.Keeper().GetNodeAccount(ctx, node.NodeAddress)
	c.Assert(err, IsNil)
	node1AfterSlash, err := mgr.Keeper().GetNodeAccount(ctx, node1.NodeAddress)
	c.Assert(err, IsNil)
	nodeBondAfterSlash := nodeAfterSlash.Bond
	node1BondAfterSlash := node1AfterSlash.Bond

	c.Assert(nodeBondBeforeSlash.Sub(nodeBondAfterSlash).Uint64(), Equals, uint64(77245041), Commentf("%d", nodeBondBeforeSlash.Sub(nodeBondAfterSlash).Uint64()))
	c.Assert(node1BondBeforeSlash.Sub(node1BondAfterSlash).Uint64(), Equals, uint64(77362257), Commentf("%d", node1BondBeforeSlash.Sub(node1BondAfterSlash).Uint64()))

	val, err := mgr.Keeper().GetMimir(ctx, mimirStopFundYggdrasil)
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(18), Commentf("%d", val))

	val, err = mgr.Keeper().GetMimir(ctx, "HaltBTCChain")
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(18), Commentf("%d", val))
}

func (s *SlashingV88Suite) TestAdjustPoolForSlashedAsset(c *C) {
	ctx, mgr := setupManagerForTest(c)
	slasher := newSlasherV88(mgr.Keeper(), mgr.EventMgr())

	pool := NewPool()
	pool.Asset = common.BTCAsset
	pool.BalanceRune = cosmos.NewUint(1000 * common.One)
	pool.BalanceAsset = cosmos.NewUint(1000 * common.One)
	pool.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	// runeValue should be value of full coin amount in rune
	coin := common.NewCoin(common.BTCAsset, cosmos.NewUint(100*common.One))
	expRuneValue := pool.RuneReimbursementForAssetWithdrawal(coin.Amount)
	runeValue := slasher.adjustPoolForSlashedAsset(ctx, coin, mgr)
	poolAfter, err := mgr.Keeper().GetPool(ctx, pool.Asset)
	c.Assert(err, IsNil)
	c.Assert(runeValue.Uint64(), Equals, expRuneValue.Uint64())
	c.Assert(poolAfter.BalanceRune.Uint64(), Equals, pool.BalanceRune.Add(runeValue).Uint64())
	c.Assert(poolAfter.BalanceAsset.Uint64(), Equals, pool.BalanceAsset.Sub(coin.Amount).Uint64())

	// when coin is more than pool balance, full pool balance should be used
	pool.BalanceAsset = cosmos.NewUint(50 * common.One)
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)
	expRuneValue = pool.RuneReimbursementForAssetWithdrawal(pool.BalanceAsset)
	runeValue = slasher.adjustPoolForSlashedAsset(ctx, coin, mgr)
	poolAfter, err = mgr.Keeper().GetPool(ctx, pool.Asset)
	c.Assert(err, IsNil)
	c.Assert(runeValue.Uint64(), Equals, expRuneValue.Uint64())
	c.Assert(poolAfter.BalanceRune.Uint64(), Equals, pool.BalanceRune.Add(runeValue).Uint64())
	c.Assert(poolAfter.BalanceAsset.Uint64(), Equals, uint64(0))

	// empty pool should return 0
	pool.BalanceAsset = cosmos.ZeroUint()
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)
	expRuneValue = cosmos.ZeroUint()
	runeValue = slasher.adjustPoolForSlashedAsset(ctx, coin, mgr)
	poolAfter, err = mgr.Keeper().GetPool(ctx, pool.Asset)
	c.Assert(err, IsNil)
	c.Assert(runeValue.Uint64(), Equals, expRuneValue.Uint64())
	c.Assert(poolAfter.BalanceRune.Uint64(), Equals, pool.BalanceRune.Uint64())
	c.Assert(poolAfter.BalanceAsset.Uint64(), Equals, pool.BalanceAsset.Uint64())
}
