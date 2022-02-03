package terra

import (
	"context"
	"fmt"
	"math/big"
	"strings"
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

var WhitelistAssets = map[string]int{"uluna": 6, "uusd": 6}

func buildUnsigned(
	txConfig client.TxConfig,
	msg *btypes.MsgSend,
	pubkey common.PubKey,
	memo string,
	gas ctypes.Coins,
	limit uint64,
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
	txBuilder.SetFeeAmount(gas)
	txBuilder.SetGasLimit(limit)

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
		1000,
		3418297,
		41,
	)

}

func fromCosmosToThorchain(c cosmos.Coin) common.Coin {
	name := fmt.Sprintf("%s.%s", common.TERRAChain.String(), c.Denom[1:])
	asset, _ := common.NewAsset(name)

	decimals, exists := WhitelistAssets[c.Denom]
	if !exists {
		return common.NewCoin(asset, ctypes.Uint(c.Amount))
	}

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
		Asset:    asset,
		Amount:   ctypes.NewUintFromBigInt(amount),
		Decimals: int64(decimals),
	}
}

func fromThorchainToCosmos(coin common.Coin) cosmos.Coin {
	denom := fmt.Sprintf("u%s", strings.ToLower(coin.Asset.Symbol.String()))
	decimals := WhitelistAssets[denom]

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
	return cosmos.NewCoin(denom, ctypes.NewIntFromBigInt(amount))
}
