package thorchain

import (
	"fmt"

	"github.com/blang/semver"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	keeper "gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type Manager interface {
	GasMgr() GasManager
	EventMgr() EventManager
	TxOutStore() TxOutStore
	VaultMgr() VaultManager
	ValidatorMgr() ValidatorManager
	ObMgr() ObserverManager
	SwapQ() SwapQueue
	Slasher() Slasher
	YggManager() YggManager
}

// GasManager define all the methods required to manage gas
type GasManager interface {
	BeginBlock()
	EndBlock(ctx cosmos.Context, keeper keeper.Keeper, eventManager EventManager)
	AddGasAsset(gas common.Gas)
	ProcessGas(ctx cosmos.Context, keeper keeper.Keeper)
	GetGas() common.Gas
	GetFee(ctx cosmos.Context, chain common.Chain) int64
}

// EventManager define methods need to be support to manage events
type EventManager interface {
	EmitPoolEvent(ctx cosmos.Context, poolEvt EventPool) error
	EmitErrataEvent(ctx cosmos.Context, errataEvent EventErrata) error
	EmitGasEvent(ctx cosmos.Context, gasEvent *EventGas) error
	EmitStakeEvent(ctx cosmos.Context, stakeEvent EventStake) error
	EmitRewardEvent(ctx cosmos.Context, rewardEvt EventRewards) error
	EmitReserveEvent(ctx cosmos.Context, reserveEvent EventReserve) error
	EmitUnstakeEvent(ctx cosmos.Context, unstakeEvt EventUnstake) error
	EmitSwapEvent(ctx cosmos.Context, swap EventSwap) error
	EmitRefundEvent(ctx cosmos.Context, refundEvt EventRefund) error
	EmitBondEvent(ctx cosmos.Context, bondEvent EventBond) error
	EmitAddEvent(ctx cosmos.Context, addEvt EventAdd) error
	EmitFeeEvent(ctx cosmos.Context, feeEvent EventFee) error
	EmitSlashEvent(ctx cosmos.Context, slashEvt EventSlash) error
	EmitOutboundEvent(ctx cosmos.Context, outbound EventOutbound) error
}

type TxOutStore interface {
	GetBlockOut(ctx cosmos.Context) (*TxOut, error)
	ClearOutboundItems(ctx cosmos.Context)
	GetOutboundItems(ctx cosmos.Context) ([]*TxOutItem, error)
	TryAddTxOutItem(ctx cosmos.Context, mgr Manager, toi *TxOutItem) (bool, error)
	UnSafeAddTxOutItem(ctx cosmos.Context, mgr Manager, toi *TxOutItem) error
	GetOutboundItemByToAddress(_ cosmos.Context, _ common.Address) []TxOutItem
}

type ObserverManager interface {
	BeginBlock()
	EndBlock(ctx cosmos.Context, keeper keeper.Keeper)
	AppendObserver(chain common.Chain, addrs []cosmos.AccAddress)
	List() []cosmos.AccAddress
}

type ValidatorManager interface {
	BeginBlock(ctx cosmos.Context, constAccessor constants.ConstantValues) error
	EndBlock(ctx cosmos.Context, mgr Manager, constAccessor constants.ConstantValues) []abci.ValidatorUpdate
	RequestYggReturn(ctx cosmos.Context, node NodeAccount, mgr Manager) error
	processRagnarok(ctx cosmos.Context, mgr Manager, constAccessor constants.ConstantValues) error
}

// VaultManager interface define the contract of Vault Manager
type VaultManager interface {
	TriggerKeygen(ctx cosmos.Context, nas NodeAccounts) error
	RotateVault(ctx cosmos.Context, vault Vault) error
	EndBlock(ctx cosmos.Context, mgr Manager, constAccessor constants.ConstantValues) error
	UpdateVaultData(ctx cosmos.Context, constAccessor constants.ConstantValues, gasManager GasManager, eventMgr EventManager) error
}

// SwapQueue interface define the contract of Swap Queue
type SwapQueue interface {
	EndBlock(ctx cosmos.Context, mgr Manager, version semver.Version, constAccessor constants.ConstantValues) error
}

type Slasher interface {
	BeginBlock(ctx cosmos.Context, req abci.RequestBeginBlock, constAccessor constants.ConstantValues)
	HandleDoubleSign(ctx cosmos.Context, addr crypto.Address, infractionHeight int64, constAccessor constants.ConstantValues) error
	LackObserving(ctx cosmos.Context, constAccessor constants.ConstantValues) error
	LackSigning(ctx cosmos.Context, constAccessor constants.ConstantValues, mgr Manager) error
	SlashNodeAccount(ctx cosmos.Context, observedPubKey common.PubKey, asset common.Asset, slashAmount cosmos.Uint, mgr Manager) error
	IncSlashPoints(ctx cosmos.Context, point int64, addresses ...cosmos.AccAddress)
	DecSlashPoints(ctx cosmos.Context, point int64, addresses ...cosmos.AccAddress)
}

type YggManager interface {
	Fund(ctx cosmos.Context, mgr Manager, constAccessor constants.ConstantValues) error
}

type Mgrs struct {
	CurrentVersion semver.Version
	gasMgr         GasManager
	eventMgr       EventManager
	txOutStore     TxOutStore
	vaultMgr       VaultManager
	validatorMgr   ValidatorManager
	obMgr          ObserverManager
	swapQ          SwapQueue
	slasher        Slasher
	yggManager     YggManager
	Keeper         keeper.Keeper
}

func NewManagers(keeper keeper.Keeper) *Mgrs {
	return &Mgrs{
		Keeper: keeper,
	}
}

// BeginBlock detect whether there are new version available, if it is available then create a new version of Mgr
func (mgr *Mgrs) BeginBlock(ctx cosmos.Context) error {
	v := mgr.Keeper.GetLowestActiveVersion(ctx)
	if v.Equals(mgr.CurrentVersion) {
		return nil
	}
	// version is different , thus all the manager need to re-create
	mgr.CurrentVersion = v
	var err error
	mgr.gasMgr, err = GetGasManager(v, mgr.Keeper)
	if err != nil {
		return fmt.Errorf("fail to create gas manager: %w", err)
	}
	mgr.eventMgr, err = GetEventManager(v)
	if err != nil {
		return fmt.Errorf("fail to get event manager: %w", err)
	}
	mgr.txOutStore, err = GetTxOutStore(mgr.Keeper, v, mgr.eventMgr, mgr.gasMgr)
	if err != nil {
		return fmt.Errorf("fail to get tx out store: %w", err)
	}

	mgr.vaultMgr, err = GetVaultManager(mgr.Keeper, v, mgr.txOutStore, mgr.eventMgr)
	if err != nil {
		return fmt.Errorf("fail to get vault manager: %w", err)
	}

	mgr.validatorMgr, err = GetValidatorManager(mgr.Keeper, v, mgr.vaultMgr, mgr.txOutStore, mgr.eventMgr)
	if err != nil {
		return fmt.Errorf("fail to get validator manager: %w", err)
	}

	mgr.obMgr, err = GetObserverManager(v)
	if err != nil {
		return fmt.Errorf("fail to get observer manager: %w", err)
	}

	mgr.swapQ, err = GetSwapQueue(mgr.Keeper, v)
	if err != nil {
		return fmt.Errorf("fail to create swap queue: %w", err)
	}

	mgr.slasher, err = GetSlasher(mgr.Keeper, v)
	if err != nil {
		return fmt.Errorf("fail to create swap queue: %w", err)
	}

	mgr.yggManager, err = GetYggManager(mgr.Keeper, v)
	if err != nil {
		return fmt.Errorf("fail to create swap queue: %w", err)
	}
	return nil
}

func (m *Mgrs) GasMgr() GasManager             { return m.gasMgr }
func (m *Mgrs) EventMgr() EventManager         { return m.eventMgr }
func (m *Mgrs) TxOutStore() TxOutStore         { return m.txOutStore }
func (m *Mgrs) VaultMgr() VaultManager         { return m.vaultMgr }
func (m *Mgrs) ValidatorMgr() ValidatorManager { return m.validatorMgr }
func (m *Mgrs) ObMgr() ObserverManager         { return m.obMgr }
func (m *Mgrs) SwapQ() SwapQueue               { return m.swapQ }
func (m *Mgrs) Slasher() Slasher               { return m.slasher }
func (m *Mgrs) YggManager() YggManager         { return m.yggManager }

func GetGasManager(version semver.Version, keeper keeper.Keeper) (GasManager, error) {
	constAcessor := constants.GetConstantValues(version)
	if version.GTE(semver.MustParse("0.1.0")) {
		return NewGasMgrV1(constAcessor, keeper), nil
	}
	return nil, errInvalidVersion
}

func GetEventManager(version semver.Version) (EventManager, error) {
	if version.GTE(semver.MustParse("0.1.0")) {
		return NewEventMgrV1(), nil
	}
	return nil, errInvalidVersion
}

// GetTxOutStore will return an implementation of the txout store that
func GetTxOutStore(keeper keeper.Keeper, version semver.Version, eventMgr EventManager, gasManager GasManager) (TxOutStore, error) {
	constAcessor := constants.GetConstantValues(version)
	if version.GTE(semver.MustParse("0.1.0")) {
		return NewTxOutStorageV1(keeper, constAcessor, eventMgr, gasManager), nil
	}
	return nil, errInvalidVersion
}

// GetVaultManager retrieve a VaultManager that is compatible with the given version
func GetVaultManager(keeper keeper.Keeper, version semver.Version, txOutStore TxOutStore, eventMgr EventManager) (VaultManager, error) {
	if version.GTE(semver.MustParse("0.1.0")) {
		return NewVaultMgrV1(keeper, txOutStore, eventMgr), nil
	}
	return nil, errInvalidVersion
}

// GetValidatorManager create a new instance of Validator Manager
func GetValidatorManager(keeper keeper.Keeper, version semver.Version, vaultMgr VaultManager, txOutStore TxOutStore, eventMgr EventManager) (ValidatorManager, error) {
	if version.GTE(semver.MustParse("0.1.0")) {
		return NewValidatorMgrV1(keeper, vaultMgr, txOutStore, eventMgr), nil
	}
	return nil, errBadVersion
}

// GetObserverManager return an instance that implements ObserverManager interface
// when there is no version can match the given semver , it will return nil
func GetObserverManager(version semver.Version) (ObserverManager, error) {
	if version.GTE(semver.MustParse("0.1.0")) {
		return NewObserverMgrV1(), nil
	}
	return nil, errInvalidVersion
}

// GetSwapQueue retrieve a SwapQueue that is compatible with the given version
func GetSwapQueue(keeper keeper.Keeper, version semver.Version) (SwapQueue, error) {
	if version.GTE(semver.MustParse("0.1.0")) {
		return NewSwapQv1(keeper), nil
	}
	return nil, errInvalidVersion
}

func GetSlasher(keeper keeper.Keeper, version semver.Version) (Slasher, error) {
	if version.GTE(semver.MustParse("0.1.0")) {
		return NewSlasherV1(keeper), nil
	}
	return nil, errInvalidVersion
}

func GetYggManager(keeper keeper.Keeper, version semver.Version) (YggManager, error) {
	if version.GTE(semver.MustParse("0.1.0")) {
		return NewYggMgrV1(keeper), nil
	}
	return nil, errInvalidVersion
}
