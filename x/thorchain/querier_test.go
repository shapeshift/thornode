package thorchain

import (
	abci "github.com/tendermint/tendermint/abci/types"
	. "gopkg.in/check.v1"

	ckeys "github.com/cosmos/cosmos-sdk/crypto/keys"
	"github.com/cosmos/cosmos-sdk/crypto/keys/hd"
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	keeper "gitlab.com/thorchain/thornode/x/thorchain/keeper"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

type QuerierSuite struct {
	kb KeybaseStore
}

var _ = Suite(&QuerierSuite{})

type TestQuerierKeeper struct {
	keeper.KVStoreDummy
	txOut *TxOut
}

func (k *TestQuerierKeeper) GetTxOut(_ cosmos.Context, _ int64) (*TxOut, error) {
	return k.txOut, nil
}

func (s *QuerierSuite) SetUpSuite(c *C) {
	kb := ckeys.NewInMemory()
	username := "test_user"
	password := "password"

	params := *hd.NewFundraiserParams(0, 118, 0)
	hdPath := params.String()
	_, err := kb.CreateAccount(username, "industry segment educate height inject hover bargain offer employ select speak outer video tornado story slow chief object junk vapor venue large shove behave", password, password, hdPath, ckeys.Secp256k1)
	c.Assert(err, IsNil)
	s.kb = KeybaseStore{
		SignerName:   username,
		SignerPasswd: password,
		Keybase:      kb,
	}
}

func (s *QuerierSuite) TestQueryKeysign(c *C) {
	ctx, _ := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(12)

	pk := GetRandomPubKey()
	toAddr := GetRandomBNBAddress()
	txOut := NewTxOut(1)
	txOutItem := &TxOutItem{
		Chain:       common.BNBChain,
		VaultPubKey: pk,
		ToAddress:   toAddr,
		InHash:      GetRandomTxHash(),
		Coin:        common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
	}
	txOut.TxArray = append(txOut.TxArray, txOutItem)
	keeper := &TestQuerierKeeper{
		txOut: txOut,
	}

	querier := NewQuerier(keeper, s.kb)

	path := []string{
		"keysign",
		"5",
		pk.String(),
	}
	res, err := querier(ctx, path, abci.RequestQuery{})
	c.Assert(err, IsNil)
	c.Assert(res, NotNil)
}

func (s *QuerierSuite) TestQueryPool(c *C) {
	ctx, keeper := setupKeeperForTest(c)

	querier := NewQuerier(keeper, s.kb)
	path := []string{"pools"}

	pubKey := GetRandomPubKey()
	asgard := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, pubKey, common.Chains{common.BNBChain})
	c.Assert(keeper.SetVault(ctx, asgard), IsNil)

	poolBNB := NewPool()
	poolBNB.Asset = common.BNBAsset
	poolBNB.PoolUnits = cosmos.NewUint(100)

	poolBTC := NewPool()
	poolBTC.Asset = common.BTCAsset
	poolBTC.PoolUnits = cosmos.NewUint(0)

	err := keeper.SetPool(ctx, poolBNB)
	c.Assert(err, IsNil)

	err = keeper.SetPool(ctx, poolBTC)
	c.Assert(err, IsNil)

	res, err := querier(ctx, path, abci.RequestQuery{})
	c.Assert(err, IsNil)

	var out types.QueryResPools
	err = keeper.Cdc().UnmarshalJSON(res, &out)
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 1)

	poolBTC.PoolUnits = cosmos.NewUint(100)
	err = keeper.SetPool(ctx, poolBTC)
	c.Assert(err, IsNil)

	res, err = querier(ctx, path, abci.RequestQuery{})
	c.Assert(err, IsNil)

	err = keeper.Cdc().UnmarshalJSON(res, &out)
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 2)
}

func (s *QuerierSuite) TestQueryNodeAccounts(c *C) {
	ctx, keeper := setupKeeperForTest(c)

	querier := NewQuerier(keeper, s.kb)
	path := []string{"nodeaccounts"}

	signer := GetRandomBech32Addr()
	bondAddr := GetRandomBNBAddress()
	emptyPubKeySet := common.PubKeySet{}
	bond := cosmos.NewUint(common.One * 100)
	nodeAccount := NewNodeAccount(signer, NodeActive, emptyPubKeySet, "", bond, bondAddr, ctx.BlockHeight())
	c.Assert(keeper.SetNodeAccount(ctx, nodeAccount), IsNil)

	res, err := querier(ctx, path, abci.RequestQuery{})
	c.Assert(err, IsNil)

	var out types.NodeAccounts
	err1 := keeper.Cdc().UnmarshalJSON(res, &out)
	c.Assert(err1, IsNil)
	c.Assert(len(out), Equals, 1)

	signer = GetRandomBech32Addr()
	bondAddr = GetRandomBNBAddress()
	emptyPubKeySet = common.PubKeySet{}
	bond = cosmos.NewUint(common.One * 200)
	nodeAccount2 := NewNodeAccount(signer, NodeActive, emptyPubKeySet, "", bond, bondAddr, ctx.BlockHeight())
	c.Assert(keeper.SetNodeAccount(ctx, nodeAccount2), IsNil)

	res, err = querier(ctx, path, abci.RequestQuery{})
	c.Assert(err, IsNil)

	err1 = keeper.Cdc().UnmarshalJSON(res, &out)
	c.Assert(err1, IsNil)
	c.Assert(len(out), Equals, 2)

	nodeAccount2.Bond = cosmos.NewUint(0)
	c.Assert(keeper.SetNodeAccount(ctx, nodeAccount2), IsNil)

	res, err = querier(ctx, path, abci.RequestQuery{})
	c.Assert(err, IsNil)

	err1 = keeper.Cdc().UnmarshalJSON(res, &out)
	c.Assert(err1, IsNil)
	c.Assert(len(out), Equals, 1)
}
