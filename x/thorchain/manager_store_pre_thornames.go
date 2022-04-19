package thorchain

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
)

//go:embed preregister_thornames.json
var preregisterTHORNames []byte

type PreRegisterTHORName struct {
	Name    string
	Address string
}

func getPreRegisterTHORNames(blockheight int64) ([]THORName, error) {
	var register []PreRegisterTHORName
	if err := json.Unmarshal(preregisterTHORNames, &register); err != nil {
		return nil, fmt.Errorf("fail to load preregistation thorname list,err: %w", err)
	}

	names := make([]THORName, 0)
	for _, reg := range register {
		addr, err := common.NewAddress(reg.Address)
		if err != nil {
			return nil, err
		}
		names = append(names, NewTHORName(reg.Name, blockheight, []THORNameAlias{THORNameAlias{Chain: common.THORChain, Address: addr}}))
	}
	return names, nil
}
