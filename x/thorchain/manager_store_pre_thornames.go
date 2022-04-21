package thorchain

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
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
		fmt.Printf("Err3: %s\n", err)
		return nil, fmt.Errorf("fail to load preregistation thorname list,err: %w", err)
	}

	names := make([]THORName, 0)
	for _, reg := range register {
		addr, err := common.NewAddress(reg.Address)
		if err != nil {
			fmt.Printf("Err1: %s\n", err)
			continue
		}
		name := NewTHORName(reg.Name, blockheight, []THORNameAlias{{Chain: common.THORChain, Address: addr}})
		acc, err := cosmos.AccAddressFromBech32(reg.Address)
		if err != nil {
			fmt.Printf("Err2: %s\n", err)
			continue
		}
		name.Owner = acc
		names = append(names, name)
	}
	return names, nil
}
