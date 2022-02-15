package terra

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	ctypes "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	btypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	. "gopkg.in/check.v1"
)

type UtilTestSuite struct{}

var _ = Suite(&UtilTestSuite{})

func (s *UtilTestSuite) SetUpSuite(c *C) {}

func (s *UtilTestSuite) TestFromCosmosToThorchain(c *C) {
	// 5 UST, 6 decimals
	cosmosCoin := cosmos.NewCoin("uusd", ctypes.NewInt(5000000))
	thorchainCoin, err := fromCosmosToThorchain(cosmosCoin)
	c.Assert(err, IsNil)

	// 5 UST, 8 decimals
	expectedThorchainAsset, err := common.NewAsset("TERRA.UST")
	c.Assert(err, IsNil)
	expectedThorchainAmount := ctypes.NewUint(500000000)
	c.Check(thorchainCoin.Asset.Equals(expectedThorchainAsset), Equals, true)
	c.Check(thorchainCoin.Amount.BigInt().Int64(), Equals, expectedThorchainAmount.BigInt().Int64())
	c.Check(thorchainCoin.Decimals, Equals, int64(6))
}

func (s *UtilTestSuite) TestFromThorchainToCosmos(c *C) {
	// 6 TERRA.USD, 8 decimals
	thorchainAsset, err := common.NewAsset("TERRA.UST")
	c.Assert(err, IsNil)
	thorchainCoin := common.Coin{
		Asset:    thorchainAsset,
		Amount:   cosmos.NewUint(600000000),
		Decimals: 6,
	}
	cosmosCoin, err := fromThorchainToCosmos(thorchainCoin)
	c.Assert(err, IsNil)

	// 6 uusd, 6 decimals
	expectedCosmosDenom := "uusd"
	expectedCosmosAmount := int64(6000000)
	c.Check(cosmosCoin.Denom, Equals, expectedCosmosDenom)
	c.Check(cosmosCoin.Amount.Int64(), Equals, expectedCosmosAmount)
}

func (s UtilTestSuite) TestGetDummyTxBuilderForSimulate(c *C) {
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	interfaceRegistry.RegisterImplementations((*ctypes.Msg)(nil), &btypes.MsgSend{})
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	txConfig := tx.NewTxConfig(marshaler, []signingtypes.SignMode{signingtypes.SignMode_SIGN_MODE_DIRECT})

	txb, err := getDummyTxBuilderForSimulate(txConfig)
	c.Assert(err, IsNil)

	tx := txb.GetTx()
	c.Check(tx.GetMemo(), Equals, "ADD:TERRA.SOMELONGCOIN:sthor1x2nh4jevz7z54j9826sluzjjpvncmh3a399cec")
	c.Check(tx.GetGas(), Equals, uint64(GasLimit))
	c.Check(tx.GetFee().IsEqual(ctypes.NewCoins(ctypes.NewCoin("uluna", ctypes.NewInt(1000)))), Equals, true)
}
