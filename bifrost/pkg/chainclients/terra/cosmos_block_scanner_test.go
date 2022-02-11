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
		gasMethod: GasMethodAverage,
		feeAsset:  feeAsset,
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
				common.NewCoin(feeAsset, ctypes.NewUint(25000000)),
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
				common.NewCoin(feeAsset, ctypes.NewUint(16000000)),
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

	err = blockScanner.updateGasCache(txIn)
	c.Assert(err, IsNil)

	// Ensure only 2 transactions in the cache
	c.Check(blockScanner.gasCacheNum, Equals, int64(2))

	gasAmt := blockScanner.getAverageFromCache()
	c.Check(gasAmt.BigInt().Int64(), Equals, int64(20988091))

	// Add a few more txIn
	txIn2 := []types.TxInItem{
		{
			BlockHeight: blockHeight,
			Tx:          "hash4",
			Memo:        "memo",
			Sender:      "sender",
			To:          "recipient",
			Coins:       common.NewCoins(),
			Gas: common.Gas{
				common.NewCoin(feeAsset, ctypes.NewUint(881655)),
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
				common.NewCoin(feeAsset, ctypes.NewUint(1999999929)),
			},
			ObservedVaultPubKey: common.EmptyPubKey,
		},
	}

	err = blockScanner.updateGasCache(txIn2)
	c.Assert(err, IsNil)
	c.Check(blockScanner.gasCacheNum, Equals, int64(4))

	newGasAmt := blockScanner.getAverageFromCache()
	c.Check(newGasAmt.BigInt().Int64(), Equals, int64(1000110180))
}

func (s *BlockScannerTestSuite) TestGetBlock(c *C) {
	feeAsset, err := common.NewAsset("TERRA.LUNA")
	c.Assert(err, IsNil)

	mockRPC := NewMockServiceClient()

	blockScanner := CosmosBlockScanner{
		feeAsset:  feeAsset,
		tmService: mockRPC,
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
	}

	block, err := blockScanner.GetBlock(1)
	c.Assert(err, IsNil)

	txInItems, err := blockScanner.processTxs(1, block.Data.Txs)
	c.Assert(err, IsNil)

	// proccessTxs should filter out everything besides MsgSend
	c.Assert(len(txInItems), Equals, 7)
}
