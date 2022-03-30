package thorchain

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

func withdraw(ctx cosmos.Context, version semver.Version, msg MsgWithdrawLiquidity, manager Manager) (cosmos.Uint, cosmos.Uint, cosmos.Uint, cosmos.Uint, cosmos.Uint, error) {
	if version.GTE(semver.MustParse("1.84.0")) {
		return withdrawV84(ctx, version, msg, manager)
	} else if version.GTE(semver.MustParse("0.76.0")) {
		return withdrawV76(ctx, version, msg, manager)
	}
	zero := cosmos.ZeroUint()
	return zero, zero, zero, zero, zero, errInvalidVersion
}
