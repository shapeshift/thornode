package types

import (
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	. "gopkg.in/check.v1"
)

type MsgObservedTxOutSuite struct{}

var _ = Suite(&MsgObservedTxOutSuite{})

func (s *MsgObservedTxOutSuite) TestMsgObservedTxOut(c *C) {
	var err error
	pk := GetRandomPubKey()
	tx := NewObservedTx(GetRandomTx(), 55, pk)
	tx.Tx.FromAddress, err = pk.GetAddress(tx.Tx.Coins[0].Asset.Chain)
	c.Assert(err, IsNil)
	acc := GetRandomBech32Addr()

	m := NewMsgObservedTxOut(ObservedTxs{tx}, acc)
	EnsureMsgBasicCorrect(m, c)
	c.Check(m.Type(), Equals, "set_observed_txout")

	m1 := NewMsgObservedTxOut(nil, acc)
	c.Assert(m1.ValidateBasic(), NotNil)
	m2 := NewMsgObservedTxOut(ObservedTxs{tx}, cosmos.AccAddress{})
	c.Assert(m2.ValidateBasic(), NotNil)

	// will not accept observations with pre-determined signers. This is
	// important to ensure an observer can fake signers from other node accounts
	// *IMPORTANT* DON'T REMOVE THIS CHECK
	tx.Signers = append(tx.Signers, GetRandomBech32Addr())
	m3 := NewMsgObservedTxOut(ObservedTxs{tx}, acc)
	c.Assert(m3.ValidateBasic(), NotNil)
}
