module gitlab.com/thorchain/bepswap/statechain

go 1.13

require (
	github.com/binance-chain/go-sdk v1.1.3
	github.com/btcsuite/btcd v0.0.0-20190824003749-130ea5bddde3 // indirect
	github.com/cosmos/cosmos-sdk v0.37.2
	github.com/didip/tollbooth v4.0.2+incompatible
	github.com/go-resty/resty/v2 v2.0.0
	github.com/gorilla/mux v1.7.0
	github.com/mattn/go-isatty v0.0.7 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.1.0 // indirect
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4 // indirect
	github.com/prometheus/common v0.7.0 // indirect
	github.com/prometheus/procfs v0.0.4 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.4.0
	github.com/tendermint/go-amino v0.15.0
	github.com/tendermint/tendermint v0.32.5
	github.com/tendermint/tm-db v0.2.0
	gitlab.com/thorchain/bepswap/common v1.0.1-0.20191018104203-d7f2639b98ff
	golang.org/x/crypto v0.0.0-20190911031432-227b76d455e7 // indirect
	golang.org/x/net v0.0.0-20190912160710-24e19bdeb0f2 // indirect
	golang.org/x/sys v0.0.0-20190912141932-bc967efca4b8 // indirect
	golang.org/x/text v0.3.2 // indirect
	google.golang.org/genproto v0.0.0-20190911173649-1774047e7e51 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127
)

replace gitlab.com/thorchain/bepswap/statechain => ../statechain
