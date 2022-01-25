package terra

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/x/auth/legacy/legacytx"
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

// Cosmos is a structure to sign and broadcast tx to atom chain used by signer mostly
type Cosmos struct {
	logger              zerolog.Logger
	cfg                 config.ChainConfiguration
	chainID             string
	grpcConn            *grpc.ClientConn
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
func NewCosmos(
	thorKeys *thorclient.Keys,
	cfg config.ChainConfiguration,
	server *tssp.TssServer,
	thorchainBridge *thorclient.ThorchainBridge,
	m *metrics.Metrics,

) (*Cosmos, error) {
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
		addr:    types.AccAddress(priv.PubKey().Address()),
		pubkey:  pk,
	}

	host := strings.Replace(cfg.RPCHost, "http://", "", -1)
	conn, err := grpc.Dial(host, grpc.WithInsecure())
	if err != nil {
		logger.Fatal().Err(err).Msg("fail to dial")
	}

	c := &Cosmos{
		chainID:         "columbus-5",
		logger:          logger,
		cfg:             cfg,
		grpcConn:        conn,
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
func (c *Cosmos) Start(globalTxsQueue chan stypes.TxIn, globalErrataQueue chan stypes.ErrataBlock, globalSolvencyQueue chan stypes.Solvency) {
	c.globalSolvencyQueue = globalSolvencyQueue
	c.tssKeyManager.Start()
	c.blockScanner.Start(globalTxsQueue)
}

// Stop Cosmos chain client
func (c *Cosmos) Stop() {
	c.tssKeyManager.Stop()
	c.blockScanner.Stop()
}

// GetConfig return the configuration used by Cosmos chain client
func (c *Cosmos) GetConfig() config.ChainConfiguration {
	return c.cfg
}

func (c *Cosmos) IsBlockScannerHealthy() bool {
	return c.blockScanner.IsHealthy()
}

func (c *Cosmos) GetChain() common.Chain {
	return common.TERRAChain
}

func (c *Cosmos) GetHeight() (int64, error) {
	return c.blockScanner.FetchLastHeight()
}

// GetAddress return current signer address, it will be bech32 encoded address
func (c *Cosmos) GetAddress(poolPubKey common.PubKey) string {
	addr, err := poolPubKey.GetAddress(c.GetChain())
	if err != nil {
		c.logger.Error().Err(err).Str("pool_pub_key", poolPubKey.String()).Msg("fail to get pool address")
		return ""
	}
	return addr.String()
}

func (c *Cosmos) processOutboundTx(tx stypes.TxOutItem, thorchainHeight int64) (*btypes.MsgSend, error) {
	vaultPubKey, err := tx.VaultPubKey.GetAddress(common.TERRAChain)
	if err != nil {
		return nil, fmt.Errorf("failed to convert address (%s) to bech32: %w", tx.VaultPubKey.String(), err)
	}

	gasFees := common.Coins{common.NewCoin(c.cosmosScanner.feeAsset, c.cosmosScanner.avgGasFee)}

	var coins types.Coins
	for _, coin := range tx.Coins {
		// deduct gas coins
		for _, gasFee := range gasFees {
			if coin.Asset.Equals(gasFee.Asset) {
				coin.Amount = common.SafeSub(coin.Amount, gasFee.Amount)
			}
		}
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
func (c *Cosmos) SignTx(tx stypes.TxOutItem, thorchainHeight int64) (signedTx []byte, err error) {
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

	msg, err := c.processOutboundTx(tx, thorchainHeight)
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

	signMsg := legacytx.StdSignMsg{
		ChainID:       c.chainID,
		AccountNumber: uint64(meta.AccountNumber),
		Sequence:      uint64(meta.SeqNumber),
		Msgs:          []types.Msg{msg},
		Memo:          tx.Memo,
	}

	rawBz, err := c.signMsg(signMsg, tx.VaultPubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}

	return []byte(hex.EncodeToString(rawBz)), nil
}

func (c *Cosmos) signMsg(msg legacytx.StdSignMsg, pubkey common.PubKey) (sigBz []byte, err error) {
	if c.localKeyManager.Pubkey().Equals(pubkey) {
		sigBz, err = c.localKeyManager.Sign(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to sign using localKeyManager: %w", err)
		}
	} else {
		sigBz, _, err = c.tssKeyManager.RemoteSign(msg.Bytes(), pubkey.String())
		if err != nil {
			return nil, fmt.Errorf("failed to sign using tssKeyManager: %w", err)
		}
	}

	pk, err := types.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeAccPub, pubkey.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get public key for signature verification: %w", err)
	}

	if !pk.VerifySignature(msg.Bytes(), sigBz) {
		return nil, fmt.Errorf("signture verification failed, pk: %s, signature: %s", pk.String(), hex.EncodeToString(sigBz))
	}

	stdTx := legacytx.StdTx{
		Msgs: msg.Msgs,
		Signatures: []legacytx.StdSignature{
			{
				PubKey:    pk,
				Signature: sigBz,
			},
		},
		Memo: msg.Memo,
	}
	encoder := legacytx.DefaultTxEncoder(thorclient.MakeLegacyCodec())
	return encoder(stdTx)
}

func (c *Cosmos) GetAccount(pkey common.PubKey) (common.Account, error) {
	addr, err := pkey.GetAddress(c.GetChain())
	if err != nil {
		return common.Account{}, fmt.Errorf("failed to convert address (%s) from bech32: %w", pkey, err)
	}
	return c.GetAccountByAddress(addr.String())
}

func (c *Cosmos) GetAccountByAddress(address string) (common.Account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	bankClient := btypes.NewQueryClient(c.grpcConn)
	bankReq := &btypes.QueryAllBalancesRequest{
		Address: address,
	}
	balances, err := bankClient.AllBalances(ctx, bankReq)
	if err != nil {
		return common.Account{}, err
	}

	nativeCoins := make([]common.Coin, 0)
	for _, balance := range balances.Balances {
		coin := fromCosmosToThorchain(balance)
		nativeCoins = append(nativeCoins, coin)
	}

	client := atypes.NewQueryClient(c.grpcConn)
	authReq := &atypes.QueryAccountRequest{
		Address: address,
	}

	acc, err := client.Account(ctx, authReq)
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

// BroadcastTx is to broadcast the tx to cosmos chain
func (c *Cosmos) BroadcastTx(tx stypes.TxOutItem, hexTx []byte) (string, error) {
	base64EncodedTx := base64.RawStdEncoding.EncodeToString(hexTx)

	txClient := txtypes.NewServiceClient(c.grpcConn)
	req := &txtypes.BroadcastTxRequest{
		TxBytes: []byte(base64EncodedTx),
		Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	res, err := txClient.BroadcastTx(ctx, req)
	if err != nil {
		c.logger.Error().Err(err).Msg("unable to broadcast tx")
		return "", err
	}
	if res.TxResponse.Code != 0 {
		c.logger.Error().Interface("response", res).Msg("non-zero error code in transaction broadcast")
		return "", errors.New("broadcast msg failed")
	}

	return res.TxResponse.TxHash, nil
}

// ConfirmationCountReady cosmos chain has almost instant finality, so doesn't need to wait for confirmation
func (c *Cosmos) ConfirmationCountReady(txIn stypes.TxIn) bool {
	return true
}

// GetConfirmationCount determine how many confirmations are required
func (c *Cosmos) GetConfirmationCount(txIn stypes.TxIn) int64 {
	return 0
}
func (c *Cosmos) reportSolvency(blockHeight int64) error {
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
