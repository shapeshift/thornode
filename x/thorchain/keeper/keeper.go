package keeper

import (
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/codec"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	kvTypes "gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
	kv1 "gitlab.com/thorchain/thornode/x/thorchain/keeper/v1"
)

type Keeper interface {
	Cdc() codec.BinaryCodec
	Version() semver.Version
	GetKey(ctx cosmos.Context, prefix kvTypes.DbPrefix, key string) string
	GetStoreVersion(ctx cosmos.Context) int64
	SetStoreVersion(ctx cosmos.Context, ver int64)
	GetRuneBalanceOfModule(ctx cosmos.Context, moduleName string) cosmos.Uint
	SendFromModuleToModule(ctx cosmos.Context, from, to string, coin common.Coins) error
	SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coin common.Coins) error
	SendFromModuleToAccount(ctx cosmos.Context, from string, to cosmos.AccAddress, coin common.Coins) error
	MintToModule(ctx cosmos.Context, module string, coin common.Coin) error
	BurnFromModule(ctx cosmos.Context, module string, coin common.Coin) error
	MintAndSendToAccount(ctx cosmos.Context, to cosmos.AccAddress, coin common.Coin) error
	GetModuleAddress(module string) (common.Address, error)
	GetModuleAccAddress(module string) cosmos.AccAddress
	GetBalance(ctx cosmos.Context, addr cosmos.AccAddress) cosmos.Coins
	HasCoins(ctx cosmos.Context, addr cosmos.AccAddress, coins cosmos.Coins) bool
	GetAccount(ctx cosmos.Context, addr cosmos.AccAddress) cosmos.Account

	// passthrough funcs
	SendCoins(ctx cosmos.Context, from, to cosmos.AccAddress, coins cosmos.Coins) error
	AddCoins(ctx cosmos.Context, addr cosmos.AccAddress, coins cosmos.Coins) error

	// Keeper Interfaces
	KeeperPool
	KeeperLastHeight
	KeeperLiquidityProvider
	KeeperNodeAccount
	KeeperObserver
	KeeperObservedTx
	KeeperTxOut
	KeeperLiquidityFees
	KeeperVault
	KeeperReserveContributors
	KeeperNetwork
	KeeperTss
	KeeperTssKeysignFail
	KeeperKeygen
	KeeperRagnarok
	KeeperErrataTx
	KeeperBanVoter
	KeeperSwapQueue
	KeeperMimir
	KeeperNetworkFee
	KeeperObservedNetworkFeeVoter
	KeeperChainContract
	KeeperSolvencyVoter
	KeeperTHORName
}

type KeeperPool interface {
	GetPoolIterator(ctx cosmos.Context) cosmos.Iterator
	GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error)
	GetPools(ctx cosmos.Context) (Pools, error)
	SetPool(ctx cosmos.Context, pool Pool) error
	PoolExist(ctx cosmos.Context, asset common.Asset) bool
	RemovePool(ctx cosmos.Context, asset common.Asset)
}

type KeeperLastHeight interface {
	SetLastSignedHeight(ctx cosmos.Context, height int64) error
	GetLastSignedHeight(ctx cosmos.Context) (int64, error)
	SetLastChainHeight(ctx cosmos.Context, chain common.Chain, height int64) error
	GetLastChainHeight(ctx cosmos.Context, chain common.Chain) (int64, error)
	GetLastChainHeights(ctx cosmos.Context) (map[common.Chain]int64, error)
	SetLastObserveHeight(ctx cosmos.Context, chain common.Chain, address cosmos.AccAddress, height int64) error
	GetLastObserveHeight(ctx cosmos.Context, address cosmos.AccAddress) (map[common.Chain]int64, error)
}

type KeeperLiquidityProvider interface {
	GetLiquidityProviderIterator(ctx cosmos.Context, _ common.Asset) cosmos.Iterator
	GetLiquidityProvider(ctx cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error)
	SetLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider)
	RemoveLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider)
	GetTotalSupply(ctx cosmos.Context, asset common.Asset) cosmos.Uint
}

type KeeperNodeAccount interface {
	TotalActiveValidators(ctx cosmos.Context) (int, error)
	ListValidatorsWithBond(ctx cosmos.Context) (NodeAccounts, error)
	ListValidatorsByStatus(ctx cosmos.Context, status NodeStatus) (NodeAccounts, error)
	ListActiveValidators(ctx cosmos.Context) (NodeAccounts, error)
	GetLowestActiveVersion(ctx cosmos.Context) semver.Version
	GetMinJoinVersion(ctx cosmos.Context) semver.Version
	GetNodeAccount(ctx cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error)
	GetNodeAccountByPubKey(ctx cosmos.Context, pk common.PubKey) (NodeAccount, error)
	SetNodeAccount(ctx cosmos.Context, na NodeAccount) error
	EnsureNodeKeysUnique(ctx cosmos.Context, consensusPubKey string, pubKeys common.PubKeySet) error
	GetNodeAccountIterator(ctx cosmos.Context) cosmos.Iterator
	GetNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress) (int64, error)
	SetNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress, _ int64)
	IncNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress, _ int64) error
	DecNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress, _ int64) error
	ResetNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress)
	GetNodeAccountJail(ctx cosmos.Context, addr cosmos.AccAddress) (Jail, error)
	SetNodeAccountJail(ctx cosmos.Context, addr cosmos.AccAddress, height int64, reason string) error
	ReleaseNodeAccountFromJail(ctx cosmos.Context, addr cosmos.AccAddress) error
	SetBondProviders(ctx cosmos.Context, _ BondProviders) error
	GetBondProviders(ctx cosmos.Context, add cosmos.AccAddress) (BondProviders, error)
}

type KeeperObserver interface {
	GetObservingAddresses(ctx cosmos.Context) ([]cosmos.AccAddress, error)
	AddObservingAddresses(ctx cosmos.Context, inAddresses []cosmos.AccAddress) error
	ClearObservingAddresses(ctx cosmos.Context)
}

type KeeperObservedTx interface {
	SetObservedTxInVoter(ctx cosmos.Context, tx ObservedTxVoter)
	GetObservedTxInVoterIterator(ctx cosmos.Context) cosmos.Iterator
	GetObservedTxInVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error)
	SetObservedTxOutVoter(ctx cosmos.Context, tx ObservedTxVoter)
	GetObservedTxOutVoterIterator(ctx cosmos.Context) cosmos.Iterator
	GetObservedTxOutVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error)
}

type KeeperTxOut interface {
	SetTxOut(ctx cosmos.Context, blockOut *TxOut) error
	AppendTxOut(ctx cosmos.Context, height int64, item TxOutItem) error
	ClearTxOut(ctx cosmos.Context, height int64) error
	GetTxOutIterator(ctx cosmos.Context) cosmos.Iterator
	GetTxOut(ctx cosmos.Context, height int64) (*TxOut, error)
	GetTxOutValue(ctx cosmos.Context, height int64) (cosmos.Uint, error)
}

type KeeperLiquidityFees interface {
	AddToLiquidityFees(ctx cosmos.Context, asset common.Asset, fee cosmos.Uint) error
	GetTotalLiquidityFees(ctx cosmos.Context, height uint64) (cosmos.Uint, error)
	GetPoolLiquidityFees(ctx cosmos.Context, height uint64, asset common.Asset) (cosmos.Uint, error)
}

type KeeperVault interface {
	GetVaultIterator(ctx cosmos.Context) cosmos.Iterator
	VaultExists(ctx cosmos.Context, pk common.PubKey) bool
	SetVault(ctx cosmos.Context, vault Vault) error
	GetVault(ctx cosmos.Context, pk common.PubKey) (Vault, error)
	HasValidVaultPools(ctx cosmos.Context) (bool, error)
	GetAsgardVaults(ctx cosmos.Context) (Vaults, error)
	GetAsgardVaultsByStatus(_ cosmos.Context, _ VaultStatus) (Vaults, error)
	GetLeastSecure(_ cosmos.Context, _ Vaults, _ int64) Vault
	GetMostSecure(_ cosmos.Context, _ Vaults, _ int64) Vault
	SortBySecurity(_ cosmos.Context, _ Vaults, _ int64) Vaults
	DeleteVault(ctx cosmos.Context, pk common.PubKey) error
}

type KeeperReserveContributors interface {
	AddFeeToReserve(ctx cosmos.Context, fee cosmos.Uint) error
}

// KeeperNetwork func to access network data in key value store
type KeeperNetwork interface {
	GetNetwork(ctx cosmos.Context) (Network, error)
	SetNetwork(ctx cosmos.Context, data Network) error
}

type KeeperTss interface {
	SetTssVoter(_ cosmos.Context, tss TssVoter)
	GetTssVoterIterator(_ cosmos.Context) cosmos.Iterator
	GetTssVoter(_ cosmos.Context, _ string) (TssVoter, error)
	SetTssKeygenMetric(_ cosmos.Context, metric *TssKeygenMetric)
	GetTssKeygenMetric(_ cosmos.Context, key common.PubKey) (*TssKeygenMetric, error)
	SetTssKeysignMetric(_ cosmos.Context, metric *TssKeysignMetric)
	GetTssKeysignMetric(_ cosmos.Context, txID common.TxID) (*TssKeysignMetric, error)
	GetLatestTssKeysignMetric(_ cosmos.Context) (*TssKeysignMetric, error)
}

type KeeperTssKeysignFail interface {
	SetTssKeysignFailVoter(_ cosmos.Context, tss TssKeysignFailVoter)
	GetTssKeysignFailVoterIterator(_ cosmos.Context) cosmos.Iterator
	GetTssKeysignFailVoter(_ cosmos.Context, _ string) (TssKeysignFailVoter, error)
}

type KeeperKeygen interface {
	SetKeygenBlock(ctx cosmos.Context, keygenBlock KeygenBlock)
	GetKeygenBlockIterator(ctx cosmos.Context) cosmos.Iterator
	GetKeygenBlock(ctx cosmos.Context, height int64) (KeygenBlock, error)
}

type KeeperBanVoter interface {
	SetBanVoter(_ cosmos.Context, _ BanVoter)
	GetBanVoter(_ cosmos.Context, _ cosmos.AccAddress) (BanVoter, error)
	GetBanVoterIterator(_ cosmos.Context) cosmos.Iterator
}

type KeeperRagnarok interface {
	RagnarokInProgress(_ cosmos.Context) bool
	GetRagnarokBlockHeight(_ cosmos.Context) (int64, error)
	SetRagnarokBlockHeight(_ cosmos.Context, _ int64)
	GetRagnarokNth(_ cosmos.Context) (int64, error)
	SetRagnarokNth(_ cosmos.Context, _ int64)
	GetRagnarokPending(_ cosmos.Context) (int64, error)
	SetRagnarokPending(_ cosmos.Context, _ int64)
	GetRagnarokWithdrawPosition(ctx cosmos.Context) (RagnarokWithdrawPosition, error)
	SetRagnarokWithdrawPosition(ctx cosmos.Context, position RagnarokWithdrawPosition)
}

type KeeperErrataTx interface {
	SetErrataTxVoter(_ cosmos.Context, _ ErrataTxVoter)
	GetErrataTxVoterIterator(_ cosmos.Context) cosmos.Iterator
	GetErrataTxVoter(_ cosmos.Context, _ common.TxID, _ common.Chain) (ErrataTxVoter, error)
}

type KeeperSwapQueue interface {
	SetSwapQueueItem(ctx cosmos.Context, msg MsgSwap, i int) error
	GetSwapQueueIterator(ctx cosmos.Context) cosmos.Iterator
	GetSwapQueueItem(ctx cosmos.Context, txID common.TxID, i int) (MsgSwap, error)
	RemoveSwapQueueItem(ctx cosmos.Context, txID common.TxID, i int)
}

type KeeperMimir interface {
	GetMimir(_ cosmos.Context, key string) (int64, error)
	SetMimir(_ cosmos.Context, key string, value int64)
	SetNodeMimir(_ cosmos.Context, key string, value int64, acc cosmos.AccAddress) error
	GetMimirIterator(ctx cosmos.Context) cosmos.Iterator
	GetNodeMimirIterator(ctx cosmos.Context) cosmos.Iterator
	DeleteMimir(_ cosmos.Context, key string) error
	GetNodePauseChain(ctx cosmos.Context, acc cosmos.AccAddress) int64
	SetNodePauseChain(ctx cosmos.Context, acc cosmos.AccAddress)
}

type KeeperNetworkFee interface {
	GetNetworkFee(ctx cosmos.Context, chain common.Chain) (NetworkFee, error)
	SaveNetworkFee(ctx cosmos.Context, chain common.Chain, networkFee NetworkFee) error
	GetNetworkFeeIterator(ctx cosmos.Context) cosmos.Iterator
}

type KeeperObservedNetworkFeeVoter interface {
	SetObservedNetworkFeeVoter(ctx cosmos.Context, networkFeeVoter ObservedNetworkFeeVoter)
	GetObservedNetworkFeeVoterIterator(ctx cosmos.Context) cosmos.Iterator
	GetObservedNetworkFeeVoter(ctx cosmos.Context, height int64, chain common.Chain) (ObservedNetworkFeeVoter, error)
	GetObservedNetworkFeeVoterV47(ctx cosmos.Context, height int64, chain common.Chain, rate int64) (ObservedNetworkFeeVoter, error)
}

type KeeperChainContract interface {
	SetChainContract(ctx cosmos.Context, cc ChainContract)
	GetChainContract(ctx cosmos.Context, chain common.Chain) (ChainContract, error)
	GetChainContracts(ctx cosmos.Context, chains common.Chains) []ChainContract
	GetChainContractIterator(ctx cosmos.Context) cosmos.Iterator
}

type KeeperSolvencyVoter interface {
	SetSolvencyVoter(_ cosmos.Context, _ SolvencyVoter)
	GetSolvencyVoter(_ cosmos.Context, _ common.TxID, _ common.Chain) (SolvencyVoter, error)
}

// NewKeeper creates new instances of the thorchain Keeper
type KeeperTHORName interface {
	THORNameExists(ctx cosmos.Context, _ string) bool
	GetTHORName(ctx cosmos.Context, _ string) (THORName, error)
	SetTHORName(ctx cosmos.Context, name THORName)
	GetTHORNameIterator(ctx cosmos.Context) cosmos.Iterator
	DeleteTHORName(ctx cosmos.Context, _ string) error
}

// NewKVStore creates new instances of the thorchain Keeper
func NewKeeper(cdc codec.BinaryCodec, coinKeeper bankkeeper.Keeper, accountKeeper authkeeper.AccountKeeper, storeKey cosmos.StoreKey) Keeper {
	version := semver.MustParse("0.0.0")
	return kv1.NewKVStore(cdc, coinKeeper, accountKeeper, storeKey, version)
}
