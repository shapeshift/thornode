package thorchain

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

func withdraw(ctx cosmos.Context, msg MsgWithdrawLiquidity, mgr Manager) (cosmos.Uint, cosmos.Uint, cosmos.Uint, cosmos.Uint, cosmos.Uint, error) {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.91.0")):
		return withdrawV91(ctx, msg, mgr)
	case version.GTE(semver.MustParse("1.89.0")):
		return withdrawV89(ctx, msg, mgr)
	case version.GTE(semver.MustParse("1.84.0")):
		return withdrawV84(ctx, msg, mgr)
	case version.GTE(semver.MustParse("0.76.0")):
		return withdrawV76(ctx, msg, mgr)
	}
	zero := cosmos.ZeroUint()
	return zero, zero, zero, zero, zero, errInvalidVersion
}
