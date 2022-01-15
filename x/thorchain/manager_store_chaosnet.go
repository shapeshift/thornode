//go:build !testnet && !stagenet
// +build !testnet,!stagenet

package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func migrateStoreV86(ctx cosmos.Context, mgr *Mgrs) {}

func importPreRegistrationTHORNames(ctx cosmos.Context, mgr Manager) error {
	names, err := getPreRegisterTHORNames(common.BlockHeight(ctx) + 5256000)
	if err != nil {
		return err
	}

	for _, name := range names {
		mgr.Keeper().SetTHORName(ctx, name)
	}
	return nil
}
