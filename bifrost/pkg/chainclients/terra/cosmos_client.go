package terra

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/tendermint/tendermint/crypto"
	memo "gitlab.com/thorchain/thornode/x/thorchain/memo"

	ctypes "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	atypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	btypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	grpc "google.golang.org/grpc"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	tssp "gitlab.com/thorchain/tss/go-tss/tss"

	"gitlab.com/thorchain/thornode/bifrost/blockscanner"
	"gitlab.com/thorchain/thornode/bifrost/config"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/signercache"

	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	stypes "gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/bifrost/tss"
	"gitlab.com/thorchain/thornode/common"
)

// CosmosSuccessCodes a transaction is considered successful if it returns 0
// or if tx is unauthorized or already in the mempool (another Bifrost already sent it)
var CosmosSuccessCodes = map[uint32]bool{
	errortypes.SuccessABCICode:                true,
	errortypes.ErrTxInMempoolCache.ABCICode(): true,
	errortypes.ErrUnauthorized.ABCICode():     true,
}

// Cosmos is a structure to sign and broadcast tx to atom chain used by signer mostly
type CosmosClient struct {
	logger              zerolog.Logger
	cfg                 config.ChainConfiguration
	chainID             string
	txConfig            client.TxConfig
	txClient            txtypes.ServiceClient
	bankClient          btypes.QueryClient
	accountClient       atypes.QueryClient
	accts               *CosmosMetaDataStore
	tssKeyManager       *tss.KeySign
	localKeyManager     *keyManager
	thorchainBridge     *thorclient.ThorchainBridge
	storage             *blockscanner.BlockScannerStorage
	blockScanner        *blockscanner.BlockScanner
	signerCacheManager  *signercache.CacheManager
	cosmosScanner       *CosmosBlockScanner
	globalSolvencyQueue chan stypes.Solvency
}

// NewClient create new instance of atom client
func NewCosmosClient(
	thorKeys *thorclient.Keys,
	cfg config.ChainConfiguration,
	server *tssp.TssServer,
	thorchainBridge *thorclient.ThorchainBridge,
	m *metrics.Metrics,

) (*CosmosClient, error) {
	logger := log.With().Str("module", common.TERRAChain.String()).Logger()

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
		addr:    ctypes.AccAddress(priv.PubKey().Address()),
		pubkey:  pk,
	}

	host := strings.Replace(cfg.RPCHost, "http://", "", -1)
	conn, err := grpc.Dial(host, grpc.WithInsecure())
	if err != nil {
		logger.Fatal().Err(err).Msg("fail to dial")
	}

	interfaceRegistry := codectypes.NewInterfaceRegistry()
	interfaceRegistry.RegisterImplementations((*ctypes.Msg)(nil), &btypes.MsgSend{})
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	txConfig := tx.NewTxConfig(marshaler, []signingtypes.SignMode{signingtypes.SignMode_SIGN_MODE_DIRECT})

	c := &CosmosClient{
		chainID:         "columbus-5",
		logger:          logger,
		cfg:             cfg,
		txConfig:        txConfig,
		txClient:        txtypes.NewServiceClient(conn),
		bankClient:      btypes.NewQueryClient(conn),
		accountClient:   atypes.NewQueryClient(conn),
		accts:           NewCosmosMetaDataStore(),
		tssKeyManager:   tssKm,
		localKeyManager: localKm,
		thorchainBridge: thorchainBridge,
	}

	var path string // if not set later, will in memory storage
	if len(c.cfg.BlockScanner.DBPath) > 0 {
		path = fmt.Sprintf("%s/%s", c.cfg.BlockScanner.DBPath, c.cfg.BlockScanner.ChainID)
	}
	c.storage, err = blockscanner.NewBlockScannerStorage(path)
	if err != nil {
		return nil, fmt.Errorf("fail to create scan storage: %w", err)
	}

	c.cosmosScanner, err = NewCosmosBlockScanner(
		c.cfg.BlockScanner,
		c.storage,
		c.thorchainBridge,
		m,
		c.reportSolvency,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos scanner: %w", err)
	}

	c.blockScanner, err = blockscanner.NewBlockScanner(c.cfg.BlockScanner, c.storage, m, c.thorchainBridge, c.cosmosScanner)
	if err != nil {
		return nil, fmt.Errorf("failed to create block scanner: %w", err)
	}

	signerCacheManager, err := signercache.NewSignerCacheManager(c.storage.GetInternalDb())
	if err != nil {
		return nil, fmt.Errorf("fail to create signer cache manager")
	}
	c.signerCacheManager = signerCacheManager

	return c, nil
}

// Start Cosmos chain client
func (c *CosmosClient) Start(globalTxsQueue chan stypes.TxIn, globalErrataQueue chan stypes.ErrataBlock, globalSolvencyQueue chan stypes.Solvency) {
	c.globalSolvencyQueue = globalSolvencyQueue
	c.tssKeyManager.Start()
	c.blockScanner.Start(globalTxsQueue)
}

// Stop Cosmos chain client
func (c *CosmosClient) Stop() {
	c.tssKeyManager.Stop()
	c.blockScanner.Stop()
}

// GetConfig return the configuration used by Cosmos chain client
func (c *CosmosClient) GetConfig() config.ChainConfiguration {
	return c.cfg
}

func (c *CosmosClient) IsBlockScannerHealthy() bool {
	return c.blockScanner.IsHealthy()
}

func (c *CosmosClient) GetChain() common.Chain {
	return common.TERRAChain
}

func (c *CosmosClient) GetHeight() (int64, error) {
	return c.blockScanner.FetchLastHeight()
}

// GetAddress return current signer address, it will be bech32 encoded address
func (c *CosmosClient) GetAddress(poolPubKey common.PubKey) string {
	addr, err := poolPubKey.GetAddress(c.GetChain())
	if err != nil {
		c.logger.Error().Err(err).Str("pool_pub_key", poolPubKey.String()).Msg("fail to get pool address")
		return ""
	}
	return addr.String()
}

func (c *CosmosClient) GetAccount(pkey common.PubKey) (common.Account, error) {
	addr, err := pkey.GetAddress(c.GetChain())
	if err != nil {
		return common.Account{}, fmt.Errorf("failed to convert address (%s) from bech32: %w", pkey, err)
	}
	return c.GetAccountByAddress(addr.String())
}

func (c *CosmosClient) GetAccountByAddress(address string) (common.Account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	bankReq := &btypes.QueryAllBalancesRequest{
		Address: address,
	}
	balances, err := c.bankClient.AllBalances(ctx, bankReq)
	if err != nil {
		return common.Account{}, err
	}

	nativeCoins := make([]common.Coin, 0)
	for _, balance := range balances.Balances {
		coin := fromCosmosToThorchain(balance)
		nativeCoins = append(nativeCoins, coin)
	}

	authReq := &atypes.QueryAccountRequest{
		Address: address,
	}

	acc, err := c.accountClient.Account(ctx, authReq)
	if err != nil {
		return common.Account{}, err
	}

	ba := new(atypes.BaseAccount)
	err = ba.Unmarshal(acc.GetAccount().Value)
	if err != nil {
		return common.Account{}, err
	}

	return common.Account{
		Sequence:      int64(ba.Sequence),
		AccountNumber: int64(ba.AccountNumber),
		Coins:         nativeCoins,
	}, nil
}

func (c *CosmosClient) processOutboundTx(tx stypes.TxOutItem) (*btypes.MsgSend, error) {
	vaultPubKey, err := tx.VaultPubKey.GetAddress(common.TERRAChain)
	if err != nil {
		return nil, fmt.Errorf("failed to convert address (%s) to bech32: %w", tx.VaultPubKey.String(), err)
	}

	var coins ctypes.Coins
	for _, coin := range tx.Coins {
		// convert to cosmos coin
		cosmosCoin := fromThorchainToCosmos(coin)
		coins = append(coins, cosmosCoin)
	}

	return &btypes.MsgSend{
		FromAddress: vaultPubKey.String(),
		ToAddress:   tx.ToAddress.String(),
		Amount:      coins.Sort(),
	}, nil
}

// SignTx sign the the given TxArrayItem
func (c *CosmosClient) SignTx(tx stypes.TxOutItem, thorchainHeight int64) (signedTx []byte, err error) {
	defer func() {
		if err != nil {
			var keysignError tss.KeysignError
			if errors.As(err, &keysignError) {
				if len(keysignError.Blame.BlameNodes) == 0 {
					c.logger.Error().Err(err).Msg("TSS doesn't know which node to blame")
					return
				}

				// key sign error forward the keysign blame to thorchain
				txID, err := c.thorchainBridge.PostKeysignFailure(keysignError.Blame, thorchainHeight, tx.Memo, tx.Coins, tx.VaultPubKey)
				if err != nil {
					c.logger.Error().Err(err).Msg("fail to post keysign failure to THORChain")
					return
				}
				c.logger.Info().Str("tx_id", txID.String()).Msgf("post keysign failure to thorchain")
			}
			c.logger.Error().Err(err).Msg("failed to sign tx")
			return
		}
	}()

	if c.signerCacheManager.HasSigned(tx.CacheHash()) {
		c.logger.Info().Interface("tx", tx).Msg("transaction already signed, ignoring...")
		return nil, nil
	}

	msg, err := c.processOutboundTx(tx)
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to process outbound tx")
		return nil, err
	}

	currentHeight, err := c.cosmosScanner.GetHeight()
	if err != nil {
		c.logger.Error().Err(err).Msg("fail to get current binance block height")
		return nil, err
	}
	meta := c.accts.Get(tx.VaultPubKey)
	if currentHeight > meta.BlockHeight {
		acc, err := c.GetAccount(tx.VaultPubKey)
		if err != nil {
			return nil, fmt.Errorf("fail to get account info: %w", err)
		}
		meta = CosmosMetadata{
			AccountNumber: acc.AccountNumber,
			SeqNumber:     acc.Sequence,
			BlockHeight:   currentHeight,
		}
		c.accts.Set(tx.VaultPubKey, meta)
	}

	var gas ctypes.Coins
	for _, gasCoin := range tx.MaxGas.ToCoins() {
		if gasCoin.Asset == c.GetChain().GetGasAsset() {
			gas = append(gas, fromThorchainToCosmos(gasCoin))
		}
	}

	gasRate := fromThorchainToCosmos(common.NewCoin(c.GetChain().GetGasAsset(), cosmos.NewUint(uint64(tx.GasRate))))
	txBuilder, err := buildUnsigned(
		c.txConfig,
		msg,
		tx.VaultPubKey,
		tx.Memo,
		gas,
		gasRate.Amount.Uint64(),
		uint64(meta.AccountNumber),
		uint64(meta.SeqNumber),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to build unsigned tx: %w", err)
	}

	txBytes, err := c.signMsg(
		txBuilder,
		tx.VaultPubKey,
		uint64(meta.AccountNumber),
		uint64(meta.SeqNumber),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}

	return txBytes, nil
}

func (c *CosmosClient) signMsg(
	txBuilder client.TxBuilder,
	pubkey common.PubKey,
	account uint64,
	sequence uint64,
) ([]byte, error) {
	cpk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeAccPub, pubkey.String())
	if err != nil {
		return nil, fmt.Errorf("unable to GetPubKeyFromBech32 from cosmos: %w", err)
	}

	modeHandler := c.txConfig.SignModeHandler()
	signingData := signing.SignerData{
		ChainID:       c.chainID,
		AccountNumber: account,
		Sequence:      sequence,
	}

	signBytes, err := modeHandler.GetSignBytes(signingtypes.SignMode_SIGN_MODE_DIRECT, signingData, txBuilder.GetTx())
	if err != nil {
		return nil, fmt.Errorf("unable to GetSignBytes on modeHandler: %w", err)
	}

	sigData := &signingtypes.SingleSignatureData{
		SignMode: signingtypes.SignMode_SIGN_MODE_DIRECT,
	}
	sig := signingtypes.SignatureV2{
		PubKey:   cpk,
		Data:     sigData,
		Sequence: sequence,
	}

	if c.localKeyManager.Pubkey().Equals(pubkey) {
		sigData.Signature, err = c.localKeyManager.Sign(signBytes)
		if err != nil {
			return nil, fmt.Errorf("unable to sign using localKeyManager: %w", err)
		}
	} else {
		hashedMsg := crypto.Sha256(signBytes)
		sigData.Signature, _, err = c.tssKeyManager.RemoteSign(hashedMsg, pubkey.String())
		if err != nil {
			return nil, err
		}
	}

	if !cpk.VerifySignature(signBytes, sigData.Signature) {
		return nil, fmt.Errorf("unable to verify signature with secpPubKey")
	}

	err = txBuilder.SetSignatures(sig)
	if err != nil {
		return nil, fmt.Errorf("unable to final SetSignatures on txBuilder: %w", err)
	}

	txBytes, err := c.txConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, fmt.Errorf("unable to encode tx: %w", err)
	}

	return txBytes, nil
}

// BroadcastTx is to broadcast the tx to cosmos chain
func (c *CosmosClient) BroadcastTx(tx stypes.TxOutItem, txBytes []byte) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	acc, err := c.GetAccount(tx.VaultPubKey)
	if err != nil {
		return "", fmt.Errorf("fail to get account info: %w", err)
	}

	var gas ctypes.Coins
	for _, gasCoin := range tx.MaxGas.ToCoins() {
		if gasCoin.Asset == c.GetChain().GetGasAsset() {
			gas = append(gas, fromThorchainToCosmos(gasCoin))
		}
	}
	gasRate := fromThorchainToCosmos(common.NewCoin(c.GetChain().GetGasAsset(), cosmos.NewUint(uint64(tx.GasRate))))

	out, err := c.processOutboundTx(tx)
	if err != nil {
		return "", fmt.Errorf("unable to processOutboundTx for simulateTx: %w", err)
	}

	txb, err := buildUnsigned(
		c.txConfig,
		out,
		tx.VaultPubKey,
		tx.Memo,
		gas,
		gasRate.Amount.Uint64(),
		uint64(acc.AccountNumber),
		uint64(acc.Sequence),
	)
	if err != nil {
		return "", fmt.Errorf("unable to buildUnsighed for simulateTx: %w", err)
	}

	simRes, err := simulateTx(txb, c.txClient)
	if err != nil {
		c.logger.Error().Err(err).Msg("simulateTx failed")
		return "", err
	}

	log.Info().Interface("simRes", simRes).Msg("simulateTx")
	expectedFees := txb.GetTx().GetFee()
	for _, coin := range expectedFees {
		if coin.Amount.LT(ctypes.NewIntFromUint64(simRes.GasInfo.GasUsed)) {
			return "", fmt.Errorf("gas too low, expected (%d) got (%d)", simRes.GasInfo.GasUsed, coin.Amount.Int64())
		}
	}

	req := &txtypes.BroadcastTxRequest{
		TxBytes: txBytes,
		Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
	}
	broadcastRes, err := c.txClient.BroadcastTx(ctx, req)
	log.Info().Interface("req", req).Interface("res", broadcastRes).Msg("BroadcastTx")
	if err != nil {
		c.logger.Error().Err(err).Msg("unable to broadcast tx")
		return "", err
	}

	if success := CosmosSuccessCodes[broadcastRes.TxResponse.Code]; !success {
		c.logger.Error().Interface("response", broadcastRes).Msg("unsuccessful error code in transaction broadcast")
		return "", errors.New("broadcast msg failed")
	}

	c.accts.SeqInc(tx.VaultPubKey)
	if err := c.signerCacheManager.SetSigned(tx.CacheHash(), broadcastRes.TxResponse.TxHash); err != nil {
		c.logger.Err(err).Msg("fail to set signer cache")
	}
	return broadcastRes.TxResponse.TxHash, nil
}

// ConfirmationCountReady cosmos chain has almost instant finality, so doesn't need to wait for confirmation
func (c *CosmosClient) ConfirmationCountReady(txIn stypes.TxIn) bool {
	return true
}

// GetConfirmationCount determine how many confirmations are required
func (c *CosmosClient) GetConfirmationCount(txIn stypes.TxIn) int64 {
	return 0
}
func (c *CosmosClient) reportSolvency(blockHeight int64) error {
	if blockHeight%900 > 0 {
		return nil
	}
	asgardVaults, err := c.thorchainBridge.GetAsgards()
	if err != nil {
		return fmt.Errorf("fail to get asgards,err: %w", err)
	}
	for _, asgard := range asgardVaults {
		acct, err := c.GetAccount(asgard.PubKey)
		if err != nil {
			c.logger.Err(err).Msgf("fail to get account balance")
			continue
		}
		select {
		case c.globalSolvencyQueue <- stypes.Solvency{
			Height: blockHeight,
			Chain:  common.TERRAChain,
			PubKey: asgard.PubKey,
			Coins:  acct.Coins,
		}:
		case <-time.After(constants.ThorchainBlockTime):
			c.logger.Info().Msgf("fail to send solvency info to THORChain, timeout")
		}
	}
	return nil
}

func (c *CosmosClient) OnObservedTxIn(txIn stypes.TxInItem, blockHeight int64) {
	m, err := memo.ParseMemo(txIn.Memo)
	if err != nil {
		c.logger.Err(err).Msgf("fail to parse memo: %s", txIn.Memo)
		return
	}
	if !m.IsOutbound() {
		return
	}
	if m.GetTxID().IsEmpty() {
		return
	}
	if err := c.signerCacheManager.SetSigned(txIn.CacheHash(c.GetChain(), m.GetTxID().String()), txIn.Tx); err != nil {
		c.logger.Err(err).Msg("fail to update signer cache")
	}
}
