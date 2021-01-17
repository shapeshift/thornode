package types

import (
	"errors"

	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
)

type MsgTssPoolSuite struct{}

var _ = Suite(&MsgTssPoolSuite{})

func (s *MsgTssPoolSuite) TestMsgTssPool(c *C) {
	pk := GetRandomPubKey()
	pks := []string{
		GetRandomPubKey().String(), GetRandomPubKey().String(), GetRandomPubKey().String(),
	}
	addr := GetRandomBech32Addr()
	keygenTime := int64(1024)
	msg := NewMsgTssPool(pks, pk, KeygenType_AsgardKeygen, 1, Blame{}, []string{common.RuneAsset().Chain.String()}, addr, keygenTime)
	c.Check(msg.Type(), Equals, "set_tss_pool")
	c.Assert(msg.ValidateBasic(), IsNil)
	EnsureMsgBasicCorrect(msg, c)

	chains := []string{common.RuneAsset().Chain.String()}
	c.Check(NewMsgTssPool(pks, pk, KeygenType_AsgardKeygen, 1, Blame{}, chains, nil, keygenTime).ValidateBasic(), NotNil)
	c.Check(NewMsgTssPool(nil, pk, KeygenType_AsgardKeygen, 1, Blame{}, chains, addr, keygenTime).ValidateBasic(), NotNil)
	c.Check(NewMsgTssPool(pks, "", KeygenType_AsgardKeygen, 1, Blame{}, chains, addr, keygenTime).ValidateBasic(), NotNil)
	c.Check(NewMsgTssPool(pks, "bogusPubkey", KeygenType_AsgardKeygen, 1, Blame{}, chains, addr, keygenTime).ValidateBasic(), NotNil)

	// fails on empty chain list
	msg = NewMsgTssPool(pks, pk, KeygenType_AsgardKeygen, 1, Blame{}, []string{}, addr, keygenTime)
	c.Check(msg.ValidateBasic(), NotNil)
	// fails on duplicates in chain list
	msg = NewMsgTssPool(pks, pk, KeygenType_AsgardKeygen, 1, Blame{}, []string{common.RuneAsset().Chain.String(), common.RuneAsset().Chain.String()}, addr, keygenTime)
	c.Check(msg.ValidateBasic(), NotNil)

	msg1 := NewMsgTssPool(pks, pk, KeygenType_AsgardKeygen, 1, Blame{}, chains, addr, keygenTime)
	msg1.ID = ""
	err1 := msg1.ValidateBasic()
	c.Assert(err1, NotNil)
	c.Check(errors.Is(err1, se.ErrUnknownRequest), Equals, true)

	msg2 := NewMsgTssPool(append(pks, ""), pk, KeygenType_AsgardKeygen, 1, Blame{}, chains, addr, keygenTime)
	err2 := msg2.ValidateBasic()
	c.Assert(err2, NotNil)
	c.Check(errors.Is(err2, se.ErrUnknownRequest), Equals, true)

	var allPks []string
	for i := 0; i < 110; i++ {
		allPks = append(allPks, GetRandomPubKey().String())
	}
	msg3 := NewMsgTssPool(allPks, pk, KeygenType_AsgardKeygen, 1, Blame{}, chains, addr, keygenTime)
	err3 := msg3.ValidateBasic()
	c.Assert(err3, NotNil)
	c.Check(errors.Is(err3, se.ErrUnknownRequest), Equals, true)
}
