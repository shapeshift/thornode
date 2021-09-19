package thorchain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type ValidatorMgrV1TestSuite struct{}

var _ = Suite(&ValidatorMgrV1TestSuite{})

func (vts *ValidatorMgrV1TestSuite) SetUpSuite(_ *C) {
	SetupConfigForTest()
}

func (vts *ValidatorMgrV1TestSuite) TestSetupValidatorNodes(c *C) {
	ctx, k := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(1)
	mgr := NewDummyMgr()
	vMgr := newValidatorMgrV1(k, mgr.VaultMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(vMgr, NotNil)
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	err := vMgr.setupValidatorNodes(ctx, 0, constAccessor)
	c.Assert(err, IsNil)

	// no node accounts at all
	err = vMgr.setupValidatorNodes(ctx, 1, constAccessor)
	c.Assert(err, NotNil)

	activeNode := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNode), IsNil)

	err = vMgr.setupValidatorNodes(ctx, 1, constAccessor)
	c.Assert(err, IsNil)

	readyNode := GetRandomValidatorNode(NodeReady)
	c.Assert(k.SetNodeAccount(ctx, readyNode), IsNil)

	// one active node and one ready node on start up
	// it should take both of the node as active
	vMgr1 := newValidatorMgrV1(k, mgr.VaultMgr(), mgr.TxOutStore(), mgr.EventMgr())

	c.Assert(vMgr1.BeginBlock(ctx, constAccessor, nil), IsNil)
	activeNodes, err := k.ListActiveValidators(ctx)
	c.Assert(err, IsNil)
	c.Assert(len(activeNodes) == 2, Equals, true)

	activeNode1 := GetRandomValidatorNode(NodeActive)
	activeNode2 := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNode1), IsNil)
	c.Assert(k.SetNodeAccount(ctx, activeNode2), IsNil)

	// three active nodes and 1 ready nodes, it should take them all
	vMgr2 := newValidatorMgrV1(k, mgr.VaultMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(vMgr2.BeginBlock(ctx, constAccessor, nil), IsNil)

	activeNodes1, err := k.ListActiveValidators(ctx)
	c.Assert(err, IsNil)
	c.Assert(len(activeNodes1) == 4, Equals, true)
}

func (vts *ValidatorMgrV1TestSuite) TestRagnarokForChaosnet(c *C) {
	ctx, mgr := setupManagerForTest(c)
	c.Assert(mgr.BeginBlock(ctx), IsNil)
	vMgr := newValidatorMgrV1(mgr.Keeper(), mgr.VaultMgr(), mgr.TxOutStore(), mgr.EventMgr())

	constAccessor := constants.NewDummyConstants(map[constants.ConstantName]int64{
		constants.DesiredValidatorSet:           12,
		constants.ArtificialRagnarokBlockHeight: 1024,
		constants.BadValidatorRate:              256,
		constants.OldValidatorRate:              256,
		constants.MinimumNodesForBFT:            4,
		constants.ChurnInterval:                 256,
		constants.ChurnRetryInterval:            720,
		constants.AsgardSize:                    30,
	}, map[constants.ConstantName]bool{
		constants.StrictBondLiquidityRatio: false,
	}, map[constants.ConstantName]string{})
	for i := 0; i < 12; i++ {
		node := GetRandomValidatorNode(NodeReady)
		c.Assert(mgr.Keeper().SetNodeAccount(ctx, node), IsNil)
	}
	c.Assert(vMgr.setupValidatorNodes(ctx, 1, constAccessor), IsNil)
	nodeAccounts, err := mgr.Keeper().ListValidatorsByStatus(ctx, NodeActive)
	c.Assert(err, IsNil)
	c.Assert(len(nodeAccounts), Equals, 12)

	// trigger ragnarok
	ctx = ctx.WithBlockHeight(1024)
	c.Assert(vMgr.BeginBlock(ctx, constAccessor, nil), IsNil)
	vault := NewVault(common.BlockHeight(ctx), ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	for _, item := range nodeAccounts {
		vault.Membership = append(vault.Membership, item.PubKeySet.Secp256k1.String())
	}
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)
	updates := vMgr.EndBlock(ctx, mgr, constAccessor)
	// ragnarok , no one leaves
	c.Assert(updates, IsNil)
	ragnarokHeight, err := mgr.Keeper().GetRagnarokBlockHeight(ctx)
	c.Assert(err, IsNil)
	c.Assert(ragnarokHeight == 1024, Equals, true, Commentf("%d == %d", ragnarokHeight, 1024))
}

func (vts *ValidatorMgrV1TestSuite) TestLowerVersion(c *C) {
	ctx, mgr := setupManagerForTest(c)
	ctx = ctx.WithBlockHeight(1440)

	vMgr := newValidatorMgrV1(mgr.Keeper(), mgr.VaultMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(vMgr, NotNil)
	c.Assert(vMgr.markLowerVersion(ctx, 360), IsNil)

	for i := 0; i < 5; i++ {
		activeNode := GetRandomValidatorNode(NodeActive)
		activeNode.Version = "0.5.0"
		c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode), IsNil)
	}
	activeNode1 := GetRandomValidatorNode(NodeActive)
	activeNode1.Version = "0.4.0"
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode1), IsNil)

	c.Assert(vMgr.markLowerVersion(ctx, 360), IsNil)
	na, err := mgr.Keeper().GetNodeAccount(ctx, activeNode1.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(na.LeaveScore, Equals, uint64(143900000000))
}

func (vts *ValidatorMgrV1TestSuite) TestBadActors(c *C) {
	ctx, mgr := setupManagerForTest(c)
	ctx = ctx.WithBlockHeight(1000)

	vMgr := newValidatorMgrV1(mgr.Keeper(), mgr.VaultMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(vMgr, NotNil)

	// no bad actors with active node accounts
	nas, err := vMgr.findBadActors(ctx, 0, 3, 720)
	c.Assert(err, IsNil)
	c.Assert(nas, HasLen, 0)

	activeNode := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode), IsNil)

	// no bad actors with active node accounts with no slash points
	nas, err = vMgr.findBadActors(ctx, 0, 3, 720)
	c.Assert(err, IsNil)
	c.Assert(nas, HasLen, 0)

	activeNode = GetRandomValidatorNode(NodeActive)
	mgr.Keeper().SetNodeAccountSlashPoints(ctx, activeNode.NodeAddress, 250)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode), IsNil)
	activeNode = GetRandomValidatorNode(NodeActive)
	mgr.Keeper().SetNodeAccountSlashPoints(ctx, activeNode.NodeAddress, 500)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode), IsNil)

	// finds the worse actor
	nas, err = vMgr.findBadActors(ctx, 0, 3, 720)
	c.Assert(err, IsNil)
	c.Assert(nas, HasLen, 1)
	c.Check(nas[0].NodeAddress.Equals(activeNode.NodeAddress), Equals, true)

	// create really bad actors (crossing the redline)
	bad1 := GetRandomValidatorNode(NodeActive)
	mgr.Keeper().SetNodeAccountSlashPoints(ctx, bad1.NodeAddress, 5000)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, bad1), IsNil)
	bad2 := GetRandomValidatorNode(NodeActive)
	mgr.Keeper().SetNodeAccountSlashPoints(ctx, bad2.NodeAddress, 5000)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, bad2), IsNil)

	nas, err = vMgr.findBadActors(ctx, 0, 3, 720)
	c.Assert(err, IsNil)
	c.Assert(nas, HasLen, 2, Commentf("%d", len(nas)))

	// inconsistent order, workaround
	var count int
	for _, bad := range nas {
		if bad.Equals(bad1) || bad.Equals(bad2) {
			count += 1
		}
	}
	c.Check(count, Equals, 2)
}

func (vts *ValidatorMgrV1TestSuite) TestFindBadActors(c *C) {
	ctx, mgr := setupManagerForTest(c)
	ctx = ctx.WithBlockHeight(1000)

	vMgr := newValidatorMgrV1(mgr.Keeper(), mgr.VaultMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(vMgr, NotNil)

	activeNode := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode), IsNil)
	mgr.Keeper().SetNodeAccountSlashPoints(ctx, activeNode.NodeAddress, 50)
	nodeAccounts, err := vMgr.findBadActors(ctx, 100, 3, 500)
	c.Assert(err, IsNil)
	c.Assert(nodeAccounts, HasLen, 0)

	activeNode1 := GetRandomValidatorNode(NodeActive)
	activeNode1.StatusSince = 900
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode1), IsNil)
	mgr.Keeper().SetNodeAccountSlashPoints(ctx, activeNode1.NodeAddress, 200)

	// not a full cycle yet, so even it has a lot slash points , doesn't mark as bad
	nodeAccounts, err = vMgr.findBadActors(ctx, 100, 3, 500)
	c.Assert(err, IsNil)
	c.Assert(nodeAccounts, HasLen, 0)

	activeNode2 := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode2), IsNil)
	mgr.Keeper().SetNodeAccountSlashPoints(ctx, activeNode2.NodeAddress, 2000)

	activeNode3 := GetRandomValidatorNode(NodeActive)
	activeNode3.StatusSince = 1000
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNode3), IsNil)
	mgr.Keeper().SetNodeAccountSlashPoints(ctx, activeNode3.NodeAddress, 2000)
	ctx = ctx.WithBlockHeight(2000)
	// node 3 is worse than node 2, so node 3 should be marked as bad
	nodeAccounts, err = vMgr.findBadActors(ctx, 100, 3, 500)
	c.Assert(err, IsNil)
	c.Assert(nodeAccounts, HasLen, 1)
	c.Assert(nodeAccounts.Contains(activeNode3), Equals, true)
}

func (vts *ValidatorMgrV1TestSuite) TestRagnarokBond(c *C) {
	ctx, k := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(1)
	ver := GetCurrentVersion()

	mgr := NewDummyMgrWithKeeper(k)
	vMgr := newValidatorMgrV1(k, mgr.VaultMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(vMgr, NotNil)

	constAccessor := constants.GetConstantValues(ver)
	err := vMgr.setupValidatorNodes(ctx, 0, constAccessor)
	c.Assert(err, IsNil)

	activeNode := GetRandomValidatorNode(NodeActive)
	activeNode.Bond = cosmos.NewUint(100)
	c.Assert(k.SetNodeAccount(ctx, activeNode), IsNil)

	disabledNode := GetRandomValidatorNode(NodeDisabled)
	disabledNode.Bond = cosmos.ZeroUint()
	c.Assert(k.SetNodeAccount(ctx, disabledNode), IsNil)

	// no unbonding for first 10
	c.Assert(vMgr.ragnarokBond(ctx, 1, mgr), IsNil)
	activeNode, err = k.GetNodeAccount(ctx, activeNode.NodeAddress)
	c.Assert(err, IsNil)
	c.Check(activeNode.Bond.Equal(cosmos.NewUint(100)), Equals, true)

	c.Assert(vMgr.ragnarokBond(ctx, 11, mgr), IsNil)
	activeNode, err = k.GetNodeAccount(ctx, activeNode.NodeAddress)
	c.Assert(err, IsNil)
	c.Check(activeNode.Bond.Equal(cosmos.NewUint(90)), Equals, true)
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Check(items, HasLen, 0, Commentf("Len %d", items))
	mgr.TxOutStore().ClearOutboundItems(ctx)

	c.Assert(vMgr.ragnarokBond(ctx, 12, mgr), IsNil)
	activeNode, err = k.GetNodeAccount(ctx, activeNode.NodeAddress)
	c.Assert(err, IsNil)
	c.Check(activeNode.Bond.Equal(cosmos.NewUint(72)), Equals, true)
	items, err = mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Check(items, HasLen, 0, Commentf("Len %d", items))
}

func (vts *ValidatorMgrV1TestSuite) TestGetChangedNodes(c *C) {
	ctx, k := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(1)
	ver := GetCurrentVersion()

	mgr := NewDummyMgrWithKeeper(k)
	vMgr := newValidatorMgrV1(k, mgr.VaultMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(vMgr, NotNil)

	constAccessor := constants.GetConstantValues(ver)
	err := vMgr.setupValidatorNodes(ctx, 0, constAccessor)
	c.Assert(err, IsNil)

	activeNode := GetRandomValidatorNode(NodeActive)
	activeNode.Bond = cosmos.NewUint(100)
	activeNode.ForcedToLeave = true
	c.Assert(k.SetNodeAccount(ctx, activeNode), IsNil)

	disabledNode := GetRandomValidatorNode(NodeDisabled)
	disabledNode.Bond = cosmos.ZeroUint()
	c.Assert(k.SetNodeAccount(ctx, disabledNode), IsNil)

	vault := NewVault(common.BlockHeight(ctx), ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	vault.Membership = append(vault.Membership, activeNode.PubKeySet.Secp256k1.String())
	c.Assert(k.SetVault(ctx, vault), IsNil)

	newNodes, removedNodes, err := vMgr.getChangedNodes(ctx, NodeAccounts{activeNode})
	c.Assert(err, IsNil)
	c.Assert(newNodes, HasLen, 0)
	c.Assert(removedNodes, HasLen, 1)
}

func (vts *ValidatorMgrV1TestSuite) TestSplitNext(c *C) {
	ctx, k := setupKeeperForTest(c)
	mgr := NewDummyMgrWithKeeper(k)
	vMgr := newValidatorMgrV1(k, mgr.VaultMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(vMgr, NotNil)

	nas := make(NodeAccounts, 0)
	for i := 0; i < 90; i++ {
		na := GetRandomValidatorNode(NodeActive)
		na.Bond = cosmos.NewUint(uint64(i))
		nas = append(nas, na)
	}
	sets := vMgr.splitNext(ctx, nas, 30)
	c.Assert(sets, HasLen, 3)
	c.Assert(sets[0], HasLen, 30)
	c.Assert(sets[1], HasLen, 30)
	c.Assert(sets[2], HasLen, 30)

	nas = make(NodeAccounts, 0)
	for i := 0; i < 100; i++ {
		na := GetRandomValidatorNode(NodeActive)
		na.Bond = cosmos.NewUint(uint64(i))
		nas = append(nas, na)
	}
	sets = vMgr.splitNext(ctx, nas, 30)
	c.Assert(sets, HasLen, 4)
	c.Assert(sets[0], HasLen, 25)
	c.Assert(sets[1], HasLen, 25)
	c.Assert(sets[2], HasLen, 25)
	c.Assert(sets[3], HasLen, 25)

	nas = make(NodeAccounts, 0)
	for i := 0; i < 3; i++ {
		na := GetRandomValidatorNode(NodeActive)
		na.Bond = cosmos.NewUint(uint64(i))
		nas = append(nas, na)
	}
	sets = vMgr.splitNext(ctx, nas, 30)
	c.Assert(sets, HasLen, 1)
	c.Assert(sets[0], HasLen, 3)
}

func (vts *ValidatorMgrV1TestSuite) TestFindCounToRemove(c *C) {
	// remove one
	c.Check(findCountToRemove(0, NodeAccounts{
		NodeAccount{LeaveScore: 12},
		NodeAccount{},
		NodeAccount{},
		NodeAccount{},
		NodeAccount{},
	}), Equals, 1)

	// don't remove one
	c.Check(findCountToRemove(0, NodeAccounts{
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{},
		NodeAccount{},
	}), Equals, 0)

	// remove one because of request to leave
	c.Check(findCountToRemove(0, NodeAccounts{
		NodeAccount{LeaveScore: 12, RequestedToLeave: true},
		NodeAccount{},
		NodeAccount{},
		NodeAccount{},
	}), Equals, 1)

	// remove one because of banned
	c.Check(findCountToRemove(0, NodeAccounts{
		NodeAccount{LeaveScore: 12, ForcedToLeave: true},
		NodeAccount{},
		NodeAccount{},
		NodeAccount{},
	}), Equals, 1)

	// don't remove more than 1/3rd of node accounts
	c.Check(findCountToRemove(0, NodeAccounts{
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
		NodeAccount{LeaveScore: 12},
	}), Equals, 3)
}

func (vts *ValidatorMgrV1TestSuite) TestFindMaxAbleToLeave(c *C) {
	c.Check(findMaxAbleToLeave(-1), Equals, 0)
	c.Check(findMaxAbleToLeave(0), Equals, 0)
	c.Check(findMaxAbleToLeave(1), Equals, 0)
	c.Check(findMaxAbleToLeave(2), Equals, 0)
	c.Check(findMaxAbleToLeave(3), Equals, 0)
	c.Check(findMaxAbleToLeave(4), Equals, 0)

	c.Check(findMaxAbleToLeave(5), Equals, 1)
	c.Check(findMaxAbleToLeave(6), Equals, 1)
	c.Check(findMaxAbleToLeave(7), Equals, 2)
	c.Check(findMaxAbleToLeave(8), Equals, 2)
	c.Check(findMaxAbleToLeave(9), Equals, 2)
	c.Check(findMaxAbleToLeave(10), Equals, 3)
	c.Check(findMaxAbleToLeave(11), Equals, 3)
	c.Check(findMaxAbleToLeave(12), Equals, 3)
}

func (vts *ValidatorMgrV1TestSuite) TestFindNextVaultNodeAccounts(c *C) {
	ctx, k := setupKeeperForTest(c)
	mgr := NewDummyMgrWithKeeper(k)
	vMgr := newValidatorMgrV1(k, mgr.VaultMgr(), mgr.TxOutStore(), mgr.EventMgr())
	c.Assert(vMgr, NotNil)
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	nas := NodeAccounts{}
	for i := 0; i < 12; i++ {
		na := GetRandomValidatorNode(NodeActive)
		nas = append(nas, na)
	}
	nas[0].LeaveScore = 1024
	k.SetNodeAccountSlashPoints(ctx, nas[0].NodeAddress, 50)
	nas[1].LeaveScore = 1025
	k.SetNodeAccountSlashPoints(ctx, nas[1].NodeAddress, 200)
	nas[2].ForcedToLeave = true
	nas[3].RequestedToLeave = true
	for _, item := range nas {
		c.Assert(k.SetNodeAccount(ctx, item), IsNil)
	}
	nasAfter, rotate, err := vMgr.nextVaultNodeAccounts(ctx, 12, constAccessor)
	c.Assert(err, IsNil)
	c.Assert(rotate, Equals, true)
	c.Assert(nasAfter, HasLen, 10)
}
