package keeper

import (
	"errors"
	"fmt"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/simapp"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/tendermint/tendermint/libs/log"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	kvTypes "gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
)

var kaboom = errors.New("Kaboom!!!")

type KVStoreDummy struct{}

func (k KVStoreDummy) Cdc() codec.BinaryCodec                  { return simapp.MakeTestEncodingConfig().Marshaler }
func (k KVStoreDummy) CoinKeeper() bankkeeper.Keeper           { return bankkeeper.BaseKeeper{} }
func (k KVStoreDummy) AccountKeeper() authkeeper.AccountKeeper { return authkeeper.AccountKeeper{} }
func (k KVStoreDummy) Logger(ctx cosmos.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", ModuleName))
}

func (k KVStoreDummy) Version() semver.Version { return semver.MustParse("1.0.0") }
func (k KVStoreDummy) GetKey(_ cosmos.Context, prefix kvTypes.DbPrefix, key string) string {
	return fmt.Sprintf("%s/1/%s", prefix, key)
}

func (k KVStoreDummy) GetStoreVersion(ctx cosmos.Context) int64      { return 1 }
func (k KVStoreDummy) SetStoreVersion(ctx cosmos.Context, ver int64) {}

func (k KVStoreDummy) GetRuneBalanceOfModule(ctx cosmos.Context, moduleName string) cosmos.Uint {
	return cosmos.ZeroUint()
}

func (k KVStoreDummy) SendFromModuleToModule(ctx cosmos.Context, from, to string, coins common.Coins) error {
	return kaboom
}

func (k KVStoreDummy) SendCoins(ctx cosmos.Context, from, to cosmos.AccAddress, coins cosmos.Coins) error {
	return kaboom
}

func (k KVStoreDummy) AddCoins(ctx cosmos.Context, _ cosmos.AccAddress, coins cosmos.Coins) error {
	return kaboom
}

func (k KVStoreDummy) SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coins common.Coins) error {
	return kaboom
}

func (k KVStoreDummy) SendFromModuleToAccount(ctx cosmos.Context, from string, to cosmos.AccAddress, coins common.Coins) error {
	return kaboom
}

func (k KVStoreDummy) MintToModule(ctx cosmos.Context, module string, coin common.Coin) error {
	return kaboom
}

func (k KVStoreDummy) BurnFromModule(ctx cosmos.Context, module string, coin common.Coin) error {
	return kaboom
}

func (k KVStoreDummy) MintAndSendToAccount(ctx cosmos.Context, to cosmos.AccAddress, coin common.Coin) error {
	return kaboom
}

func (k KVStoreDummy) GetModuleAddress(module string) (common.Address, error) {
	return "", kaboom
}

func (k KVStoreDummy) GetModuleAccAddress(module string) cosmos.AccAddress {
	return nil
}

func (k KVStoreDummy) GetAccount(ctx cosmos.Context, addr cosmos.AccAddress) cosmos.Account {
	return nil
}

func (k KVStoreDummy) GetBalance(ctx cosmos.Context, addr cosmos.AccAddress) cosmos.Coins {
	return nil
}

func (k KVStoreDummy) HasCoins(ctx cosmos.Context, addr cosmos.AccAddress, coins cosmos.Coins) bool {
	return false
}

func (k KVStoreDummy) SetLastSignedHeight(_ cosmos.Context, _ int64) error { return kaboom }
func (k KVStoreDummy) GetLastSignedHeight(_ cosmos.Context) (int64, error) {
	return 0, kaboom
}

func (k KVStoreDummy) SetLastChainHeight(_ cosmos.Context, _ common.Chain, _ int64) error {
	return kaboom
}

func (k KVStoreDummy) GetLastChainHeight(_ cosmos.Context, _ common.Chain) (int64, error) {
	return 0, kaboom
}

func (k KVStoreDummy) GetLastChainHeights(ctx cosmos.Context) (map[common.Chain]int64, error) {
	return nil, kaboom
}

func (k KVStoreDummy) GetRagnarokBlockHeight(_ cosmos.Context) (int64, error) {
	return 0, kaboom
}
func (k KVStoreDummy) SetRagnarokBlockHeight(_ cosmos.Context, _ int64) {}
func (k KVStoreDummy) GetRagnarokNth(_ cosmos.Context) (int64, error) {
	return 0, kaboom
}
func (k KVStoreDummy) SetRagnarokNth(_ cosmos.Context, _ int64) {}
func (k KVStoreDummy) GetRagnarokPending(_ cosmos.Context) (int64, error) {
	return 0, kaboom
}
func (k KVStoreDummy) SetRagnarokPending(_ cosmos.Context, _ int64) {}
func (k KVStoreDummy) RagnarokInProgress(_ cosmos.Context) bool     { return false }
func (k KVStoreDummy) GetRagnarokWithdrawPosition(ctx cosmos.Context) (RagnarokWithdrawPosition, error) {
	return RagnarokWithdrawPosition{}, kaboom
}
func (k KVStoreDummy) SetRagnarokWithdrawPosition(_tx cosmos.Context, _ RagnarokWithdrawPosition) {}

func (k KVStoreDummy) GetPoolBalances(_ cosmos.Context, _, _ common.Asset) (cosmos.Uint, cosmos.Uint) {
	return cosmos.ZeroUint(), cosmos.ZeroUint()
}

func (k KVStoreDummy) GetPoolIterator(_ cosmos.Context) cosmos.Iterator {
	return NewDummyIterator()
}
func (k KVStoreDummy) SetPoolData(_ cosmos.Context, _ common.Asset, _ PoolStatus) {}
func (k KVStoreDummy) GetPoolDataIterator(_ cosmos.Context) cosmos.Iterator {
	return NewDummyIterator()
}
func (k KVStoreDummy) EnableAPool(_ cosmos.Context) {}

func (k KVStoreDummy) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
	return Pool{}, kaboom
}
func (k KVStoreDummy) GetPools(_ cosmos.Context) (Pools, error)        { return nil, kaboom }
func (k KVStoreDummy) SetPool(_ cosmos.Context, _ Pool) error          { return kaboom }
func (k KVStoreDummy) PoolExist(_ cosmos.Context, _ common.Asset) bool { return false }
func (k KVStoreDummy) RemovePool(_ cosmos.Context, _ common.Asset)     {}
func (k KVStoreDummy) GetLiquidityProviderIterator(_ cosmos.Context, _ common.Asset) cosmos.Iterator {
	return nil
}

func (k KVStoreDummy) GetLiquidityProvider(_ cosmos.Context, _ common.Asset, _ common.Address) (LiquidityProvider, error) {
	return LiquidityProvider{}, kaboom
}
func (k KVStoreDummy) SetLiquidityProvider(_ cosmos.Context, _ LiquidityProvider)    {}
func (k KVStoreDummy) RemoveLiquidityProvider(_ cosmos.Context, _ LiquidityProvider) {}
func (k KVStoreDummy) GetTotalSupply(ctx cosmos.Context, asset common.Asset) cosmos.Uint {
	return cosmos.ZeroUint()
}

func (k KVStoreDummy) TotalActiveValidators(_ cosmos.Context) (int, error) { return 0, kaboom }
func (k KVStoreDummy) ListValidatorsWithBond(_ cosmos.Context) (NodeAccounts, error) {
	return nil, kaboom
}

func (k KVStoreDummy) ListValidatorsByStatus(_ cosmos.Context, _ NodeStatus) (NodeAccounts, error) {
	return nil, kaboom
}

func (k KVStoreDummy) ListActiveValidators(_ cosmos.Context) (NodeAccounts, error) {
	return nil, kaboom
}

func (k KVStoreDummy) GetLowestActiveVersion(_ cosmos.Context) semver.Version {
	return semver.Version{
		Major: 0,
		Minor: 1,
		Patch: 0,
	}
}
func (k KVStoreDummy) GetMinJoinVersion(_ cosmos.Context) semver.Version   { return semver.Version{} }
func (k KVStoreDummy) GetMinJoinVersionV1(_ cosmos.Context) semver.Version { return semver.Version{} }
func (k KVStoreDummy) GetNodeAccount(_ cosmos.Context, _ cosmos.AccAddress) (NodeAccount, error) {
	return NodeAccount{}, kaboom
}

func (k KVStoreDummy) GetNodeAccountByPubKey(_ cosmos.Context, _ common.PubKey) (NodeAccount, error) {
	return NodeAccount{}, kaboom
}

func (k KVStoreDummy) SetNodeAccount(_ cosmos.Context, _ NodeAccount) error { return kaboom }
func (k KVStoreDummy) EnsureNodeKeysUnique(_ cosmos.Context, _ string, _ common.PubKeySet) error {
	return kaboom
}
func (k KVStoreDummy) GetNodeAccountIterator(_ cosmos.Context) cosmos.Iterator { return nil }

func (k KVStoreDummy) GetNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress) (int64, error) {
	return 0, kaboom
}
func (k KVStoreDummy) SetNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress, _ int64) {}
func (k KVStoreDummy) ResetNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress)        {}
func (k KVStoreDummy) IncNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress, _ int64) error {
	return kaboom
}

func (k KVStoreDummy) DecNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress, _ int64) error {
	return kaboom
}

func (k KVStoreDummy) GetNodeAccountJail(ctx cosmos.Context, addr cosmos.AccAddress) (Jail, error) {
	return Jail{}, kaboom
}

func (k KVStoreDummy) SetNodeAccountJail(ctx cosmos.Context, addr cosmos.AccAddress, height int64, reason string) error {
	return kaboom
}

func (k KVStoreDummy) ReleaseNodeAccountFromJail(ctx cosmos.Context, addr cosmos.AccAddress) error {
	return kaboom
}
func (k KVStoreDummy) SetBondProviders(ctx cosmos.Context, _ BondProviders) error { return kaboom }
func (k KVStoreDummy) GetBondProviders(ctx cosmos.Context, _ cosmos.AccAddress) (BondProviders, error) {
	return BondProviders{}, kaboom
}

func (k KVStoreDummy) GetObservingAddresses(_ cosmos.Context) ([]cosmos.AccAddress, error) {
	return nil, kaboom
}

func (k KVStoreDummy) AddObservingAddresses(_ cosmos.Context, _ []cosmos.AccAddress) error {
	return kaboom
}
func (k KVStoreDummy) ClearObservingAddresses(_ cosmos.Context)                      {}
func (k KVStoreDummy) SetObservedTxInVoter(_ cosmos.Context, _ ObservedTxVoter)      {}
func (k KVStoreDummy) GetObservedTxInVoterIterator(_ cosmos.Context) cosmos.Iterator { return nil }
func (k KVStoreDummy) GetObservedTxInVoter(_ cosmos.Context, _ common.TxID) (ObservedTxVoter, error) {
	return ObservedTxVoter{}, kaboom
}
func (k KVStoreDummy) SetObservedTxOutVoter(_ cosmos.Context, _ ObservedTxVoter)      {}
func (k KVStoreDummy) GetObservedTxOutVoterIterator(_ cosmos.Context) cosmos.Iterator { return nil }
func (k KVStoreDummy) GetObservedTxOutVoter(_ cosmos.Context, _ common.TxID) (ObservedTxVoter, error) {
	return ObservedTxVoter{}, kaboom
}
func (k KVStoreDummy) SetTssVoter(_ cosmos.Context, _ TssVoter)             {}
func (k KVStoreDummy) GetTssVoterIterator(_ cosmos.Context) cosmos.Iterator { return nil }
func (k KVStoreDummy) GetTssVoter(_ cosmos.Context, _ string) (TssVoter, error) {
	return TssVoter{}, kaboom
}

func (k KVStoreDummy) GetKeygenBlock(_ cosmos.Context, _ int64) (KeygenBlock, error) {
	return KeygenBlock{}, kaboom
}
func (k KVStoreDummy) SetKeygenBlock(_ cosmos.Context, _ KeygenBlock)          {}
func (k KVStoreDummy) GetKeygenBlockIterator(_ cosmos.Context) cosmos.Iterator { return nil }
func (k KVStoreDummy) GetTxOut(_ cosmos.Context, _ int64) (*TxOut, error)      { return nil, kaboom }
func (k KVStoreDummy) GetTxOutValue(_ cosmos.Context, _ int64) (cosmos.Uint, error) {
	return cosmos.ZeroUint(), kaboom
}
func (k KVStoreDummy) SetTxOut(_ cosmos.Context, _ *TxOut) error                { return kaboom }
func (k KVStoreDummy) AppendTxOut(_ cosmos.Context, _ int64, _ TxOutItem) error { return kaboom }
func (k KVStoreDummy) ClearTxOut(_ cosmos.Context, _ int64) error               { return kaboom }
func (k KVStoreDummy) GetTxOutIterator(_ cosmos.Context) cosmos.Iterator        { return nil }
func (k KVStoreDummy) AddToLiquidityFees(_ cosmos.Context, _ common.Asset, _ cosmos.Uint) error {
	return kaboom
}

func (k KVStoreDummy) GetTotalLiquidityFees(_ cosmos.Context, _ uint64) (cosmos.Uint, error) {
	return cosmos.ZeroUint(), kaboom
}

func (k KVStoreDummy) GetPoolLiquidityFees(_ cosmos.Context, _ uint64, _ common.Asset) (cosmos.Uint, error) {
	return cosmos.ZeroUint(), kaboom
}

func (k KVStoreDummy) GetChains(_ cosmos.Context) (common.Chains, error)  { return nil, kaboom }
func (k KVStoreDummy) SetChains(_ cosmos.Context, _ common.Chains)        {}
func (k KVStoreDummy) GetVaultIterator(_ cosmos.Context) cosmos.Iterator  { return nil }
func (k KVStoreDummy) VaultExists(_ cosmos.Context, _ common.PubKey) bool { return false }
func (k KVStoreDummy) FindPubKeyOfAddress(_ cosmos.Context, _ common.Address, _ common.Chain) (common.PubKey, error) {
	return common.EmptyPubKey, kaboom
}
func (k KVStoreDummy) SetVault(_ cosmos.Context, _ Vault) error { return kaboom }
func (k KVStoreDummy) GetVault(_ cosmos.Context, _ common.PubKey) (Vault, error) {
	return Vault{}, kaboom
}
func (k KVStoreDummy) GetAsgardVaults(_ cosmos.Context) (Vaults, error) { return nil, kaboom }
func (k KVStoreDummy) GetAsgardVaultsByStatus(_ cosmos.Context, _ VaultStatus) (Vaults, error) {
	return nil, kaboom
}
func (k KVStoreDummy) GetLeastSecure(_ cosmos.Context, _ Vaults, _ int64) Vault  { return Vault{} }
func (k KVStoreDummy) GetMostSecure(_ cosmos.Context, _ Vaults, _ int64) Vault   { return Vault{} }
func (k KVStoreDummy) SortBySecurity(_ cosmos.Context, _ Vaults, _ int64) Vaults { return nil }
func (k KVStoreDummy) DeleteVault(_ cosmos.Context, _ common.PubKey) error       { return kaboom }

func (k KVStoreDummy) HasValidVaultPools(_ cosmos.Context) (bool, error)     { return false, kaboom }
func (k KVStoreDummy) AddFeeToReserve(_ cosmos.Context, _ cosmos.Uint) error { return kaboom }
func (k KVStoreDummy) GetNetwork(_ cosmos.Context) (Network, error)          { return Network{}, kaboom }
func (k KVStoreDummy) SetNetwork(_ cosmos.Context, _ Network) error          { return kaboom }

func (k KVStoreDummy) SetTssKeysignFailVoter(_ cosmos.Context, tss TssKeysignFailVoter) {
}

func (k KVStoreDummy) GetTssKeysignFailVoterIterator(_ cosmos.Context) cosmos.Iterator {
	return nil
}

func (k KVStoreDummy) GetTssKeysignFailVoter(_ cosmos.Context, _ string) (TssKeysignFailVoter, error) {
	return TssKeysignFailVoter{}, kaboom
}

func (k KVStoreDummy) GetGas(_ cosmos.Context, _ common.Asset) ([]cosmos.Uint, error) {
	return nil, kaboom
}
func (k KVStoreDummy) SetGas(_ cosmos.Context, _ common.Asset, _ []cosmos.Uint) {}
func (k KVStoreDummy) GetGasIterator(ctx cosmos.Context) cosmos.Iterator        { return nil }

func (k KVStoreDummy) SetErrataTxVoter(_ cosmos.Context, _ ErrataTxVoter)        {}
func (k KVStoreDummy) GetErrataTxVoterIterator(_ cosmos.Context) cosmos.Iterator { return nil }
func (k KVStoreDummy) GetErrataTxVoter(_ cosmos.Context, _ common.TxID, _ common.Chain) (ErrataTxVoter, error) {
	return ErrataTxVoter{}, kaboom
}
func (k KVStoreDummy) SetBanVoter(_ cosmos.Context, _ BanVoter) {}
func (k KVStoreDummy) GetBanVoter(_ cosmos.Context, _ cosmos.AccAddress) (BanVoter, error) {
	return BanVoter{}, kaboom
}

func (k KVStoreDummy) GetBanVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	return nil
}
func (k KVStoreDummy) SetSwapQueueItem(ctx cosmos.Context, msg MsgSwap, i int) error { return kaboom }
func (k KVStoreDummy) GetSwapQueueIterator(ctx cosmos.Context) cosmos.Iterator       { return nil }
func (k KVStoreDummy) RemoveSwapQueueItem(ctx cosmos.Context, _ common.TxID, _ int)  {}
func (k KVStoreDummy) GetSwapQueueItem(ctx cosmos.Context, txID common.TxID, _ int) (MsgSwap, error) {
	return MsgSwap{}, kaboom
}
func (k KVStoreDummy) GetMimir(_ cosmos.Context, key string) (int64, error) { return 0, kaboom }
func (k KVStoreDummy) SetMimir(_ cosmos.Context, key string, value int64)   {}
func (k KVStoreDummy) SetNodeMimir(_ cosmos.Context, key string, value int64, acc cosmos.AccAddress) error {
	return kaboom
}
func (k KVStoreDummy) DeleteMimir(_ cosmos.Context, key string) error          { return kaboom }
func (k KVStoreDummy) GetMimirIterator(ctx cosmos.Context) cosmos.Iterator     { return nil }
func (k KVStoreDummy) GetNodeMimirIterator(ctx cosmos.Context) cosmos.Iterator { return nil }
func (k KVStoreDummy) GetNodePauseChain(ctx cosmos.Context, acc cosmos.AccAddress) int64 {
	return int64(-1)
}
func (k KVStoreDummy) SetNodePauseChain(ctx cosmos.Context, acc cosmos.AccAddress) {}

func (k KVStoreDummy) GetNetworkFee(ctx cosmos.Context, chain common.Chain) (NetworkFee, error) {
	return NetworkFee{}, kaboom
}

func (k KVStoreDummy) SaveNetworkFee(ctx cosmos.Context, chain common.Chain, networkFee NetworkFee) error {
	return kaboom
}

func (k KVStoreDummy) GetNetworkFeeIterator(ctx cosmos.Context) cosmos.Iterator {
	return nil
}

func (k KVStoreDummy) SetObservedNetworkFeeVoter(ctx cosmos.Context, networkFeeVoter ObservedNetworkFeeVoter) {
}

func (k KVStoreDummy) GetObservedNetworkFeeVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	return nil
}

func (k KVStoreDummy) GetObservedNetworkFeeVoter(ctx cosmos.Context, height int64, chain common.Chain, rate int64) (ObservedNetworkFeeVoter, error) {
	return ObservedNetworkFeeVoter{}, nil
}

func (k KVStoreDummy) SetLastObserveHeight(ctx cosmos.Context, chain common.Chain, address cosmos.AccAddress, height int64) error {
	return kaboom
}

func (k KVStoreDummy) GetLastObserveHeight(ctx cosmos.Context, address cosmos.AccAddress) (map[common.Chain]int64, error) {
	return nil, kaboom
}

func (k KVStoreDummy) SetTssKeygenMetric(_ cosmos.Context, metric *TssKeygenMetric) {
}

func (k KVStoreDummy) GetTssKeygenMetric(_ cosmos.Context, key common.PubKey) (*TssKeygenMetric, error) {
	return nil, kaboom
}

func (k KVStoreDummy) SetTssKeysignMetric(_ cosmos.Context, metric *TssKeysignMetric) {
}

func (k KVStoreDummy) GetTssKeysignMetric(_ cosmos.Context, txID common.TxID) (*TssKeysignMetric, error) {
	return nil, kaboom
}

func (k KVStoreDummy) GetLatestTssKeysignMetric(_ cosmos.Context) (*TssKeysignMetric, error) {
	return nil, kaboom
}
func (k KVStoreDummy) SetChainContract(ctx cosmos.Context, cc ChainContract) {}
func (k KVStoreDummy) GetChainContract(ctx cosmos.Context, chain common.Chain) (ChainContract, error) {
	return ChainContract{}, kaboom
}

func (k KVStoreDummy) GetChainContractIterator(ctx cosmos.Context) cosmos.Iterator {
	return nil
}

func (k KVStoreDummy) GetChainContracts(ctx cosmos.Context, chains common.Chains) []ChainContract {
	return nil
}
func (k KVStoreDummy) SetSolvencyVoter(_ cosmos.Context, _ SolvencyVoter) {}
func (k KVStoreDummy) GetSolvencyVoter(_ cosmos.Context, _ common.TxID, _ common.Chain) (SolvencyVoter, error) {
	return SolvencyVoter{}, kaboom
}

func (k KVStoreDummy) THORNameExists(ctx cosmos.Context, _ string) bool { return false }
func (k KVStoreDummy) GetTHORName(ctx cosmos.Context, _ string) (THORName, error) {
	return THORName{}, kaboom
}
func (k KVStoreDummy) SetTHORName(ctx cosmos.Context, name THORName)          {}
func (k KVStoreDummy) GetTHORNameIterator(ctx cosmos.Context) cosmos.Iterator { return nil }
func (k KVStoreDummy) DeleteTHORName(ctx cosmos.Context, _ string) error      { return kaboom }

// a mock cosmos.Iterator implementation for testing purposes
type DummyIterator struct {
	cosmos.Iterator
	placeholder int
	keys        [][]byte
	values      [][]byte
	err         error
}

func NewDummyIterator() *DummyIterator {
	return &DummyIterator{
		keys:   make([][]byte, 0),
		values: make([][]byte, 0),
	}
}

func (iter *DummyIterator) AddItem(key, value []byte) {
	iter.keys = append(iter.keys, key)
	iter.values = append(iter.values, value)
}

func (iter *DummyIterator) Next() {
	iter.placeholder++
}

func (iter *DummyIterator) Valid() bool {
	return iter.placeholder < len(iter.keys)
}

func (iter *DummyIterator) Key() []byte {
	return iter.keys[iter.placeholder]
}

func (iter *DummyIterator) Value() []byte {
	return iter.values[iter.placeholder]
}

func (iter *DummyIterator) Close() error {
	iter.placeholder = 0
	return nil
}

func (iter *DummyIterator) Error() error {
	return iter.err
}

func (iter *DummyIterator) Domain() (start, end []byte) {
	return nil, nil
}
