package types

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
)

type StakerSuite struct{}

var _ = Suite(&StakerSuite{})

func (StakerSuite) TestStaker(c *C) {
	staker := Staker{
		Asset:           common.BNBAsset,
		RuneAddress:     GetRandomBNBAddress(),
		AssetAddress:    GetRandomBTCAddress(),
		LastStakeHeight: 12,
	}
	c.Check(staker.IsValid(), IsNil)
	c.Check(len(staker.Key()) > 0, Equals, true)
	staker1 := Staker{
		Asset:           common.BNBAsset,
		RuneAddress:     GetRandomBNBAddress(),
		AssetAddress:    GetRandomBTCAddress(),
		LastStakeHeight: 0,
	}
	c.Check(staker1.IsValid(), NotNil)

	staker2 := Staker{
		Asset:           common.BNBAsset,
		RuneAddress:     common.NoAddress,
		AssetAddress:    GetRandomBTCAddress(),
		LastStakeHeight: 100,
	}
	c.Check(staker2.IsValid(), NotNil)

	staker3 := Staker{
		Asset:           common.BNBAsset,
		RuneAddress:     GetRandomBNBAddress(),
		AssetAddress:    common.NoAddress,
		LastStakeHeight: 100,
	}
	c.Check(staker3.IsValid(), NotNil)
}
