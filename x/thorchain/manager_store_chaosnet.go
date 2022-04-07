//go:build !testnet && !stagenet
// +build !testnet,!stagenet

package thorchain

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func migrateStoreV86(ctx cosmos.Context, mgr *Mgrs) {}
