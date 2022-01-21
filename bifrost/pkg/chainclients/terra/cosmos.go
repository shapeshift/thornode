package terra

import (
	"context"
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
	"gitlab.com/thorchain/thornode/x/thorchain"
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
	addr, err := poolPubKey.GetAddress(common.TERRAChain)
	if err != nil {
		c.logger.Error().Err(err).Str("pool_pub_key", poolPubKey.String()).Msg("fail to get pool address")
		return ""
	}
	return addr.String()
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
			c.logger.Error().Err(err).Msg("fail to get witness")
			return
		}
	}()

	if c.signerCacheManager.HasSigned(tx.CacheHash()) {
		c.logger.Info().Interface("tx", tx).Msg("transaction already signed, ignoring...")
		return nil, nil
	}

	fromBz, err := types.GetFromBech32(c.GetAddress(tx.VaultPubKey), "terra")
	if err != nil {
		return nil, fmt.Errorf("failed to convert address (%s) to bech32: %w", c.GetAddress(tx.VaultPubKey), err)
	}
	fromAddr := cosmos.AccAddress(fromBz)

	toBz, err := types.GetFromBech32(tx.ToAddress.String(), "terra")
	if err != nil {
		return nil, fmt.Errorf("failed to convert address (%s) to bech32: %w", tx.ToAddress, err)
	}
	toAddr := cosmos.AccAddress(toBz)

	var gasFees common.Coins

	// For yggdrasil, we need to leave some coins to pay for the fee. Note, this
	// logic is per chain, given that different networks charge fees differently.
	if strings.EqualFold(tx.Memo, thorchain.NewYggdrasilReturn(thorchainHeight).String()) {
		gasFees = common.Coins{
			common.NewCoin(c.cosmosScanner.feeAsset, c.cosmosScanner.avgGasFee),
		}
	}

	var coins types.Coins
	for _, coin := range tx.Coins {
		// deduct gas coins
		for _, gasFee := range gasFees {
			if coin.Asset.Equals(gasFee.Asset) {
				coin.Amount = common.SafeSub(coin.Amount, gasFee.Amount)
			}
		}

		coins = append(coins, types.NewCoin(coin.Asset.Symbol.String(), types.NewIntFromBigInt(coin.Amount.BigInt())))
	}

	msg := btypes.NewMsgSend(fromAddr, toAddr, coins.Sort())
	if err := msg.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("invalid MsgSend: %w", err)
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
		Memo:          tx.Memo,
		Msgs:          []types.Msg{msg},
		Sequence:      uint64(meta.SeqNumber),
		AccountNumber: uint64(meta.AccountNumber),
	}

	rawBz, err := c.localKeyManager.Sign(signMsg)
	if err != nil {
		return nil, fmt.Errorf("unable to sign tx: %w", err)
	}

	signedTx = []byte(hex.EncodeToString(rawBz))
	c.logger.Info().Str("hexTx", hex.EncodeToString(rawBz)).Msg("signTx")
	return signedTx, nil
}

func (b *Cosmos) GetAccount(pkey common.PubKey) (common.Account, error) {
	addr := b.GetAddress(pkey)
	address, err := types.AccAddressFromBech32(addr)
	if err != nil {
		b.logger.Error().Err(err).Msgf("fail to get parse address: %s", addr)
		return common.Account{}, err
	}
	return b.GetAccountByAddress(address.String())
}

func (c *Cosmos) GetAccountByAddress(address string) (common.Account, error) {
	bankClient := btypes.NewQueryClient(c.grpcConn)
	bankReq := &btypes.QueryAllBalancesRequest{
		Address: address,
	}
	balances, err := bankClient.AllBalances(context.Background(), bankReq)
	if err != nil {
		return common.Account{}, err
	}

	nativeCoins := make([]common.Coin, 0)
	for _, balance := range balances.Balances {
		coin, _ := sdkCoinToCommonCoin(balance)
		nativeCoins = append(nativeCoins, coin)
	}

	client := atypes.NewQueryClient(c.grpcConn)
	authReq := &atypes.QueryAccountRequest{
		Address: address,
	}

	acc, err := client.Account(context.Background(), authReq)
	if err != nil {
		return common.Account{}, err
	}

	ba := new(atypes.BaseAccount)
	err = ba.Unmarshal(acc.GetAccount().Value)
	if err != nil {
		return common.Account{}, err
	}

	if err != nil {
		return common.Account{}, err
	}

	return common.Account{
		Sequence:      int64(ba.Sequence),
		AccountNumber: int64(ba.AccountNumber),
		Coins:         nativeCoins,
		HasMemoFlag:   false,
	}, nil
}

// BroadcastTx is to broadcast the tx to cosmos chain
func (b *Cosmos) BroadcastTx(tx stypes.TxOutItem, hexTx []byte) (string, error) {
	txClient := txtypes.NewServiceClient(b.grpcConn)
	req := &txtypes.BroadcastTxRequest{
		TxBytes: hexTx,
		Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
	}

	res, err := txClient.BroadcastTx(context.Background(), req)
	if err != nil {
		b.logger.Error().Err(err).Msg("unable to broadcast tx")
		return "", err
	}

	return res.TxResponse.TxHash, nil
}

// ConfirmationCountReady cosmos chain has almost instant finality, so doesn't need to wait for confirmation
func (b *Cosmos) ConfirmationCountReady(txIn stypes.TxIn) bool {
	return true
}

// GetConfirmationCount determine how many confirmations are required
func (b *Cosmos) GetConfirmationCount(txIn stypes.TxIn) int64 {
	return 0
}
func (b *Cosmos) reportSolvency(blockHeight int64) error {
	if blockHeight%900 > 0 {
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
			Height: blockHeight,
			Chain:  common.TERRAChain,
			PubKey: asgard.PubKey,
			Coins:  acct.Coins,
		}:
		case <-time.After(constants.ThorchainBlockTime):
			b.logger.Info().Msgf("fail to send solvency info to THORChain, timeout")
		}
	}
	return nil
}
