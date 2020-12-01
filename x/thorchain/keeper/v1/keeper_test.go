package keeperv1

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/types"
	. "gopkg.in/check.v1"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/store"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/params"
	"github.com/cosmos/cosmos-sdk/x/supply"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"

	"gitlab.com/thorchain/thornode/cmd"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func TestPackage(t *testing.T) { TestingT(t) }

func FundModule(c *C, ctx cosmos.Context, k KVStore, name string, amt uint64) {
	coin, err := common.NewCoin(common.RuneNative, cosmos.NewUint(amt*common.One)).Native()
	c.Assert(err, IsNil)
	err = k.Supply().MintCoins(ctx, ModuleName, cosmos.NewCoins(coin))
	c.Assert(err, IsNil)
	err = k.Supply().SendCoinsFromModuleToModule(ctx, ModuleName, name, cosmos.NewCoins(coin))
	c.Assert(err, IsNil)
}

func FundAccount(c *C, ctx cosmos.Context, k KVStore, addr cosmos.AccAddress, amt uint64) {
	coin, err := common.NewCoin(common.RuneNative, cosmos.NewUint(amt*common.One)).Native()
	c.Assert(err, IsNil)
	err = k.Supply().MintCoins(ctx, ModuleName, cosmos.NewCoins(coin))
	c.Assert(err, IsNil)
	err = k.Supply().SendCoinsFromModuleToAccount(ctx, ModuleName, addr, cosmos.NewCoins(coin))
	c.Assert(err, IsNil)
}

// nolint: deadcode unused
// create a codec used only for testing
func makeTestCodec() *codec.Codec {
	cdc := codec.New()
	bank.RegisterCodec(cdc)
	auth.RegisterCodec(cdc)
	RegisterCodec(cdc)
	supply.RegisterCodec(cdc)
	cosmos.RegisterCodec(cdc)
	codec.RegisterCrypto(cdc)
	return cdc
}

var (
	multiPerm    = "multiple permissions account"
	randomPerm   = "random permission"
	holder       = "holder"
	keyThorchain = cosmos.NewKVStoreKey(StoreKey)
)

func setupKeeperForTest(c *C) (cosmos.Context, KVStore) {
	types.SetCoinDenomRegex(func() string {
		return cmd.DenomRegex
	})
	keyAcc := cosmos.NewKVStoreKey(auth.StoreKey)
	keyParams := cosmos.NewKVStoreKey(params.StoreKey)
	tkeyParams := cosmos.NewTransientStoreKey(params.TStoreKey)
	keySupply := cosmos.NewKVStoreKey(supply.StoreKey)

	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(keyAcc, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keySupply, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyParams, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyThorchain, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkeyParams, cosmos.StoreTypeTransient, db)
	err := ms.LoadLatestVersion()
	c.Assert(err, IsNil)

	ctx := cosmos.NewContext(ms, abci.Header{ChainID: "thorchain"}, false, log.NewNopLogger())
	ctx = ctx.WithBlockHeight(18)
	cdc := makeTestCodec()

	pk := params.NewKeeper(cdc, keyParams, tkeyParams)
	ak := auth.NewAccountKeeper(cdc, keyAcc, pk.Subspace(auth.DefaultParamspace), auth.ProtoBaseAccount)
	bk := bank.NewBaseKeeper(ak, pk.Subspace(bank.DefaultParamspace), nil)

	maccPerms := map[string][]string{
		auth.FeeCollectorName: nil,
		holder:                nil,
		supply.Minter:         {supply.Minter},
		supply.Burner:         {supply.Burner},
		multiPerm:             {supply.Minter, supply.Burner, supply.Staking},
		randomPerm:            {"random"},
		ModuleName:            {supply.Minter},
		ReserveName:           {},
		AsgardName:            {},
		BondName:              {supply.Staking},
	}
	supplyKeeper := supply.NewKeeper(cdc, keySupply, ak, bk, maccPerms)
	totalSupply := cosmos.NewCoins(cosmos.NewCoin("bep", cosmos.NewInt(1000*common.One)))
	supplyKeeper.SetSupply(ctx, supply.NewSupply(totalSupply))
	k := NewKVStore(bk, supplyKeeper, ak, keyThorchain, cdc)

	FundModule(c, ctx, k, AsgardName, common.One)

	return ctx, k
}

type KeeperTestSuit struct{}

var _ = Suite(&KeeperTestSuit{})

func (KeeperTestSuit) TestKeeperVersion(c *C) {
	ctx, k := setupKeeperForTest(c)
	c.Check(k.GetStoreVersion(ctx), Equals, int64(6))
	c.Check(k.CoinKeeper(), NotNil)
	c.Check(k.Version(), Equals, version)

	k.SetStoreVersion(ctx, 2)
	c.Check(k.GetStoreVersion(ctx), Equals, int64(2))

	c.Check(k.GetRuneBalanceOfModule(ctx, AsgardName).Equal(cosmos.NewUint(100000000*common.One)), Equals, true)
	coinToSend := common.NewCoin(common.RuneNative, cosmos.NewUint(1*common.One))
	c.Check(k.SendFromModuleToModule(ctx, AsgardName, BondName, coinToSend), IsNil)

	acct := GetRandomBech32Addr()
	c.Check(k.SendFromModuleToAccount(ctx, AsgardName, acct, coinToSend), IsNil)

	// check get account balance
	coins := k.CoinKeeper().GetCoins(ctx, acct)
	c.Check(coins, HasLen, 1)

	c.Check(k.SendFromAccountToModule(ctx, acct, AsgardName, coinToSend), IsNil)

	// check no account balance
	coins = k.CoinKeeper().GetCoins(ctx, GetRandomBech32Addr())
	c.Check(coins, HasLen, 0)
}
