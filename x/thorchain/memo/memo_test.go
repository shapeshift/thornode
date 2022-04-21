package thorchain

import (
	"fmt"
	"testing"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	kv1 "gitlab.com/thorchain/thornode/x/thorchain/keeper/v1"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

type MemoSuite struct{}

func TestPackage(t *testing.T) { TestingT(t) }

var _ = Suite(&MemoSuite{})

func (s *MemoSuite) SetUpSuite(c *C) {
	types.SetupConfigForTest()
}

func (s *MemoSuite) TestTxType(c *C) {
	for _, trans := range []TxType{TxAdd, TxWithdraw, TxSwap, TxOutbound, TxDonate, TxBond, TxUnbond, TxLeave, TxSwitch} {
		tx, err := StringToTxType(trans.String())
		c.Assert(err, IsNil)
		c.Check(tx, Equals, trans)
		c.Check(tx.IsEmpty(), Equals, false)
	}
}

func (s *MemoSuite) TestParseWithAbbreviated(c *C) {
	ctx := cosmos.Context{}
	k := kv1.KVStore{}

	// happy paths
	memo, err := ParseMemoWithTHORNames(ctx, k, "d:"+common.RuneAsset().String())
	c.Assert(err, IsNil)
	c.Check(memo.GetAsset().String(), Equals, common.RuneAsset().String())
	c.Check(memo.IsType(TxDonate), Equals, true, Commentf("MEMO: %+v", memo))
	c.Check(memo.IsInbound(), Equals, true)
	c.Check(memo.IsInternal(), Equals, false)
	c.Check(memo.IsOutbound(), Equals, false)

	memo, err = ParseMemoWithTHORNames(ctx, k, "+:"+common.RuneAsset().String())
	c.Assert(err, IsNil)
	c.Check(memo.GetAsset().String(), Equals, common.RuneAsset().String())
	c.Check(memo.IsType(TxAdd), Equals, true, Commentf("MEMO: %+v", memo))
	c.Check(memo.IsInbound(), Equals, true)
	c.Check(memo.IsInternal(), Equals, false)
	c.Check(memo.IsOutbound(), Equals, false)

	_, err = ParseMemoWithTHORNames(ctx, k, "add:BTC.BTC:tbnb1yeuljgpkg2c2qvx3nlmgv7gvnyss6ye2u8rasf:xxxx")
	c.Assert(err, IsNil)

	memo, err = ParseMemoWithTHORNames(ctx, k, fmt.Sprintf("-:%s:25", common.RuneAsset().String()))
	c.Assert(err, IsNil)
	c.Check(memo.GetAsset().String(), Equals, common.RuneAsset().String())
	c.Check(memo.IsType(TxWithdraw), Equals, true, Commentf("MEMO: %+v", memo))
	c.Check(memo.GetAmount().Uint64(), Equals, uint64(25), Commentf("%d", memo.GetAmount().Uint64()))
	c.Check(memo.IsInbound(), Equals, true)
	c.Check(memo.IsInternal(), Equals, false)
	c.Check(memo.IsOutbound(), Equals, false)

	memo, err = ParseMemoWithTHORNames(ctx, k, "=:"+common.RuneAsset().String()+":bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6:870000000")
	c.Assert(err, IsNil)
	c.Check(memo.GetAsset().String(), Equals, common.RuneAsset().String())
	c.Check(memo.IsType(TxSwap), Equals, true, Commentf("MEMO: %+v", memo))
	c.Check(memo.GetDestination().String(), Equals, "bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6")
	c.Log(memo.GetSlipLimit().Uint64())
	c.Check(memo.GetSlipLimit().Equal(cosmos.NewUint(870000000)), Equals, true)
	c.Check(memo.IsInbound(), Equals, true)
	c.Check(memo.IsInternal(), Equals, false)
	c.Check(memo.IsOutbound(), Equals, false)

	memo, err = ParseMemoWithTHORNames(ctx, k, "=:"+common.RuneAsset().String()+":bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6")
	c.Assert(err, IsNil)
	c.Check(memo.GetAsset().String(), Equals, common.RuneAsset().String())
	c.Check(memo.IsType(TxSwap), Equals, true, Commentf("MEMO: %+v", memo))
	c.Check(memo.GetDestination().String(), Equals, "bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6")
	c.Check(memo.GetSlipLimit().Uint64(), Equals, uint64(0))
	c.Check(memo.IsInbound(), Equals, true)

	memo, err = ParseMemoWithTHORNames(ctx, k, "=:"+common.RuneAsset().String()+":bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6:")
	c.Assert(err, IsNil)
	c.Check(memo.GetAsset().String(), Equals, common.RuneAsset().String())
	c.Check(memo.IsType(TxSwap), Equals, true, Commentf("MEMO: %+v", memo))
	c.Check(memo.GetDestination().String(), Equals, "bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6")
	c.Check(memo.GetSlipLimit().Equal(cosmos.ZeroUint()), Equals, true)

	memo, err = ParseMemoWithTHORNames(ctx, k, "OUT:MUKVQILIHIAUSEOVAXBFEZAJKYHFJYHRUUYGQJZGFYBYVXCXYNEMUOAIQKFQLLCX")
	c.Assert(err, IsNil)
	c.Check(memo.IsType(TxOutbound), Equals, true, Commentf("%s", memo.GetType()))
	c.Check(memo.IsOutbound(), Equals, true)
	c.Check(memo.IsInbound(), Equals, false)
	c.Check(memo.IsInternal(), Equals, false)

	memo, err = ParseMemoWithTHORNames(ctx, k, "REFUND:MUKVQILIHIAUSEOVAXBFEZAJKYHFJYHRUUYGQJZGFYBYVXCXYNEMUOAIQKFQLLCX")
	c.Assert(err, IsNil)
	c.Check(memo.IsType(TxRefund), Equals, true)
	c.Check(memo.IsOutbound(), Equals, true)

	memo, err = ParseMemoWithTHORNames(ctx, k, "leave:whatever")
	c.Assert(err, NotNil)
	c.Check(memo.IsType(TxUnknown), Equals, true)

	addr := types.GetRandomBech32Addr()
	memo, err = ParseMemoWithTHORNames(ctx, k, fmt.Sprintf("leave:%s", addr.String()))
	c.Assert(err, IsNil)
	c.Check(memo.IsType(TxLeave), Equals, true)
	c.Check(memo.GetAccAddress().String(), Equals, addr.String())

	memo, err = ParseMemoWithTHORNames(ctx, k, "yggdrasil+:30")
	c.Assert(err, IsNil)
	c.Check(memo.IsType(TxYggdrasilFund), Equals, true)
	c.Check(memo.IsInbound(), Equals, false)
	c.Check(memo.IsInternal(), Equals, true)
	memo, err = ParseMemoWithTHORNames(ctx, k, "yggdrasil-:30")
	c.Assert(err, IsNil)
	c.Check(memo.IsType(TxYggdrasilReturn), Equals, true)
	c.Check(memo.IsInternal(), Equals, true)
	memo, err = ParseMemoWithTHORNames(ctx, k, "migrate:100")
	c.Assert(err, IsNil)
	c.Check(memo.IsType(TxMigrate), Equals, true)
	c.Check(memo.IsInternal(), Equals, true)

	memo, err = ParseMemoWithTHORNames(ctx, k, "ragnarok:100")
	c.Assert(err, IsNil)
	c.Check(memo.IsType(TxRagnarok), Equals, true)
	c.Check(memo.IsOutbound(), Equals, true)

	mem := fmt.Sprintf("switch:%s", types.GetRandomBech32Addr())
	memo, err = ParseMemoWithTHORNames(ctx, k, mem)
	c.Assert(err, IsNil)
	c.Check(memo.IsType(TxSwitch), Equals, true)
	c.Check(memo.IsInbound(), Equals, true)

	memo, err = ParseMemoWithTHORNames(ctx, k, "reserve")
	c.Check(err, IsNil)
	c.Check(memo.IsType(TxReserve), Equals, true)
	c.Check(memo.IsInbound(), Equals, true)
	c.Check(memo.IsInternal(), Equals, false)
	c.Check(memo.IsOutbound(), Equals, false)

	memo, err = ParseMemoWithTHORNames(ctx, k, "noop")
	c.Check(err, IsNil)
	c.Check(memo.IsType(TxNoOp), Equals, true)
	c.Check(memo.IsInbound(), Equals, true)
	c.Check(memo.IsInternal(), Equals, false)
	c.Check(memo.IsOutbound(), Equals, false)

	memo, err = ParseMemoWithTHORNames(ctx, k, "noop:novault")
	c.Check(err, IsNil)
	c.Check(memo.IsType(TxNoOp), Equals, true)
	c.Check(memo.IsInbound(), Equals, true)
	c.Check(memo.IsInternal(), Equals, false)
	c.Check(memo.IsOutbound(), Equals, false)

	// unhappy paths
	_, err = ParseMemoWithTHORNames(ctx, k, "")
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "bogus")
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "CREATE") // missing symbol
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "c:") // bad symbol
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "-:bnb") // withdraw basis points is optional
	c.Assert(err, IsNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "-:bnb:twenty-two") // bad amount
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "=:bnb:bad_DES:5.6") // bad destination
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, ">:bnb:bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6:five") // bad slip limit
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "!:key:val") // not enough arguments
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "!:bogus:key:value") // bogus admin command type
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "nextpool:whatever")
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "migrate")
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "switch")
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "switch:")
	c.Assert(err, NotNil)
}

func (s *MemoSuite) TestParse(c *C) {
	ctx := cosmos.Context{}
	k := kv1.KVStore{}
	k.SetVersion(types.GetCurrentVersion())

	// happy paths
	memo, err := ParseMemoWithTHORNames(ctx, k, "d:"+common.RuneAsset().String())
	c.Assert(err, IsNil)
	c.Check(memo.GetAsset().String(), Equals, common.RuneAsset().String())
	c.Check(memo.IsType(TxDonate), Equals, true, Commentf("MEMO: %+v", memo))
	c.Check(memo.String(), Equals, "DONATE:"+common.RuneAsset().String())

	memo, err = ParseMemoWithTHORNames(ctx, k, "ADD:"+common.RuneAsset().String())
	c.Assert(err, IsNil)
	c.Check(memo.GetAsset().String(), Equals, common.RuneAsset().String())
	c.Check(memo.IsType(TxAdd), Equals, true, Commentf("MEMO: %+v", memo))
	c.Check(memo.String(), Equals, "")

	_, err = ParseMemoWithTHORNames(ctx, k, "ADD:BTC.BTC")
	c.Assert(err, IsNil)
	memo, err = ParseMemoWithTHORNames(ctx, k, "ADD:BTC.BTC:bc1qwqdg6squsna38e46795at95yu9atm8azzmyvckulcc7kytlcckxswvvzej")
	c.Assert(err, IsNil)
	c.Check(memo.GetDestination().String(), Equals, "bc1qwqdg6squsna38e46795at95yu9atm8azzmyvckulcc7kytlcckxswvvzej")
	c.Check(memo.IsType(TxAdd), Equals, true, Commentf("MEMO: %+v", memo))

	_, err = ParseMemoWithTHORNames(ctx, k, "ADD:BNB.BNB:tbnb18f55frcvknxvcpx2vvpfedvw4l8eutuhca3lll:tthor176xrckly4p7efq7fshhcuc2kax3dyxu9hguzl7:1000")
	c.Assert(err, IsNil)

	memo, err = ParseMemoWithTHORNames(ctx, k, "WITHDRAW:"+common.RuneAsset().String()+":25")
	c.Assert(err, IsNil)
	c.Check(memo.GetAsset().String(), Equals, common.RuneAsset().String())
	c.Check(memo.IsType(TxWithdraw), Equals, true, Commentf("MEMO: %+v", memo))
	c.Check(memo.GetAmount().Equal(cosmos.NewUint(25)), Equals, true, Commentf("%d", memo.GetAmount().Uint64()))

	memo, err = ParseMemoWithTHORNames(ctx, k, "SWAP:"+common.RuneAsset().String()+":bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6:870000000")
	c.Assert(err, IsNil)
	c.Check(memo.GetAsset().String(), Equals, common.RuneAsset().String())
	c.Check(memo.IsType(TxSwap), Equals, true, Commentf("MEMO: %+v", memo))
	c.Check(memo.GetDestination().String(), Equals, "bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6")
	c.Log(memo.GetSlipLimit().String())
	c.Check(memo.GetSlipLimit().Equal(cosmos.NewUint(870000000)), Equals, true)

	memo, err = ParseMemoWithTHORNames(ctx, k, "SWAP:"+common.RuneAsset().String()+":bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6")
	c.Assert(err, IsNil)
	c.Check(memo.GetAsset().String(), Equals, common.RuneAsset().String())
	c.Check(memo.IsType(TxSwap), Equals, true, Commentf("MEMO: %+v", memo))
	c.Check(memo.GetDestination().String(), Equals, "bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6")
	c.Check(memo.GetSlipLimit().Uint64(), Equals, uint64(0))

	memo, err = ParseMemoWithTHORNames(ctx, k, "SWAP:"+common.RuneAsset().String()+":bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6:")
	c.Assert(err, IsNil)
	c.Check(memo.GetAsset().String(), Equals, common.RuneAsset().String())
	c.Check(memo.IsType(TxSwap), Equals, true, Commentf("MEMO: %+v", memo))
	c.Check(memo.GetDestination().String(), Equals, "bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6")
	c.Check(memo.GetSlipLimit().Uint64(), Equals, uint64(0))

	whiteListAddr := types.GetRandomBech32Addr()
	bondProvider := types.GetRandomBech32Addr()
	memo, err = ParseMemoWithTHORNames(ctx, k, fmt.Sprintf("BOND:%s:%s", whiteListAddr, bondProvider))
	c.Assert(err, IsNil)
	c.Assert(memo.IsType(TxBond), Equals, true)
	c.Assert(memo.GetAccAddress().String(), Equals, whiteListAddr.String())
	mem, err := ParseBondMemo(k.Version(), []string{"BOND", whiteListAddr.String(), bondProvider.String()})
	c.Assert(err, IsNil)
	c.Assert(mem.BondProviderAddress.String(), Equals, bondProvider.String())
	c.Assert(mem.NodeOperatorFee, Equals, int64(-1))
	mem, err = ParseBondMemo(k.Version(), []string{"BOND", whiteListAddr.String(), bondProvider.String(), "0"})
	c.Assert(err, IsNil)
	c.Assert(mem.BondProviderAddress.String(), Equals, bondProvider.String())
	c.Assert(mem.NodeOperatorFee, Equals, int64(0))
	mem, err = ParseBondMemo(k.Version(), []string{"BOND", whiteListAddr.String(), bondProvider.String(), "1000"})
	c.Assert(err, IsNil)
	c.Assert(mem.BondProviderAddress.String(), Equals, bondProvider.String())
	c.Assert(mem.NodeOperatorFee, Equals, int64(1000))

	memo, err = ParseMemoWithTHORNames(ctx, k, "leave:"+types.GetRandomBech32Addr().String())
	c.Assert(err, IsNil)
	c.Assert(memo.IsType(TxLeave), Equals, true)

	memo, err = ParseMemoWithTHORNames(ctx, k, "unbond:"+whiteListAddr.String()+":300")
	c.Assert(err, IsNil)
	c.Assert(memo.IsType(TxUnbond), Equals, true)
	c.Assert(memo.GetAccAddress().String(), Equals, whiteListAddr.String())
	c.Assert(memo.GetAmount().Equal(cosmos.NewUint(300)), Equals, true)
	unbondMemo, err := ParseUnbondMemo(k.Version(), []string{"UNBOND", whiteListAddr.String(), "400", bondProvider.String()})
	c.Assert(err, IsNil)
	c.Assert(unbondMemo.BondProviderAddress.String(), Equals, bondProvider.String())

	memo, err = ParseMemoWithTHORNames(ctx, k, "migrate:100")
	c.Assert(err, IsNil)
	c.Check(memo.IsType(TxMigrate), Equals, true)
	c.Check(memo.GetBlockHeight(), Equals, int64(100))
	c.Check(memo.String(), Equals, "MIGRATE:100")

	txID := types.GetRandomTxHash()
	memo, err = ParseMemoWithTHORNames(ctx, k, "OUT:"+txID.String())
	c.Check(err, IsNil)
	c.Check(memo.IsOutbound(), Equals, true)
	c.Check(memo.GetTxID(), Equals, txID)
	c.Check(memo.String(), Equals, "OUT:"+txID.String())

	refundMemo := "REFUND:" + txID.String()
	memo, err = ParseMemoWithTHORNames(ctx, k, refundMemo)
	c.Check(err, IsNil)
	c.Check(memo.GetTxID(), Equals, txID)
	c.Check(memo.String(), Equals, refundMemo)

	yggFundMemo := "YGGDRASIL+:100"
	memo, err = ParseMemoWithTHORNames(ctx, k, yggFundMemo)
	c.Check(err, IsNil)
	c.Check(memo.GetBlockHeight(), Equals, int64(100))
	c.Check(memo.String(), Equals, yggFundMemo)

	yggReturnMemo := "YGGDRASIL-:100"
	memo, err = ParseMemoWithTHORNames(ctx, k, yggReturnMemo)
	c.Check(err, IsNil)
	c.Check(memo.GetBlockHeight(), Equals, int64(100))
	c.Check(memo.String(), Equals, yggReturnMemo)

	ragnarokMemo := "RAGNAROK:1024"
	memo, err = ParseMemoWithTHORNames(ctx, k, ragnarokMemo)
	c.Check(err, IsNil)
	c.Check(memo.IsType(TxRagnarok), Equals, true)
	c.Check(memo.GetBlockHeight(), Equals, int64(1024))
	c.Check(memo.String(), Equals, ragnarokMemo)

	baseMemo := MemoBase{}
	c.Check(baseMemo.String(), Equals, "")
	c.Check(baseMemo.GetAmount().Uint64(), Equals, cosmos.ZeroUint().Uint64())
	c.Check(baseMemo.GetDestination(), Equals, common.NoAddress)
	c.Check(baseMemo.GetSlipLimit().Uint64(), Equals, cosmos.ZeroUint().Uint64())
	c.Check(baseMemo.GetTxID(), Equals, common.TxID(""))
	c.Check(baseMemo.GetAccAddress().Empty(), Equals, true)
	c.Check(baseMemo.IsEmpty(), Equals, true)
	c.Check(baseMemo.GetBlockHeight(), Equals, int64(0))

	// unhappy paths
	_, err = ParseMemoWithTHORNames(ctx, k, "")
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "bogus")
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "CREATE") // missing symbol
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "CREATE:") // bad symbol
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "withdraw") // not enough parameters
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "withdraw:bnb") // withdraw basis points is optional
	c.Assert(err, IsNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "withdraw:bnb:twenty-two") // bad amount
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "swap") // not enough parameters
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "swap:bnb:PROVIDER-1:5.6") // bad destination
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "swap:bnb:bad_DES:5.6") // bad destination
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "swap:bnb:bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6:five") // bad slip limit
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "admin:key:val") // not enough arguments
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "admin:bogus:key:value") // bogus admin command type
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "migrate:abc")
	c.Assert(err, NotNil)

	_, err = ParseMemoWithTHORNames(ctx, k, "withdraw:A")
	c.Assert(err, IsNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "leave")
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "out") // not enough parameter
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "bond") // not enough parameter
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "refund") // not enough parameter
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "yggdrasil+") // not enough parameter
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "yggdrasil+:A") // invalid block height
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "yggdrasil-") // not enough parameter
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "yggdrasil-:B") // invalid block height
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "ragnarok") // not enough parameter
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "ragnarok:what") // not enough parameter
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "bond:what") // invalid address
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "switch:what") // invalid address
	c.Assert(err, NotNil)
	_, err = ParseMemoWithTHORNames(ctx, k, "whatever") // not support
	c.Assert(err, NotNil)
}
