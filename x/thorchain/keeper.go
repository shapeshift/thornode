package thorchain

import (
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/supply"
	"github.com/tendermint/tendermint/libs/log"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type Keeper interface {
	Cdc() *codec.Codec
	Supply() supply.Keeper
	CoinKeeper() bank.Keeper
	Logger(ctx cosmos.Context) log.Logger
	GetKey(ctx cosmos.Context, prefix dbPrefix, key string) string
	GetRuneBalaceOfModule(ctx cosmos.Context, moduleName string) cosmos.Uint
	SendFromModuleToModule(ctx cosmos.Context, from, to string, coin common.Coin) cosmos.Error
	SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coin common.Coin) cosmos.Error
	SendFromModuleToAccount(ctx cosmos.Context, from string, to cosmos.AccAddress, coin common.Coin) cosmos.Error

	// Keeper Interfaces
	KeeperPool
	KeeperLastHeight
	KeeperStaker
	KeeperNodeAccount
	KeeperObserver
	KeeperObservedTx
	KeeperTxOut
	KeeperLiquidityFees
	KeeperEvents
	KeeperVault
	KeeperReserveContributors
	KeeperVaultData
	KeeperTss
	KeeperTssKeysignFail
	KeeperKeygen
	KeeperRagnarok
	KeeperGas
	KeeperTxMarker
	KeeperErrataTx
	KeeperBanVoter
	KeeperSwapQueue
	KeeperMimir
}

// NOTE: Always end a dbPrefix with a slash ("/"). This is to ensure that there
// are no prefixes that contain another prefix. In the scenario where this is
// true, an iterator for a specific type, will get more than intended, and may
// include a different type. The slash is used to protect us from this
// scenario.
// Also, use underscores between words and use lowercase characters only
type dbPrefix string

const (
	prefixObservedTx         dbPrefix = "observed_tx/"
	prefixPool               dbPrefix = "pool/"
	prefixTxOut              dbPrefix = "txout/"
	prefixTotalLiquidityFee  dbPrefix = "total_liquidity_fee/"
	prefixPoolLiquidityFee   dbPrefix = "pool_liquidity_fee/"
	prefixStaker             dbPrefix = "staker/"
	prefixEvents             dbPrefix = "events/"
	prefixTxHashEvents       dbPrefix = "tx_events/"
	prefixPendingEvents      dbPrefix = "pending_events/"
	prefixCurrentEventID     dbPrefix = "current_event_id/"
	prefixLastChainHeight    dbPrefix = "last_chain_height/"
	prefixLastSignedHeight   dbPrefix = "last_signed_height/"
	prefixNodeAccount        dbPrefix = "node_account/"
	prefixActiveObserver     dbPrefix = "active_observer/"
	prefixVaultPool          dbPrefix = "vault/"
	prefixVaultAsgardIndex   dbPrefix = "vault_asgard_index/"
	prefixVaultData          dbPrefix = "vault_data/"
	prefixObservingAddresses dbPrefix = "observing_addresses/"
	prefixReserves           dbPrefix = "reserves/"
	prefixTss                dbPrefix = "tss/"
	prefixKeygen             dbPrefix = "keygen/"
	prefixRagnarok           dbPrefix = "ragnarok/"
	prefixGas                dbPrefix = "gas/"
	prefixSupportedTxMarker  dbPrefix = "marker/"
	prefixErrataTx           dbPrefix = "errata/"
	prefixBanVoter           dbPrefix = "ban/"
	prefixNodeSlashPoints    dbPrefix = "slash/"
	prefixSwapQueueItem      dbPrefix = "swapitem/"
	prefixMimir              dbPrefix = "mimir/"
)

func dbError(ctx cosmos.Context, wrapper string, err error) error {
	err = fmt.Errorf("KVStore Error: %s: %w", wrapper, err)
	ctx.Logger().Error(err.Error())
	return err
}

// KVStore Keeper maintains the link to data storage and exposes getter/setter methods for the various parts of the state machine
type KVStore struct {
	coinKeeper   bank.Keeper
	supplyKeeper supply.Keeper
	storeKey     cosmos.StoreKey // Unexposed key to access store from cosmos.Context
	cdc          *codec.Codec    // The wire codec for binary encoding/decoding.
}

// NewKVStore creates new instances of the thorchain Keeper
func NewKVStore(coinKeeper bank.Keeper, supplyKeeper supply.Keeper, storeKey cosmos.StoreKey, cdc *codec.Codec) KVStore {
	return KVStore{
		coinKeeper:   coinKeeper,
		supplyKeeper: supplyKeeper,
		storeKey:     storeKey,
		cdc:          cdc,
	}
}

func (k KVStore) Cdc() *codec.Codec {
	return k.cdc
}

func (k KVStore) Supply() supply.Keeper {
	return k.supplyKeeper
}

func (k KVStore) CoinKeeper() bank.Keeper {
	return k.coinKeeper
}

func (k KVStore) Logger(ctx cosmos.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", ModuleName))
}

func (k KVStore) GetKey(ctx cosmos.Context, prefix dbPrefix, key string) string {
	version := getVersion(k.GetLowestActiveVersion(ctx), prefix)
	return fmt.Sprintf("%s%d/%s", prefix, version.Minor, strings.ToUpper(key))
}

func (k KVStore) GetRuneBalaceOfModule(ctx cosmos.Context, moduleName string) cosmos.Uint {
	addr := k.supplyKeeper.GetModuleAddress(moduleName)
	coins := k.coinKeeper.GetCoins(ctx, addr)
	amt := coins.AmountOf(common.RuneNative.Native())
	return cosmos.NewUintFromBigInt(amt.BigInt())
}

func (k KVStore) SendFromModuleToModule(ctx cosmos.Context, from, to string, coin common.Coin) cosmos.Error {
	coins := cosmos.NewCoins(
		cosmos.NewCoin(coin.Asset.Native(), cosmos.NewIntFromBigInt(coin.Amount.BigInt())),
	)
	return k.Supply().SendCoinsFromModuleToModule(ctx, from, to, coins)
}

func (k KVStore) SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coin common.Coin) cosmos.Error {
	coins := cosmos.NewCoins(
		cosmos.NewCoin(coin.Asset.Native(), cosmos.NewIntFromBigInt(coin.Amount.BigInt())),
	)
	return k.Supply().SendCoinsFromAccountToModule(ctx, from, to, coins)
}

func (k KVStore) SendFromModuleToAccount(ctx cosmos.Context, from string, to cosmos.AccAddress, coin common.Coin) cosmos.Error {
	coins := cosmos.NewCoins(
		cosmos.NewCoin(coin.Asset.Native(), cosmos.NewIntFromBigInt(coin.Amount.BigInt())),
	)
	return k.Supply().SendCoinsFromModuleToAccount(ctx, from, to, coins)
}
