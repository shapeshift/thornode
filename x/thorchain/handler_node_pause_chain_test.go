package thorchain

import (
	. "gopkg.in/check.v1"
)

var _ = Suite(&HandlerNodePauseChainSuite{})

type HandlerNodePauseChainSuite struct{}

func (s *HandlerNodePauseChainSuite) TestValidate(c *C) {
	ctx, k := setupKeeperForTest(c)

	node := GetRandomValidatorNode(NodeActive)
	k.SetNodeAccount(ctx, node)

	handler := NewNodePauseChainHandler(NewDummyMgrWithKeeper(k))
	// happy path
	msg := NewMsgNodePauseChain(300, node.NodeAddress)
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// invalid msg
	msg = &MsgNodePauseChain{}
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
}

func (s *HandlerNodePauseChainSuite) TestHandle(c *C) {
	ctx, k := setupKeeperForTest(c)

	node := GetRandomValidatorNode(NodeActive)
	node2 := GetRandomValidatorNode(NodeActive)
	node3 := GetRandomValidatorNode(NodeActive)
	k.SetNodeAccount(ctx, node)
	k.SetNodeAccount(ctx, node2)
	k.SetNodeAccount(ctx, node3)

	handler := NewNodePauseChainHandler(NewDummyMgrWithKeeper(k))

	// happy path
	msg := NewMsgNodePauseChain(300, node.NodeAddress)
	err := handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	pause, err := k.GetMimir(ctx, "NodePauseChainGlobal")
	c.Assert(err, IsNil)
	c.Check(pause, Equals, int64(738))

	msg = NewMsgNodePauseChain(1, node2.NodeAddress)
	err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	pause, err = k.GetMimir(ctx, "NodePauseChainGlobal")
	c.Assert(err, IsNil)
	c.Check(pause, Equals, int64(1458))

	msg = NewMsgNodePauseChain(-1, node3.NodeAddress)
	err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	pause, err = k.GetMimir(ctx, "NodePauseChainGlobal")
	c.Assert(err, IsNil)
	c.Check(pause, Equals, int64(738))

	msg = NewMsgNodePauseChain(1, node2.NodeAddress)
	err = handler.handle(ctx, *msg)
	c.Assert(err, NotNil)
	pause, err = k.GetMimir(ctx, "NodePauseChainGlobal")
	c.Assert(err, IsNil)
	c.Check(pause, Equals, int64(738))
}
