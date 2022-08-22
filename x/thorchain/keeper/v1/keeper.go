package keeperv1

import (
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/codec"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
	kvTypes "gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
)

// NOTE: Always end a dbPrefix with a slash ("/"). This is to ensure that there
// are no prefixes that contain another prefix. In the scenario where this is
// true, an iterator for a specific type, will get more than intended, and may
// include a different type. The slash is used to protect us from this
// scenario.
// Also, use underscores between words and use lowercase characters only

const (
	prefixStoreVersion            kvTypes.DbPrefix = "_ver/"
	prefixObservedTxIn            kvTypes.DbPrefix = "observed_tx_in/"
	prefixObservedTxOut           kvTypes.DbPrefix = "observed_tx_out/"
	prefixObservedLink            kvTypes.DbPrefix = "ob_link/"
	prefixPool                    kvTypes.DbPrefix = "pool/"
	prefixTxOut                   kvTypes.DbPrefix = "txout/"
	prefixTotalLiquidityFee       kvTypes.DbPrefix = "total_liquidity_fee/"
	prefixPoolLiquidityFee        kvTypes.DbPrefix = "pool_liquidity_fee/"
	prefixLiquidityProvider       kvTypes.DbPrefix = "lp/"
	prefixLastChainHeight         kvTypes.DbPrefix = "last_chain_height/"
	prefixLastSignedHeight        kvTypes.DbPrefix = "last_signed_height/"
	prefixLastObserveHeight       kvTypes.DbPrefix = "last_observe_height/"
	prefixNodeAccount             kvTypes.DbPrefix = "node_account/"
	prefixBondProviders           kvTypes.DbPrefix = "bond_providers/"
	prefixVault                   kvTypes.DbPrefix = "vault/"
	prefixVaultAsgardIndex        kvTypes.DbPrefix = "vault_asgard_index/"
	prefixNetwork                 kvTypes.DbPrefix = "network/"
	prefixPOL                     kvTypes.DbPrefix = "pol/"
	prefixObservingAddresses      kvTypes.DbPrefix = "observing_addresses/"
	prefixTss                     kvTypes.DbPrefix = "tss/"
	prefixTssKeysignFailure       kvTypes.DbPrefix = "tssKeysignFailure/"
	prefixKeygen                  kvTypes.DbPrefix = "keygen/"
	prefixRagnarokHeight          kvTypes.DbPrefix = "ragnarokHeight/"
	prefixRagnarokNth             kvTypes.DbPrefix = "ragnarokNth/"
	prefixRagnarokPending         kvTypes.DbPrefix = "ragnarokPending/"
	prefixRagnarokPosition        kvTypes.DbPrefix = "ragnarokPosition/"
	prefixRagnarokPoolHeight      kvTypes.DbPrefix = "ragnarokPool/"
	prefixErrataTx                kvTypes.DbPrefix = "errata/"
	prefixBanVoter                kvTypes.DbPrefix = "ban/"
	prefixNodeSlashPoints         kvTypes.DbPrefix = "slash/"
	prefixNodeJail                kvTypes.DbPrefix = "jail/"
	prefixSwapQueueItem           kvTypes.DbPrefix = "swapitem/"
	prefixOrderBookItem           kvTypes.DbPrefix = "o/"
	prefixOrderBookLimitIndex     kvTypes.DbPrefix = "olim/"
	prefixOrderBookMarketIndex    kvTypes.DbPrefix = "omark/"
	prefixOrderBookProcessor      kvTypes.DbPrefix = "oproc/"
	prefixMimir                   kvTypes.DbPrefix = "mimir/"
	prefixNodeMimir               kvTypes.DbPrefix = "nodemimir/"
	prefixNodePauseChain          kvTypes.DbPrefix = "node_pause_chain/"
	prefixNetworkFee              kvTypes.DbPrefix = "network_fee/"
	prefixNetworkFeeVoter         kvTypes.DbPrefix = "network_fee_voter/"
	prefixTssKeygenMetric         kvTypes.DbPrefix = "tss_keygen_metric/"
	prefixTssKeysignMetric        kvTypes.DbPrefix = "tss_keysign_metric/"
	prefixTssKeysignMetricLatest  kvTypes.DbPrefix = "latest_tss_keysign_metric/"
	prefixChainContract           kvTypes.DbPrefix = "chain_contract/"
	prefixSolvencyVoter           kvTypes.DbPrefix = "solvency_voter/"
	prefixTHORName                kvTypes.DbPrefix = "thorname/"
	prefixRollingPoolLiquidityFee kvTypes.DbPrefix = "rolling_pool_liquidity_fee/"
	prefixVersion                 kvTypes.DbPrefix = "version/"
)

func dbError(ctx cosmos.Context, wrapper string, err error) error {
	err = fmt.Errorf("KVStore Error: %s: %w", wrapper, err)
	ctx.Logger().Error("keeper error", "error", err)
	return err
}

// KVStore Keeper maintains the link to data storage and exposes getter/setter methods for the various parts of the state machine
type KVStore struct {
	cdc           codec.BinaryCodec
	coinKeeper    bankkeeper.Keeper
	accountKeeper authkeeper.AccountKeeper
	storeKey      cosmos.StoreKey // Unexposed key to access store from cosmos.Context
	version       semver.Version
}

// NewKVStore creates new instances of the thorchain Keeper
func NewKVStore(cdc codec.BinaryCodec, coinKeeper bankkeeper.Keeper, accountKeeper authkeeper.AccountKeeper, storeKey cosmos.StoreKey, version semver.Version) KVStore {
	return KVStore{
		coinKeeper:    coinKeeper,
		accountKeeper: accountKeeper,
		storeKey:      storeKey,
		cdc:           cdc,
		version:       version,
	}
}

// Cdc return the amino codec
func (k KVStore) Cdc() codec.BinaryCodec {
	return k.cdc
}

// GetVersion return the current version
func (k KVStore) GetVersion() semver.Version {
	return k.version
}

func (k *KVStore) SetVersion(ver semver.Version) {
	k.version = ver
}

// GetKey return a key that can be used to store into key value store
func (k KVStore) GetKey(ctx cosmos.Context, prefix kvTypes.DbPrefix, key string) string {
	return fmt.Sprintf("%s/%s", prefix, strings.ToUpper(key))
}

// SetStoreVersion save the store version
func (k KVStore) SetStoreVersion(ctx cosmos.Context, value int64) {
	key := k.GetKey(ctx, prefixStoreVersion, "")
	store := ctx.KVStore(k.storeKey)
	ver := ProtoInt64{Value: value}
	store.Set([]byte(key), k.cdc.MustMarshal(&ver))
}

// GetStoreVersion get the current key value store version
func (k KVStore) GetStoreVersion(ctx cosmos.Context) int64 {
	key := k.GetKey(ctx, prefixStoreVersion, "")
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		// thornode start at version 0.38.0, thus when there is no store version , it return 38
		return 38
	}
	var ver ProtoInt64
	buf := store.Get([]byte(key))
	k.cdc.MustUnmarshal(buf, &ver)
	return ver.Value
}

// getIterator - get an iterator for given prefix
func (k KVStore) getIterator(ctx cosmos.Context, prefix types.DbPrefix) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefix))
}

// del - delete data from the kvstore
func (k KVStore) del(ctx cosmos.Context, key string) {
	store := ctx.KVStore(k.storeKey)
	if store.Has([]byte(key)) {
		store.Delete([]byte(key))
	}
}

// has - kvstore has key
func (k KVStore) has(ctx cosmos.Context, key string) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has([]byte(key))
}

func (k KVStore) setInt64(ctx cosmos.Context, key string, record int64) {
	store := ctx.KVStore(k.storeKey)
	value := ProtoInt64{Value: record}
	buf := k.cdc.MustMarshal(&value)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getInt64(ctx cosmos.Context, key string, record *int64) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	value := ProtoInt64{}
	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, &value); err != nil {
		return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	*record = value.GetValue()
	return true, nil
}

func (k KVStore) setUint64(ctx cosmos.Context, key string, record uint64) {
	store := ctx.KVStore(k.storeKey)
	value := ProtoUint64{Value: record}
	buf := k.cdc.MustMarshal(&value)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getUint64(ctx cosmos.Context, key string, record *uint64) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	value := ProtoUint64{Value: *record}
	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, &value); err != nil {
		return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	*record = value.GetValue()
	return true, nil
}

func (k KVStore) setAccAddresses(ctx cosmos.Context, key string, record []cosmos.AccAddress) {
	store := ctx.KVStore(k.storeKey)
	value := ProtoAccAddresses{Value: record}
	buf := k.cdc.MustMarshal(&value)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getAccAddresses(ctx cosmos.Context, key string, record *[]cosmos.AccAddress) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	var value ProtoAccAddresses
	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, &value); err != nil {
		return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	*record = value.Value
	return true, nil
}

func (k KVStore) setStrings(ctx cosmos.Context, key string, record []string) {
	store := ctx.KVStore(k.storeKey)
	value := ProtoStrings{Value: record}
	buf := k.cdc.MustMarshal(&value)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getStrings(ctx cosmos.Context, key string, record *[]string) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	var value ProtoStrings
	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, &value); err != nil {
		return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	*record = value.Value
	return true, nil
}

// GetRuneBalanceOfModule get the RUNE balance
func (k KVStore) GetRuneBalanceOfModule(ctx cosmos.Context, moduleName string) cosmos.Uint {
	return k.GetBalanceOfModule(ctx, moduleName, common.RuneNative.Native())
}

func (k KVStore) GetBalanceOfModule(ctx cosmos.Context, moduleName, denom string) cosmos.Uint {
	addr := k.accountKeeper.GetModuleAddress(moduleName)
	coin := k.coinKeeper.GetBalance(ctx, addr, denom)
	return cosmos.NewUintFromBigInt(coin.Amount.BigInt())
}

// SendFromModuleToModule transfer asset from one module to another
func (k KVStore) SendFromModuleToModule(ctx cosmos.Context, from, to string, coins common.Coins) error {
	cosmosCoins := make(cosmos.Coins, len(coins))
	for i, c := range coins {
		cosmosCoins[i] = cosmos.NewCoin(c.Asset.Native(), cosmos.NewIntFromBigInt(c.Amount.BigInt()))
	}
	return k.coinKeeper.SendCoinsFromModuleToModule(ctx, from, to, cosmosCoins)
}

func (k KVStore) SendCoins(ctx cosmos.Context, from, to cosmos.AccAddress, coins cosmos.Coins) error {
	return k.coinKeeper.SendCoins(ctx, from, to, coins)
}

func (k KVStore) AddCoins(ctx cosmos.Context, addr cosmos.AccAddress, coins cosmos.Coins) error {
	return k.coinKeeper.SendCoinsFromModuleToAccount(ctx, ModuleName, addr, coins)
}

// SendFromAccountToModule transfer fund from one account to a module
func (k KVStore) SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coins common.Coins) error {
	cosmosCoins := make(cosmos.Coins, len(coins))
	for i, c := range coins {
		cosmosCoins[i] = cosmos.NewCoin(c.Asset.Native(), cosmos.NewIntFromBigInt(c.Amount.BigInt()))
	}
	return k.coinKeeper.SendCoinsFromAccountToModule(ctx, from, to, cosmosCoins)
}

// SendFromModuleToAccount transfer fund from module to an account
func (k KVStore) SendFromModuleToAccount(ctx cosmos.Context, from string, to cosmos.AccAddress, coins common.Coins) error {
	cosmosCoins := make(cosmos.Coins, len(coins))
	for i, c := range coins {
		cosmosCoins[i] = cosmos.NewCoin(c.Asset.Native(), cosmos.NewIntFromBigInt(c.Amount.BigInt()))
	}
	return k.coinKeeper.SendCoinsFromModuleToAccount(ctx, from, to, cosmosCoins)
}

func (k KVStore) BurnFromModule(ctx cosmos.Context, module string, coin common.Coin) error {
	coinToBurn, err := coin.Native()
	if err != nil {
		return fmt.Errorf("fail to parse coins: %w", err)
	}
	coinsToBurn := cosmos.Coins{coinToBurn}
	err = k.coinKeeper.BurnCoins(ctx, module, coinsToBurn)
	if err != nil {
		return fmt.Errorf("fail to burn assets: %w", err)
	}

	return nil
}

func (k KVStore) MintToModule(ctx cosmos.Context, module string, coin common.Coin) error {
	coinToMint, err := coin.Native()
	if err != nil {
		return fmt.Errorf("fail to parse coins: %w", err)
	}
	coinsToMint := cosmos.Coins{coinToMint}
	err = k.coinKeeper.MintCoins(ctx, module, coinsToMint)
	if err != nil {
		return fmt.Errorf("fail to mint assets: %w", err)
	}

	if k.GetVersion().GTE(semver.MustParse("1.95.0")) {
		// check if we've exceeded max rune supply cap. If we have, there could
		// be an issue (infinite mint bug/exploit), or maybe runway rune
		// hyperinflation. In any case, pause everything and allow the
		// community time to find a solution to the issue.
		coin := k.coinKeeper.GetSupply(ctx, common.RuneAsset().Native())
		maxAmt, _ := k.GetMimir(ctx, "MaxRuneSupply")
		if maxAmt > 0 && coin.Amount.GT(cosmos.NewInt(maxAmt)) {
			k.SetMimir(ctx, "HaltTrading", 1)
			k.SetMimir(ctx, "HaltChainGlobal", 1)
			k.SetMimir(ctx, "PauseLP", 1)
			k.SetMimir(ctx, "HaltTHORChain", 1)
		}
	}

	return nil
}

func (k KVStore) MintAndSendToAccount(ctx cosmos.Context, to cosmos.AccAddress, coin common.Coin) error {
	// Mint coins into the reserve
	if err := k.MintToModule(ctx, ModuleName, coin); err != nil {
		return err
	}
	return k.SendFromModuleToAccount(ctx, ModuleName, to, common.NewCoins(coin))
}

func (k KVStore) GetModuleAddress(module string) (common.Address, error) {
	return common.NewAddress(k.accountKeeper.GetModuleAddress(module).String())
}

func (k KVStore) GetModuleAccAddress(module string) cosmos.AccAddress {
	return k.accountKeeper.GetModuleAddress(module)
}

func (k KVStore) GetBalance(ctx cosmos.Context, addr cosmos.AccAddress) cosmos.Coins {
	return k.coinKeeper.GetAllBalances(ctx, addr)
}

func (k KVStore) HasCoins(ctx cosmos.Context, addr cosmos.AccAddress, coins cosmos.Coins) bool {
	balance := k.coinKeeper.GetAllBalances(ctx, addr)
	return balance.IsAllGTE(coins)
}

func (k KVStore) GetAccount(ctx cosmos.Context, addr cosmos.AccAddress) cosmos.Account {
	return k.accountKeeper.GetAccount(ctx, addr)
}
