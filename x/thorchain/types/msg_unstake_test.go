package types

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type MsgUnstakeSuite struct{}

var _ = Suite(&MsgUnstakeSuite{})

func (MsgUnstakeSuite) TestMsgUnstake(c *C) {
	txID := GetRandomTxHash()
	tx := common.NewTx(
		txID,
		GetRandomBNBAddress(),
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.BTCAsset, cosmos.NewUint(100000000)),
		},
		BNBGasFeeSingleton,
		"",
	)
	runeAddr := GetRandomRUNEAddress()
	acc1 := GetRandomBech32Addr()
	m := NewMsgSetUnStake(tx, runeAddr, cosmos.NewUint(10000), common.BNBAsset, acc1)
	EnsureMsgBasicCorrect(m, c)
	c.Check(m.Type(), Equals, "set_unstake")

	inputs := []struct {
		publicAddress       common.Address
		withdrawBasisPoints cosmos.Uint
		asset               common.Asset
		requestTxHash       common.TxID
		signer              cosmos.AccAddress
	}{
		{
			publicAddress:       common.NoAddress,
			withdrawBasisPoints: cosmos.NewUint(10000),
			asset:               common.BNBAsset,
			requestTxHash:       txID,
			signer:              acc1,
		},
		{
			publicAddress:       runeAddr,
			withdrawBasisPoints: cosmos.NewUint(12000),
			asset:               common.BNBAsset,
			requestTxHash:       txID,
			signer:              acc1,
		},
		{
			publicAddress:       runeAddr,
			withdrawBasisPoints: cosmos.ZeroUint(),
			asset:               common.BNBAsset,
			requestTxHash:       txID,
			signer:              acc1,
		},
		{
			publicAddress:       runeAddr,
			withdrawBasisPoints: cosmos.NewUint(10000),
			asset:               common.Asset{},
			requestTxHash:       txID,
			signer:              acc1,
		},
		{
			publicAddress:       runeAddr,
			withdrawBasisPoints: cosmos.NewUint(10000),
			asset:               common.BNBAsset,
			requestTxHash:       common.TxID(""),
			signer:              acc1,
		},
		{
			publicAddress:       runeAddr,
			withdrawBasisPoints: cosmos.NewUint(10000),
			asset:               common.BNBAsset,
			requestTxHash:       txID,
			signer:              cosmos.AccAddress{},
		},
	}
	for _, item := range inputs {
		tx := common.Tx{ID: item.requestTxHash}
		m := NewMsgSetUnStake(tx, item.publicAddress, item.withdrawBasisPoints, item.asset, item.signer)
		c.Assert(m.ValidateBasic(), NotNil)
	}
}
