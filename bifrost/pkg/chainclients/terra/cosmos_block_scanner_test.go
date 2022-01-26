package terra

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	ctypes "github.com/cosmos/cosmos-sdk/types"
	"gitlab.com/thorchain/thornode/app"
	"gitlab.com/thorchain/thornode/bifrost/config"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/common"

	"gitlab.com/thorchain/thornode/cmd"
	. "gopkg.in/check.v1"
)

type BlockScannerTestSuite struct {
	m      *metrics.Metrics
	bridge *thorclient.ThorchainBridge
	keys   *thorclient.Keys
}

var _ = Suite(&BlockScannerTestSuite{})

func (s *BlockScannerTestSuite) SetUpSuite(c *C) {
	s.m = GetMetricForTest(c)
	c.Assert(s.m, NotNil)
	cfg := config.ClientConfiguration{
		ChainID:         "thorchain",
		ChainHost:       "localhost",
		SignerName:      "bob",
		SignerPasswd:    "password",
		ChainHomeFolder: "",
	}

	kb := cKeys.NewInMemory()
	_, _, err := kb.NewMnemonic(cfg.SignerName, cKeys.English, cmd.THORChainHDPath, hd.Secp256k1)
	c.Assert(err, IsNil)
	thorKeys := thorclient.NewKeysWithKeybase(kb, cfg.SignerName, cfg.SignerPasswd)
	c.Assert(err, IsNil)
	s.bridge, err = thorclient.NewThorchainBridge(cfg, s.m, thorKeys)
	c.Assert(err, IsNil)
	s.keys = thorKeys
}

func (s *BlockScannerTestSuite) TestCalculateAverageGasFees(c *C) {
	feeAsset, err := common.NewAsset("TERRA.LUNA")
	c.Assert(err, IsNil)

	blockScanner := CosmosBlockScanner{
		feeAsset: feeAsset,
	}
	blockHeight := int64(1)

	nonFeeAsset, err := common.NewAsset("TERRA.CW20")
	c.Assert(err, IsNil)

	// Only transactions with gas paid in the fee asset are relevant here.
	// One for 1 LUNA and another for 3 LUNA.
	// We should expect the average gas fee to be 2 LUNA.
	txIn := []types.TxInItem{
		{
			BlockHeight: blockHeight,
			Tx:          "hash1",
			Memo:        "memo",
			Sender:      "sender",
			To:          "recipient",
			Coins:       common.NewCoins(),
			Gas: common.Gas{
				common.NewCoin(feeAsset, ctypes.NewUint(100000000)),
			},
			ObservedVaultPubKey: common.EmptyPubKey,
		},
		{
			BlockHeight: blockHeight,
			Tx:          "hash2",
			Memo:        "memo",
			Sender:      "sender",
			To:          "recipient",
			Coins:       common.NewCoins(),
			Gas: common.Gas{
				common.NewCoin(feeAsset, ctypes.NewUint(300000000)),
			},
			ObservedVaultPubKey: common.EmptyPubKey,
		},
		{
			BlockHeight: blockHeight,
			Tx:          "hash3",
			Memo:        "memo",
			Sender:      "sender",
			To:          "recipient",
			Coins:       common.NewCoins(),
			Gas: common.Gas{
				// Make sure that transactions paid in asset other than fee asset
				// are not included in the average
				common.NewCoin(nonFeeAsset, ctypes.NewUint(500000000)),
			},
			ObservedVaultPubKey: common.EmptyPubKey,
		},
	}
	avgGasFees, err := blockScanner.calculateAverageGasFees(blockHeight, txIn)
	c.Assert(err, IsNil)
	c.Check(avgGasFees.BigInt().Int64(), Equals, int64(200000000))
}

func (s *BlockScannerTestSuite) TestGetBlock(c *C) {
	feeAsset, err := common.NewAsset("TERRA.LUNA")
	c.Assert(err, IsNil)

	mockRpc := NewMockServiceClient()

	blockScanner := CosmosBlockScanner{
		feeAsset:  feeAsset,
		tmService: mockRpc,
	}

	block, err := blockScanner.GetBlock(1)

	c.Assert(err, IsNil)
	c.Assert(len(block.Data.Txs), Equals, 50)
	c.Assert(block.Header.Height, Equals, int64(6000011))
}

func (s *BlockScannerTestSuite) TestProcessTxs(c *C) {
	feeAsset, err := common.NewAsset("TERRA.LUNA")
	c.Assert(err, IsNil)

	mockRpc := NewMockServiceClient()

	encodingConfig := app.MakeEncodingConfig()
	ctx := client.Context{}
	ctx = ctx.WithInterfaceRegistry(encodingConfig.InterfaceRegistry)
	cdc := codec.NewProtoCodec(ctx.InterfaceRegistry)

	blockScanner := CosmosBlockScanner{
		feeAsset:  feeAsset,
		tmService: mockRpc,
		cdc:       cdc,
		avgGasFee: ctypes.NewUint(0),
	}

	block, err := blockScanner.GetBlock(1)
	c.Assert(err, IsNil)

	txInItems, err := blockScanner.processTxs(1, block.Data.Txs)
	c.Assert(err, IsNil)

	// proccessTxs should filter out everything besides MsgSend
	c.Assert(len(txInItems), Equals, 7)
}
