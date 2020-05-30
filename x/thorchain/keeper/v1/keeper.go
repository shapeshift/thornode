package keeperv1

import (
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/supply"
	"github.com/tendermint/tendermint/libs/log"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	kvTypes "gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
)

// NOTE: Always end a dbPrefix with a slash ("/"). This is to ensure that there
// are no prefixes that contain another prefix. In the scenario where this is
// true, an iterator for a specific type, will get more than intended, and may
// include a different type. The slash is used to protect us from this
// scenario.
// Also, use underscores between words and use lowercase characters only

const (
	version                  int64            = 1
	prefixStoreVersion       kvTypes.DbPrefix = "_ver"
	prefixObservedTxIn       kvTypes.DbPrefix = "observed_tx_in/"
	prefixObservedTxOut      kvTypes.DbPrefix = "observed_tx_out/"
	prefixPool               kvTypes.DbPrefix = "pool/"
	prefixTxOut              kvTypes.DbPrefix = "txout/"
	prefixTotalLiquidityFee  kvTypes.DbPrefix = "total_liquidity_fee/"
	prefixPoolLiquidityFee   kvTypes.DbPrefix = "pool_liquidity_fee/"
	prefixStaker             kvTypes.DbPrefix = "staker/"
	prefixLastChainHeight    kvTypes.DbPrefix = "last_chain_height/"
	prefixLastSignedHeight   kvTypes.DbPrefix = "last_signed_height/"
	prefixNodeAccount        kvTypes.DbPrefix = "node_account/"
	prefixActiveObserver     kvTypes.DbPrefix = "active_observer/"
	prefixVaultPool          kvTypes.DbPrefix = "vault/"
	prefixVaultAsgardIndex   kvTypes.DbPrefix = "vault_asgard_index/"
	prefixVaultData          kvTypes.DbPrefix = "vault_data/"
	prefixObservingAddresses kvTypes.DbPrefix = "observing_addresses/"
	prefixReserves           kvTypes.DbPrefix = "reserves/"
	prefixTss                kvTypes.DbPrefix = "tss/"
	prefixKeygen             kvTypes.DbPrefix = "keygen/"
	prefixRagnarok           kvTypes.DbPrefix = "ragnarok/"
	prefixGas                kvTypes.DbPrefix = "gas/"
	prefixSupportedTxMarker  kvTypes.DbPrefix = "marker/"
	prefixErrataTx           kvTypes.DbPrefix = "errata/"
	prefixBanVoter           kvTypes.DbPrefix = "ban/"
	prefixNodeSlashPoints    kvTypes.DbPrefix = "slash/"
	prefixSwapQueueItem      kvTypes.DbPrefix = "swapitem/"
	prefixMimir              kvTypes.DbPrefix = "mimir/"
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
	version      int64
}

// NewKVStore creates new instances of the thorchain Keeper
func NewKVStore(coinKeeper bank.Keeper, supplyKeeper supply.Keeper, storeKey cosmos.StoreKey, cdc *codec.Codec) KVStore {
	return KVStore{
		coinKeeper:   coinKeeper,
		supplyKeeper: supplyKeeper,
		storeKey:     storeKey,
		cdc:          cdc,
		version:      version,
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

func (k KVStore) Version() int64 {
	return k.version
}

func (k KVStore) Logger(ctx cosmos.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", ModuleName))
}

func (k KVStore) GetKey(ctx cosmos.Context, prefix kvTypes.DbPrefix, key string) string {
	return fmt.Sprintf("%s/%s", prefix, strings.ToUpper(key))
}

func (k KVStore) GetStoreVersion(ctx cosmos.Context) int64 {
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

func (k KVStore) SetStoreVersion(ctx cosmos.Context, value int64) {
	key := k.GetKey(ctx, prefixStoreVersion, "")
	store := ctx.KVStore(k.storeKey)
	key = k.GetKey(ctx, prefixStoreVersion, key)
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(value))
}

func (k KVStore) GetRuneBalaceOfModule(ctx cosmos.Context, moduleName string) cosmos.Uint {
	addr := k.supplyKeeper.GetModuleAddress(moduleName)
	coins := k.coinKeeper.GetCoins(ctx, addr)
	amt := coins.AmountOf(common.RuneNative.Native())
	return cosmos.NewUintFromBigInt(amt.BigInt())
}

func (k KVStore) SendFromModuleToModule(ctx cosmos.Context, from, to string, coin common.Coin) error {
	coins := cosmos.NewCoins(
		cosmos.NewCoin(coin.Asset.Native(), cosmos.NewIntFromBigInt(coin.Amount.BigInt())),
	)
	return k.Supply().SendCoinsFromModuleToModule(ctx, from, to, coins)
}

func (k KVStore) SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coin common.Coin) error {
	coins := cosmos.NewCoins(
		cosmos.NewCoin(coin.Asset.Native(), cosmos.NewIntFromBigInt(coin.Amount.BigInt())),
	)
	return k.Supply().SendCoinsFromAccountToModule(ctx, from, to, coins)
}

func (k KVStore) SendFromModuleToAccount(ctx cosmos.Context, from string, to cosmos.AccAddress, coin common.Coin) error {
	coins := cosmos.NewCoins(
		cosmos.NewCoin(coin.Asset.Native(), cosmos.NewIntFromBigInt(coin.Amount.BigInt())),
	)
	return k.Supply().SendCoinsFromModuleToAccount(ctx, from, to, coins)
}
