package thorchain

import (
	"github.com/blang/semver"
	abci "github.com/tendermint/tendermint/abci/types"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type VersionedValidatorDummyMgr struct {
}

func NewVersionedValidatorDummyMgr() VersionedValidatorDummyMgr {
	return VersionedValidatorDummyMgr{}
}

func (VersionedValidatorDummyMgr) BeginBlock(ctx cosmos.Context, version semver.Version, constAccessor constants.ConstantValues) error {
	return nil
}

func (VersionedValidatorDummyMgr) EndBlock(ctx cosmos.Context, version semver.Version, constAccessor constants.ConstantValues) []abci.ValidatorUpdate {
	return nil
}

func (VersionedValidatorDummyMgr) RequestYggReturn(ctx cosmos.Context, version semver.Version, node NodeAccount) error {
	return nil
}

// ValidatorDummyMgr is to manage a list of validators , and rotate them
type ValidatorDummyMgr struct {
}

// NewValidatorDummyMgr create a new instance of ValidatorDummyMgr
func NewValidatorDummyMgr() *ValidatorDummyMgr {
	return &ValidatorDummyMgr{}
}

func (vm *ValidatorDummyMgr) BeginBlock(_ cosmos.Context) error { return kaboom }
func (vm *ValidatorDummyMgr) EndBlock(_ cosmos.Context, _ constants.ConstantValues) []abci.ValidatorUpdate {
	return nil
}

func (vm *ValidatorDummyMgr) RequestYggReturn(_ cosmos.Context, _ NodeAccount, _ TxOutStore) error {
	return kaboom
}
