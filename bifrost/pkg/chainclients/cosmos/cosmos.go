package cosmos

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/types"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/thorchain/thornode/constants"
	tssp "gitlab.com/thorchain/tss/go-tss/tss"

	"gitlab.com/thorchain/thornode/bifrost/blockscanner"
	"gitlab.com/thorchain/thornode/bifrost/config"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	stypes "gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/bifrost/tss"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// Cosmos is a structure to sign and broadcast tx to atom chain used by signer mostly
type Cosmos struct {
	logger              zerolog.Logger
	cfg                 config.ChainConfiguration
	cdc                 *codec.LegacyAmino
	chainID             string
	isTestNet           bool
	client              *http.Client
	accts               *CosmosMetaDataStore
	tssKeyManager       *tss.KeySign
	localKeyManager     *keyManager
	thorchainBridge     *thorclient.ThorchainBridge
	storage             *blockscanner.BlockScannerStorage
	blockScanner        *blockscanner.BlockScanner
	cosmosScanner       *CosmosBlockScanner
	globalSolvencyQueue chan stypes.Solvency
}

// NewClient create new instance of atom client
func NewCosmos(
	thorKeys *thorclient.Keys,
	cfg config.ChainConfiguration,
	server *tssp.TssServer,
	thorchainBridge *thorclient.ThorchainBridge,
	m *metrics.Metrics,

) (*Cosmos, error) {
	tssKm, err := tss.NewKeySign(server, thorchainBridge)
	if err != nil {
		return nil, fmt.Errorf("fail to create tss signer: %w", err)
	}

	priv, err := thorKeys.GetPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("fail to get private key: %w", err)
	}

	temp, err := cryptocodec.ToTmPubKeyInterface(priv.PubKey())
	if err != nil {
		return nil, fmt.Errorf("fail to get tm pub key: %w", err)
	}
	pk, err := common.NewPubKeyFromCrypto(temp)
	if err != nil {
		return nil, fmt.Errorf("fail to get pub key: %w", err)
	}
	if thorchainBridge == nil {
		return nil, errors.New("thorchain bridge is nil")
	}
	localKm := &keyManager{
		privKey: priv,
		addr:    types.AccAddress(priv.PubKey().Address()),
		pubkey:  pk,
	}

	b := &Cosmos{
		logger:          log.With().Str("module", "binance").Logger(),
		cfg:             cfg,
		cdc:             thorclient.MakeLegacyCodec(),
		accts:           NewCosmosMetaDataStore(),
		client:          &http.Client{},
		tssKeyManager:   tssKm,
		localKeyManager: localKm,
		thorchainBridge: thorchainBridge,
	}

	// if err := b.checkIsTestNet(); err != nil {
	// 	b.logger.Error().Err(err).Msg("fail to check if is testnet")
	// 	return b, err
	// }

	var path string // if not set later, will in memory storage
	if len(b.cfg.BlockScanner.DBPath) > 0 {
		path = fmt.Sprintf("%s/%s", b.cfg.BlockScanner.DBPath, b.cfg.BlockScanner.ChainID)
	}
	b.storage, err = blockscanner.NewBlockScannerStorage(path)
	if err != nil {
		return nil, fmt.Errorf("fail to create scan storage: %w", err)
	}

	b.cosmosScanner, err = NewCosmosBlockScanner(
		b.cfg.BlockScanner,
		b.storage,
		b.isTestNet,
		b.thorchainBridge,
		m,
		b.reportSolvency,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos scanner: %w", err)
	}

	b.blockScanner, err = blockscanner.NewBlockScanner(b.cfg.BlockScanner, b.storage, m, b.thorchainBridge, b.cosmosScanner)
	if err != nil {
		return nil, fmt.Errorf("failed to create block scanner: %w", err)
	}

	return b, nil
}

// Start Binance chain client
func (b *Cosmos) Start(globalTxsQueue chan stypes.TxIn, globalErrataQueue chan stypes.ErrataBlock, globalSolvencyQueue chan stypes.Solvency) {
	b.globalSolvencyQueue = globalSolvencyQueue
	b.tssKeyManager.Start()
	b.blockScanner.Start(globalTxsQueue)
}

// Stop Binance chain client
func (b *Cosmos) Stop() {
	b.tssKeyManager.Stop()
	b.blockScanner.Stop()
}

// GetConfig return the configuration used by Binance chain client
func (b *Cosmos) GetConfig() config.ChainConfiguration {
	return b.cfg
}

func (b *Cosmos) IsBlockScannerHealthy() bool {
	return b.blockScanner.IsHealthy()
}

func (b *Cosmos) GetChain() common.Chain {
	return common.GAIAChain
}

func (b *Cosmos) GetHeight() (int64, error) {
	return b.blockScanner.FetchLastHeight()
}

// GetAddress return current signer address, it will be bech32 encoded address
func (b *Cosmos) GetAddress(poolPubKey common.PubKey) string {
	addr, err := poolPubKey.GetAddress(common.GAIAChain)
	if err != nil {
		b.logger.Error().Err(err).Str("pool_pub_key", poolPubKey.String()).Msg("fail to get pool address")
		return ""
	}
	return addr.String()
}

func (b *Cosmos) checkAccountMemoFlag(addr string) bool {
	acct, _ := b.GetAccountByAddress(addr)
	return acct.HasMemoFlag
}

// SignTx sign the the given TxArrayItem
func (b *Cosmos) SignTx(tx stypes.TxOutItem, thorchainHeight int64) ([]byte, error) {
	toAddr, err := types.AccAddressFromBech32(tx.ToAddress.String())
	if err != nil {
		b.logger.Error().Err(err).Msgf("fail to parse account address(%s)", tx.ToAddress.String())
		// if we fail to parse the to address , then we log an error and move on
		return nil, nil
	}

	if b.checkAccountMemoFlag(toAddr.String()) {
		b.logger.Info().Msgf("address: %s has memo flag set , ignore tx", tx.ToAddress)
		return nil, nil
	}
	var gasCoin common.Coins

	// for yggdrasil, need to left some coin to pay for fee, this logic is per chain, given different chain charge fees differently
	// if strings.EqualFold(tx.Memo, thorchain.NewYggdrasilReturn(thorchainHeight).String()) {
	// 	gas := b.getGasFee(uint64(len(tx.Coins)))
	// 	gasCoin = gas.ToCoins()
	// }
	var coins types.Coins

	for _, coin := range tx.Coins {
		// deduct gas coin
		for _, gc := range gasCoin {
			if coin.Asset.Equals(gc.Asset) {
				coin.Amount = common.SafeSub(coin.Amount, gc.Amount)
			}
		}

		coins = append(coins, types.Coin{
			Denom:  coin.Asset.Symbol.String(),
			Amount: types.NewIntFromBigInt(coin.Amount.BigInt()),
		})
	}

	currentHeight, err := b.cosmosScanner.GetHeight()
	if err != nil {
		b.logger.Error().Err(err).Msg("fail to get current binance block height")
		return nil, err
	}
	meta := b.accts.Get(tx.VaultPubKey)
	if currentHeight > meta.BlockHeight {
		acc, err := b.GetAccount(tx.VaultPubKey)
		if err != nil {
			return nil, fmt.Errorf("fail to get account info: %w", err)
		}
		meta = CosmosMetadata{
			AccountNumber: acc.AccountNumber,
			SeqNumber:     acc.Sequence,
			BlockHeight:   currentHeight,
		}
		b.accts.Set(tx.VaultPubKey, meta)
	}
	b.logger.Info().Int64("account_number", meta.AccountNumber).Int64("sequence_number", meta.SeqNumber).Int64("block height", meta.BlockHeight).Msg("account info")
	return []byte(""), nil
}

// func (b *Cosmos) sign(signMsg legacytx.StdSignMsg, poolPubKey common.PubKey) ([]byte, error) {
// 	if b.localKeyManager.Pubkey().Equals(poolPubKey) {
// 		return b.localKeyManager.Sign(signMsg)
// 	}
// 	return b.tssKeyManager.SignWithPool(signMsg, poolPubKey)
// }

// // signMsg is design to sign a given message until it success or the same message had been send out by other signer
// func (b *Cosmos) signMsg(signMsg btx.StdSignMsg, from string, poolPubKey common.PubKey, thorchainHeight int64, txOutItem stypes.TxOutItem) ([]byte, error) {
// 	rawBytes, err := b.sign(signMsg, poolPubKey)
// 	if err == nil && rawBytes != nil {
// 		return rawBytes, nil
// 	}
// 	var keysignError tss.KeysignError
// 	if errors.As(err, &keysignError) {
// 		// don't know which node to blame , so just return
// 		if len(keysignError.Blame.BlameNodes) == 0 {
// 			return nil, err
// 		}
// 		// fail to sign a message , forward keysign failure to THORChain , so the relevant party can be blamed
// 		txID, errPostKeysignFail := b.thorchainBridge.PostKeysignFailure(keysignError.Blame, thorchainHeight, txOutItem.Memo, txOutItem.Coins, poolPubKey)
// 		if errPostKeysignFail != nil {
// 			b.logger.Error().Err(errPostKeysignFail).Msg("fail to post keysign failure to thorchain")
// 			return nil, multierror.Append(err, errPostKeysignFail)
// 		}
// 		b.logger.Info().Str("tx_id", txID.String()).Msgf("post keysign failure to thorchain")
// 	}
// 	b.logger.Error().Err(err).Msgf("fail to sign msg with memo: %s", signMsg.Memo)
// 	return nil, err
// }

func (b *Cosmos) GetAccount(pkey common.PubKey) (common.Account, error) {
	addr := b.GetAddress(pkey)
	address, err := types.AccAddressFromBech32(addr)
	if err != nil {
		b.logger.Error().Err(err).Msgf("fail to get parse address: %s", addr)
		return common.Account{}, err
	}
	return b.GetAccountByAddress(address.String())
}

func (b *Cosmos) GetAccountByAddress(address string) (common.Account, error) {
	u, err := url.Parse(b.cfg.RPCHost)
	if err != nil {
		log.Fatal().Msgf("Error parsing rpc (%s): %s", b.cfg.RPCHost, err)
		return common.Account{}, err
	}
	u.Path = "/abci_query"
	v := u.Query()
	v.Set("path", fmt.Sprintf("\"/account/%s\"", address))
	u.RawQuery = v.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return common.Account{}, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			b.logger.Error().Err(err).Msg("fail to close response body")
		}
	}()

	type queryResult struct {
		Jsonrpc string `json:"jsonrpc"`
		ID      string `json:"id"`
		Result  struct {
			Response struct {
				Key         string `json:"key"`
				Value       string `json:"value"`
				BlockHeight string `json:"height"`
			} `json:"response"`
		} `json:"result"`
	}

	var result queryResult
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return common.Account{}, err
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		return common.Account{}, err
	}

	// data, err := base64.StdEncoding.DecodeString(result.Result.Response.Value)
	// if err != nil {
	// 	return common.Account{}, err
	// }

	// cdc := ttypes.NewCodec()
	// var acc types.AppAccount
	// err = cdc.UnmarshalBinaryBare(data, &acc)
	// if err != nil {
	// 	return common.Account{}, err
	// }
	// coins, err := common.GetCoins(common.ATOMChain, acc.BaseAccount.Coins)
	// if err != nil {
	// 	return common.Account{}, err
	// }

	// account := common.NewAccount(acc.BaseAccount.Sequence, acc.BaseAccount.AccountNumber, coins, acc.Flags > 0)
	return common.Account{}, nil
}

// BroadcastTx is to broadcast the tx to binance chain
func (b *Cosmos) BroadcastTx(tx stypes.TxOutItem, hexTx []byte) (string, error) {
	u, err := url.Parse(b.cfg.RPCHost)
	if err != nil {
		log.Error().Msgf("Error parsing rpc (%s): %s", b.cfg.RPCHost, err)
		return "", err
	}
	u.Path = "broadcast_tx_commit"
	values := u.Query()
	values.Set("tx", "0x"+string(hexTx))
	u.RawQuery = values.Encode()
	resp, err := http.Post(u.String(), "", nil)
	if err != nil {
		return "", fmt.Errorf("fail to broadcast tx to binance chain: %w", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("fail to read response body: %w", err)
	}

	// NOTE: we can actually see two different json responses for the same end.
	// This complicates things pretty well.
	// Sample 1: { "height": "0", "txhash": "D97E8A81417E293F5B28DDB53A4AD87B434CA30F51D683DA758ECC2168A7A005", "raw_log": "[{\"msg_index\":0,\"success\":true,\"log\":\"\",\"events\":[{\"type\":\"message\",\"attributes\":[{\"key\":\"action\",\"value\":\"set_observed_txout\"}]}]}]", "logs": [ { "msg_index": 0, "success": true, "log": "", "events": [ { "type": "message", "attributes": [ { "key": "action", "value": "set_observed_txout" } ] } ] } ] }
	// Sample 2: { "height": "0", "txhash": "6A9AA734374D567D1FFA794134A66D3BF614C4EE5DDF334F21A52A47C188A6A2", "code": 4, "raw_log": "{\"codespace\":\"sdk\",\"code\":4,\"message\":\"signature verification failed; verify correct account sequence and chain-id\"}" }
	// Sample 3: {\"jsonrpc\": \"2.0\",\"id\": \"\",\"result\": {  \"check_tx\": {    \"code\": 65541,    \"log\": \"{\\\"codespace\\\":1,\\\"code\\\":5,\\\"abci_code\\\":65541,\\\"message\\\":\\\"insufficient fund. you got 29602BNB,351873676FSN-F1B,1094620960FTM-585,10119750400LOK-3C0,191723639522RUNE-67C,13629773TATIC-E9C,4169469575TCAN-014,10648250188TOMOB-1E1,1155074377TUSDB-000, but 37500BNB fee needed.\\\"}\",    \"events\": [      {}    ]  },  \"deliver_tx\": {},  \"hash\": \"406A3F68B17544F359DF8C94D4E28A626D249BC9C4118B51F7B4CE16D45AF616\",  \"height\": \"0\"}\n}

	b.logger.Info().Str("body", string(body)).Msgf("broadcast response from Binance Chain,memo:%s", tx.Memo)
	var commit stypes.BroadcastResult
	err = b.cdc.UnmarshalJSON(body, &commit)
	if err != nil {
		b.logger.Error().Err(err).Msgf("fail unmarshal commit: %s", string(body))
		return "", fmt.Errorf("fail to unmarshal commit: %w", err)
	}
	// check for any failure logs
	// Error code 4 is used for bad account sequence number. We expect to
	// see this often because in TSS, multiple nodes will broadcast the
	// same sequence number but only one will be successful. We can just
	// drop and ignore in these scenarios. In 1of1 signing, we can also
	// drop and ignore. The reason being, thorchain will attempt to again
	// later.
	checkTx := commit.Result.CheckTx
	if checkTx.Code > 0 && checkTx.Code != cosmos.CodeUnauthorized {
		err := errors.New(checkTx.Log)
		b.logger.Info().Str("body", string(body)).Msg("broadcast response from Binance Chain")
		b.logger.Error().Err(err).Msg("fail to broadcast")
		return "", fmt.Errorf("fail to broadcast: %w", err)
	}

	deliverTx := commit.Result.DeliverTx
	if deliverTx.Code > 0 {
		err := errors.New(deliverTx.Log)
		b.logger.Error().Err(err).Msg("fail to broadcast")
		return "", fmt.Errorf("fail to broadcast: %w", err)
	}

	// increment sequence number
	b.accts.SeqInc(tx.VaultPubKey)

	return commit.Result.Hash.String(), nil
}

// ConfirmationCountReady binance chain has almost instant finality , so doesn't need to wait for confirmation
func (b *Cosmos) ConfirmationCountReady(txIn stypes.TxIn) bool {
	return true
}

// GetConfirmationCount determinate how many confirmation it required
func (b *Cosmos) GetConfirmationCount(txIn stypes.TxIn) int64 {
	return 0
}
func (b *Cosmos) reportSolvency(bnbBlockHeight int64) error {
	if bnbBlockHeight%900 > 0 {
		return nil
	}
	asgardVaults, err := b.thorchainBridge.GetAsgards()
	if err != nil {
		return fmt.Errorf("fail to get asgards,err: %w", err)
	}
	for _, asgard := range asgardVaults {
		acct, err := b.GetAccount(asgard.PubKey)
		if err != nil {
			b.logger.Err(err).Msgf("fail to get account balance")
			continue
		}
		select {
		case b.globalSolvencyQueue <- stypes.Solvency{
			Height: bnbBlockHeight,
			Chain:  common.BNBChain,
			PubKey: asgard.PubKey,
			Coins:  acct.Coins,
		}:
		case <-time.After(constants.ThorchainBlockTime):
			b.logger.Info().Msgf("fail to send solvency info to THORChain, timeout")
		}
	}
	return nil
}
