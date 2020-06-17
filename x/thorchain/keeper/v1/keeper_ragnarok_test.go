package keeperv1

import (
	. "gopkg.in/check.v1"
)

type KeeperRagnarokSuite struct{}

var _ = Suite(&KeeperRagnarokSuite{})

func (s *KeeperRagnarokSuite) TestVault(c *C) {
	ctx, k := setupKeeperForTest(c)

	k.SetRagnarokBlockHeight(ctx, 12)
	height, err := k.GetRagnarokBlockHeight(ctx)
	c.Assert(err, IsNil)
	c.Assert(height, Equals, int64(12))

	k.SetRagnarokNth(ctx, 2)
	nth, err := k.GetRagnarokNth(ctx)
	c.Assert(err, IsNil)
	c.Assert(nth, Equals, int64(2))

	k.SetRagnarokPending(ctx, 4)
	pending, err := k.GetRagnarokPending(ctx)
	c.Assert(err, IsNil)
	c.Assert(pending, Equals, int64(4))
}
