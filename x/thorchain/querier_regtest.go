//go:build regtest
// +build regtest

package thorchain

import "gitlab.com/thorchain/thornode/common/cosmos"

func init() {
	initManager = func(mgr *Mgrs, ctx cosmos.Context) {
		_ = mgr.BeginBlock(ctx)
	}
}
