//go:build !testnet && !stagenet
// +build !testnet,!stagenet

package thorchain

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func migrateStoreV80(ctx cosmos.Context, mgr Manager) {}
