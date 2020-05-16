package types

import (
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	. "gopkg.in/check.v1"
)

type MsgReserveContributorSuite struct{}

var _ = Suite(&MsgReserveContributorSuite{})

func (s *MsgReserveContributorSuite) TestMsgReserveContributor(c *C) {
	addr := GetRandomBNBAddress()
	amt := cosmos.NewUint(378 * common.One)
	res := NewReserveContributor(addr, amt)
	signer := GetRandomBech32Addr()

	msg := NewMsgReserveContributor(GetRandomTx(), res, signer)
	c.Check(msg.Contributor.IsEmpty(), Equals, false)
	c.Check(msg.Signer.Equals(signer), Equals, true)
}
