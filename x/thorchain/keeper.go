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
	GetStoreVersion(ctx cosmos.Context) int64
	SetStoreVersion(ctx cosmos.Context, ver int64)
	GetRuneBalaceOfModule(ctx cosmos.Context, moduleName string) cosmos.Uint
	SendFromModuleToModule(ctx cosmos.Context, from, to string, coin common.Coin) error
	SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coin common.Coin) error
	SendFromModuleToAccount(ctx cosmos.Context, from string, to cosmos.AccAddress, coin common.Coin) error

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
	prefixStoreVersion       dbPrefix = "_ver"
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

// KVStoreV1 Keeper maintains the link to data storage and exposes getter/setter methods for the various parts of the state machine
type KVStoreV1 struct {
	coinKeeper   bank.Keeper
	supplyKeeper supply.Keeper
	storeKey     cosmos.StoreKey // Unexposed key to access store from cosmos.Context
	cdc          *codec.Codec    // The wire codec for binary encoding/decoding.
}

// NewKVStoreV1 creates new instances of the thorchain Keeper
func NewKVStoreV1(coinKeeper bank.Keeper, supplyKeeper supply.Keeper, storeKey cosmos.StoreKey, cdc *codec.Codec) KVStoreV1 {
	return KVStoreV1{
		coinKeeper:   coinKeeper,
		supplyKeeper: supplyKeeper,
		storeKey:     storeKey,
		cdc:          cdc,
	}
}

func (k KVStoreV1) Cdc() *codec.Codec {
	return k.cdc
}

func (k KVStoreV1) Supply() supply.Keeper {
	return k.supplyKeeper
}

func (k KVStoreV1) CoinKeeper() bank.Keeper {
	return k.coinKeeper
}

func (k KVStoreV1) Logger(ctx cosmos.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", ModuleName))
}

func (k KVStoreV1) GetKey(ctx cosmos.Context, prefix dbPrefix, key string) string {
	return fmt.Sprintf("%s/%s", prefix, strings.ToUpper(key))
}

func (k KVStoreV1) GetStoreVersion(ctx cosmos.Context) int64 {
	key := prefixStoreVersion
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return 1
	}
	var value int64
	buf := store.Get([]byte(key))
	k.cdc.MustUnmarshalBinaryLengthPrefixed(buf, &value)
	return value
}

func (k KVStoreV1) SetStoreVersion(ctx cosmos.Context, value int64) {
	key := k.GetKey(ctx, prefixStoreVersion, "")
	store := ctx.KVStore(k.storeKey)
	key = k.GetKey(ctx, prefixStoreVersion, key)
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(value))
}

func (k KVStoreV1) GetRuneBalaceOfModule(ctx cosmos.Context, moduleName string) cosmos.Uint {
	addr := k.supplyKeeper.GetModuleAddress(moduleName)
	coins := k.coinKeeper.GetCoins(ctx, addr)
	amt := coins.AmountOf(common.RuneNative.Native())
	return cosmos.NewUintFromBigInt(amt.BigInt())
}

func (k KVStoreV1) SendFromModuleToModule(ctx cosmos.Context, from, to string, coin common.Coin) error {
	coins := cosmos.NewCoins(
		cosmos.NewCoin(coin.Asset.Native(), cosmos.NewIntFromBigInt(coin.Amount.BigInt())),
	)
	return k.Supply().SendCoinsFromModuleToModule(ctx, from, to, coins)
}

func (k KVStoreV1) SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coin common.Coin) error {
	coins := cosmos.NewCoins(
		cosmos.NewCoin(coin.Asset.Native(), cosmos.NewIntFromBigInt(coin.Amount.BigInt())),
	)
	return k.Supply().SendCoinsFromAccountToModule(ctx, from, to, coins)
}

func (k KVStoreV1) SendFromModuleToAccount(ctx cosmos.Context, from string, to cosmos.AccAddress, coin common.Coin) error {
	coins := cosmos.NewCoins(
		cosmos.NewCoin(coin.Asset.Native(), cosmos.NewIntFromBigInt(coin.Amount.BigInt())),
	)
	return k.Supply().SendCoinsFromModuleToAccount(ctx, from, to, coins)
}
