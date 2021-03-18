package common

import (
	. "gopkg.in/check.v1"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type CoinSuite struct{}

var _ = Suite(&CoinSuite{})

func (s CoinSuite) TestCoin(c *C) {
	coin := NewCoin(BNBAsset, cosmos.NewUint(230000000))
	c.Check(coin.Asset.Equals(BNBAsset), Equals, true)
	c.Check(coin.Amount.Uint64(), Equals, uint64(230000000))
	c.Check(coin.Valid(), IsNil)
	c.Check(coin.IsEmpty(), Equals, false)
	c.Check(NoCoin.IsEmpty(), Equals, true)

	c.Check(coin.IsNative(), Equals, false)
	_, err := coin.Native()
	c.Assert(err, NotNil)
	coin = NewCoin(RuneNative, cosmos.NewUint(230))
	c.Check(coin.IsNative(), Equals, true)
	sdkCoin, err := coin.Native()
	c.Assert(err, IsNil)
	c.Check(sdkCoin.Denom, Equals, "rune")
	c.Check(sdkCoin.Amount.Equal(cosmos.NewInt(230)), Equals, true)
}

func (s CoinSuite) TestDistinct(c *C) {
	coins := Coins{
		NewCoin(BNBAsset, cosmos.NewUint(1000)),
		NewCoin(BNBAsset, cosmos.NewUint(1000)),
		NewCoin(BTCAsset, cosmos.NewUint(1000)),
		NewCoin(BTCAsset, cosmos.NewUint(1000)),
	}
	newCoins := coins.Distinct()
	c.Assert(len(newCoins), Equals, 2)
}

func (s CoinSuite) TestAdds(c *C) {
	coins := Coins{
		NewCoin(BNBAsset, cosmos.NewUint(1000)),
	}
	newCoins := coins.Adds(NewCoins(
		NewCoin(BNBAsset, cosmos.NewUint(1000)),
		NewCoin(BTCAsset, cosmos.NewUint(1000)),
	))
	c.Assert(len(newCoins), Equals, 2)
	c.Check(newCoins.GetCoin(BNBAsset).Amount.Uint64(), Equals, uint64(2000))
	c.Check(newCoins.GetCoin(BTCAsset).Amount.Uint64(), Equals, uint64(1000))
}
