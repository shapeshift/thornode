package ethereum

import (
	"encoding/json"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/ethereum/go-ethereum/common"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/bifrost/blockscanner"
	"gitlab.com/thorchain/thornode/bifrost/config"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/ethereum/types"
	"gitlab.com/thorchain/thornode/bifrost/pubkeymanager"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	stypes "gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/cmd"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain"
)

func Test(t *testing.T) { TestingT(t) }

type BlockScannerTestSuite struct {
	m      *metrics.Metrics
	bridge *thorclient.ThorchainBridge
	keys   *thorclient.Keys
}

var _ = Suite(&BlockScannerTestSuite{})

func (s *BlockScannerTestSuite) SetUpSuite(c *C) {
	thorchain.SetupConfigForTest()
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
	s.keys = thorKeys
	s.bridge, err = thorclient.NewThorchainBridge(cfg, s.m, thorKeys)
	c.Assert(err, IsNil)
}

func getConfigForTest(rpcHost string) config.BlockScannerConfiguration {
	return config.BlockScannerConfiguration{
		RPCHost:                    rpcHost,
		StartBlockHeight:           1, // avoids querying thorchain for block height
		BlockScanProcessors:        1,
		HttpRequestTimeout:         time.Second,
		HttpRequestReadTimeout:     time.Second * 30,
		HttpRequestWriteTimeout:    time.Second * 30,
		MaxHttpRequestRetry:        3,
		BlockHeightDiscoverBackoff: time.Second,
		BlockRetryInterval:         time.Second,
	}
}

func (s *BlockScannerTestSuite) TestNewBlockScanner(c *C) {
	storage, err := blockscanner.NewBlockScannerStorage("")
	c.Assert(err, IsNil)
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		c.Assert(err, IsNil)
		type RPCRequest struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      interface{}     `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		var rpcRequest RPCRequest
		err = json.Unmarshal(body, &rpcRequest)
		c.Assert(err, IsNil)
		if rpcRequest.Method == "eth_chainId" {
			_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x539"}`))
			c.Assert(err, IsNil)
		}
		if rpcRequest.Method == "eth_gasPrice" {
			_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
			c.Assert(err, IsNil)
		}
	}))
	ethClient, err := ethclient.Dial(server.URL)
	c.Assert(err, IsNil)
	pubKeyManager, err := pubkeymanager.NewPubKeyManager(s.bridge, s.m)
	c.Assert(err, IsNil)

	bs, err := NewETHScanner(getConfigForTest(""), nil, big.NewInt(int64(types.Mainnet)), ethClient, s.bridge, s.m, pubKeyManager)
	c.Assert(err, NotNil)
	c.Assert(bs, IsNil)

	bs, err = NewETHScanner(getConfigForTest("127.0.0.1"), storage, big.NewInt(int64(types.Mainnet)), ethClient, s.bridge, nil, pubKeyManager)
	c.Assert(err, NotNil)
	c.Assert(bs, IsNil)

	bs, err = NewETHScanner(getConfigForTest("127.0.0.1"), storage, big.NewInt(int64(types.Mainnet)), nil, s.bridge, s.m, pubKeyManager)
	c.Assert(err, NotNil)
	c.Assert(bs, IsNil)

	bs, err = NewETHScanner(getConfigForTest("127.0.0.1"), storage, big.NewInt(int64(types.Mainnet)), ethClient, s.bridge, s.m, nil)
	c.Assert(err, NotNil)
	c.Assert(bs, IsNil)

	bs, err = NewETHScanner(getConfigForTest("127.0.0.1"), storage, big.NewInt(int64(types.Mainnet)), ethClient, s.bridge, s.m, pubKeyManager)
	c.Assert(err, IsNil)
	c.Assert(bs, NotNil)
}

func (s *BlockScannerTestSuite) TestProcessBlock(c *C) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch {
		case req.RequestURI == thorclient.PubKeysEndpoint:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/vaults/pubKeys.json")
		case req.RequestURI == thorclient.InboundAddressesEndpoint:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/inbound_addresses/inbound_addresses.json")
		case req.RequestURI == thorclient.AsgardVault:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/vaults/asgard.json")
		case strings.HasPrefix(req.RequestURI, thorclient.NodeAccountEndpoint):
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/nodeaccount/template.json")
		case strings.HasPrefix(req.RequestURI, thorclient.LastBlockEndpoint):
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/lastblock/bnb.json")
		case strings.HasPrefix(req.RequestURI, thorclient.AuthAccountEndpoint):
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/auth/accounts/template.json")
		default:
			body, err := ioutil.ReadAll(req.Body)
			c.Assert(err, IsNil)
			defer req.Body.Close()
			type RPCRequest struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      interface{}     `json:"id"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params"`
			}
			var rpcRequest RPCRequest
			err = json.Unmarshal(body, &rpcRequest)
			if err != nil {
				return
			}
			if rpcRequest.Method == "eth_chainId" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_gasPrice" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x3b9aca00"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_getTransactionReceipt" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"root":"0x","status":"0x1","cumulativeGasUsed":"0xf6b5","logsBloom":"0x00000000000000000002000000000000000000000000000000000000000000000000000000000000000000000000000000100800000002000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000004000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000","logs":[{"address":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","topics":["0xef519b7eb82aaf6ac376a6df2d793843ebfd593de5f1a0601d3cc6ab49ebb395","0x000000000000000000000000fb337706200a55009e6bbd41e4dc164d59bc9aa2"],"data":"0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000f4240000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000384144443a4554482e4554483a7474686f723137303471753333746c6d346c75746363343638303879683334387a726d7239796e6d77346c380000000000000000","blockNumber":"0x6","transactionHash":"0xc318fc70ec225c3e5e5fd3f59e205fdfb4d27ef878dd321b30278d891db0c6a9","transactionIndex":"0x0","blockHash":"0x2f202f8aa7355e77bfbdcd63c08f7c4e43e0bcca61b45fe6a2bdb950d777fa38","logIndex":"0x0","removed":false}],"transactionHash":"0xc318fc70ec225c3e5e5fd3f59e205fdfb4d27ef878dd321b30278d891db0c6a9","contractAddress":"0x0000000000000000000000000000000000000000","gasUsed":"0xf6b5","blockHash":"0x2f202f8aa7355e77bfbdcd63c08f7c4e43e0bcca61b45fe6a2bdb950d777fa38","blockNumber":"0x6","transactionIndex":"0x0"}}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_call" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x52554e45"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_getBlockByNumber" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"difficulty":"0x2","extraData":"0xd88301091a846765746888676f312e31352e36856c696e757800000000000000e86d9af8b427b780cd1e6f7cabd2f9231ccac25d313ed475351ed64ac19f21491461ed1fae732d3bbf73a5866112aec23b0ca436185685b9baee4f477a950f9400","gasLimit":"0x9e0f54","gasUsed":"0xabd3","hash":"0xb273789207ce61a1ec0314fdb88efe6c6b554a9505a97ff3dff05aa691e220ac","logsBloom":"0x00010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000040000000000000000010000200020000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000000000000000040000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000010000000000000000000000000000000000000000000000020000000000000","miner":"0x0000000000000000000000000000000000000000","mixHash":"0x0000000000000000000000000000000000000000000000000000000000000000","nonce":"0x0000000000000000","number":"0x6b","parentHash":"0xf18470c54efec284fb5ad57c0ee4afe2774d61393bd5224ac5484b39a0a07556","receiptsRoot":"0x794a74d56ec50769a1400f7ae0887061b0ec3ea6702589a0b45b9102df2c9954","sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347","size":"0x30a","stateRoot":"0x1c84090d7f5dc8137d6762e3d4babe10b30bf61fa827618346ae1ba8600a9629","timestamp":"0x6008f03a","totalDifficulty":"0xd7","transactions":[{"blockHash":"0xb273789207ce61a1ec0314fdb88efe6c6b554a9505a97ff3dff05aa691e220ac","blockNumber":"0x6b","from":"0xfabb9cc6ec839b1214bb11c53377a56a6ed81762","gas":"0x23273","gasPrice":"0x1","hash":"0x501d0b7fc8fcdff367280dc8b0c077f6beb9e324ad9550e2c0e34a2fa8e99aed","input":"0x095ea7b3000000000000000000000000e65e9d372f8cacc7b6dfcd4af6507851ed31bb4400000000000000000000000000000000000000000000000000000000ee6b2800","nonce":"0x1","to":"0x40bcd4db8889a8bf0b1391d0c819dcd9627f9d0a","transactionIndex":"0x0","value":"0x0","v":"0xa95","r":"0x614fa842510a4293d25ce4799a01a3d3cfeada4b79d7157c755bb4872984fff","s":"0x351e831427ca7e2f1b5f45b5101cc1d170d6fd8e7129378c8d55a6a436f403dc"}],"transactionsRoot":"0x4247bb112edbe20ee8cf406864b335f4a3aa215f65ea686c9820f056c637aca6","uncles":[]}}`))
				c.Assert(err, IsNil)
			}
		}
	}))
	ethClient, err := ethclient.Dial(server.URL)
	c.Assert(err, IsNil)
	c.Assert(ethClient, NotNil)
	storage, err := blockscanner.NewBlockScannerStorage("")
	c.Assert(err, IsNil)
	u, err := url.Parse(server.URL)
	c.Assert(err, IsNil)
	bridge, err := thorclient.NewThorchainBridge(config.ClientConfiguration{
		ChainID:         "thorchain",
		ChainHost:       u.Host,
		SignerName:      "bob",
		SignerPasswd:    "password",
		ChainHomeFolder: "",
	}, s.m, s.keys)
	c.Assert(err, IsNil)
	pubKeyMgr, err := pubkeymanager.NewPubKeyManager(bridge, s.m)
	c.Assert(err, IsNil)
	pubKeyMgr.Start()
	defer pubKeyMgr.Stop()
	bs, err := NewETHScanner(getConfigForTest(server.URL), storage, big.NewInt(1337), ethClient, bridge, s.m, pubKeyMgr)
	c.Assert(err, IsNil)
	c.Assert(bs, NotNil)
	txIn, err := bs.FetchTxs(int64(1))
	c.Assert(err, IsNil)
	c.Check(len(txIn.TxArray), Equals, 1)
}

func httpTestHandler(c *C, rw http.ResponseWriter, fixture string) {
	var content []byte
	var err error

	switch fixture {
	case "500":
		rw.WriteHeader(http.StatusInternalServerError)
	default:
		content, err = ioutil.ReadFile(fixture)
		if err != nil {
			c.Fatal(err)
		}
	}

	rw.Header().Set("Content-Type", "application/json")
	if _, err := rw.Write(content); err != nil {
		c.Fatal(err)
	}
}

func (s *BlockScannerTestSuite) TestFromTxToTxIn(c *C) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch {
		case req.RequestURI == thorclient.PubKeysEndpoint:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/vaults/pubKeys.json")
		case req.RequestURI == thorclient.InboundAddressesEndpoint:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/inbound_addresses/inbound_addresses.json")
		case req.RequestURI == thorclient.AsgardVault:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/vaults/asgard.json")
		case strings.HasPrefix(req.RequestURI, thorclient.NodeAccountEndpoint):
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/nodeaccount/template.json")
		default:
			body, err := ioutil.ReadAll(req.Body)
			c.Assert(err, IsNil)
			type RPCRequest struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      interface{}     `json:"id"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params"`
			}
			var rpcRequest RPCRequest
			err = json.Unmarshal(body, &rpcRequest)
			if err != nil {
				return
			}
			if rpcRequest.Method == "eth_chainId" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_gasPrice" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_call" {
				if string(rpcRequest.Params) == `[{"data":"0x95d89b41","from":"0x0000000000000000000000000000000000000000","to":"0x3b7fa4dd21c6f9ba3ca375217ead7cab9d6bf483"},"latest"]` ||
					string(rpcRequest.Params) == `[{"data":"0x95d89b41","from":"0x0000000000000000000000000000000000000000","to":"0x40bcd4db8889a8bf0b1391d0c819dcd9627f9d0a"},"latest"]` {
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000003544b4e0000000000000000000000000000000000000000000000000000000000"}`))
					c.Assert(err, IsNil)
					return
				} else if string(rpcRequest.Params) == `[{"data":"0x313ce567","from":"0x0000000000000000000000000000000000000000","to":"0x3b7fa4dd21c6f9ba3ca375217ead7cab9d6bf483"},"latest"]` {
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`))
					c.Assert(err, IsNil)
					return
				}
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x52554e45"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_getTransactionReceipt" {
				if string(rpcRequest.Params) == `["0xa132791c8f868ac84bcffc0c2c8076f35c0b8fa1f7358428917892f0edddc550"]` {
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"root":"0x","status":"0x1","cumulativeGasUsed":"0xe8c5","logsBloom":"0x00000000000000000002000000000000000000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000020000000000000000000800000000000000000000000800000000000000000001000000000000800000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000004000000000000000000000000000000000400000000000000000000000000000020000020000000000000000000000000000000000000000000000000000000000000","logs":[{"address":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","topics":["0xef519b7eb82aaf6ac376a6df2d793843ebfd593de5f1a0601d3cc6ab49ebb395","0x00000000000000000000000058e99c9c4a20f5f054c737389fdd51d7ed9c7d2a","0x0000000000000000000000000000000000000000000000000000000000000000"],"data":"0x0000000000000000000000000000000000000000000000004563918244f40000000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000384144443a4554482e4554483a7474686f72313678786e30636164727575773661327177707633356176306d6568727976647a7a6a7a3361660000000000000000","blockNumber":"0x22","transactionHash":"0xa132791c8f868ac84bcffc0c2c8076f35c0b8fa1f7358428917892f0edddc550","transactionIndex":"0x0","blockHash":"0x2383a22acdbe27d3c7c56a0452ae5e7edfbebeabe3a9a047c87716dafc8fa9d0","logIndex":"0x0","removed":false}],"transactionHash":"0xa132791c8f868ac84bcffc0c2c8076f35c0b8fa1f7358428917892f0edddc550","contractAddress":"0x0000000000000000000000000000000000000000","gasUsed":"0xe8c5","blockHash":"0x2383a22acdbe27d3c7c56a0452ae5e7edfbebeabe3a9a047c87716dafc8fa9d0","blockNumber":"0x22","transactionIndex":"0x0"}}`))
					c.Assert(err, IsNil)
					return
				} else if string(rpcRequest.Params) == `["0x817665ed5d08f6bcc47e409c147187fe0450201152ea1c80c85edf103d623acd"]` {
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"root":"0x","status":"0x1","cumulativeGasUsed":"0x13d20","logsBloom":"0x00000000000000000002000020000000000000000000000000000000000000000000000000004000000000000000000000000800000000000000000000000000000000000000000000000008000000000000000000000000000000000000000010000200000000000000000000000000000000000000000000000810000000000000000001010000000000800000000000000000000000000000000000040000000000000000002000000000000000000000000000003400000000000004000000000002000000000000000000000400000000000000000000000000000000000020000000000000000000002000000000000000010000800000000000000000","logs":[{"address":"0x3b7fa4dd21c6f9ba3ca375217ead7cab9d6bf483","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x0000000000000000000000003fd2d4ce97b082d4bce3f9fee2a3d60668d2f473","0x000000000000000000000000e65e9d372f8cacc7b6dfcd4af6507851ed31bb44"],"data":"0x0000000000000000000000000000000000000000000000004563918244f40000","blockNumber":"0x20","transactionHash":"0x817665ed5d08f6bcc47e409c147187fe0450201152ea1c80c85edf103d623acd","transactionIndex":"0x0","blockHash":"0xe2ac172ea4c9b390adff7b21a4fe134251e60ba1d31a1acc0fb0d3bad350e34f","logIndex":"0x0","removed":false},{"address":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","topics":["0xef519b7eb82aaf6ac376a6df2d793843ebfd593de5f1a0601d3cc6ab49ebb395","0x00000000000000000000000058e99c9c4a20f5f054c737389fdd51d7ed9c7d2a","0x0000000000000000000000003b7fa4dd21c6f9ba3ca375217ead7cab9d6bf483"],"data":"0x0000000000000000000000000000000000000000000000004563918244f40000000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000634144443a4554482e544b4e2d3078336237464134646432316336663942413363613337353231374541443743416239443662463438333a7474686f72313678786e30636164727575773661327177707633356176306d6568727976647a7a6a7a3361660000000000000000000000000000000000000000000000000000000000","blockNumber":"0x20","transactionHash":"0x817665ed5d08f6bcc47e409c147187fe0450201152ea1c80c85edf103d623acd","transactionIndex":"0x0","blockHash":"0xe2ac172ea4c9b390adff7b21a4fe134251e60ba1d31a1acc0fb0d3bad350e34f","logIndex":"0x1","removed":false}],"transactionHash":"0x817665ed5d08f6bcc47e409c147187fe0450201152ea1c80c85edf103d623acd","contractAddress":"0x0000000000000000000000000000000000000000","gasUsed":"0x13d20","blockHash":"0xe2ac172ea4c9b390adff7b21a4fe134251e60ba1d31a1acc0fb0d3bad350e34f","blockNumber":"0x20","transactionIndex":"0x0"}}`))
					c.Assert(err, IsNil)
					return
				} else if string(rpcRequest.Params) == `["0x9761b42228d5ba217691b0697dcf1cbe9aa6c8b6767afb9b21883538ddd2e9e9"]` {
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"root":"0x","status":"0x1","cumulativeGasUsed":"0x8b15","logsBloom":"0x00000000000000000002010000000000000000100000004000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000001000000000000004000000000000000000000000000000000000000000100000000000000000000000000000000000020000000000000000000000000000000000000000000000001000","logs":[{"address":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","topics":["0xa9cd03aa3c1b4515114539cd53d22085129d495cb9e9f9af77864526240f1bf7","0x00000000000000000000000088eaf40bc58dec39d9bf700b4a47bfcab6c2693e","0x000000000000000000000000f6da288748ec4c77642f6c5543717539b3ae001b"],"data":"0x0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003b986a2b000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000444f55543a3035423839443744354239343942384331304530303942374539394145424346333442313442433343433036453243313336383946354433383134414231353200000000000000000000000000000000000000000000000000000000","blockNumber":"0x66","transactionHash":"0x9761b42228d5ba217691b0697dcf1cbe9aa6c8b6767afb9b21883538ddd2e9e9","transactionIndex":"0x0","blockHash":"0xd745e1a64e9558e68eb2c1a95b7ca87cce133695b0607ab8960f91933baddd86","logIndex":"0x0","removed":false}],"transactionHash":"0x9761b42228d5ba217691b0697dcf1cbe9aa6c8b6767afb9b21883538ddd2e9e9","contractAddress":"0x0000000000000000000000000000000000000000","gasUsed":"0x8b15","blockHash":"0xd745e1a64e9558e68eb2c1a95b7ca87cce133695b0607ab8960f91933baddd86","blockNumber":"0x66","transactionIndex":"0x0"}}`))
					c.Assert(err, IsNil)
					return
				} else if string(rpcRequest.Params) == `["0x507c45e2009306f06c45f3c4b4443e16ea8876b02904272d7b7af00dbc00b6c7"]` {
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"root":"0x","status":"0x1","cumulativeGasUsed":"0xd4ca","logsBloom":"0x00000000000000000002000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004000000000000000008000000000000000000000000000000000000000002000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000001000000000000004000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000010000800000000000000000","logs":[{"address":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","topics":["0x05b90458f953d3fcb2d7fb25616a2fddeca749d0c47cc5c9832d0266b5346eea","0x0000000000000000000000003fd2d4ce97b082d4bce3f9fee2a3d60668d2f473","0x0000000000000000000000009f4aab49a9cd8fc54dcb3701846f608a6f2c44da"],"data":"0x0000000000000000000000003b7fa4dd21c6f9ba3ca375217ead7cab9d6bf48300000000000000000000000000000000000000000000000000000000000f42400000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000568656c6c6f000000000000000000000000000000000000000000000000000000","blockNumber":"0x29","transactionHash":"0x507c45e2009306f06c45f3c4b4443e16ea8876b02904272d7b7af00dbc00b6c7","transactionIndex":"0x0","blockHash":"0xa251447386519d92cf33bee032b89849adad1ba5be5461feb50b768385b5d1a9","logIndex":"0x0","removed":false}],"transactionHash":"0x507c45e2009306f06c45f3c4b4443e16ea8876b02904272d7b7af00dbc00b6c7","contractAddress":"0x0000000000000000000000000000000000000000","gasUsed":"0xd4ca","blockHash":"0xa251447386519d92cf33bee032b89849adad1ba5be5461feb50b768385b5d1a9","blockNumber":"0x29","transactionIndex":"0x0"}}`))
					c.Assert(err, IsNil)
					return
				}
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{
				"transactionHash":"0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b",
				"transactionIndex":"0x0",
				"blockNumber":"0x1",
				"blockHash":"0x78bfef68fccd4507f9f4804ba5c65eb2f928ea45b3383ade88aaa720f1209cba",
				"cumulativeGasUsed":"0xc350",
				"gasUsed":"0x4dc",
				"logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
				"logs":[],
				"status":"0x1"
			}}`))
				c.Assert(err, IsNil)
			}
		}
	}))
	ethClient, err := ethclient.Dial(server.URL)
	c.Assert(err, IsNil)
	c.Assert(ethClient, NotNil)
	storage, err := blockscanner.NewBlockScannerStorage("")
	c.Assert(err, IsNil)
	c.Assert(storage, NotNil)
	u, err := url.Parse(server.URL)
	c.Assert(err, IsNil)

	cfg := config.ClientConfiguration{
		ChainID:         "thorchain",
		ChainHost:       u.Host,
		SignerName:      "bob",
		SignerPasswd:    "password",
		ChainHomeFolder: "",
	}
	bridge, err := thorclient.NewThorchainBridge(cfg, s.m, s.keys)
	c.Assert(err, IsNil)
	c.Assert(bridge, NotNil)
	pkeyMgr, err := pubkeymanager.NewPubKeyManager(bridge, s.m)
	pkeyMgr.Start()
	defer pkeyMgr.Stop()
	c.Assert(err, IsNil)
	bs, err := NewETHScanner(getConfigForTest(server.URL), storage, big.NewInt(int64(types.Mainnet)), ethClient, s.bridge, s.m, pkeyMgr)
	c.Assert(err, IsNil)
	c.Assert(bs, NotNil)

	// send directly to ETH address
	encodedTx := `{
		"blockHash":"0x1d59ff54b1eb26b013ce3cb5fc9dab3705b415a67127a003c3e61eb445bb8df2",
		"blockNumber":"0x5daf3b",
		"from":"0xa7d9ddbe1f17865597fbd27ec712455208b6b76d",
		"gas":"0xc350",
		"gasPrice":"0x4a817c800",
		"hash":"0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b",
		"input":"0x68656c6c6f21",
		"nonce":"0x15",
		"to":"0xf02c1c8e6114b1dbe8937a39260b5b0a374432bb",
		"transactionIndex":"0x41",
		"value":"0xf3dbb76162000",
		"v":"0x25",
		"r":"0x1b5e176d927f8e9ab405058b2d2457392da3e20f328b16ddabcebc33eaac5fea",
		"s":"0x4ba69724e8f69de52f0125ad8b3c5c2cef33019bac3249e2c0a2192766d1721c"
	}`
	tx := etypes.NewTransaction(0, common.HexToAddress(ethToken), nil, 0, nil, nil)
	err = tx.UnmarshalJSON([]byte(encodedTx))
	c.Assert(err, IsNil)

	txInItem, err := bs.fromTxToTxIn(tx)
	c.Assert(err, IsNil)
	c.Assert(txInItem, NotNil)
	c.Check(txInItem.Sender, Equals, "0xa7d9ddbe1f17865597fbd27ec712455208b6b76d")
	c.Check(txInItem.To, Equals, "0xf02c1c8e6114b1dbe8937a39260b5b0a374432bb")
	c.Check(len(txInItem.Coins), Equals, 1)
	c.Check(txInItem.Coins[0].Asset.String(), Equals, "ETH.ETH")
	c.Check(
		txInItem.Coins[0].Amount.Equal(cosmos.NewUint(4290000000000000)),
		Equals,
		true,
	)
	c.Check(
		txInItem.Gas[0].Amount.Equal(cosmos.NewUint(100000)),
		Equals,
		true,
	)

	bs, err = NewETHScanner(getConfigForTest(server.URL), storage, big.NewInt(1337), ethClient, s.bridge, s.m, pkeyMgr)
	c.Assert(err, IsNil)
	c.Assert(bs, NotNil)
	// smart contract - deposit
	encodedTx = `{"nonce":"0x4","gasPrice":"0x1","gas":"0x177b8","to":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","value":"0x0","input":"0x1fece7b400000000000000000000000058e99c9c4a20f5f054c737389fdd51d7ed9c7d2a0000000000000000000000003b7fa4dd21c6f9ba3ca375217ead7cab9d6bf4830000000000000000000000000000000000000000000000004563918244f40000000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000634144443a4554482e544b4e2d3078336237464134646432316336663942413363613337353231374541443743416239443662463438333a7474686f72313678786e30636164727575773661327177707633356176306d6568727976647a7a6a7a3361660000000000000000000000000000000000000000000000000000000000","v":"0xa95","r":"0x8a82b49901d67748c6840d7417d7307a40e6093579f6f73f7222cb52622f92cd","s":"0x21a1097c02306b177a0ca1a6e9f9599a8c4bab9926893493e966253c436977fd","hash":"0x817665ed5d08f6bcc47e409c147187fe0450201152ea1c80c85edf103d623acd"}`
	tx = etypes.NewTransaction(0, common.HexToAddress(ethToken), nil, 0, nil, nil)
	c.Assert(tx.UnmarshalJSON([]byte(encodedTx)), IsNil)
	txInItem, err = bs.fromTxToTxIn(tx)
	c.Assert(err, IsNil)
	c.Assert(txInItem, NotNil)
	c.Assert(txInItem.Sender, Equals, "0x3fd2d4ce97b082d4bce3f9fee2a3d60668d2f473")
	c.Assert(txInItem.To, Equals, "0x58e99C9c4a20f5F054C737389FdD51D7eD9c7d2a")
	c.Assert(txInItem.Memo, Equals, "ADD:ETH.TKN-0x3b7FA4dd21c6f9BA3ca375217EAD7CAb9D6bF483:tthor16xxn0cadruuw6a2qwpv35av0mehryvdzzjz3af")
	c.Assert(txInItem.Tx, Equals, "817665ed5d08f6bcc47e409c147187fe0450201152ea1c80c85edf103d623acd")
	c.Assert(txInItem.Coins[0].Asset.String(), Equals, "ETH.TKN-0X3B7FA4DD21C6F9BA3CA375217EAD7CAB9D6BF483")
	c.Assert(txInItem.Coins[0].Amount.Equal(cosmos.NewUint(500000000)), Equals, true)

	// smart contract - depositETH
	encodedTx = `{"nonce":"0x5","gasPrice":"0x1","gas":"0xe8c5","to":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","value":"0x4563918244f40000","input":"0x1fece7b400000000000000000000000058e99c9c4a20f5f054c737389fdd51d7ed9c7d2a00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000384144443a4554482e4554483a7474686f72313678786e30636164727575773661327177707633356176306d6568727976647a7a6a7a3361660000000000000000","v":"0xa96","r":"0x46b81d77656e26b199438349244593b9f3131224acfc39a7e0c09e2cd08dc1d8","s":"0x36427688c3ffef46b9c99fd2b0f8e191b85dae908f9d76116a878317398382ad","hash":"0xa132791c8f868ac84bcffc0c2c8076f35c0b8fa1f7358428917892f0edddc550"}`
	tx = &etypes.Transaction{}
	c.Assert(tx.UnmarshalJSON([]byte(encodedTx)), IsNil)
	txInItem, err = bs.fromTxToTxIn(tx)
	c.Assert(err, IsNil)
	c.Assert(txInItem, NotNil)
	c.Assert(txInItem.Sender, Equals, "0x3fd2d4ce97b082d4bce3f9fee2a3d60668d2f473")
	c.Assert(txInItem.To, Equals, "0x58e99C9c4a20f5F054C737389FdD51D7eD9c7d2a")
	c.Assert(txInItem.Memo, Equals, "ADD:ETH.ETH:tthor16xxn0cadruuw6a2qwpv35av0mehryvdzzjz3af")
	c.Assert(txInItem.Tx, Equals, "a132791c8f868ac84bcffc0c2c8076f35c0b8fa1f7358428917892f0edddc550")
	c.Assert(txInItem.Coins[0].Asset.String(), Equals, "ETH.ETH")
	c.Assert(txInItem.Coins[0].Amount.Equal(cosmos.NewUint(500000000)), Equals, true)

	// smart contract - transferOut
	encodedTx = `{"nonce":"0x0","gasPrice":"0x1","gas":"0x8b15","to":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","value":"0x3b986a2b","input":"0x574da717000000000000000000000000f6da288748ec4c77642f6c5543717539b3ae001b0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003b972080000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000444f55543a3035423839443744354239343942384331304530303942374539394145424346333442313442433343433036453243313336383946354433383134414231353200000000000000000000000000000000000000000000000000000000","v":"0xa95","r":"0xb0058b2cbb0194ba1cda0539225c012fd6540f2c06246fe72013aa7d0695e8e8","s":"0x3cce02a0e2964c1f38b78e8d1168e4e28c147b5603fe12c66583a4ef84f81133","hash":"0x9761b42228d5ba217691b0697dcf1cbe9aa6c8b6767afb9b21883538ddd2e9e9"}`
	tx = &etypes.Transaction{}
	c.Assert(tx.UnmarshalJSON([]byte(encodedTx)), IsNil)
	txInItem, err = bs.fromTxToTxIn(tx)
	c.Assert(err, IsNil)
	c.Assert(txInItem, NotNil)
	c.Assert(txInItem.Sender, Equals, "0x88EaF40BC58dec39d9bf700B4a47BfCAb6c2693e")
	c.Assert(txInItem.To, Equals, "0xF6dA288748eC4c77642F6c5543717539B3Ae001b")
	c.Assert(txInItem.Memo, Equals, "OUT:05B89D7D5B949B8C10E009B7E99AEBCF34B14BC3CC06E2C13689F5D3814AB152")
	c.Assert(txInItem.Tx, Equals, "9761b42228d5ba217691b0697dcf1cbe9aa6c8b6767afb9b21883538ddd2e9e9")
	c.Assert(txInItem.Coins[0].Asset.String(), Equals, "ETH.ETH")
	c.Assert(txInItem.Coins[0].Amount.Equal(cosmos.NewUint(999844395)), Equals, true)

	// smart contract - allowance
	encodedTx = `{"nonce":"0x5","gasPrice":"0x1","gas":"0xd4ca","to":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","value":"0x0","input":"0x1b738b32000000000000000000000000e65e9d372f8cacc7b6dfcd4af6507851ed31bb440000000000000000000000009f4aab49a9cd8fc54dcb3701846f608a6f2c44da0000000000000000000000003b7fa4dd21c6f9ba3ca375217ead7cab9d6bf48300000000000000000000000000000000000000000000000000000000000f424000000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000000568656c6c6f000000000000000000000000000000000000000000000000000000","v":"0xa96","r":"0x4358ec822ab5406cda41e0d1039c6adf13edcad206d90827f1a1943ab8702d83","s":"0x67be06820911b4d63f39a5072c77ad2caddb45c71ca1dc8a1fdd9f3f5cb0d750","hash":"0x507c45e2009306f06c45f3c4b4443e16ea8876b02904272d7b7af00dbc00b6c7"}`
	tx = &etypes.Transaction{}
	c.Assert(tx.UnmarshalJSON([]byte(encodedTx)), IsNil)
	txInItem, err = bs.fromTxToTxIn(tx)
	c.Assert(err, IsNil)
	c.Assert(txInItem, NotNil)
	c.Assert(txInItem.Sender, Equals, "0x3fd2D4cE97B082d4BcE3f9fee2A3D60668D2f473")
	c.Assert(txInItem.To, Equals, "0x9F4AaB49A9cd8FC54Dcb3701846f608a6f2C44dA")
	c.Assert(txInItem.Memo, Equals, "hello")
	c.Assert(txInItem.Tx, Equals, "507c45e2009306f06c45f3c4b4443e16ea8876b02904272d7b7af00dbc00b6c7")
	c.Assert(txInItem.Coins[0].Asset.String(), Equals, "ETH.TKN-0X3B7FA4DD21C6F9BA3CA375217EAD7CAB9D6BF483")
	c.Assert(txInItem.Coins[0].Amount.Equal(cosmos.NewUint(1000000)), Equals, true)
}

func (s *BlockScannerTestSuite) TestProcessReOrg(c *C) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch {
		case req.RequestURI == thorclient.PubKeysEndpoint:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/vaults/pubKeys.json")
		case req.RequestURI == thorclient.InboundAddressesEndpoint:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/inbound_addresses/inbound_addresses.json")
		case req.RequestURI == thorclient.AsgardVault:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/vaults/asgard.json")
		case strings.HasPrefix(req.RequestURI, thorclient.NodeAccountEndpoint):
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/nodeaccount/template.json")
		default:
			body, err := ioutil.ReadAll(req.Body)
			c.Assert(err, IsNil)
			type RPCRequest struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      interface{}     `json:"id"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params"`
			}
			var rpcRequest RPCRequest
			err = json.Unmarshal(body, &rpcRequest)
			c.Assert(err, IsNil)
			if rpcRequest.Method == "eth_chainId" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_gasPrice" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_getTransactionReceipt" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32700,"message":"Not found tx"},"id": null}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_call" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x52554e45"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_getBlockByNumber" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{
				"parentHash":"0x8b535592eb3192017a527bbf8e3596da86b3abea51d6257898b2ced9d3a83826",
				"difficulty": "0x31962a3fc82b",
				"extraData": "0x4477617266506f6f6c",
				"gasLimit": "0x47c3d8",
				"gasUsed": "0x0",
				"hash": "0x78bfef68fccd4507f9f4804ba5c65eb2f928ea45b3383ade88aaa720f1209cba",
				"logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
				"miner": "0x2a65aca4d5fc5b5c859090a6c34d164135398226",
				"nonce": "0xa5e8fb780cc2cd5e",
				"number": "0x0",
				"receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
				"sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
				"size": "0x20e",
				"stateRoot": "0xdc6ed0a382e50edfedb6bd296892690eb97eb3fc88fd55088d5ea753c48253dc",
				"timestamp": "0x579f4981",
				"totalDifficulty": "0x25cff06a0d96f4bee",
				"transactions": [{
					"blockHash":"0x78bfef68fccd4507f9f4804ba5c65eb2f928ea45b3383ade88aaa720f1209cba",
					"blockNumber":"0x1",
					"from":"0xa7d9ddbe1f17865597fbd27ec712455208b6b76d",
					"gas":"0xc350",
					"gasPrice":"0x4a817c800",
					"hash":"0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b",
					"input":"0x68656c6c6f21",
					"nonce":"0x15",
					"to":"0xf02c1c8e6114b1dbe8937a39260b5b0a374432bb",
					"transactionIndex":"0x0",
					"value":"0xf3dbb76162000",
					"v":"0x25",
					"r":"0x1b5e176d927f8e9ab405058b2d2457392da3e20f328b16ddabcebc33eaac5fea",
					"s":"0x4ba69724e8f69de52f0125ad8b3c5c2cef33019bac3249e2c0a2192766d1721c"
				}],
				"transactionsRoot": "0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b",
				"uncles": [
			]}}`))
				c.Assert(err, IsNil)
			}
		}
	}))
	ethClient, err := ethclient.Dial(server.URL)
	c.Assert(err, IsNil)
	c.Assert(ethClient, NotNil)
	storage, err := blockscanner.NewBlockScannerStorage("")
	c.Assert(err, IsNil)
	bridge, err := thorclient.NewThorchainBridge(config.ClientConfiguration{
		ChainID:         "thorchain",
		ChainHost:       server.Listener.Addr().String(),
		SignerName:      "bob",
		SignerPasswd:    "password",
		ChainHomeFolder: "",
	}, s.m, s.keys)
	c.Assert(err, IsNil)
	c.Assert(bridge, NotNil)
	pkeyMgr, err := pubkeymanager.NewPubKeyManager(bridge, s.m)
	pkeyMgr.Start()
	defer pkeyMgr.Stop()
	bs, err := NewETHScanner(getConfigForTest(server.URL), storage, big.NewInt(int64(types.Mainnet)), ethClient, s.bridge, s.m, pkeyMgr)
	c.Assert(err, IsNil)
	c.Assert(bs, NotNil)
	block, err := CreateBlock(0)
	c.Assert(err, IsNil)
	c.Assert(block, NotNil)
	blockNew, err := CreateBlock(1)
	c.Assert(err, IsNil)
	c.Assert(blockNew, NotNil)
	blockMeta := types.NewBlockMeta(block, stypes.TxIn{TxArray: []stypes.TxInItem{{Tx: "0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b"}}})
	// add one UTXO which will trigger the re-org process next
	c.Assert(bs.blockMetaAccessor.SaveBlockMeta(0, blockMeta), IsNil)
	bs.globalErrataQueue = make(chan stypes.ErrataBlock, 1)
	c.Assert(bs.processReorg(blockNew), IsNil)
	// make sure there is errata block in the queue
	c.Assert(bs.globalErrataQueue, HasLen, 1)
	blockMeta, err = bs.blockMetaAccessor.GetBlockMeta(0)
	c.Assert(err, IsNil)
	c.Assert(blockMeta, NotNil)
}
