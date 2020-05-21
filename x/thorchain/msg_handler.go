package thorchain

import (
	"github.com/blang/semver"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// MsgHandler is an interface expect all handler to implement
type MsgHandler interface {
	Run(ctx cosmos.Context, msg cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error)
}
