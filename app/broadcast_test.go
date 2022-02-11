package app

import (
	"fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/client"
	clientrest "github.com/cosmos/cosmos-sdk/client/rest"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/assert"
	"gitlab.com/thorchain/thornode/x/thorchain/client/rest"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

func TestBroadcastTxRequest(t *testing.T) {
	types.SetupConfigForTest()
	ctx := client.Context{}
	ctx = ctx.WithChainID("thorchain")
	ctx = ctx.WithHomeDir("~/.thornode")
	ctx = ctx.WithFromName("thorchain")
	addr, err := sdk.AccAddressFromBech32("tthor15nd6c6ljfd2s6t2pa4q493espqy3n3rlgy5auy")
	assert.Nil(t, err)
	ctx = ctx.WithFromAddress(addr)
	ctx = ctx.WithBroadcastMode("sync")

	encodingConfig := MakeEncodingConfig()
	ctx = ctx.WithCodec(encodingConfig.Marshaler)
	ctx = ctx.WithInterfaceRegistry(encodingConfig.InterfaceRegistry)
	ctx = ctx.WithTxConfig(encodingConfig.TxConfig)
	ctx = ctx.WithLegacyAmino(encodingConfig.Amino)
	clientCtx := ctx.WithAccountRetriever(authtypes.AccountRetriever{})
	var req rest.BroadcastReq

	body := `{
    "tx": {
        "msg": [
            {
                "type": "thorchain/MsgDeposit",
                "value": {
                    "coins": [
                        {
                            "asset": "THOR.RUNE",
                            "amount": "100000000000"
                        }
                    ],
                    "memo": "ADD:BNB.BNB:tbnb1mkymsmnqenxthlmaa9f60kd6wgr9yjy9h5mz6q",
                    "signer": "tthor1wz78qmrkplrdhy37tw0tnvn0tkm5pqd6zdp257"
                }
            }
        ],
        "fee": {
            "amount": [],
            "gas": "100000000"
        },
        "memo": "",
        "signatures": [
            {
                "signature": "YAYS26NpukCvh9D0krBLtjrwiHFtxaqRjTP8mpEwfNQsihhBs01VLNwpReNQPZEdUlGT+QvnSdBwaa3KhEKJRg==",
                "pub_key": {
                    "type": "tendermint/PubKeySecp256k1",
                    "value": "AiNYu8vRKEQBOjRMSuuG6hjCe1w0gwfKMaLdgIENjFT3"
                },
                "account_number": "2",
                "sequence": "1"
            }
        ]
    },
    "mode": "sync"
}`

	// NOTE: amino is used intentionally here, don't migrate it!
	err = clientCtx.LegacyAmino.UnmarshalJSON([]byte(body), &req)
	if err != nil {
		err := fmt.Errorf("this transaction cannot be broadcasted via legacy REST endpoints, because it does not support"+
			" Amino serialization. Please either use CLI, gRPC, gRPC-gateway, or directly query the Tendermint RPC"+
			" endpoint to broadcast this transaction. The new REST endpoint (via gRPC-gateway) is POST /cosmos/tx/v1beta1/txs."+
			" Please also see the REST endpoints migration guide at %s for more info", clientrest.DeprecationURL)
		panic(err)
	}

	txBytes, err := rest.ConvertAndEncodeStdTx(clientCtx.TxConfig, req.Tx)
	if err != nil {
		panic(err)
	}

	clientCtx = clientCtx.WithBroadcastMode(req.Mode)

	res, err := clientCtx.BroadcastTx(txBytes)
	if err != nil {
		panic(err)
	}
	_ = res
}
