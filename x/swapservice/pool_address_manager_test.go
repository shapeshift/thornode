package swapservice

import (
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"gitlab.com/thorchain/bepswap/common"
	. "gopkg.in/check.v1"
)

type PoolAddressManagerSuite struct{}

var _ = Suite(&PoolAddressManagerSuite{})

func (ps *PoolAddressManagerSuite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

func (PoolAddressManagerSuite) TestSetupInitialPoolAddresses(c *C) {
	ctx, k := setupKeeperForTest(c)
	poolAddrMgr := NewPoolAddressManager(k)
	c.Assert(poolAddrMgr, NotNil)
	// incorrect block height
	pa, err := poolAddrMgr.setupInitialPoolAddresses(ctx, 0)
	c.Assert(err, NotNil)
	c.Assert(pa.IsEmpty(), Equals, true)

	pa1, err := poolAddrMgr.setupInitialPoolAddresses(ctx, 1)
	c.Assert(err, NotNil)
	c.Assert(pa1.IsEmpty(), Equals, true)

	bnb := GetRandomBNBAddress()
	addr := GetRandomBech32Addr()
	bepConsPubKey := `bepcpub1zcjduepq4kn64fcjhf0fp20gp8var0rm25ca9jy6jz7acem8gckh0nkplznq85gdrg`
	trustAccount := NewTrustAccount(bnb, addr, bepConsPubKey)
	err = trustAccount.IsValid()
	c.Assert(err, IsNil)
	nodeAddress := GetRandomBech32Addr()
	bond := sdk.NewUint(100 * common.One)
	bondAddr := GetRandomBNBAddress()
	na := NewNodeAccount(nodeAddress, NodeActive, trustAccount, bond, bondAddr)
	k.SetNodeAccount(ctx, na)

	pa2, err := poolAddrMgr.setupInitialPoolAddresses(ctx, 1)
	c.Assert(err, IsNil)
	c.Assert(pa2.IsEmpty(), Equals, false)
	c.Assert(pa2.Current.String(), Equals, bnb.String())
	c.Assert(pa2.Next.String(), Equals, bnb.String())

	// Two nodes
	na1 := GetRandomNodeAccount(NodeActive)
	k.SetNodeAccount(ctx, na1)

	// given we have two active node account, thus we will rotate pool , however which pool will be chosen first
	// it will based on the alphabetic order of their signer BNB address
	nas := NodeAccounts{
		na, na1,
	}
	sort.Sort(nas)

	// with two active nodes
	pa3, err := poolAddrMgr.setupInitialPoolAddresses(ctx, 1)
	c.Assert(err, IsNil)
	c.Assert(pa3.IsEmpty(), Equals, false)
	c.Assert(pa3.Current.String(), Equals, nas[0].Accounts.SignerBNBAddress.String())
	c.Assert(pa3.Next.String(), Equals, nas[1].Accounts.SignerBNBAddress.String())

	nodeAccounts := NodeAccounts{na, na1}
	// with more than two  active nodes
	for i := 0; i < 10; i++ {
		na2 := GetRandomNodeAccount(NodeActive)
		k.SetNodeAccount(ctx, na2)
		nodeAccounts = append(nodeAccounts, na2)
	}

	sort.Sort(nodeAccounts)

	pa4, err := poolAddrMgr.setupInitialPoolAddresses(ctx, 1)
	c.Assert(err, IsNil)
	c.Assert(pa4.IsEmpty(), Equals, false)
	c.Assert(pa4.Current.String(), Equals, nodeAccounts[0].Accounts.SignerBNBAddress.String())
	c.Assert(pa4.Next.String(), Equals, nodeAccounts[1].Accounts.SignerBNBAddress.String())
	c.Logf("%+v", pa4)
	rotatePerBlockHeight := k.GetAdminConfigRotatePerBlockHeight(ctx, sdk.AccAddress{})
	rotateAt := rotatePerBlockHeight + 1
	txOutStore := NewTxOutStore(&MockTxOutSetter{})
	txOutStore.NewBlock(uint64(rotateAt))
	newPa := poolAddrMgr.rotatePoolAddress(ctx, rotateAt, pa4, txOutStore)
	c.Assert(newPa.IsEmpty(), Equals, false)
	c.Assert(newPa.Previous.String(), Equals, pa4.Current.String())
	c.Assert(newPa.Current.String(), Equals, pa4.Next.String())
	c.Assert(newPa.Next.String(), Equals, nodeAccounts[2].Accounts.SignerBNBAddress.String())
	c.Assert(newPa.RotateAt, Equals, int64(rotatePerBlockHeight*2+1))
	txOutStore.CommitBlock(ctx)
	poolBNB := createTempNewPoolForTest(ctx, k, "BNB", c)
	poolTCan := createTempNewPoolForTest(ctx, k, "TCAN-014", c)
	poolLoki := createTempNewPoolForTest(ctx, k, "LOK-3C0", c)

	txOutStore.NewBlock(uint64(rotatePerBlockHeight*2 + 1))
	newPa1 := poolAddrMgr.rotatePoolAddress(ctx, rotatePerBlockHeight*2+1, newPa, txOutStore)
	c.Logf("new pool addresses %+v", newPa1)
	c.Assert(newPa1.IsEmpty(), Equals, false)
	c.Assert(newPa1.Previous.String(), Equals, newPa.Current.String())
	c.Assert(newPa1.Current.String(), Equals, newPa.Next.String())
	c.Assert(newPa1.Next.String(), Equals, nodeAccounts[3].Accounts.SignerBNBAddress.String())
	c.Assert(newPa1.RotateAt, Equals, int64(rotatePerBlockHeight*3+1))
	c.Assert(len(txOutStore.blockOut.TxArray) > 0, Equals, true)
	c.Assert(txOutStore.blockOut.Valid(), IsNil)
	totalBond := sdk.ZeroUint()
	for _, item := range nodeAccounts {
		totalBond = totalBond.Add(item.Bond)
	}
	for _, item := range txOutStore.blockOut.TxArray {
		c.Assert(item.Valid(), IsNil)
		// make sure the fund is sending from previous pool address to current
		c.Assert(item.ToAddress.String(), Equals, newPa1.Current.String())
		c.Assert(len(item.Coins) > 0, Equals, true)
		if item.Coins[0].Denom == poolBNB.Ticker {
			c.Assert(item.Coins[0].Amount.Uint64(), Equals, poolBNB.BalanceToken.Uint64()-batchTransactionFee)
		}
		if item.Coins[0].Denom.String() == poolTCan.Ticker.String() {
			c.Assert(item.Coins[0].Amount.Uint64(), Equals, poolTCan.BalanceToken.Uint64()-batchTransactionFee)
		}
		if item.Coins[0].Denom.String() == poolLoki.Ticker.String() {
			c.Check(item.Coins[0].Amount.Uint64(), Equals, poolLoki.BalanceToken.Uint64()-batchTransactionFee)
		}
		if common.IsRune(item.Coins[0].Denom) {
			totalRune := poolBNB.BalanceRune.Add(poolLoki.BalanceRune).Add(poolTCan.BalanceRune).Add(totalBond)
			c.Assert(item.Coins[0].Amount.String(), Equals, totalRune.SubUint64(batchTransactionFee).String())
		}
	}
	txOutStore.CommitBlock(ctx)
}

func createTempNewPoolForTest(ctx sdk.Context, k Keeper, ticker string, c *C) *Pool {
	p := NewPool()
	t, err := common.NewTicker(ticker)
	c.Assert(err, IsNil)
	p.Ticker = t
	// limiting balance to 59 bits, because the math done with floats looses
	// precision if the number is greater than 59 bits.
	// https://stackoverflow.com/questions/30897208/how-to-change-a-float64-number-to-uint64-in-a-right-way
	// https://github.com/golang/go/issues/29463
	p.BalanceRune = sdk.NewUint(1535169738538008)
	p.BalanceToken = sdk.NewUint(1535169738538008)
	k.SetPool(ctx, p)
	return &p
}
