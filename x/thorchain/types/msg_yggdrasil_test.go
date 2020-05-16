package types

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type MsgYggdrasilSuite struct{}

var _ = Suite(&MsgYggdrasilSuite{})

func (s *MsgYggdrasilSuite) TestMsgYggdrasil(c *C) {
	tx := GetRandomTx()
	pk := GetRandomPubKey()
	coins := common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(500*common.One)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(400*common.One)),
	}
	signer := GetRandomBech32Addr()
	msg := NewMsgYggdrasil(tx, pk, 12, true, coins, signer)
	c.Check(msg.PubKey.Equals(pk), Equals, true)
	c.Check(msg.AddFunds, Equals, true)
	c.Check(msg.Coins, HasLen, len(coins))
	c.Check(msg.Tx.Equals(tx), Equals, true)
	c.Check(msg.Signer.Equals(signer), Equals, true)
	c.Check(msg.BlockHeight, Equals, int64(12))
}
