module gitlab.com/thorchain/thornode

go 1.15

require (
	github.com/99designs/keyring v1.1.6
	github.com/binance-chain/ledger-cosmos-go v0.9.9 // indirect
	github.com/binance-chain/tss-lib v1.3.2
	github.com/blang/semver v3.5.1+incompatible
	github.com/btcsuite/btcd v0.21.0-beta
	github.com/btcsuite/btcutil v1.0.2
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/cosmos/cosmos-sdk v0.40.0
	github.com/cosmos/go-bip39 v1.0.0
	github.com/decred/dcrd/dcrec/edwards v1.0.0
	github.com/didip/tollbooth v4.0.2+incompatible
	github.com/ethereum/go-ethereum v1.9.25
	github.com/gcash/bchd v0.17.1
	github.com/gcash/bchutil v0.0.0-20201025062739-fc759989ee3e
	github.com/gogo/protobuf v1.3.2
	github.com/google/go-cmp v0.5.4 // indirect
	github.com/google/uuid v1.2.0 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/go-retryablehttp v0.6.4
	github.com/ipfs/go-log v1.0.4
	github.com/ltcsuite/ltcd v0.20.1-beta.0.20201210074626-c807bfe31ef0
	github.com/ltcsuite/ltcutil v1.0.2-beta
	github.com/magiconair/properties v1.8.4
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/prometheus/client_golang v1.8.0
	github.com/rakyll/statik v0.1.7
	github.com/regen-network/cosmos-proto v0.3.1 // indirect
	github.com/rjeczalik/notify v0.9.2 // indirect
	github.com/rs/zerolog v1.20.0
	github.com/spf13/cast v1.3.1
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/syndtr/goleveldb v1.0.1-0.20200815110645-5c35d600f0ca
	github.com/tendermint/btcd v0.1.1
	github.com/tendermint/go-amino v0.16.0
	github.com/tendermint/tendermint v0.34.2
	github.com/tendermint/tm-db v0.6.3
	gitlab.com/thorchain/bifrost/bchd-txscript v0.0.0-20210123055555-abb86a2e300a
	gitlab.com/thorchain/bifrost/ltcd-txscript v0.0.0-20210123055845-c0f9cad51f13
	gitlab.com/thorchain/bifrost/txscript v0.0.0-20210123055850-29da989e6f5a
	gitlab.com/thorchain/binance-sdk v1.2.3-0.20210117202539-d569b6b9ba5d
	gitlab.com/thorchain/tss/go-tss v1.2.9-0.20210117231716-2d12e95fd812
	golang.org/x/mod v0.4.0 // indirect
	golang.org/x/tools v0.0.0-20210101214203-2dba1e4ea05c // indirect
	google.golang.org/genproto v0.0.0-20210201184850-646a494a81ea // indirect
	gopkg.in/check.v1 v1.0.0-20200902074654-038fdea0a05b
	gopkg.in/ini.v1 v1.52.0 // indirect
	honnef.co/go/tools v0.0.1-2020.1.6 // indirect
)

replace (
	github.com/agl/ed25519 => github.com/binance-chain/edwards25519 v0.0.0-20200305024217-f36fc4b53d43
	github.com/binance-chain/tss-lib => gitlab.com/thorchain/tss/tss-lib v0.0.0-20201118045712-70b2cb4bf916
	github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.2-alpha.regen.4
	github.com/tendermint/go-amino => github.com/binance-chain/bnc-go-amino v0.14.1-binance.1
	github.com/zondax/ledger-go => github.com/binance-chain/ledger-go v0.9.1
)
