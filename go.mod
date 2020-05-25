module gitlab.com/thorchain/thornode

go 1.13

require (
	github.com/binance-chain/go-sdk v1.2.2
	github.com/binance-chain/ledger-cosmos-go v0.9.9 // indirect
	github.com/binance-chain/tss-lib v1.3.1
	github.com/blang/semver v3.5.1+incompatible
	github.com/btcsuite/btcd v0.20.1-beta.0.20200414114020-8b54b0b96418
	github.com/btcsuite/btcutil v1.0.2
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/cosmos/cosmos-sdk v0.38.4
	github.com/didip/tollbooth v4.0.2+incompatible
	github.com/ethereum/go-ethereum v1.9.12
	github.com/go-kit/kit v0.10.0
	github.com/gorilla/mux v1.7.4
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/go-retryablehttp v0.6.4
	github.com/ipfs/go-datastore v0.4.4 // indirect
	github.com/ipfs/go-log v1.0.2
	github.com/libp2p/go-libp2p-kad-dht v0.5.0 // indirect
	github.com/multiformats/go-multiaddr v0.2.1
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/prometheus/client_golang v1.5.0
	github.com/prometheus/procfs v0.0.10 // indirect
	github.com/rjeczalik/notify v0.9.2 // indirect
	github.com/rs/zerolog v1.18.0
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v0.0.6
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.6.2
	github.com/syndtr/goleveldb v1.0.1-0.20190923125748-758128399b1d
	github.com/tendermint/btcd v0.1.1
	github.com/tendermint/go-amino v0.15.1
	github.com/tendermint/tendermint v0.33.3
	github.com/tendermint/tm-db v0.5.0
	github.com/zondax/ledger-go v0.11.0 // indirect
	gitlab.com/thorchain/tss/go-tss v0.0.0-20200521211844-8c2925d834b5
	gitlab.com/thorchain/txscript v0.0.0-20200413023754-8aaf3443d92b
	go.uber.org/multierr v1.5.0 // indirect
	golang.org/x/crypto v0.0.0-20200510223506-06a226fb4e37
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/sys v0.0.0-20200515095857-1151b9dac4a9 // indirect
	golang.org/x/tools v0.0.0-20200409210453-700752c24408 // indirect
	google.golang.org/genproto v0.0.0-20200304201815-d429ff31ee6c // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f
	gopkg.in/ini.v1 v1.52.0 // indirect
)

replace (
	github.com/binance-chain/go-sdk => gitlab.com/thorchain/binance-sdk v1.2.2
	github.com/tendermint/go-amino => github.com/binance-chain/bnc-go-amino v0.14.1-binance.1
)
