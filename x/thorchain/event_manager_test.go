package thorchain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type EventManagerTestSuite struct{}

var _ = Suite(&EventManagerTestSuite{})

func (s *EventManagerTestSuite) TestEmitPoolEvent(c *C) {
	ctx, k := setupKeeperForTest(c)
	eventMgr := NewEventMgr()
	c.Assert(eventMgr, NotNil)
	ctx = ctx.WithBlockHeight(1024)
	c.Assert(eventMgr.EmitPoolEvent(ctx, k, common.BlankTxID, EventSuccess, NewEventPool(common.BNBAsset, PoolEnabled)), IsNil)
}

func (s *EventManagerTestSuite) TestEmitErrataEvent(c *C) {
	ctx, k := setupKeeperForTest(c)
	eventMgr := NewEventMgr()
	c.Assert(eventMgr, NotNil)
	ctx = ctx.WithBlockHeight(1024)
	errataEvent := NewEventErrata(GetRandomTxHash(), PoolMods{
		PoolMod{
			Asset:    common.BNBAsset,
			RuneAmt:  cosmos.ZeroUint(),
			RuneAdd:  false,
			AssetAmt: cosmos.NewUint(100),
			AssetAdd: true,
		},
	})
	c.Assert(eventMgr.EmitErrataEvent(ctx, k, common.BlankTxID, errataEvent), IsNil)
}

func (s *EventManagerTestSuite) TestEmitGasEvent(c *C) {
	ctx, k := setupKeeperForTest(c)
	eventMgr := NewEventMgr()
	c.Assert(eventMgr, NotNil)
	ctx = ctx.WithBlockHeight(1024)
	gasEvent := NewEventGas()
	gasEvent.Pools = append(gasEvent.Pools, GasPool{
		Asset:    common.BNBAsset,
		AssetAmt: cosmos.ZeroUint(),
		RuneAmt:  cosmos.NewUint(1024),
		Count:    1,
	})
	c.Assert(eventMgr.EmitGasEvent(ctx, k, gasEvent), IsNil)
}
