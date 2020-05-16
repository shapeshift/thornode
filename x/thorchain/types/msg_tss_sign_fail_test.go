package types

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/tss/go-tss/blame"
)

type MsgTssKeysignFailSuite struct{}

var _ = Suite(&MsgTssKeysignFailSuite{})

func (s MsgTssKeysignFailSuite) TestMsgTssKeysignFail(c *C) {
	b := blame.Blame{
		FailReason: "fail to TSS sign",
		BlameNodes: []blame.Node{
			blame.Node{Pubkey: GetRandomPubKey().String()},
			blame.Node{Pubkey: GetRandomPubKey().String()},
		},
	}
	coins := common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(100)),
	}
	msg := NewMsgTssKeysignFail(1, b, "hello", coins, GetRandomBech32Addr())
	c.Check(msg.Type(), Equals, "set_tss_keysign_fail")
	EnsureMsgBasicCorrect(msg, c)
	c.Check(NewMsgTssKeysignFail(1, blame.Blame{}, "hello", coins, GetRandomBech32Addr()), NotNil)
	c.Check(NewMsgTssKeysignFail(1, b, "", coins, GetRandomBech32Addr()), NotNil)
	c.Check(NewMsgTssKeysignFail(1, b, "hello", common.Coins{}, GetRandomBech32Addr()), NotNil)
	c.Check(NewMsgTssKeysignFail(1, b, "hello", common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(100)),
		common.NewCoin(common.EmptyAsset, cosmos.ZeroUint()),
	}, GetRandomBech32Addr()), NotNil)
	c.Check(NewMsgTssKeysignFail(1, b, "hello", coins, cosmos.AccAddress{}), NotNil)
}
