package keeperv1

import (
	. "gopkg.in/check.v1"
)

type KeeperMimirSuite struct{}

var _ = Suite(&KeeperMimirSuite{})

func (s *KeeperMimirSuite) TestMimir(c *C) {
	ctx, k := setupKeeperForTest(c)

	k.SetMimir(ctx, "foo", 14)

	val, err := k.GetMimir(ctx, "foo")
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(14))

	val, err = k.GetMimir(ctx, "bogus")
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(-1))

	// test that releasing the kraken removes previously set key/values
	k.SetMimir(ctx, KRAKEN, 0)
	val, err = k.GetMimir(ctx, "foo")
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(-1))

	// test that we cannot put the kraken back in the cage
	k.SetMimir(ctx, KRAKEN, -1)
	k.SetMimir(ctx, "foo", 33)
	val, err = k.GetMimir(ctx, "foo")
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(-1))
	c.Check(k.GetMimirIterator(ctx), NotNil)

	addr := GetRandomBech32Addr()
	k.SetNodePauseChain(ctx, addr)
	pause := k.GetNodePauseChain(ctx, addr)
	c.Assert(pause, Equals, int64(18))
}

func (s *KeeperMimirSuite) TestNodeMimir(c *C) {
	ctx, k := setupKeeperForTest(c)
	key := "foo"
	ver := "0.77.0"

	na1 := GetRandomValidatorNode(NodeActive)
	na1.Version = ver
	na2 := GetRandomValidatorNode(NodeActive)
	na2.Version = ver
	na3 := GetRandomValidatorNode(NodeActive)
	na3.Version = ver
	c.Assert(k.SetNodeAccount(ctx, na1), IsNil)
	c.Assert(k.SetNodeAccount(ctx, na2), IsNil)
	c.Assert(k.SetNodeAccount(ctx, na3), IsNil)

	k.SetMimir(ctx, key, 14)
	// without node mimirs set
	val, err := k.GetMimir(ctx, key)
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(14))

	c.Assert(k.SetNodeMimir(ctx, key, 10, na1.NodeAddress), IsNil)
	c.Assert(k.SetNodeMimir(ctx, key, 20, na2.NodeAddress), IsNil)
	c.Assert(k.SetNodeMimir(ctx, key, 20, na3.NodeAddress), IsNil)

	// with node mimirs set
	val, err = k.GetMimir(ctx, key)
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(20))

	// lose consensus of node mimirs
	c.Assert(k.SetNodeMimir(ctx, key, 30, na3.NodeAddress), IsNil)
	val, err = k.GetMimir(ctx, key)
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(14))
}
