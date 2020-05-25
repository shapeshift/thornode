package keeperv1

import (
	. "gopkg.in/check.v1"
)

type KeeperVaultDataSuite struct{}

var _ = Suite(&KeeperVaultDataSuite{})

func (KeeperVaultDataSuite) TestVaultData(c *C) {
	ctx, k := setupKeeperForTest(c)
	vd := NewVaultData()
	err := k.SetVaultData(ctx, vd)
	c.Assert(err, IsNil)
}
