package common

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

var (
	// EmptyAsset empty asset, not valid
	EmptyAsset = Asset{Chain: EmptyChain, Symbol: "", Ticker: ""}
	// BNBAsset BNB
	BNBAsset = Asset{Chain: BNBChain, Symbol: "BNB", Ticker: "BNB"}
	// BTCAsset BTC
	BTCAsset = Asset{Chain: BTCChain, Symbol: "BTC", Ticker: "BTC"}
	// LTCAsset BTC
	LTCAsset = Asset{Chain: LTCChain, Symbol: "LTC", Ticker: "LTC"}
	// BCHAsset BCH
	BCHAsset = Asset{Chain: BCHChain, Symbol: "BCH", Ticker: "BCH"}
	// ETHAsset ETH
	ETHAsset = Asset{Chain: ETHChain, Symbol: "ETH", Ticker: "ETH"}
	// Rune67CAsset RUNE on Binance test net
	Rune67CAsset = Asset{Chain: BNBChain, Symbol: "RUNE-67C", Ticker: "RUNE"} // testnet asset on binance ganges
	// RuneB1AAsset RUNE on Binance main net
	RuneB1AAsset = Asset{Chain: BNBChain, Symbol: "RUNE-B1A", Ticker: "RUNE"} // mainnet
	// RuneNative RUNE on thorchain
	RuneNative      = Asset{Chain: THORChain, Symbol: "RUNE", Ticker: "RUNE"}
	RuneERC20Asset  = Asset{Chain: ETHChain, Symbol: "RUNE-0x3155ba85d5f96b2d030a4966af206230e46849cb", Ticker: "RUNE"}
	SyntheticPrefix = "thor"
)

// NewAsset parse the given input into Asset object
func NewAsset(input string) (Asset, error) {
	var err error
	var asset Asset
	var sym string
	parts := strings.SplitN(input, ".", 2)
	if len(parts) == 1 {
		asset.Chain = THORChain
		sym = parts[0]
	} else {
		asset.Chain, err = NewChain(parts[0])
		if err != nil {
			return EmptyAsset, err
		}
		sym = parts[1]
	}

	asset.Symbol, err = NewSymbol(sym)
	if err != nil {
		return EmptyAsset, err
	}

	parts = strings.SplitN(sym, "-", 2)
	asset.Ticker, err = NewTicker(parts[0])
	if err != nil {
		return EmptyAsset, err
	}

	return asset, nil
}

// Equals determinate whether two assets are equivalent
func (a Asset) Equals(a2 Asset) bool {
	return a.Chain.Equals(a2.Chain) && a.Symbol.Equals(a2.Symbol) && a.Ticker.Equals(a2.Ticker)
}

func (a Asset) isNativeUtilityAsset() bool {
	if !a.Chain.Equals(THORChain) {
		return false
	}
	if !strings.Contains(a.Symbol.String(), "/") {
		return false
	}
	return true
}

// Get layer1 asset version
func (a Asset) GetLayer1Asset() Asset {
	if !a.IsSyntheticAsset() {
		return a
	}
	parts := strings.Split(a.Symbol.String(), "/")
	chain := parts[0][len(SyntheticPrefix):len(parts[0])]
	return Asset{
		Chain:  Chain(chain),
		Symbol: Symbol(parts[1]),
		Ticker: a.Ticker,
	}
}

// Get synthetic asset of asset
func (a Asset) GetSyntheticAsset() Asset {
	if a.IsSyntheticAsset() {
		return a
	}
	return Asset{
		Chain:  THORChain,
		Symbol: Symbol(strings.ToLower(fmt.Sprintf("%s%s/%s", SyntheticPrefix, a.Chain, a.Symbol))),
		Ticker: a.Ticker,
	}
}

// Check if asset is a pegged asset
func (a Asset) IsSyntheticAsset() bool {
	if !a.isNativeUtilityAsset() {
		return false
	}
	if !strings.HasPrefix(strings.ToLower(a.Symbol.String()), SyntheticPrefix) {
		return false
	}
	return true
}

// Native return native asset, only relevant on THORChain
func (a Asset) Native() string {
	if !a.Chain.Equals(THORChain) {
		return ""
	}
	return strings.ToLower(a.Symbol.String())
}

// IsEmpty will be true when any of the field is empty, chain,symbol or ticker
func (a Asset) IsEmpty() bool {
	return a.Chain.IsEmpty() || a.Symbol.IsEmpty() || a.Ticker.IsEmpty()
}

// String implement fmt.Stringer , return the string representation of Asset
func (a Asset) String() string {
	return fmt.Sprintf("%s.%s", a.Chain.String(), a.Symbol.String())
}

// IsGasAsset check whether asset is base asset used to pay for gas
func (a Asset) IsGasAsset() bool {
	gasAsset := a.Chain.GetGasAsset()
	if gasAsset.IsEmpty() {
		return false
	}
	return a.Equals(gasAsset)
}

// IsRune is a helper function ,return true only when the asset represent RUNE
func (a Asset) IsRune() bool {
	return a.Equals(Rune67CAsset) || a.Equals(RuneB1AAsset) || a.Equals(RuneNative) || a.Equals(RuneERC20Asset)
}

// IsBNB is a helper function, return true only when the asset represent BNB
func (a Asset) IsBNB() bool {
	return a.Equals(BNBAsset)
}

// MarshalJSON implement Marshaler interface
func (a Asset) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

// UnmarshalJSON implement Unmarshaler interface
func (a *Asset) UnmarshalJSON(data []byte) error {
	var err error
	var assetStr string
	if err := json.Unmarshal(data, &assetStr); err != nil {
		return err
	}
	*a, err = NewAsset(assetStr)
	return err
}

// RuneAsset return RUNE Asset depends on different environment
func RuneAsset() Asset {
	return RuneNative
}

// BEP2RuneAsset is RUNE on BEP2
func BEP2RuneAsset() Asset {
	if strings.EqualFold(os.Getenv("NET"), "testnet") || strings.EqualFold(os.Getenv("NET"), "mocknet") {
		return Rune67CAsset
	}
	return RuneB1AAsset
}
