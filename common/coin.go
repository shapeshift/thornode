package common

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

// THORChainDecimals indicate the number of decimal points used in THORChain
const THORChainDecimals = 8

// NoCoin is empty Coin
var NoCoin = Coin{
	Asset:  EmptyAsset,
	Amount: cosmos.ZeroUint(),
}

// Coins represent a slice of Coin
type Coins []Coin

// NewCoin return a new instance of Coin
func NewCoin(asset Asset, amount cosmos.Uint) Coin {
	return Coin{
		Asset:  asset,
		Amount: amount,
	}
}

// NewCoins create a new Coins structure
func NewCoins(coins ...Coin) Coins {
	result := make(Coins, len(coins))
	for i, c := range coins {
		result[i] = c
	}
	return result
}

// Equals compare two coins to see whether they represent the same information
func (c Coin) Equals(cc Coin) bool {
	if !c.Asset.Equals(cc.Asset) {
		return false
	}
	if !c.Amount.Equal(cc.Amount) {
		return false
	}
	return true
}

// IsEmpty check whether asset is empty and also amount is zero
func (c Coin) IsEmpty() bool {
	if c.Asset.IsEmpty() {
		return true
	}
	if c.Amount.IsZero() {
		return true
	}
	return false
}

// Valid return an error if the coin is not correct
func (c Coin) Valid() error {
	if c.Asset.IsEmpty() {
		return errors.New("Denom cannot be empty")
	}
	if c.Amount.IsZero() {
		return errors.New("Amount cannot be zero")
	}

	return nil
}

// IsNative check whether the coin is native on THORChain
func (c Coin) IsNative() bool {
	return c.Asset.GetChain().Equals(THORChain)
}

// Native create a new instance of cosmos.Coin
func (c Coin) Native() (cosmos.Coin, error) {
	if !c.IsNative() {
		return cosmos.Coin{}, errors.New("coin is not on thorchain")
	}
	return cosmos.NewCoin(
		c.Asset.Native(),
		cosmos.NewIntFromBigInt(c.Amount.BigInt()),
	), nil
}

// String implement fmt.Stringer
func (c Coin) String() string {
	return fmt.Sprintf("%s %s", c.Amount.String(), c.Asset.String())
}

// WithDecimals update coin with a decimal
func (c Coin) WithDecimals(decimal int64) Coin {
	c.Decimals = decimal
	return c
}

// Valid check whether all the coins are valid , if not , then return an error
func (cs Coins) Valid() error {
	for _, coin := range cs {
		if err := coin.Valid(); err != nil {
			return err
		}
	}
	return nil
}

// Equals Check if two lists of coins are equal to each other. Order does not matter
func (cs1 Coins) Equals(cs2 Coins) bool {
	if len(cs1) != len(cs2) {
		return false
	}

	// sort both lists
	sort.Slice(cs1[:], func(i, j int) bool {
		return cs1[i].Asset.String() < cs1[j].Asset.String()
	})
	sort.Slice(cs2[:], func(i, j int) bool {
		return cs2[i].Asset.String() < cs2[j].Asset.String()
	})

	for i := range cs1 {
		if !cs1[i].Equals(cs2[i]) {
			return false
		}
	}

	return true
}

func (cs Coins) IsEmpty() bool {
	if len(cs) == 0 {
		return true
	}
	for _, coin := range cs {
		if !coin.IsEmpty() {
			return false
		}
	}
	return true
}

func (cs Coins) Native() (cosmos.Coins, error) {
	var err error
	coins := make(cosmos.Coins, len(cs))
	for i, coin := range cs {
		coins[i], err = coin.Native()
		if err != nil {
			return nil, err
		}
	}
	return coins, nil
}

// String implement fmt.Stringer
func (cs Coins) String() string {
	coins := make([]string, len(cs))
	for i, c := range cs {
		coins[i] = c.String()
	}
	return strings.Join(coins, ", ")
}

// Contains check whether the given coin is in the list
func (cs Coins) Contains(c Coin) bool {
	for _, item := range cs {
		if c.Equals(item) {
			return true
		}
	}
	return false
}

// GetCoin gets a specific coin by asset. Assumes there is only one of this coin in the
// list.
func (cs Coins) GetCoin(asset Asset) Coin {
	for _, item := range cs {
		if asset.Equals(item.Asset) {
			return item
		}
	}
	return NoCoin
}

// Distinct return a new Coins ,which duplicated coins had been removed
func (cs Coins) Distinct() Coins {
	newCoins := Coins{}
	for _, item := range cs {
		if !newCoins.Contains(item) {
			newCoins = append(newCoins, item)
		}
	}
	return newCoins
}

func (cs Coins) Add(coin Coin) Coins {
	for i, c := range cs {
		if c.Asset.Equals(coin.Asset) {
			cs[i].Amount = cs[i].Amount.Add(coin.Amount)
			return cs
		}
	}

	return append(cs, coin)
}

func (cs Coins) Adds(coins Coins) Coins {
	for _, c := range coins {
		cs = cs.Add(c)
	}
	return cs
}

// HasNoneNativeRune check whether the coins contains NoneNativeRUNE
//  At the moment, there are BEP2 RUNE, ERC20 RUNE and Native RUNE
func (cs Coins) HasNoneNativeRune() bool {
	for _, c := range cs {
		if c.Asset.IsRune() && !c.IsNative() {
			return true
		}
	}
	return false
}

// NoneEmpty return a new Coins , which ignore the coin that is empty
// either Coin asset is empty or amount is empty
func (cs Coins) NoneEmpty() Coins {
	newCoins := Coins{}
	for _, item := range cs {
		if item.IsEmpty() {
			continue
		}
		newCoins = append(newCoins, item)
	}
	return newCoins
}
