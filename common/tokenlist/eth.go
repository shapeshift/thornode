package tokenlist

import (
	"encoding/json"
	"time"

	"github.com/blang/semver"
)

var (
	ethTokenListV93 ETHTokenList
	ethTokenListV95 ETHTokenList
)

// ERC20Token is a struct to represent the token
type ERC20Token struct {
	Address  string `json:"address"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Decimals int    `json:"decimals"`
}

type ETHTokenList struct {
	Name      string       `json:"name"`
	LogoURI   string       `json:"logoURI"`
	Tokens    []ERC20Token `json:"tokens"`
	Keywords  []string     `json:"keywords"`
	Timestamp time.Time    `json:"timestamp"`
}

func init() {
	if err := json.Unmarshal(ethTokenListRawV93, &ethTokenListV93); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(ethTokenListRawV95, &ethTokenListV95); err != nil {
		panic(err)
	}
}

func GetETHTokenList(version semver.Version) ETHTokenList {
	switch {
	case version.GTE(semver.MustParse("1.95.0")):
		return ethTokenListV95
	case version.GTE(semver.MustParse("1.93.0")):
		return ethTokenListV93
	default:
		return ethTokenListV93
	}
}
