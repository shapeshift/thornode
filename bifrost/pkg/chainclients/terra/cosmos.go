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
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain"
	tssp "gitlab.com/thorchain/tss/go-tss/tss"

	"gitlab.com/thorchain/thornode/bifrost/blockscanner"
	"gitlab.com/thorchain/thornode/bifrost/config"
	"gitlab.com/thorchain/thornode/bifrost/metrics"

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

	host := strings.Replace(cfg.RPCHost, "http://", "", -1)
	conn, err := grpc.Dial(host, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("fail to dial")
	}

	b := &Cosmos{
		logger:          log.With().Str("module", "cosmos").Logger(),
		cfg:             cfg,
		grpcConn:        conn,
		accts:           NewCosmosMetaDataStore(),
		tssKeyManager:   tssKm,
		localKeyManager: localKm,
		thorchainBridge: thorchainBridge,
	}

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

// Start Cosmos chain client
func (b *Cosmos) Start(globalTxsQueue chan stypes.TxIn, globalErrataQueue chan stypes.ErrataBlock, globalSolvencyQueue chan stypes.Solvency) {
	b.globalSolvencyQueue = globalSolvencyQueue
	b.tssKeyManager.Start()
	b.blockScanner.Start(globalTxsQueue)
}

// Stop Cosmos chain client
func (b *Cosmos) Stop() {
	b.tssKeyManager.Stop()
	b.blockScanner.Stop()
}

// GetConfig return the configuration used by Cosmos chain client
func (b *Cosmos) GetConfig() config.ChainConfiguration {
	return b.cfg
}

func (b *Cosmos) IsBlockScannerHealthy() bool {
	return b.blockScanner.IsHealthy()
}

func (b *Cosmos) GetChain() common.Chain {
	return common.TERRAChain
}

func (b *Cosmos) GetHeight() (int64, error) {
	return b.blockScanner.FetchLastHeight()
}

// GetAddress return current signer address, it will be bech32 encoded address
func (b *Cosmos) GetAddress(poolPubKey common.PubKey) string {
	addr, err := poolPubKey.GetAddress(common.TERRAChain)
	if err != nil {
		b.logger.Error().Err(err).Str("pool_pub_key", poolPubKey.String()).Msg("fail to get pool address")
		return ""
	}
	return addr.String()
}

// SignTx sign the the given TxArrayItem
func (b *Cosmos) SignTx(tx stypes.TxOutItem, thorchainHeight int64) ([]byte, error) {
	fromAddr, err := types.AccAddressFromBech32(b.GetAddress(tx.VaultPubKey))
	if err != nil {
		return nil, fmt.Errorf("failed to convert address (%s) to bech32: %w", tx.ToAddress, err)
	}

	toAddr, err := types.AccAddressFromBech32(tx.ToAddress.String())
	if err != nil {
		return nil, fmt.Errorf("failed to convert address (%s) to bech32: %w", tx.ToAddress, err)
	}

	var gasFees common.Coins

	// For yggdrasil, we need to leave some coins to pay for the fee. Note, this
	// logic is per chain, given that different networks charge fees differently.
	if strings.EqualFold(tx.Memo, thorchain.NewYggdrasilReturn(thorchainHeight).String()) {
		gasFees = common.Coins{
			common.NewCoin(b.cosmosScanner.avgGasFee.Asset, b.cosmosScanner.avgGasFee.Amount),
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
	signMsg := legacytx.StdSignMsg{
		ChainID:       b.chainID,
		Memo:          tx.Memo,
		Msgs:          []types.Msg{msg},
		Sequence:      uint64(meta.SeqNumber),
		AccountNumber: uint64(meta.AccountNumber),
	}

	rawBz, err := b.localKeyManager.Sign(signMsg)
	if err != nil {
		return nil, fmt.Errorf("unable to sign tx: %w", err)
	}

	hexTx := []byte(hex.EncodeToString(rawBz))
	b.logger.Info().Str("hexTx", string(hexTx))
	b.logger.Info().Int64("account_number", meta.AccountNumber).Int64("sequence_number", meta.SeqNumber).Int64("block height", meta.BlockHeight).Msg("account info")
	return hexTx, nil
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

func (b *Cosmos) GetAccountByAddress(address string) (common.Account, error) {
	bankClient := btypes.NewQueryClient(b.grpcConn)
	bankReq := &btypes.QueryAllBalancesRequest{
		Address: address,
	}
	balances, err := bankClient.AllBalances(context.Background(), bankReq)
	if err != nil {
		return common.Account{}, err
	}

	nativeCoins := make([]common.Coin, 0)
	for _, coin := range balances.Balances {
		c, _ := sdkCoinToCommonCoin(coin)
		nativeCoins = append(nativeCoins, c)
	}

	c := atypes.NewQueryClient(b.grpcConn)
	authReq := &atypes.QueryAccountRequest{
		Address: address,
	}

	acc, err := c.Account(context.Background(), authReq)
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
		log.Error().Err(err).Msg("unable to broadcast tx")
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
