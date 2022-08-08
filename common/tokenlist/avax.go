package tokenlist

import (
	"encoding/json"

	"github.com/blang/semver"
	"gitlab.com/thorchain/thornode/common/tokenlist/avaxtokens"
)

var avaxTokenListV95 EVMTokenList

func init() {
	if err := json.Unmarshal(avaxtokens.AVAXTokenListRawV95, &avaxTokenListV95); err != nil {
		panic(err)
	}
}

func GetAVAXTokenList(version semver.Version) EVMTokenList {
	switch {
	case version.GTE(semver.MustParse("1.95.0")):
		return avaxTokenListV95
	default:
		return avaxTokenListV95
	}
}
