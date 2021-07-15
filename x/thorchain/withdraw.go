package thorchain

import (
	"github.com/blang/semver"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func withdraw(ctx cosmos.Context, version semver.Version, msg MsgWithdrawLiquidity, manager Manager) (cosmos.Uint, cosmos.Uint, cosmos.Uint, cosmos.Uint, cosmos.Uint, error) {
	if version.GTE(semver.MustParse("0.58.0")) {
		return withdrawV58(ctx, version, msg, manager)
	} else if version.GTE(semver.MustParse("0.55.0")) {
		return withdrawV55(ctx, version, msg, manager)
	} else if version.GTE(semver.MustParse("0.50.0")) {
		return withdrawV50(ctx, version, msg, manager)
	} else if version.GTE(semver.MustParse("0.49.0")) {
		return withdrawV49(ctx, version, msg, manager)
	} else if version.GTE(semver.MustParse("0.47.0")) {
		return withdrawV47(ctx, version, msg, manager)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return withdrawV1(ctx, version, msg, manager)
	}
	zero := cosmos.ZeroUint()
	return zero, zero, zero, zero, zero, errInvalidVersion
}