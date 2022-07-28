//go:build !testnet && !stagenet
// +build !testnet,!stagenet

package thorchain

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func migrateStoreV86(ctx cosmos.Context, mgr *Mgrs) {}

func importPreRegistrationTHORNames(ctx cosmos.Context, mgr Manager) error {
	oneYear := fetchConfigInt64(ctx, mgr, constants.BlocksPerYear)
	names, err := getPreRegisterTHORNames(ctx, ctx.BlockHeight()+oneYear)
	if err != nil {
		return err
	}

	for _, name := range names {
		mgr.Keeper().SetTHORName(ctx, name)
	}
	return nil
}

func migrateStoreV88(ctx cosmos.Context, mgr Manager) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v88", "error", err)
		}
	}()

	err := importPreRegistrationTHORNames(ctx, mgr)
	if err != nil {
		ctx.Logger().Error("fail to migrate store to v88", "error", err)
	}
}
