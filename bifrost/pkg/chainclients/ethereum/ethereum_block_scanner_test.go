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
			_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
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
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"difficulty":"0x2","extraData":"0xd88301090e846765746888676f312e31342e32856c696e757800000000000000ef855333e6b03b825c2f1381f111e278232688e21ba8c36aa35689505d9470704420825b302cd70cc6610f1334a3d7c801ac4b8871bd9f0692c1c96f0f60ee0f01","gasLimit":"0x7a1200","gasUsed":"0xfbc9","hash":"0x45f139a64f563e12f61824a4b44edc2c955818d176b160538ae24f566a006c00","logsBloom":"0x00000000000000000002000000000000000000100000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000400000000000800000000080000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000004000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000","miner":"0x0000000000000000000000000000000000000000","mixHash":"0x0000000000000000000000000000000000000000000000000000000000000000","nonce":"0x0000000000000000","number":"0x7","parentHash":"0x2f202f8aa7355e77bfbdcd63c08f7c4e43e0bcca61b45fe6a2bdb950d777fa38","receiptsRoot":"0xe1cf0352843e29447633b9f1710e443f2582691e4278febf322c0bb7f86202cc","sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347","size":"0x38c","stateRoot":"0x303f9a24ba76fa8f350d36f4cef139e6be023f95646e2602cf9e6f939f91beea","timestamp":"0x5fde861b","totalDifficulty":"0xf","transactions":[{"blockHash":"0x45f139a64f563e12f61824a4b44edc2c955818d176b160538ae24f566a006c00","blockNumber":"0x7","from":"0xfb337706200a55009e6bbd41e4dc164d59bc9aa2","gas":"0x17cdc","gasPrice":"0x1","hash":"0x042602a2dff77111f3e711ab7c81adbcbc9a2d87973f4afb8dc0f2856021ec74","input":"0x31a053cf000000000000000000000000fd5111db462a68cfd6df19fb110dc8e9116a90e9000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000444f55543a3841313034343144354241424535443444434138443531324646363236313039394135343741393739394536334337323238384530453742303534313444433200000000000000000000000000000000000000000000000000000000","nonce":"0x0","to":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","transactionIndex":"0x0","value":"0xd6d8","v":"0x41","r":"0xbce697be8572d1543cd8c191c409cee2b4999a538e707286b5e14f7e8ff442b8","s":"0x4b8f8e8a14fb60dbe981f6ddbb31300bbc2ce8753ad6b82bdce8147280cd8e43"}],"transactionsRoot":"0xd42e9b932bffb89da313a7f9370d83c2fb4082a2d8ff162b70dcb36330a476db","uncles":[]}}`))
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
	bs, err := NewETHScanner(getConfigForTest(server.URL), storage, big.NewInt(int64(types.Localnet)), ethClient, bridge, s.m, pubKeyMgr)
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
				if string(rpcRequest.Params) == `["0x63842859cdc141ce3556e0672fbbe6f10bcf58186ab2382c29a52186adffada1"]` {
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"root":"0x","status":"0x1","cumulativeGasUsed":"0x158aa","logsBloom":"0x00000000000000000002000020000000000000000000000000000000000000000000000000004000000000000000000000100800000002000000000000000000000000000000000000000008000000000000000000000000000000000000000010000200000001000000000000000000000000000000000000000810000000000000000000000000000000000000000000000000000000000000000000040000000000000000002000000000000000000000000000001400000000000004000000000002000000000000000000000400000000000000000000000000000000000000000000000000000000000000000000000000010000800000000000000000","logs":[{"address":"0x3b7fa4dd21c6f9ba3ca375217ead7cab9d6bf483","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x0000000000000000000000003fd2d4ce97b082d4bce3f9fee2a3d60668d2f473","0x000000000000000000000000e65e9d372f8cacc7b6dfcd4af6507851ed31bb44"],"data":"0x00000000000000000000000000000000000000000000000000000000000f4240","blockNumber":"0x5","transactionHash":"0x63842859cdc141ce3556e0672fbbe6f10bcf58186ab2382c29a52186adffada1","transactionIndex":"0x0","blockHash":"0xdfd8ea4201fbfee4e1267422a5410e448e925489130fc1b3bd212a7a0e2d0a00","logIndex":"0x0","removed":false},{"address":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","topics":["0xef519b7eb82aaf6ac376a6df2d793843ebfd593de5f1a0601d3cc6ab49ebb395","0x000000000000000000000000fb337706200a55009e6bbd41e4dc164d59bc9aa2"],"data":"0x0000000000000000000000003b7fa4dd21c6f9ba3ca375217ead7cab9d6bf48300000000000000000000000000000000000000000000000000000000000f4240000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000634144443a4554482e544b4e2d3078336237464134646432316336663942413363613337353231374541443743416239443662463438333a7474686f723137303471753333746c6d346c75746363343638303879683334387a726d7239796e6d77346c380000000000000000000000000000000000000000000000000000000000","blockNumber":"0x5","transactionHash":"0x63842859cdc141ce3556e0672fbbe6f10bcf58186ab2382c29a52186adffada1","transactionIndex":"0x0","blockHash":"0xdfd8ea4201fbfee4e1267422a5410e448e925489130fc1b3bd212a7a0e2d0a00","logIndex":"0x1","removed":false}],"transactionHash":"0x63842859cdc141ce3556e0672fbbe6f10bcf58186ab2382c29a52186adffada1","contractAddress":"0x0000000000000000000000000000000000000000","gasUsed":"0x158aa","blockHash":"0xdfd8ea4201fbfee4e1267422a5410e448e925489130fc1b3bd212a7a0e2d0a00","blockNumber":"0x5","transactionIndex":"0x0"}}`))
					c.Assert(err, IsNil)
					return
				} else if string(rpcRequest.Params) == `["0xc318fc70ec225c3e5e5fd3f59e205fdfb4d27ef878dd321b30278d891db0c6a9"]` {
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"root":"0x","status":"0x1","cumulativeGasUsed":"0xf6b5","logsBloom":"0x00000000000000000002000000000000000000000000000000000000000000000000000000000000000000000000000000100800000002000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000004000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000","logs":[{"address":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","topics":["0xef519b7eb82aaf6ac376a6df2d793843ebfd593de5f1a0601d3cc6ab49ebb395","0x000000000000000000000000fb337706200a55009e6bbd41e4dc164d59bc9aa2"],"data":"0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000f4240000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000384144443a4554482e4554483a7474686f723137303471753333746c6d346c75746363343638303879683334387a726d7239796e6d77346c380000000000000000","blockNumber":"0x6","transactionHash":"0xc318fc70ec225c3e5e5fd3f59e205fdfb4d27ef878dd321b30278d891db0c6a9","transactionIndex":"0x0","blockHash":"0x2f202f8aa7355e77bfbdcd63c08f7c4e43e0bcca61b45fe6a2bdb950d777fa38","logIndex":"0x0","removed":false}],"transactionHash":"0xc318fc70ec225c3e5e5fd3f59e205fdfb4d27ef878dd321b30278d891db0c6a9","contractAddress":"0x0000000000000000000000000000000000000000","gasUsed":"0xf6b5","blockHash":"0x2f202f8aa7355e77bfbdcd63c08f7c4e43e0bcca61b45fe6a2bdb950d777fa38","blockNumber":"0x6","transactionIndex":"0x0"}}`))
					c.Assert(err, IsNil)
					return
				} else if string(rpcRequest.Params) == `["0x2b7dc9b021b08976ad181bbc42b59845525eb2a728dc2df442909aeeb23b8956"]` {
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"root":"0x","status":"0x1","cumulativeGasUsed":"0x747f","logsBloom":"0x00010000000000000002010000000000000000000000000000000000000000000000000000000000000000000000000400000000000010000000000000000000000000002000000000000008000000000000000000000040000000000000000010000200020000000000000000000000000000000000008000000010000000000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000001000000000000004000000000002000000000000000000000000000000000000000040001000000000000000000000020000000000000000000000000000000000000020000000000000","logs":[{"address":"0x40bcd4db8889a8bf0b1391d0c819dcd9627f9d0a","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x000000000000000000000000e65e9d372f8cacc7b6dfcd4af6507851ed31bb44","0x000000000000000000000000fabb9cc6ec839b1214bb11c53377a56a6ed81762"],"data":"0x00000000000000000000000000000000000000000000000000000000a900f1d5","blockNumber":"0x48","transactionHash":"0x2b7dc9b021b08976ad181bbc42b59845525eb2a728dc2df442909aeeb23b8956","transactionIndex":"0x0","blockHash":"0x930ba8a3d4d39e260d58f72424cf0bd7167687d1d60115cc493e88fd99edfe99","logIndex":"0x0","removed":false},{"address":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","topics":["0xa9cd03aa3c1b4515114539cd53d22085129d495cb9e9f9af77864526240f1bf7","0x0000000000000000000000007d182d6a138eaa06f6f452bc3f8fc57e17d1e193"],"data":"0x000000000000000000000000fabb9cc6ec839b1214bb11c53377a56a6ed8176200000000000000000000000040bcd4db8889a8bf0b1391d0c819dcd9627f9d0a00000000000000000000000000000000000000000000000000000000a900f1d5000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000444f55543a4636454238393335464241423041313346454646303434343542353234304441353630463341433244414636443332414339434331323137384644444441354200000000000000000000000000000000000000000000000000000000","blockNumber":"0x48","transactionHash":"0x2b7dc9b021b08976ad181bbc42b59845525eb2a728dc2df442909aeeb23b8956","transactionIndex":"0x0","blockHash":"0x930ba8a3d4d39e260d58f72424cf0bd7167687d1d60115cc493e88fd99edfe99","logIndex":"0x1","removed":false}],"transactionHash":"0x2b7dc9b021b08976ad181bbc42b59845525eb2a728dc2df442909aeeb23b8956","contractAddress":"0x0000000000000000000000000000000000000000","gasUsed":"0x747f","blockHash":"0x930ba8a3d4d39e260d58f72424cf0bd7167687d1d60115cc493e88fd99edfe99","blockNumber":"0x48","transactionIndex":"0x0"}}`))
					c.Assert(err, IsNil)
					return
				} else if string(rpcRequest.Params) == `["0xe8e903a9f1612a56b88022d64eb052bfca404dda91bb511709ef6de26b15da83"]` {
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"root":"0x","status":"0x1","cumulativeGasUsed":"0xa35c","logsBloom":"0x00000000000000000002000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004000000000000000008000000000000000000000000000000000000000002000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000001000000000000004000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000010000800000000000000000","logs":[{"address":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","topics":["0x05b90458f953d3fcb2d7fb25616a2fddeca749d0c47cc5c9832d0266b5346eea","0x0000000000000000000000003fd2d4ce97b082d4bce3f9fee2a3d60668d2f473","0x0000000000000000000000009f4aab49a9cd8fc54dcb3701846f608a6f2c44da"],"data":"0x0000000000000000000000003b7fa4dd21c6f9ba3ca375217ead7cab9d6bf48300000000000000000000000000000000000000000000000000000000000f42400000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000568656c6c6f000000000000000000000000000000000000000000000000000000","blockNumber":"0x27","transactionHash":"0xe8e903a9f1612a56b88022d64eb052bfca404dda91bb511709ef6de26b15da83","transactionIndex":"0x0","blockHash":"0x3c3d49280c8b21919672b6ef21bf42fa9c4475736299c89fe3a299a6bf0b2bbb","logIndex":"0x0","removed":false}],"transactionHash":"0xe8e903a9f1612a56b88022d64eb052bfca404dda91bb511709ef6de26b15da83","contractAddress":"0x0000000000000000000000000000000000000000","gasUsed":"0xa35c","blockHash":"0x3c3d49280c8b21919672b6ef21bf42fa9c4475736299c89fe3a299a6bf0b2bbb","blockNumber":"0x27","transactionIndex":"0x0"}}`))
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
	tx := &etypes.Transaction{}
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
		txInItem.Gas[0].Amount.Equal(cosmos.NewUint(50000*20000000000)),
		Equals,
		true,
	)

	bs, err = NewETHScanner(getConfigForTest(server.URL), storage, big.NewInt(int64(types.Localnet)), ethClient, s.bridge, s.m, pkeyMgr)
	c.Assert(err, IsNil)
	c.Assert(bs, NotNil)
	// smart contract - deposit
	encodedTx = `{"nonce":"0x4","gasPrice":"0x1","gas":"0xf4240","to":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","value":"0x0","input":"0x1fece7b4000000000000000000000000fb337706200a55009e6bbd41e4dc164d59bc9aa20000000000000000000000003b7fa4dd21c6f9ba3ca375217ead7cab9d6bf48300000000000000000000000000000000000000000000000000000000000f4240000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000634144443a4554482e544b4e2d3078336237464134646432316336663942413363613337353231374541443743416239443662463438333a7474686f723137303471753333746c6d346c75746363343638303879683334387a726d7239796e6d77346c380000000000000000000000000000000000000000000000000000000000","v":"0x41","r":"0x1566f15d8a624700bd6975948445fd3678a4e6d070892988244a70ff0ae6f172","s":"0xaff193b3fc2a935e05ec86076fa4cfcfc579c683ef1904734303f417e09a004","hash":"0x63842859cdc141ce3556e0672fbbe6f10bcf58186ab2382c29a52186adffada1"}`
	tx = &etypes.Transaction{}
	c.Assert(tx.UnmarshalJSON([]byte(encodedTx)), IsNil)
	txInItem, err = bs.fromTxToTxIn(tx)
	c.Assert(err, IsNil)
	c.Assert(txInItem, NotNil)
	c.Assert(txInItem.Sender, Equals, "0x3fd2d4ce97b082d4bce3f9fee2a3d60668d2f473")
	c.Assert(txInItem.To, Equals, "0xfb337706200a55009e6bBD41E4dC164D59Bc9AA2")
	c.Assert(txInItem.Memo, Equals, "ADD:ETH.TKN-0x3b7FA4dd21c6f9BA3ca375217EAD7CAb9D6bF483:tthor1704qu33tlm4lutcc46808yh348zrmr9ynmw4l8")
	c.Assert(txInItem.Tx, Equals, "63842859cdc141ce3556e0672fbbe6f10bcf58186ab2382c29a52186adffada1")
	c.Assert(txInItem.Coins[0].Asset.String(), Equals, "ETH.TKN-0X3B7FA4DD21C6F9BA3CA375217EAD7CAB9D6BF483")
	c.Assert(txInItem.Coins[0].Amount.Equal(cosmos.NewUint(1000000)), Equals, true)

	// smart contract - depositETH
	encodedTx = `{"nonce":"0x5","gasPrice":"0x1","gas":"0xf4240","to":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","value":"0xf4240","input":"0x31a053cf000000000000000000000000fb337706200a55009e6bbd41e4dc164d59bc9aa2000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000384144443a4554482e4554483a7474686f723137303471753333746c6d346c75746363343638303879683334387a726d7239796e6d77346c380000000000000000","v":"0x41","r":"0x208cecdceb27e5a1dd35ddf315d36596efa188aa78a21084374a9ff3678d9953","s":"0x28beee65febcf7771044832f813fce0401bebe72f3508471b26a6cfa71992f2c","hash":"0xc318fc70ec225c3e5e5fd3f59e205fdfb4d27ef878dd321b30278d891db0c6a9"}`
	tx = &etypes.Transaction{}
	c.Assert(tx.UnmarshalJSON([]byte(encodedTx)), IsNil)
	txInItem, err = bs.fromTxToTxIn(tx)
	c.Assert(err, IsNil)
	c.Assert(txInItem, NotNil)
	c.Assert(txInItem.Sender, Equals, "0x3fd2d4ce97b082d4bce3f9fee2a3d60668d2f473")
	c.Assert(txInItem.To, Equals, "0xfb337706200a55009e6bBD41E4dC164D59Bc9AA2")
	c.Assert(txInItem.Memo, Equals, "ADD:ETH.ETH:tthor1704qu33tlm4lutcc46808yh348zrmr9ynmw4l8")
	c.Assert(txInItem.Tx, Equals, "c318fc70ec225c3e5e5fd3f59e205fdfb4d27ef878dd321b30278d891db0c6a9")
	c.Assert(txInItem.Coins[0].Asset.String(), Equals, "ETH.ETH")
	c.Assert(txInItem.Coins[0].Amount.Equal(cosmos.NewUint(1000000)), Equals, true)

	// smart contract - transferOut
	encodedTx = `{"nonce":"0x4","gasPrice":"0x1","gas":"0x13880","to":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","value":"0x0","input":"0x574da717000000000000000000000000fabb9cc6ec839b1214bb11c53377a56a6ed8176200000000000000000000000040bcd4db8889a8bf0b1391d0c819dcd9627f9d0a00000000000000000000000000000000000000000000000000000000a900f1d5000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000444f55543a4636454238393335464241423041313346454646303434343542353234304441353630463341433244414636443332414339434331323137384644444441354200000000000000000000000000000000000000000000000000000000","v":"0x42","r":"0x15854e2270255df8cae5a8ef2d08df84a802fa5e194bd7bdc20ee31b2e3ce3da","s":"0x4f4adc1e1b7c17e789d3dfd9b090eb21fda8561db86e44594b47f7e9ccb418c3","hash":"0x2b7dc9b021b08976ad181bbc42b59845525eb2a728dc2df442909aeeb23b8956"}`
	tx = &etypes.Transaction{}
	c.Assert(tx.UnmarshalJSON([]byte(encodedTx)), IsNil)
	txInItem, err = bs.fromTxToTxIn(tx)
	c.Assert(err, IsNil)
	c.Assert(txInItem, NotNil)
	c.Assert(txInItem.Sender, Equals, "0x7d182D6a138eAa06f6f452bc3F8fC57e17D1E193")
	c.Assert(txInItem.To, Equals, "0xFabB9cC6Ec839b1214bB11c53377A56A6Ed81762")
	c.Assert(txInItem.Memo, Equals, "OUT:F6EB8935FBAB0A13FEFF04445B5240DA560F3AC2DAF6D32AC9CC12178FDDDA5B")
	c.Assert(txInItem.Tx, Equals, "2b7dc9b021b08976ad181bbc42b59845525eb2a728dc2df442909aeeb23b8956")
	c.Assert(txInItem.Coins[0].Asset.String(), Equals, "ETH.TKN-0X40BCD4DB8889A8BF0B1391D0C819DCD9627F9D0A")
	c.Assert(txInItem.Coins[0].Amount.Equal(cosmos.NewUint(2835411413)), Equals, true)

	// smart contract - allowance
	encodedTx = `{"nonce":"0x7","gasPrice":"0x1","gas":"0xa35c","to":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","value":"0x0","input":"0x795b8f790000000000000000000000009f4aab49a9cd8fc54dcb3701846f608a6f2c44da0000000000000000000000003b7fa4dd21c6f9ba3ca375217ead7cab9d6bf48300000000000000000000000000000000000000000000000000000000000f42400000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000000568656c6c6f000000000000000000000000000000000000000000000000000000","v":"0x42","r":"0xb487d78604243d96c1848fcde83fc33a0ad87f2707b9930811d2c34fcdce407a","s":"0x748e245ddba2d57e7f9a1fdcefde9b2e4daddd08eaba97e0ec4044e0e6d39075","hash":"0xe8e903a9f1612a56b88022d64eb052bfca404dda91bb511709ef6de26b15da83"}`
	tx = &etypes.Transaction{}
	c.Assert(tx.UnmarshalJSON([]byte(encodedTx)), IsNil)
	txInItem, err = bs.fromTxToTxIn(tx)
	c.Assert(err, IsNil)
	c.Assert(txInItem, NotNil)
	c.Assert(txInItem.Sender, Equals, "0x3fd2D4cE97B082d4BcE3f9fee2A3D60668D2f473")
	c.Assert(txInItem.To, Equals, "0x9F4AaB49A9cd8FC54Dcb3701846f608a6f2C44dA")
	c.Assert(txInItem.Memo, Equals, "hello")
	c.Assert(txInItem.Tx, Equals, "e8e903a9f1612a56b88022d64eb052bfca404dda91bb511709ef6de26b15da83")
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
