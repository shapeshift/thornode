package thorchain

import (
	. "gopkg.in/check.v1"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type KeeperVaultDataSuite struct{}

var _ = Suite(&KeeperVaultDataSuite{})

func (KeeperVaultDataSuite) TestVaultData(c *C) {
	ctx, k := setupKeeperForTest(c)
	vd := NewVaultData()
	err := k.SetVaultData(ctx, vd)
	c.Assert(err, IsNil)
}

func (KeeperVaultDataSuite) TestGetTotalActiveNodeWithBound(c *C) {
	ctx, k := setupKeeperForTest(c)

	node1 := GetRandomNodeAccount(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, node1), IsNil)
	node2 := GetRandomNodeAccount(NodeActive)
	node2.Bond = cosmos.ZeroUint()
	c.Assert(k.SetNodeAccount(ctx, node2), IsNil)
	n, err := getTotalActiveNodeWithBond(ctx, k)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, int64(1))
}
