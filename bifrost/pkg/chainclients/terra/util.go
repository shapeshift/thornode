package terra

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	ctypes "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	btypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

const ThorchainDecimals = 8

// buildUnsigned takes a MsgSend and other parameters and returns a txBuilder
// It can be used to simulateTx or as the input to signMsg before BraodcastTx
func buildUnsigned(
	txConfig client.TxConfig,
	msg *btypes.MsgSend,
	pubkey common.PubKey,
	memo string,
	fee ctypes.Coins,
	account uint64,
	sequence uint64,
) (client.TxBuilder, error) {
	cpk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeAccPub, pubkey.String())
	if err != nil {
		return nil, fmt.Errorf("unable to GetPubKeyFromBech32 from cosmos: %w", err)
	}
	txBuilder := txConfig.NewTxBuilder()

	err = txBuilder.SetMsgs(msg)
	if err != nil {
		return nil, fmt.Errorf("unable to SetMsgs on txBuilder: %w", err)
	}

	txBuilder.SetMemo(memo)
	txBuilder.SetFeeAmount(fee)
	txBuilder.SetGasLimit(GasLimit)

	sigData := &signingtypes.SingleSignatureData{
		SignMode: signingtypes.SignMode_SIGN_MODE_DIRECT,
	}
	sig := signingtypes.SignatureV2{
		PubKey:   cpk,
		Data:     sigData,
		Sequence: sequence,
	}

	err = txBuilder.SetSignatures(sig)
	if err != nil {
		return nil, fmt.Errorf("unable to initial SetSignatures on txBuilder: %w", err)
	}

	return txBuilder, nil
}

// simulateTx takes a transaction builder and client and returns a simulate response
// useful for calculating how much gas a transaction would take
func simulateTx(txb client.TxBuilder, txClient txtypes.ServiceClient) (*txtypes.SimulateResponse, error) {
	protoProvider, ok := txb.(tx.ProtoTxProvider)
	if !ok {
		return &txtypes.SimulateResponse{}, fmt.Errorf("expected proto tx builder, got %T", txb)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return txClient.Simulate(
		ctx,
		&txtypes.SimulateRequest{
			Tx: protoProvider.GetProtoTx(),
		},
	)
}

func getDummyTxBuilderForSimulate(txConfig client.TxConfig) (client.TxBuilder, error) {
	// The sender is a dead stagenet vault with some Luna dust left over
	// It will always have a small balance and its sequence will never change
	// Thus, we use it as an account to craft a dummy tx that can be used to simulate gas
	// This is more reliable than using gas averages.

	msg := &btypes.MsgSend{
		FromAddress: "terra126kpfewtlc7agqjrwdl2wfg0txkphsaw65t39n",
		ToAddress:   "terra126kpfewtlc7agqjrwdl2wfg0txkphsaw65t39n",
		Amount:      cosmos.NewCoins(cosmos.NewCoin("uluna", ctypes.NewInt(1000))),
	}

	return buildUnsigned(
		txConfig,
		msg,
		common.PubKey("sthorpub1addwnpepqwqwswthukczxyas0yhte2pn0r4g3uxux0d83mzfremvegs6lr7z2glhvdw"),
		"ADD:TERRA.SOMELONGCOIN:sthor1x2nh4jevz7z54j9826sluzjjpvncmh3a399cec",
		ctypes.NewCoins(ctypes.NewCoin("uluna", ctypes.NewInt(1000))),
		3418297,
		41,
	)
}

func fromCosmosToThorchain(c cosmos.Coin) (common.Coin, error) {
	cosmosAsset, exists := GetAssetByCosmosDenom(c.Denom)
	if !exists {
		return common.Coin{}, fmt.Errorf("asset does not exist / not whitelisted by client")
	}

	thorAsset, err := common.NewAsset(fmt.Sprintf("%s.%s", common.TERRAChain.String(), cosmosAsset.THORChainSymbol))
	if err != nil {
		return common.Coin{}, fmt.Errorf("invalid thorchain asset: %w", err)
	}

	decimals := cosmosAsset.CosmosDecimals
	amount := c.Amount.BigInt()
	var exp big.Int
	// Decimals are more than native THORChain, so divide...
	if decimals > ThorchainDecimals {
		decimalDiff := int64(decimals - ThorchainDecimals)
		amount.Quo(amount, exp.Exp(big.NewInt(10), big.NewInt(decimalDiff), nil))
	} else if decimals < ThorchainDecimals {
		// Decimals are less than native THORChain, so multiply...
		decimalDiff := int64(ThorchainDecimals - decimals)
		amount.Mul(amount, exp.Exp(big.NewInt(10), big.NewInt(decimalDiff), nil))
	}
	return common.Coin{
		Asset:    thorAsset,
		Amount:   ctypes.NewUintFromBigInt(amount),
		Decimals: int64(decimals),
	}, nil
}

func fromThorchainToCosmos(coin common.Coin) (cosmos.Coin, error) {
	asset, exists := GetAssetByThorchainSymbol(coin.Asset.Symbol.String())
	if !exists {
		return cosmos.Coin{}, fmt.Errorf("asset does not exist / not whitelisted by client")
	}

	decimals := asset.CosmosDecimals
	amount := coin.Amount.BigInt()
	var exp big.Int
	if decimals > ThorchainDecimals {
		decimalDiff := int64(decimals - ThorchainDecimals)
		amount.Mul(amount, exp.Exp(big.NewInt(10), big.NewInt(decimalDiff), nil))
	} else if decimals < ThorchainDecimals {
		// Decimals are less than native THORChain, so multiply...
		decimalDiff := int64(ThorchainDecimals - decimals)
		amount.Quo(amount, exp.Exp(big.NewInt(10), big.NewInt(decimalDiff), nil))
	}
	return cosmos.NewCoin(asset.CosmosDenom, ctypes.NewIntFromBigInt(amount)), nil
}
